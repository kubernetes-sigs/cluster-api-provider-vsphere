/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspherevms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspherevms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch

// AddVMControllerToManager adds the VM controller to the provided manager.
func AddVMControllerToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {

	var (
		controlledType     = &infrav1.VSphereVM{}
		controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()
		controlledTypeGVK  = infrav1.GroupVersion.WithKind(controlledTypeName)

		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(controlledTypeName))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, controllerNameShort)
	)

	// Build the controller context.
	controllerContext := &context.ControllerContext{
		ControllerManagerContext: ctx,
		Name:                     controllerNameShort,
		Recorder:                 record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		Logger:                   ctx.Logger.WithName(controllerNameShort),
	}

	return ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledType).
		// Watch a GenericEvent channel for the controlled resource.
		//
		// This is useful when there are events outside of Kubernetes that
		// should cause a resource to be synchronized, such as a goroutine
		// waiting on some asynchronous, external task to complete.
		Watches(
			&source.Channel{Source: ctx.GetGenericEventChannelFor(controlledTypeGVK)},
			&handler.EnqueueRequestForObject{},
		).
		Complete(vmReconciler{ControllerContext: controllerContext})
}

type vmReconciler struct {
	*context.ControllerContext
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
// nolint:gocognit
func (r vmReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Get the VSphereVM resource for this request.
	vsphereVM := &infrav1.VSphereVM{}
	if err := r.Client.Get(r, req.NamespacedName, vsphereVM); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Info("VSphereVM not found, won't reconcile", "key", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Get or create an authenticated session to the vSphere endpoint.
	authSession, err := session.GetOrCreate(r.Context,
		vsphereVM.Spec.Server, vsphereVM.Spec.Datacenter,
		r.ControllerManagerContext.Username, r.ControllerManagerContext.Password)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create vSphere session")
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(vsphereVM, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			vsphereVM.GroupVersionKind(),
			vsphereVM.Namespace,
			vsphereVM.Name)
	}

	// Create the VM context for this request.
	vmContext := &context.VMContext{
		ControllerContext: r.ControllerContext,
		VSphereVM:         vsphereVM,
		Session:           authSession,
		Logger:            r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:       patchHelper,
	}

	// Print the task-ref upon entry and upon exit.
	vmContext.Logger.V(4).Info(
		"VSphereVM.Status.TaskRef OnEntry",
		"task-ref", vmContext.VSphereVM.Status.TaskRef)
	defer func() {
		vmContext.Logger.V(4).Info(
			"VSphereVM.Status.TaskRef OnExit",
			"task-ref", vmContext.VSphereVM.Status.TaskRef)
	}()

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		// Patch the VSphereVM resource.
		if err := vmContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			vmContext.Logger.Error(err, "patch failed", "vm", vmContext.String())
		}

		// localObj is a deep copy of the VSphereVM resource that was
		// fetched at the top of this Reconcile function.
		localObj := vmContext.VSphereVM.DeepCopy()

		// Fetch the up-to-date VSphereVM resource into remoteObj until the
		// fetched resource has a a different ResourceVersion than the local
		// object.
		//
		// FYI - resource versions are opaque, numeric strings and should not
		// be compared with < or >, only for equality -
		// https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions.
		//
		// Since CAPV is currently deployed with a single replica, and this
		// controller has a max concurrency of one, the only agent updating the
		// VSphereVM resource should be this controller.
		//
		// So if the remote resource's ResourceVersion is different than the
		// ResourceVersion of the resource fetched at the beginning of this
		// reconcile request, then that means the remote resource should be
		// newer than the local resource.
		// nolint:errcheck
		wait.PollImmediateInfinite(time.Second*1, func() (bool, error) {
			// remoteObj refererences the same VSphereVM resource as it exists
			// on the API server post the patch operation above. In a perfect world,
			// the Status for localObj and remoteObj should be the same.
			remoteObj := &infrav1.VSphereVM{}
			if err := vmContext.Client.Get(vmContext, req.NamespacedName, remoteObj); err != nil {
				if apierrors.IsNotFound(err) {
					// It's possible that the remote resource cannot be found
					// because it has been removed. Do not error, just exit.
					return true, nil
				}

				// There was an issue getting the remote resource. Sleep for a
				// second and try again.
				vmContext.Logger.Error(err, "failed to get VSphereVM while exiting reconcile")
				return false, nil
			}

			// If the remote resource version is not the same as the local
			// resource version, then it means we were able to get a resource
			// newer than the one we already had.
			if localObj.ResourceVersion != remoteObj.ResourceVersion {
				vmContext.Logger.Info(
					"resource is patched",
					"local-resource-version", localObj.ResourceVersion,
					"remote-resource-version", remoteObj.ResourceVersion)
				return true, nil
			}

			// If the resources are the same resource version, then a previous
			// patch may not have resulted in any changes. Check to see if the
			// remote status is the same as the local status.
			if cmp.Equal(localObj.Status, remoteObj.Status, cmpopts.EquateEmpty()) {
				vmContext.Logger.Info(
					"resource patch was not required",
					"local-resource-version", localObj.ResourceVersion,
					"remote-resource-version", remoteObj.ResourceVersion)
				return true, nil
			}

			// The remote resource version is the same as the local resource
			// version, which means the local cache is not yet up-to-date.
			vmContext.Logger.Info(
				"resource is not patched",
				"local-resource-version", localObj.ResourceVersion,
				"remote-resource-version", remoteObj.ResourceVersion)
			return false, nil
		})
	}()

	cluster, err := clusterutilv1.GetClusterFromMetadata(r.ControllerContext, r.Client, vsphereVM.ObjectMeta)
	if err == nil {
		if clusterutilv1.IsPaused(cluster, vsphereVM) {
			r.Logger.V(4).Info("VSphereVM %s/%s linked to a cluster that is paused",
				vsphereVM.Namespace, vsphereVM.Name)
			return reconcile.Result{}, nil
		}
	}

	// Handle deleted machines
	if !vsphereVM.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(vmContext)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(vmContext)
}

func (r vmReconciler) reconcileDelete(ctx *context.VMContext) (reconcile.Result, error) {
	ctx.Logger.Info("Handling deleted VSphereVM")

	// TODO(akutz) Implement selection of VM service based on vSphere version
	var vmService services.VirtualMachineService = &govmomi.VMService{}

	vm, err := vmService.DestroyVM(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to destroy VM")
	}

	// Requeue the operation until the VM is "notfound".
	if vm.State != infrav1.VirtualMachineStateNotFound {
		ctx.Logger.Info("vm state is not reconciled", "expected-vm-state", infrav1.VirtualMachineStateNotFound, "actual-vm-state", vm.State)
		return reconcile.Result{}, nil
	}

	// The VM is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(ctx.VSphereVM, infrav1.VMFinalizer)

	return reconcile.Result{}, nil
}

func (r vmReconciler) reconcileNormal(ctx *context.VMContext) (reconcile.Result, error) {

	if ctx.VSphereVM.Status.FailureReason != nil || ctx.VSphereVM.Status.FailureMessage != nil {
		r.Logger.Info("VM is failed, won't reconcile", "namespace", ctx.VSphereVM.Namespace, "name", ctx.VSphereVM.Name)
		return reconcile.Result{}, nil
	}
	// If the VSphereVM doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.VSphereVM, infrav1.VMFinalizer)

	// TODO(akutz) Implement selection of VM service based on vSphere version
	var vmService services.VirtualMachineService = &govmomi.VMService{}

	// Get or create the VM.
	vm, err := vmService.ReconcileVM(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile VM")
	}

	// Do not proceed until the backend VM is marked ready.
	if vm.State != infrav1.VirtualMachineStateReady {
		ctx.Logger.Info(
			"VM state is not reconciled",
			"expected-vm-state", infrav1.VirtualMachineStateReady,
			"actual-vm-state", vm.State)
		return reconcile.Result{}, nil
	}

	// Update the VSphereVM's BIOS UUID.
	ctx.VSphereVM.Spec.BiosUUID = vm.BiosUUID

	// Update the VSphereVM's network status.
	r.reconcileNetwork(ctx, vm)

	// Once the network is online the VM is considered ready.
	ctx.VSphereVM.Status.Ready = true
	ctx.Logger.Info("VSphereVM is ready")

	return reconcile.Result{}, nil
}

func (r vmReconciler) reconcileNetwork(ctx *context.VMContext, vm infrav1.VirtualMachine) {
	ctx.VSphereVM.Status.Network = vm.Network
	ipAddrs := make([]string, 0, len(vm.Network))
	for _, netStatus := range ctx.VSphereVM.Status.Network {
		ipAddrs = append(ipAddrs, netStatus.IPAddrs...)
	}
	ctx.VSphereVM.Status.Addresses = ipAddrs
}
