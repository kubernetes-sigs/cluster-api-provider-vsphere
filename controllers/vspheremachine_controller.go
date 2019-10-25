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
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/ipam"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/config"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi"
)

// VSphereMachineReconciler reconciles a VSphereMachine object
type VSphereMachineReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *VSphereMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	parentContext := goctx.Background()

	logger := r.Log.
		WithName(controllerName).
		WithName(fmt.Sprintf("namespace=%s", req.Namespace)).
		WithName(fmt.Sprintf("vsphereMachine=%s", req.Name))

	// Fetch the VSphereMachine instance.
	vsphereMachine := &infrav1.VSphereMachine{}
	if err := r.Get(parentContext, req.NamespacedName, vsphereMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	logger = logger.WithName(vsphereMachine.APIVersion)

	// Fetch the Machine.
	machine, err := clusterutilv1.GetOwnerMachine(parentContext, r.Client, vsphereMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		logger.Info("Waiting for Machine Controller to set OwnerRef on VSphereMachine")
		return reconcile.Result{RequeueAfter: config.DefaultRequeue}, nil
	}

	logger = logger.WithName(fmt.Sprintf("machine=%s", machine.Name))

	// Fetch the Cluster.
	cluster, err := clusterutilv1.GetClusterFromMetadata(parentContext, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithName(fmt.Sprintf("cluster=%s", cluster.Name))

	vsphereCluster := &infrav1.VSphereCluster{}

	vsphereClusterName := client.ObjectKey{
		Namespace: vsphereMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(parentContext, vsphereClusterName, vsphereCluster); err != nil {
		logger.Info("Waiting for VSphereCluster")
		return reconcile.Result{RequeueAfter: config.DefaultRequeue}, nil
	}

	logger = logger.WithName(fmt.Sprintf("vsphereCluster=%s", vsphereCluster.Name))

	// Create the cluster context.
	clusterContext, err := context.NewClusterContext(&context.ClusterContextParams{
		Context:        parentContext,
		Cluster:        cluster,
		VSphereCluster: vsphereCluster,
		Client:         r.Client,
		Logger:         logger,
	})
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to create cluster context")
	}

	// Create the machine context
	machineContext, err := context.NewMachineContextFromClusterContext(
		clusterContext,
		machine,
		vsphereMachine)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to create machine context")
	}

	// Always close the context when exiting this function so we can persist any VSphereMachine changes.
	defer func() {
		if err := machineContext.Patch(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !vsphereMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machineContext)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(machineContext)
}

// SetupWithManager adds this controller to the provided manager.
func (r *VSphereMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.VSphereMachine{}).Watches(
		&source.Kind{Type: &clusterv1.Machine{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: clusterutilv1.MachineToInfrastructureMapFunc(schema.GroupVersionKind{
				Group:   infrav1.SchemeBuilder.GroupVersion.Group,
				Version: infrav1.SchemeBuilder.GroupVersion.Version,
				Kind:    "VSphereMachine",
			}),
		},
	).Complete(r)
}

func (r *VSphereMachineReconciler) reconcileDelete(ctx *context.MachineContext) (reconcile.Result, error) {
	ctx.Logger.Info("Handling deleted VSphereMachine")

	// TODO(akutz) Implement selection of VM service based on vSphere version
	var vmService services.VirtualMachineService = &govmomi.VMService{}

	vm, err := vmService.DestroyVM(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to destroy VM")
	}

	// Requeue the operation until the VM is "notfound".
	if vm.State != infrav1.VirtualMachineStateNotFound {
		ctx.Logger.V(6).Info("requeuing operation until vm state is reconciled", "expected-vm-state", infrav1.VirtualMachineStateNotFound, "actual-vm-state", vm.State)
		return reconcile.Result{RequeueAfter: config.DefaultRequeue}, nil
	}

	// NetApp
	var ipamService = &ipam.IPAMService{}
	if err := ipamService.ReleaseIPAM(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// The VM is deleted so remove the finalizer.
	ctx.VSphereMachine.Finalizers = clusterutilv1.Filter(ctx.VSphereMachine.Finalizers, infrav1.MachineFinalizer)

	return reconcile.Result{}, nil
}

func (r *VSphereMachineReconciler) reconcileNormal(ctx *context.MachineContext) (reconcile.Result, error) {
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
		ctx.Logger.Info("Cluster infrastructure is not ready yet, requeuing machine")
		return reconcile.Result{RequeueAfter: config.DefaultRequeue}, nil
	}

	// Make sure bootstrap data is available and populated.
	if ctx.Machine.Spec.Bootstrap.Data == nil {
		ctx.Logger.Info("Waiting for bootstrap data to be available")
		return reconcile.Result{RequeueAfter: config.DefaultRequeue}, nil
	}

	// NetApp
	var ipamService = &ipam.IPAMService{}
	if err := ipamService.ReconcileIPAM(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// TODO(akutz) Implement selection of VM service based on vSphere version
	var vmService services.VirtualMachineService = &govmomi.VMService{}

	// Get or create the VM.
	vm, err := vmService.ReconcileVM(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile VM")
	}

	if vm.State != infrav1.VirtualMachineStateReady {
		ctx.Logger.V(6).Info("requeuing operation until vm state is reconciled", "expected-vm-state", infrav1.VirtualMachineStateReady, "actual-vm-state", vm.State)
		return reconcile.Result{RequeueAfter: config.DefaultRequeue}, nil
	}

	if ok, err := r.reconcileNetwork(ctx, vm, vmService); !ok {
		if err != nil {
			return reconcile.Result{}, err
		}
		ctx.Logger.V(6).Info("requeuing operation until vm network is reconciled")
		return reconcile.Result{RequeueAfter: config.DefaultRequeue}, nil
	}

	if err := r.reconcileProviderID(ctx, vm, vmService); err != nil {
		return reconcile.Result{}, err
	}

	// Once the provider ID is set then the VSphereMachine is InfrastructureReady
	ctx.VSphereMachine.Status.Ready = true
	ctx.Logger.V(6).Info("VSphereMachine is infrastructure-ready")

	return reconcile.Result{}, nil
}

func (r *VSphereMachineReconciler) reconcileNetwork(ctx *context.MachineContext, vm infrav1.VirtualMachine, vmService services.VirtualMachineService) (bool, error) {
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
		ctx.Logger.V(6).Info("requeuing to wait on IP addresses")
		return false, nil
	}

	// Use the collected IP addresses to assign the Machine's addresses.
	ctx.VSphereMachine.Status.Addresses = ipAddrs

	return true, nil
}

func (r *VSphereMachineReconciler) reconcileProviderID(ctx *context.MachineContext, vm infrav1.VirtualMachine, vmService services.VirtualMachineService) error {
	providerID := fmt.Sprintf("vsphere://%s", vm.BiosUUID)
	if ctx.VSphereMachine.Spec.ProviderID == nil || *ctx.VSphereMachine.Spec.ProviderID != providerID {
		ctx.VSphereMachine.Spec.ProviderID = &providerID
		ctx.Logger.V(6).Info("updated provider ID", "provider-id", providerID)
	}
	return nil
}
