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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch

// AddMachineControllerToManager adds the machine controller to the provided
// manager.
func AddMachineControllerToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {

	var (
		controlledType     = &infrav1.VSphereMachine{}
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
		// Watch the CAPI resource that owns this infrastructure resource.
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: clusterutilv1.MachineToInfrastructureMapFunc(controlledTypeGVK),
			},
		).
		// Watch a GenericEvent channel for the controlled resource.
		//
		// This is useful when there are events outside of Kubernetes that
		// should cause a resource to be synchronized, such as a goroutine
		// waiting on some asynchronous, external task to complete.
		Watches(
			&source.Channel{Source: ctx.GetGenericEventChannelFor(controlledTypeGVK)},
			&handler.EnqueueRequestForObject{},
		).
		Complete(machineReconciler{ControllerContext: controllerContext})
}

type machineReconciler struct {
	*context.ControllerContext
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r machineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {

	// Get the VSphereMachine resource for this request.
	vsphereMachine := &infrav1.VSphereMachine{}
	if err := r.Client.Get(r, req.NamespacedName, vsphereMachine); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Info("VSphereMachine not found, won't reconcile", "key", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the CAPI Machine.
	machine, err := clusterutilv1.GetOwnerMachine(r, r.Client, vsphereMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		r.Logger.Info("Waiting for Machine Controller to set OwnerRef on VSphereMachine")
		return reconcile.Result{}, nil
	}

	// Fetch the CAPI Cluster.
	cluster, err := clusterutilv1.GetClusterFromMetadata(r, r.Client, machine.ObjectMeta)
	if err != nil {
		r.Logger.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	// Fetch the VSphereCluster
	vsphereCluster := &infrav1.VSphereCluster{}
	vsphereClusterName := client.ObjectKey{
		Namespace: vsphereMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(r, vsphereClusterName, vsphereCluster); err != nil {
		r.Logger.Info("Waiting for VSphereCluster")
		return reconcile.Result{}, nil
	}

	// Get or create an authenticated session to the vSphere endpoint.
	authSession, err := session.GetOrCreate(r.Context,
		vsphereCluster.Spec.Server, vsphereMachine.Spec.Datacenter,
		r.ControllerManagerContext.Username, r.ControllerManagerContext.Password)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to create vSphere session")
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(vsphereMachine, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			vsphereMachine.GroupVersionKind(),
			vsphereMachine.Namespace,
			vsphereMachine.Name)
	}

	// Create the machine context for this request.
	machineContext := &context.MachineContext{
		ClusterContext: &context.ClusterContext{
			ControllerContext: r.ControllerContext,
			Cluster:           cluster,
			VSphereCluster:    vsphereCluster,
		},
		Machine:        machine,
		VSphereMachine: vsphereMachine,
		Session:        authSession,
		Logger:         r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:    patchHelper,
	}

	// Print the task-ref upon entry and upon exit.
	machineContext.Logger.V(4).Info(
		"VSphereMachine.Status.TaskRef OnEntry",
		"task-ref", machineContext.VSphereMachine.Status.TaskRef)
	defer func() {
		machineContext.Logger.V(4).Info(
			"VSphereMachine.Status.TaskRef OnExit",
			"task-ref", machineContext.VSphereMachine.Status.TaskRef)
	}()

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		// Patch the VSphereMachine resource.
		if err := machineContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			machineContext.Logger.Error(err, "patch failed", "machine", machineContext.String())
		}

		// localObj is a deep copy of the VSphereMachine resource that was
		// fetched at the top of this Reconcile function.
		localObj := machineContext.VSphereMachine.DeepCopy()

		// Fetch the up-to-date VSphereMachine resource into remoteObj until the
		// fetched resource has a a different ResourceVersion than the local
		// object.
		//
		// FYI - resource versions are opaque, numeric strings and should not
		// be compared with < or >, only for equality -
		// https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions.
		//
		// Since CAPV is currently deployed with a single replica, and this
		// controller has a max concurrency of one, the only agent updating the
		// VSphereMachine resource should be this controller.
		//
		// So if the remote resource's ResourceVersion is different than the
		// ResourceVersion of the resource fetched at the beginning of this
		// reconcile request, then that means the remote resource should be
		// newer than the local resource.
		wait.PollImmediateInfinite(time.Second*1, func() (bool, error) {
			// remoteObj refererences the same VSphereMachine resource as it exists
			// on the API server post the patch operation above. In a perfect world,
			// the Status for localObj and remoteObj should be the same.
			remoteObj := &infrav1.VSphereMachine{}
			if err := machineContext.Client.Get(machineContext, req.NamespacedName, remoteObj); err != nil {
				if apierrors.IsNotFound(err) {
					// It's possible that the remote resource cannot be found
					// because it has been removed. Do not error, just exit.
					return true, nil
				}

				// There was an issue getting the remote resource. Sleep for a
				// second and try again.
				machineContext.Logger.Error(err, "failed to get VSphereMachine while exiting reconcile")
				return false, nil
			}

			// If the remote resource version is not the same as the local
			// resource version, then it means we were able to get a resource
			// newer than the one we already had.
			if localObj.ResourceVersion != remoteObj.ResourceVersion {
				machineContext.Logger.Info(
					"resource is patched",
					"local-resource-version", localObj.ResourceVersion,
					"remote-resource-version", remoteObj.ResourceVersion)
				return true, nil
			}

			// If the resources are the same resource version, then a previous
			// patch may not have resulted in any changes. Check to see if the
			// remote status is the same as the local status.
			if cmp.Equal(localObj.Status, remoteObj.Status) {
				machineContext.Logger.Info(
					"resource patch was not required",
					"local-resource-version", localObj.ResourceVersion,
					"remote-resource-version", remoteObj.ResourceVersion)
				return true, nil
			}

			// The remote resource version is the same as the local resource
			// version, which means the local cache is not yet up-to-date.
			machineContext.Logger.Info(
				"resource is not patched",
				"local-resource-version", localObj.ResourceVersion,
				"remote-resource-version", remoteObj.ResourceVersion)
			return false, nil
		})
	}()

	// Handle deleted machines
	if !vsphereMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machineContext)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(machineContext)
}

func (r machineReconciler) reconcileDelete(ctx *context.MachineContext) (reconcile.Result, error) {
	ctx.Logger.Info("Handling deleted VSphereMachine")

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
	ctx.VSphereMachine.Finalizers = clusterutilv1.Filter(ctx.VSphereMachine.Finalizers, infrav1.MachineFinalizer)

	return reconcile.Result{}, nil
}

func (r machineReconciler) reconcileNormal(ctx *context.MachineContext) (reconcile.Result, error) {
	// If the VSphereMachine is in an error state, return early.
	if ctx.VSphereMachine.Status.ErrorReason != nil || ctx.VSphereMachine.Status.ErrorMessage != nil {
		ctx.Logger.Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the VSphereMachine doesn't have our finalizer, add it.
	if !clusterutilv1.Contains(ctx.VSphereMachine.Finalizers, infrav1.MachineFinalizer) {
		ctx.VSphereMachine.Finalizers = append(ctx.VSphereMachine.Finalizers, infrav1.MachineFinalizer)
	}

	if !ctx.Cluster.Status.InfrastructureReady {
		ctx.Logger.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	// Make sure bootstrap data is available and populated.
	if ctx.Machine.Spec.Bootstrap.DataSecretName == nil {
		ctx.Logger.Info("Waiting for bootstrap data to be available")
		return reconcile.Result{}, nil
	}

	// TODO(akutz) Implement selection of VM service based on vSphere version
	var vmService services.VirtualMachineService = &govmomi.VMService{}

	// Get or create the VM.
	vm, err := vmService.ReconcileVM(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile VM")
	}

	if vm.State != infrav1.VirtualMachineStateReady {
		ctx.Logger.Info("vm state is not reconciled", "expected-vm-state", infrav1.VirtualMachineStateReady, "actual-vm-state", vm.State)
		return reconcile.Result{}, nil
	}

	if ok, err := r.reconcileNetwork(ctx, vm, vmService); !ok {
		if err != nil {
			return reconcile.Result{}, err
		}
		ctx.Logger.Info("waiting on vm networking")
		return reconcile.Result{}, nil
	}

	if err := r.reconcileProviderID(ctx, vm, vmService); err != nil {
		return reconcile.Result{}, err
	}

	// Once the provider ID is set then the VSphereMachine is InfrastructureReady
	ctx.VSphereMachine.Status.Ready = true
	ctx.Logger.Info("VSphereMachine is infrastructure-ready")

	return reconcile.Result{}, nil
}

func (r machineReconciler) reconcileNetwork(ctx *context.MachineContext, vm infrav1.VirtualMachine, vmService services.VirtualMachineService) (bool, error) {
	expNetCount, actNetCount := len(ctx.VSphereMachine.Spec.Network.Devices), len(vm.Network)
	if expNetCount != actNetCount {
		return false, errors.Errorf("invalid network count for %q: exp=%d act=%d", ctx, expNetCount, actNetCount)
	}
	ctx.VSphereMachine.Status.Network = vm.Network

	// If the VM is powered on then issue requeues until all of the VM's
	// networks have IP addresses.
	var ipAddrs []corev1.NodeAddress

	for _, netStatus := range ctx.VSphereMachine.Status.Network {
		for _, ip := range netStatus.IPAddrs {
			ipAddrs = append(ipAddrs, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: ip,
			})
		}
	}

	if len(ipAddrs) == 0 {
		ctx.Logger.Info("waiting on IP addresses")
		return false, nil
	}

	// Use the collected IP addresses to assign the Machine's addresses.
	ctx.VSphereMachine.Status.Addresses = ipAddrs

	return true, nil
}

func (r machineReconciler) reconcileProviderID(ctx *context.MachineContext, vm infrav1.VirtualMachine, vmService services.VirtualMachineService) error {
	providerID := infrautilv1.ConvertUUIDToProviderID(vm.BiosUUID)
	if providerID == "" {
		return errors.Errorf("invalid BIOS UUID %s for %s", vm.BiosUUID, ctx)
	}
	if ctx.VSphereMachine.Spec.ProviderID == nil || *ctx.VSphereMachine.Spec.ProviderID != providerID {
		ctx.VSphereMachine.Spec.ProviderID = &providerID
		ctx.Logger.Info("updated provider ID", "provider-id", providerID)
	}
	return nil
}
