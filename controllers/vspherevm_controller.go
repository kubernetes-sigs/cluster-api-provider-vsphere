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
	goctx "context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspherevms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspherevms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch

// AddVMControllerToManager adds the VM controller to the provided manager.
//nolint:forcetypeassert
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
	r := vmReconciler{ControllerContext: controllerContext}
	controller, err := ctrl.NewControllerManagedBy(mgr).
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
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles}).
		Build(r)
	if err != nil {
		return err
	}

	err = controller.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.clusterToVSphereVMs),
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldCluster := e.ObjectOld.(*clusterv1.Cluster)
				newCluster := e.ObjectNew.(*clusterv1.Cluster)
				return oldCluster.Spec.Paused && !newCluster.Spec.Paused
			},
			CreateFunc: func(e event.CreateEvent) bool {
				if _, ok := e.Object.GetAnnotations()[clusterv1.PausedAnnotation]; !ok {
					return false
				}
				return true
			},
		})
	if err != nil {
		return err
	}
	return nil
}

type vmReconciler struct {
	*context.ControllerContext
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r vmReconciler) Reconcile(ctx goctx.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Get the VSphereVM resource for this request.
	vsphereVM := &infrav1.VSphereVM{}
	if err := r.Client.Get(r, req.NamespacedName, vsphereVM); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Info("VSphereVM not found, won't reconcile", "key", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
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

	authSession, err := r.retrieveVcenterSession(ctx, vsphereVM)
	if err != nil {
		conditions.MarkFalse(vsphereVM, infrav1.VCenterAvailableCondition, infrav1.VCenterUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, err
	}
	conditions.MarkTrue(vsphereVM, infrav1.VCenterAvailableCondition)

	// Fetch the owner VSphereMachine.
	vsphereMachine, err := util.GetOwnerVSphereMachine(r, r.Client, vsphereVM.ObjectMeta)
	// vsphereMachine can be nil in cases where custom mover other than clusterctl
	// moves the resources without ownerreferences set
	// in that case nil vsphereMachine can cause panic and CrashLoopBackOff the pod
	// preventing vspheremachine_controller from setting the ownerref
	if err != nil || vsphereMachine == nil {
		r.Logger.Info("Owner VSphereMachine not found, won't reconcile", "key", req.NamespacedName)
		return reconcile.Result{}, nil
	}

	// Fetch the CAPI Machine.
	machine, err := clusterutilv1.GetOwnerMachine(r, r.Client, vsphereMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		r.Logger.Info("Waiting for OwnerRef to be set on VSphereMachine", "key", vsphereMachine.Name)
		return reconcile.Result{}, nil
	}

	var vsphereFailureDomain *infrav1.VSphereFailureDomain
	if failureDomain := machine.Spec.FailureDomain; failureDomain != nil {
		vsphereDeploymentZone := &infrav1.VSphereDeploymentZone{}
		if err := r.Client.Get(r, apitypes.NamespacedName{Name: *failureDomain}, vsphereDeploymentZone); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to find vsphere deployment zone %s", *failureDomain)
		}

		vsphereFailureDomain = &infrav1.VSphereFailureDomain{}
		if err := r.Client.Get(r, apitypes.NamespacedName{Name: vsphereDeploymentZone.Spec.FailureDomain}, vsphereFailureDomain); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to find vsphere failure domain %s", vsphereDeploymentZone.Spec.FailureDomain)
		}
	}

	// Create the VM context for this request.
	vmContext := &context.VMContext{
		ControllerContext:    r.ControllerContext,
		VSphereVM:            vsphereVM,
		VSphereFailureDomain: vsphereFailureDomain,
		Session:              authSession,
		Logger:               r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:          patchHelper,
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
		// always update the readyCondition.
		conditions.SetSummary(vmContext.VSphereVM,
			conditions.WithConditions(
				infrav1.VMProvisionedCondition,
				infrav1.VCenterAvailableCondition,
			),
		)

		// Patch the VSphereVM resource.
		if err := vmContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			vmContext.Logger.Error(err, "patch failed", "vm", vmContext.String())
		}
	}()

	cluster, err := clusterutilv1.GetClusterFromMetadata(r.ControllerContext, r.Client, vsphereVM.ObjectMeta)
	if err == nil {
		if annotations.IsPaused(cluster, vsphereVM) {
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

	conditions.MarkFalse(ctx.VSphereVM, infrav1.VMProvisionedCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	vm, err := vmService.DestroyVM(ctx)
	if err != nil {
		conditions.MarkFalse(ctx.VSphereVM, infrav1.VMProvisionedCondition, "DeletionFailed", clusterv1.ConditionSeverityWarning, err.Error())
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

	if r.isWaitingForStaticIPAllocation(ctx) {
		conditions.MarkFalse(ctx.VSphereVM, infrav1.VMProvisionedCondition, infrav1.WaitingForStaticIPAllocationReason, clusterv1.ConditionSeverityInfo, "")
		ctx.Logger.Info("vm is waiting for static ip to be available")
		return reconcile.Result{}, nil
	}

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
	ctx.Logger.Info("vm bios-uuid", "biosuuid", vm.BiosUUID)

	// defensive check to ensure we are not removing the biosUUID
	if vm.BiosUUID != "" {
		ctx.VSphereVM.Spec.BiosUUID = vm.BiosUUID
	} else {
		return reconcile.Result{}, errors.Errorf("bios uuid is empty while VM is ready")
	}

	// Update the VSphereVM's network status.
	r.reconcileNetwork(ctx, vm)

	// we didn't get any addresses, requeue
	if len(ctx.VSphereVM.Status.Addresses) == 0 {
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Once the network is online the VM is considered ready.
	ctx.VSphereVM.Status.Ready = true
	conditions.MarkTrue(ctx.VSphereVM, infrav1.VMProvisionedCondition)
	ctx.Logger.Info("VSphereVM is ready")

	return reconcile.Result{}, nil
}

// isWaitingForStaticIPAllocation checks whether the VM should wait for a static IP
// to be allocated.
// It checks the state of both DHCP4 and DHCP6 for all the network devices and if
// any static IP addresses are specified.
func (r vmReconciler) isWaitingForStaticIPAllocation(ctx *context.VMContext) bool {
	devices := ctx.VSphereVM.Spec.Network.Devices
	for _, dev := range devices {
		if !dev.DHCP4 && !dev.DHCP6 && len(dev.IPAddrs) == 0 {
			// Static IP is not available yet
			return true
		}
	}

	return false
}

func (r vmReconciler) reconcileNetwork(ctx *context.VMContext, vm infrav1.VirtualMachine) {
	ctx.VSphereVM.Status.Network = vm.Network
	ipAddrs := make([]string, 0, len(vm.Network))
	for _, netStatus := range ctx.VSphereVM.Status.Network {
		ipAddrs = append(ipAddrs, netStatus.IPAddrs...)
	}
	ctx.VSphereVM.Status.Addresses = ipAddrs
}

func (r *vmReconciler) clusterToVSphereVMs(a ctrlclient.Object) []reconcile.Request {
	requests := []reconcile.Request{}
	vms := &infrav1.VSphereVMList{}
	err := r.Client.List(goctx.Background(), vms, ctrlclient.MatchingLabels(
		map[string]string{
			clusterv1.ClusterLabelName: a.GetName(),
		},
	))
	if err != nil {
		return requests
	}
	for _, vm := range vms.Items {
		r := reconcile.Request{
			NamespacedName: apitypes.NamespacedName{
				Name:      vm.Name,
				Namespace: vm.Namespace,
			},
		}
		requests = append(requests, r)
	}
	return requests
}

func (r *vmReconciler) retrieveVcenterSession(ctx goctx.Context, vsphereVM *infrav1.VSphereVM) (*session.Session, error) {
	// Get cluster object and then get VSphereCluster object

	params := session.NewParams().
		WithServer(vsphereVM.Spec.Server).
		WithDatacenter(vsphereVM.Spec.Datacenter).
		WithUserInfo(r.ControllerContext.Username, r.ControllerContext.Password).
		WithThumbprint(vsphereVM.Spec.Thumbprint).
		WithFeatures(session.Feature{
			KeepAliveDuration: r.KeepAliveDuration,
		})
	cluster, err := clusterutilv1.GetClusterFromMetadata(r.ControllerContext, r.Client, vsphereVM.ObjectMeta)
	if err != nil {
		r.Logger.Info("VsphereVM is missing cluster label or cluster does not exist")
		return session.GetOrCreate(r.Context,
			params)
	}

	key := ctrlclient.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	vsphereCluster := &infrav1.VSphereCluster{}
	err = r.Client.Get(r, key, vsphereCluster)
	if err != nil {
		r.Logger.Info("VSphereCluster couldn't be retrieved")
		return session.GetOrCreate(r.Context,
			params)
	}

	if vsphereCluster.Spec.IdentityRef != nil {
		creds, err := identity.GetCredentials(ctx, r.Client, vsphereCluster, r.Namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve credentials from IdentityRef")
		}
		params = params.WithUserInfo(creds.Username, creds.Password)
		return session.GetOrCreate(r.Context,
			params)
	}

	// Fallback to using credentials provided to the manager
	return session.GetOrCreate(r.Context,
		params)
}
