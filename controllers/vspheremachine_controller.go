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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
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
		// Watch any VSphereVM resources owned by the controlled type.
		Owns(&infrav1.VSphereVM{}).
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
		ControllerContext: r.ControllerContext,
		Cluster:           cluster,
		VSphereCluster:    vsphereCluster,
		Machine:           machine,
		VSphereMachine:    vsphereMachine,
		Logger:            r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:       patchHelper,
	}

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

	// TODO(akutz) Determine the version of vSphere.
	if err := r.reconcileDeletePre7(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// The VM is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(ctx.VSphereMachine, infrav1.MachineFinalizer)

	return reconcile.Result{}, nil
}

func (r machineReconciler) reconcileDeletePre7(ctx *context.MachineContext) error {
	// Get ready to find the associated VSphereVM resource.
	vm := &infrav1.VSphereVM{}
	vmKey := apitypes.NamespacedName{
		Namespace: ctx.VSphereMachine.Namespace,
		Name:      ctx.Machine.Name,
	}

	// Attempt to find the associated VSphereVM resource.
	if err := ctx.Client.Get(ctx, vmKey, vm); err != nil {
		// If an error occurs finding the VSphereVM resource other than
		// IsNotFound, then return the error. Otherwise it means the VSphereVM
		// is already deleted, and that's okay.
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get VSphereVM %s", vmKey)
		}
	} else if vm.GetDeletionTimestamp().IsZero() {
		// If the VSphereVM was found and it's not already enqueued for
		// deletion, go ahead and attempt to delete it.
		if err := ctx.Client.Delete(ctx, vm); err != nil {
			return errors.Wrapf(err, "failed to delete VSphereVM %v", vmKey)
		}

		// Go ahead and return here since the deletion of the VSphereVM resource
		// will trigger a new reconcile for this VSphereMachine resource.
		return nil
	}

	return nil
}

func (r machineReconciler) reconcileNormal(ctx *context.MachineContext) (reconcile.Result, error) {
	// If the VSphereMachine is in an error state, return early.
	if ctx.VSphereMachine.Status.ErrorReason != nil || ctx.VSphereMachine.Status.ErrorMessage != nil {
		ctx.Logger.Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the VSphereMachine doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.VSphereMachine, infrav1.MachineFinalizer)

	if !ctx.Cluster.Status.InfrastructureReady {
		ctx.Logger.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if ctx.Machine.Spec.Bootstrap.DataSecretName == nil {
		ctx.Logger.Info("Waiting for bootstrap data to be available")
		return reconcile.Result{}, nil
	}

	// TODO(akutz) Determine the version of vSphere.
	vm, err := r.reconcileNormalPre7(ctx)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Convert the VM resource to unstructured data.
	vmData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"failed to convert %s to unstructured data",
			vm.GetObjectKind().GroupVersionKind().String())
	}
	vmObj := &unstructured.Unstructured{Object: vmData}

	// Reconcile the VSphereMachine's provider ID using the VM's BIOS UUID.
	if ok, err := r.reconcileProviderID(ctx, vmObj); !ok {
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err,
				"unexpected error while reconciling provider ID for %s", ctx)
		}
		ctx.Logger.Info("provider ID is not reconciled")
		return reconcile.Result{}, nil
	}

	// Reconcile the VSphereMachine's node addresses from the VM's IP addresses.
	if ok, err := r.reconcileNetwork(ctx, vmObj); !ok {
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err,
				"unexpected error while reconciling network for %s", ctx)
		}
		ctx.Logger.Info("network is not reconciled")
		return reconcile.Result{}, nil
	}

	// Reconcile the VSphereMachine's ready state from the VM's ready state.
	if ok, err := r.reconcileReadyState(ctx, vmObj); !ok {
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err,
				"unexpected error while reconciling ready state for %s", ctx)
		}
		ctx.Logger.Info("ready state is not reconciled")
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func (r machineReconciler) reconcileNormalPre7(ctx *context.MachineContext) (runtime.Object, error) {
	// Create or update the VSphereVM resource.
	vm := &infrav1.VSphereVM{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.VSphereMachine.Namespace,
			Name:      ctx.Machine.Name,
		},
	}
	mutateFn := func() (err error) {
		// Ensure this VSphereMachine is marked as the ControllerOwner of the
		// VSphereVM resource.
		if err = ctrlutil.SetControllerReference(ctx.VSphereMachine, vm, ctx.Scheme); err != nil {
			return errors.Wrapf(err,
				"failed to set %s as owner of VSphereVM %s/%s", ctx,
				vm.Namespace, vm.Name)
		}

		// Instruct the VSphereVM to use the CAPI bootstrap data resource.
		vm.Spec.BootstrapRef = ctx.Machine.Spec.Bootstrap.ConfigRef

		// Initialize the VSphereVM's labels map if it is nil.
		if vm.Labels == nil {
			vm.Labels = map[string]string{}

			// If the labels map was nil upon entering this function and there
			// are not any labels upon exiting this function, then remove the
			// labels map to prevent an unnecessary change.
			defer func() {
				if err == nil && len(vm.Labels) == 0 {
					vm.Labels = nil
				}
			}()
		}

		// Ensure the VSphereVM has a label that can be used when searching for
		// resources associated with the target cluster.
		vm.Labels[clusterv1.ClusterLabelName] = ctx.Machine.Labels[clusterv1.ClusterLabelName]

		// For convenience, add a label that makes it easy to figure out if the
		// VSphereVM resource is part of some control plane.
		if val, ok := ctx.Machine.Labels[clusterv1.MachineControlPlaneLabelName]; ok {
			vm.Labels[clusterv1.MachineControlPlaneLabelName] = val
		}

		// Copy the VSphereMachine's VM clone spec into the VSphereVM's
		// clone spec.
		ctx.VSphereMachine.Spec.VirtualMachineCloneSpec.DeepCopyInto(&vm.Spec.VirtualMachineCloneSpec)

		// Several of the VSphereVM's clone spec properties can be derived
		// from multiple places. The order is:
		//
		//   1. From the VSphereMachine.Spec (the DeepCopyInto above)
		//   2. From the VSphereCluster.Spec.CloudProviderConfiguration.Workspace
		//   3. From the VSphereCluster.Spec
		vsphereCloudConfig := ctx.VSphereCluster.Spec.CloudProviderConfiguration.Workspace
		if vm.Spec.Server == "" {
			if vm.Spec.Server = vsphereCloudConfig.Server; vm.Spec.Server == "" {
				vm.Spec.Server = ctx.VSphereCluster.Spec.Server
			}
		}
		if vm.Spec.Datacenter == "" {
			vm.Spec.Datacenter = vsphereCloudConfig.Datacenter
		}
		if vm.Spec.Datastore == "" {
			vm.Spec.Datastore = vsphereCloudConfig.Datastore
		}
		if vm.Spec.Folder == "" {
			vm.Spec.Folder = vsphereCloudConfig.Folder
		}
		if vm.Spec.ResourcePool == "" {
			vm.Spec.ResourcePool = vsphereCloudConfig.ResourcePool
		}
		return nil
	}
	if _, err := ctrlutil.CreateOrUpdate(ctx, ctx.Client, vm, mutateFn); err != nil {
		if apierrors.IsAlreadyExists(err) {
			ctx.Logger.Info("VSphereVM already exists")
			return nil, err
		}
		ctx.Logger.Error(err, "failed to CreateOrUpdate VSphereVM",
			"namespace", vm.Namespace, "name", vm.Name)
		return nil, err
	}

	return vm, nil
}

func (r machineReconciler) reconcileNetwork(ctx *context.MachineContext, vm *unstructured.Unstructured) (bool, error) {
	if networkStatusListOfIfaces, ok, _ := unstructured.NestedSlice(vm.Object, "status", "network"); ok {
		networkStatusList := []infrav1.NetworkStatus{}
		for i, networkStatusListMemberIface := range networkStatusListOfIfaces {
			if buf, err := json.Marshal(networkStatusListMemberIface); err != nil {
				ctx.Logger.Error(err,
					"unsupported data for member of status.network list",
					"index", i)
			} else {
				var networkStatus infrav1.NetworkStatus
				err := json.Unmarshal(buf, &networkStatus)
				if err == nil && networkStatus.MACAddr == "" {
					err = errors.New("macAddr is required")
				}
				if err != nil {
					ctx.Logger.Error(err,
						"unsupported data for member of status.network list",
						"index", i, "data", string(buf))
				} else {
					networkStatusList = append(networkStatusList, networkStatus)
				}
			}
		}
		ctx.VSphereMachine.Status.Network = networkStatusList
	}

	if addresses, ok, _ := unstructured.NestedStringSlice(vm.Object, "status", "addresses"); ok {
		var nodeAddresses []corev1.NodeAddress
		for _, addr := range addresses {
			nodeAddresses = append(nodeAddresses, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: addr,
			})
		}
		ctx.VSphereMachine.Status.Addresses = nodeAddresses
	}

	if len(ctx.VSphereMachine.Status.Addresses) == 0 {
		ctx.Logger.Info("waiting on IP addresses")
		return false, nil
	}

	return true, nil
}

func (r machineReconciler) reconcileProviderID(ctx *context.MachineContext, vm *unstructured.Unstructured) (bool, error) {
	biosUUID, ok, err := unstructured.NestedString(vm.Object, "spec", "biosUUID")
	if !ok {
		if err != nil {
			return false, errors.Wrapf(err,
				"unexpected error when getting spec.biosUUID from %s %s/%s for %s",
				vm.GroupVersionKind(),
				vm.GetNamespace(),
				vm.GetName(),
				ctx)
		}
		ctx.Logger.Info("spec.biosUUID not found",
			"vmGVK", vm.GroupVersionKind().String(),
			"vmNamespace", vm.GetNamespace(),
			"vmName", vm.GetName())
		return false, nil
	}
	if biosUUID == "" {
		ctx.Logger.Info("spec.biosUUID is empty",
			"vmGVK", vm.GroupVersionKind().String(),
			"vmNamespace", vm.GetNamespace(),
			"vmName", vm.GetName())
		return false, nil
	}

	providerID := infrautilv1.ConvertUUIDToProviderID(biosUUID)
	if providerID == "" {
		return false, errors.Errorf("invalid BIOS UUID %s from %s %s/%s for %s",
			biosUUID,
			vm.GroupVersionKind(),
			vm.GetNamespace(),
			vm.GetName(),
			ctx)
	}
	if ctx.VSphereMachine.Spec.ProviderID == nil || *ctx.VSphereMachine.Spec.ProviderID != providerID {
		ctx.VSphereMachine.Spec.ProviderID = &providerID
		ctx.Logger.Info("updated provider ID", "provider-id", providerID)
	}

	return true, nil
}

func (r machineReconciler) reconcileReadyState(ctx *context.MachineContext, vm *unstructured.Unstructured) (bool, error) {
	ready, ok, err := unstructured.NestedBool(vm.Object, "status", "ready")
	if !ok {
		if err != nil {
			return false, errors.Wrapf(err,
				"unexpected error when getting status.ready from %s %s/%s for %s",
				vm.GroupVersionKind(),
				vm.GetNamespace(),
				vm.GetName(),
				ctx)
		}
		ctx.Logger.Info("status.ready not found",
			"vmGVK", vm.GroupVersionKind().String(),
			"vmNamespace", vm.GetNamespace(),
			"vmName", vm.GetName())
		return false, nil
	}
	if !ready {
		ctx.Logger.Info("status.ready is false",
			"vmGVK", vm.GroupVersionKind().String(),
			"vmNamespace", vm.GetNamespace(),
			"vmName", vm.GetName())
		return false, nil
	}

	ctx.VSphereMachine.Status.Ready = true
	return true, nil
}
