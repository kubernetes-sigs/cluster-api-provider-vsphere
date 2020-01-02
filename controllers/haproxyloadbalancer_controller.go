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
	"net/url"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

var (
	controlledType     = &infrav1.HAProxyLoadBalancer{}
	controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=haproxyloadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=haproxyloadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch

// AddHAProxyLoadBalancerControllerToManager adds the HAProxy load balancer
// controller to the provided manager.
func AddHAProxyLoadBalancerControllerToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {

	var (
		controlledTypeGVK   = infrav1.GroupVersion.WithKind(controlledTypeName)
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

	reconciler := haproxylbReconciler{ControllerContext: controllerContext}

	return ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledType).
		// Watch any VSphereVM resources owned by the controlled type.
		Owns(&infrav1.VSphereVM{}).
		// Watch the CAPI machines that are members of the control plane which
		// this HAProxyLoadBalancer servies.
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(reconciler.controlPlaneMachineToHAProxyLoadBalancer),
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
		Complete(reconciler)
}

type haproxylbReconciler struct {
	*context.ControllerContext
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r haproxylbReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {

	// Get the HAProxyLoadBalancer resource for this request.
	haproxylb := &infrav1.HAProxyLoadBalancer{}
	if err := r.Client.Get(r, req.NamespacedName, haproxylb); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Info("HAProxyLoadBalancer not found, won't reconcile", "key", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(haproxylb, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			haproxylb.GroupVersionKind(),
			haproxylb.Namespace,
			haproxylb.Name)
	}

	// Create the HAProxyLoadBalancer context for this request.
	ctx := &context.HAProxyLoadBalancerContext{
		ControllerContext:   r.ControllerContext,
		HAProxyLoadBalancer: haproxylb,
		Logger:              r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:         patchHelper,
	}

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		// Patch the HAProxyLoadBalancer resource.
		if err := ctx.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			ctx.Logger.Error(err, "patch failed", "resource", ctx.String())
		}
	}()

	// Handle deleted haproxyloadbalancers
	if !haproxylb.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx)
	}

	// Fetch the CAPI Cluster.
	cluster, err := clusterutilv1.GetOwnerCluster(r, r.Client, haproxylb.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		r.Logger.Info("Waiting for VSphereCluster Controller to set OwnerRef on HAProxyLoadBalancer")
		return reconcile.Result{}, nil
	}
	ctx.Cluster = cluster

	// Fetch the VSphereCluster
	vsphereCluster := &infrav1.VSphereCluster{}
	vsphereClusterKey := client.ObjectKey{
		Namespace: cluster.Spec.InfrastructureRef.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if vsphereClusterKey.Namespace == "" {
		vsphereClusterKey.Namespace = haproxylb.Namespace
	}
	if err := r.Client.Get(r, vsphereClusterKey, vsphereCluster); err != nil {
		r.Logger.Info("Waiting for VSphereCluster")
		return reconcile.Result{}, nil
	}
	ctx.VSphereCluster = vsphereCluster

	// Handle non-deleted haproxyloadbalancers
	return r.reconcileNormal(ctx)
}

func (r haproxylbReconciler) reconcileDelete(ctx *context.HAProxyLoadBalancerContext) (reconcile.Result, error) {
	ctx.Logger.Info("Handling deleted HAProxyLoadBalancer")

	// TODO(akutz) Determine the version of vSphere.
	if err := r.reconcileDeletePre7(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// The VM is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(ctx.HAProxyLoadBalancer, infrav1.HAProxyLoadBalancerFinalizer)

	return reconcile.Result{}, nil
}

func (r haproxylbReconciler) reconcileDeletePre7(ctx *context.HAProxyLoadBalancerContext) error {
	if ctx.HAProxyLoadBalancer.Spec.VirtualMachineConfiguration == nil {
		ctx.Logger.Info("skipping deletion of VSphereVM since this HAProxyLoadBalancer doesn't have a VirtualMachineConfiguration")
		return nil
	}

	// Get ready to find the associated VSphereVM resource.
	vm := &infrav1.VSphereVM{}
	vmKey := apitypes.NamespacedName{
		Namespace: ctx.HAProxyLoadBalancer.Namespace,
		Name:      ctx.HAProxyLoadBalancer.Name,
	}

	// Attempt to find the associated VSphereVM resource.
	if err := ctx.Client.Get(ctx, vmKey, vm); err != nil {
		// If an error occurs finding the VSphereVM resource other than
		// IsNotFound, then return the error. Otherwise it means the VSphereVM
		// is already deleted, and that's okay.
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get VSphereVM %s for %s", vmKey, ctx)
		}
	} else if vm.GetDeletionTimestamp().IsZero() {
		// If the VSphereVM was found and it's not already enqueued for
		// deletion, go ahead and attempt to delete it.
		if err := ctx.Client.Delete(ctx, vm); err != nil {
			return errors.Wrapf(err, "failed to delete VSphereVM %v for %s", vmKey, ctx)
		}

		// Go ahead and return here since the deletion of the VSphereVM resource
		// will trigger a new reconcile for this HAProxyLoadBalancer resource.
		return nil
	}

	return nil
}

func (r haproxylbReconciler) reconcileNormal(ctx *context.HAProxyLoadBalancerContext) (reconcile.Result, error) {
	// If the HAProxyLoadBalancer doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.HAProxyLoadBalancer, infrav1.HAProxyLoadBalancerFinalizer)

	// TODO(akutz) Determine the version of vSphere.
	vm, err := r.reconcileNormalPre7(ctx)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	var vmObj *unstructured.Unstructured
	if vm != nil {
		// Convert the VM resource to unstructured data.
		vmData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err,
				"failed to convert %s to unstructured data",
				vm.GetObjectKind().GroupVersionKind())
		}
		vmObj = &unstructured.Unstructured{Object: vmData}
	}

	// Reconcile the HAProxyLoadBalancer's address.
	if ok, err := r.reconcileNetwork(ctx, vmObj); !ok {
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err,
				"unexpected error while reconciling network for %s", ctx)
		}
		ctx.Logger.Info("network is not reconciled")
		return reconcile.Result{}, nil
	}

	// Reconcile the HAProxyLoadBalancer's ready state.
	if ok, err := r.reconcileReady(ctx, vmObj); !ok {
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err,
				"unexpected error while reconciling ready state for %s", ctx)
		}
		ctx.Logger.Info("ready state is not reconciled")
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func (r haproxylbReconciler) reconcileNormalPre7(ctx *context.HAProxyLoadBalancerContext) (runtime.Object, error) {
	if ctx.HAProxyLoadBalancer.Spec.VirtualMachineConfiguration == nil {
		ctx.Logger.Info("skipping creation of VSphereVM since this HAProxyLoadBalancer doesn't have a VirtualMachineConfiguration")
		return nil, nil
	}

	// Create or update the VSphereVM resource.
	vm := &infrav1.VSphereVM{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.HAProxyLoadBalancer.Namespace,
			Name:      ctx.HAProxyLoadBalancer.Name,
		},
	}
	mutateFn := func() (err error) {
		// Ensure this HAProxyLoadBalancer is marked as the ControllerOwner of the
		// VSphereVM resource.
		if err = ctrlutil.SetControllerReference(ctx.HAProxyLoadBalancer, vm, ctx.Scheme); err != nil {
			return errors.Wrapf(err,
				"failed to set %s as owner of VSphereVM %s/%s", ctx,
				vm.Namespace, vm.Name)
		}

		// TODO(akutz) Create the HAProxyLoadBalancer VM's bootstrap data.

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
		vm.Labels[clusterv1.ClusterLabelName] = ctx.Cluster.Name

		// Copy the HAProxyLoadBalancer's VM clone spec into the VSphereVM's
		// clone spec.
		ctx.HAProxyLoadBalancer.Spec.VirtualMachineConfiguration.DeepCopyInto(&vm.Spec.VirtualMachineCloneSpec)

		// Several of the VSphereVM's clone spec properties can be derived
		// from multiple places. The order is:
		//
		//   1. From the HAProxyLoadBalancer.Spec (the DeepCopyInto above)
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

func (r haproxylbReconciler) reconcileReady(ctx *context.HAProxyLoadBalancerContext, vm *unstructured.Unstructured) (bool, error) {
	if vm != nil {
		ready, ok, err := unstructured.NestedBool(vm.Object, "status", "ready")
		switch {
		case err != nil:
			return false, errors.Wrapf(err,
				"unexpected error getting status.ready from VM %s %s/%s for %s",
				vm.GroupVersionKind(),
				vm.GetNamespace(),
				vm.GetName(),
				ctx)
		case !ok:
			ctx.Logger.Info(
				"waiting on vm to report ready state",
				"vmAPIVersion", vm.GetAPIVersion(),
				"vmKind", vm.GetKind(),
				"vmNamespace", vm.GetNamespace(),
				"vmName", vm.GetName())
		case !ready:
			ctx.Logger.Info(
				"waiting on vm to be ready",
				"vmAPIVersion", vm.GetAPIVersion(),
				"vmKind", vm.GetKind(),
				"vmNamespace", vm.GetNamespace(),
				"vmName", vm.GetName())
		}
	}

	// The HAProxyLoadBalancer is finally ready.
	ctx.HAProxyLoadBalancer.Status.Ready = true
	ctx.Logger.Info("HAProxyLoadBalancer is ready")
	return true, nil
}

func (r haproxylbReconciler) reconcileNetwork(ctx *context.HAProxyLoadBalancerContext, vm *unstructured.Unstructured) (bool, error) {
	var (
		newAddr string
		oldAddr = ctx.HAProxyLoadBalancer.Status.Address
	)

	// If there is no VM then parse the network information from the HAProxy
	// API config.
	if vm == nil {
		serverURL, err := url.Parse(ctx.HAProxyAPIConfig.Server)
		if err != nil {
			return false, errors.Wrapf(err,
				"unexpected error parsing HAProxyAPIConfig.Server=%q for %s",
				ctx.HAProxyAPIConfig.Server, ctx)
		}
		newAddr = serverURL.Hostname()
		if newAddr == "" {
			return false, errors.Errorf("HAProxyAPIConfig.Server=%q is invalid for %s", newAddr, ctx)
		}
		ctx.Logger.Info("discovered IP address from HAPI config", "addressValue", newAddr)
	} else {
		// Otherwise the IP for the load balancer is obtained from the VM's
		// status.addresses field.
		addresses, ok, err := unstructured.NestedStringSlice(vm.Object, "status", "addresses")
		if !ok {
			if err != nil {
				return false, errors.Wrapf(err,
					"unexpected error getting status.addresses from VM %s %s/%s for %s",
					vm.GroupVersionKind(),
					vm.GetNamespace(),
					vm.GetName(),
					ctx)
			}
			ctx.Logger.Info("waiting on vm for ip address",
				"vmAPIVersion", vm.GetAPIVersion(),
				"vmKind", vm.GetKind(),
				"vmNamespace", vm.GetNamespace(),
				"vmName", vm.GetName())
			return false, nil
		}
		for _, addr := range addresses {
			if addr == "" {
				continue
			}
			newAddr = addr
			ctx.Logger.Info("discovered IP address from VM",
				"addressValue", newAddr,
				"vmAPIVersion", vm.GetAPIVersion(),
				"vmKind", vm.GetKind(),
				"vmNamespace", vm.GetNamespace(),
				"vmName", vm.GetName())
			break
		}
	}

	switch {
	case newAddr == "":
		ctx.Logger.Info("waiting on IP address")
		return false, nil
	case ctx.HAProxyLoadBalancer.Status.Address == "":
		ctx.HAProxyLoadBalancer.Status.Address = newAddr
		ctx.Logger.Info("initialized IP address", newAddr)
	default:
		ctx.HAProxyLoadBalancer.Status.Address = newAddr
		ctx.Logger.Info("updated IP address", "newAddressValue", newAddr, "oldAddressValue", oldAddr)
	}

	return true, nil
}

// controlPlaneMachineToHAProxyLoadBalancer is a handler.ToRequestsFunc to be
// used to trigger reconcile events for an HAProxyLoadBalancer when a CAPI
// Machine is reconciled and it has IP addresses and is a member of the same
// control plane that the HAProxyLoadBalancer services.
func (r haproxylbReconciler) controlPlaneMachineToHAProxyLoadBalancer(o handler.MapObject) []ctrl.Request {
	machine, ok := o.Object.(*clusterv1.Machine)
	if !ok {
		r.Logger.Error(errors.New("invalid type"),
			"Expected to receive a CAPI Machine resource",
			"expectedType", "Machine",
			"actualType", fmt.Sprintf("%T", o.Object))
		return nil
	}
	if !infrautilv1.IsControlPlaneMachine(machine) {
		return nil
	}
	if len(machine.Status.Addresses) == 0 {
		return nil
	}

	cluster, err := clusterutilv1.GetClusterFromMetadata(r, r.Client, machine.ObjectMeta)
	if err != nil {
		r.Logger.Error(err,
			"Machine is missing cluster label or cluster does not exist",
			"machineAPIVersion", machine.APIVersion,
			"machineKind", machine.Kind,
			"machineNamespace", machine.Namespace,
			"machineName", machine.Name)
		return nil
	}

	if cluster.Spec.InfrastructureRef == nil {
		return nil
	}

	// The infraClusterRef may not specify the namespace as it's assumed to be
	// in the same namespace as the Cluster. When the namespace is empty, set it
	// to the same namespace as the Cluster.
	infraClusterRef := cluster.Spec.InfrastructureRef
	if infraClusterRef.Namespace == "" {
		infraClusterRef.Namespace = cluster.Namespace
	}

	infraClusterKey := client.ObjectKey{
		Namespace: infraClusterRef.Namespace,
		Name:      infraClusterRef.Name,
	}
	infraCluster := &unstructured.Unstructured{Object: map[string]interface{}{}}
	infraCluster.SetAPIVersion(infraClusterRef.APIVersion)
	infraCluster.SetKind(infraClusterRef.Kind)
	if err := r.Client.Get(r, infraClusterKey, infraCluster); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Error(err,
				"Waiting on infrastructure cluster",
				"infraClusterAPIVersion", infraClusterRef.APIVersion,
				"infraClusterKind", infraClusterRef.Kind,
				"infraClusterNamespace", infraClusterRef.Namespace,
				"infraClusterName", infraClusterRef.Name)
			return nil
		}
		r.Logger.Error(err,
			"Unexpected error while waiting on infrastructure cluster",
			"infraClusterAPIVersion", infraClusterRef.APIVersion,
			"infraClusterKind", infraClusterRef.Kind,
			"infraClusterNamespace", infraClusterRef.Namespace,
			"infraClusterName", infraClusterRef.Name)
		return nil
	}

	loadBalancerRef := &corev1.ObjectReference{}
	if err := clusterutilv1.UnstructuredUnmarshalField(infraCluster, loadBalancerRef, "spec", "loadBalancerRef"); err != nil {
		if err != clusterutilv1.ErrUnstructuredFieldNotFound {
			r.Logger.Error(err,
				"Unexpected error getting infrastructure cluster's spec.loadBalancerRef",
				"infraClusterAPIVersion", infraCluster.GetAPIVersion(),
				"infraClusterKind", infraCluster.GetKind(),
				"infraClusterNamespace", infraCluster.GetNamespace(),
				"infraClusterName", infraCluster.GetName())
		}
		return nil
	}

	// The loadBalancerRef may not specify the namespace as it's assumed to be
	// in the same namespace as the Cluster. When the namespace is empty, set it
	// to the same namespace as the Cluster.
	if loadBalancerRef.Namespace == "" {
		loadBalancerRef.Namespace = cluster.Namespace
	}

	if loadBalancerRef.Name == "" {
		r.Logger.Error(err, "Infrastructure cluster's spec.loadBalancerRef.Name is empty",
			"infraClusterAPIVersion", infraCluster.GetAPIVersion(),
			"infraClusterKind", infraCluster.GetKind(),
			"infraClusterNamespace", infraCluster.GetNamespace(),
			"infraClusterName", infraCluster.GetName(),
			"loadBalancerRefAPIVersion", loadBalancerRef.APIVersion,
			"loadBalancerRefKind", loadBalancerRef.Kind,
			"loadBalancerRefNamespace", loadBalancerRef.Namespace,
			"loadBalancerRefName", loadBalancerRef.Name)
		return nil
	}

	if loadBalancerRef.Kind != controlledTypeName {
		return nil
	}

	if loadBalancerRef.APIVersion != infrav1.GroupVersion.String() {
		return nil
	}

	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: loadBalancerRef.Namespace,
			Name:      loadBalancerRef.Name,
		},
	}}
}
