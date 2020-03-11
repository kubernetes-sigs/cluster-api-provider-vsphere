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

	"github.com/antihax/optional"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	hapi "sigs.k8s.io/cluster-api-provider-vsphere/contrib/haproxy/openapi"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/haproxy"
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
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch;delete

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

	controller, err := ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledType).
		// Watch any VSphereVM resources owned by the controlled type.
		Watches(
			&source.Kind{Type: &infrav1.VSphereVM{}},
			&handler.EnqueueRequestForOwner{OwnerType: controlledType, IsController: false},
		).
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
		Build(reconciler)
	if err != nil {
		return err
	}
	err = controller.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(reconciler.reconcileClusterToHAProxyLoadBalancers),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldCluster := e.ObjectOld.(*clusterv1.Cluster)
				newCluster := e.ObjectNew.(*clusterv1.Cluster)
				return oldCluster.Spec.Paused && !newCluster.Spec.Paused
			},
			CreateFunc: func(e event.CreateEvent) bool {
				if _, ok := e.Meta.GetAnnotations()[clusterv1.PausedAnnotation]; !ok {
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

	cluster, err := clusterutilv1.GetClusterFromMetadata(r.Context, r.Client, haproxylb.ObjectMeta)
	if err == nil {
		if clusterutilv1.IsPaused(cluster, haproxylb) {
			r.Logger.V(4).Info("HAProxyLoadBalancer %s/%s linked to a cluster that is paused, won't reconcile",
				haproxylb.Namespace, haproxylb.Name)
			return reconcile.Result{}, nil
		}
		ctx.Cluster = cluster
	}
	// Handle deleted haproxyloadbalancers
	if !haproxylb.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx)
	}

	// check if we got the cluster as it is needed for reconcileNormal
	if ctx.Cluster == nil {
		return reconcile.Result{}, err
	}

	// Handle non-deleted haproxyloadbalancers
	return r.reconcileNormal(ctx)
}

func (r haproxylbReconciler) reconcileDelete(ctx *context.HAProxyLoadBalancerContext) (reconcile.Result, error) {
	ctx.Logger.Info("Handling deleted HAProxyLoadBalancer")

	if err := r.reconcileDeleteVM(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileDeleteSecrets(ctx); err != nil {
		return reconcile.Result{}, err
	}

	// The VM is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(ctx.HAProxyLoadBalancer, infrav1.HAProxyLoadBalancerFinalizer)

	return reconcile.Result{}, nil
}

func (r haproxylbReconciler) reconcileDeleteSecrets(ctx *context.HAProxyLoadBalancerContext) error {
	if err := haproxy.DeleteCASecret(ctx, ctx.Client, ctx.HAProxyLoadBalancer.Namespace, ctx.HAProxyLoadBalancer.Name); err != nil {
		return err
	}
	if err := haproxy.DeleteBootstrapSecret(ctx, ctx.Client, ctx.HAProxyLoadBalancer.Namespace, ctx.HAProxyLoadBalancer.Name); err != nil {
		return err
	}
	if err := haproxy.DeleteConfigSecret(ctx, ctx.Client, ctx.HAProxyLoadBalancer.Namespace, ctx.HAProxyLoadBalancer.Name); err != nil {
		return err
	}
	return nil
}

func (r haproxylbReconciler) reconcileDeleteVM(ctx *context.HAProxyLoadBalancerContext) error {
	// TODO(akutz) Determine the version of vSphere.
	return r.reconcileDeleteVMPre7(ctx)
}

func (r haproxylbReconciler) reconcileDeleteVMPre7(ctx *context.HAProxyLoadBalancerContext) error {
	// Get ready to find the associated VSphereVM resource.
	vm := &infrav1.VSphereVM{}
	vmKey := apitypes.NamespacedName{
		Namespace: ctx.HAProxyLoadBalancer.Namespace,
		Name:      ctx.HAProxyLoadBalancer.Name + "-lb",
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
			if !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to delete VSphereVM %v", vmKey)
			}
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

	if !ctx.HAProxyLoadBalancer.Status.Ready {
		// Create the HAProxyLoadBalancer's signing certificate/key pair secret.
		if err := haproxy.CreateCASecret(ctx, ctx.Client, ctx.Cluster, ctx.HAProxyLoadBalancer); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return reconcile.Result{}, errors.Wrapf(
					err, "failed to create signing certificate/key pair secret for %s", ctx)
			}
		}
		// Create the HAProxyLoadBalancer's bootstrap data secret.
		if err := haproxy.CreateBootstrapSecret(ctx, ctx.Client, ctx.Cluster, ctx.HAProxyLoadBalancer); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return reconcile.Result{}, errors.Wrapf(
					err, "failed to create bootstrap secret for %s", ctx)
			}
		}
	}

	// Reconcile the load balancer VM.
	vm, err := r.reconcileVM(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"unexpected error while reconciling vm for %s", ctx)
	}

	if !ctx.HAProxyLoadBalancer.Status.Ready {
		// Reconcile the HAProxyLoadBalancer's address.
		if ok, err := r.reconcileNetwork(ctx, vm); !ok {
			if err != nil {
				return reconcile.Result{}, errors.Wrapf(err,
					"unexpected error while reconciling network for %s", ctx)
			}
			ctx.Logger.Info("network is not reconciled")
			return reconcile.Result{}, nil
		}

		// Create the HAProxyLoadBalancer's API config secret.
		if err := haproxy.CreateConfigSecret(ctx, ctx.Client, ctx.Cluster, ctx.HAProxyLoadBalancer); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return reconcile.Result{}, errors.Wrapf(
					err, "failed to create API config secret for %s", ctx)
			}
		}

		// Mark the load balancer as ready.
		ctx.HAProxyLoadBalancer.Status.Ready = true
		ctx.Logger.Info("HAProxyLoadBalancer is ready")
	}

	// Reconcile the HAProxyLoadBalancer's backend servers.
	if err := r.reconcileLoadBalancerConfiguration(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"unexpected error while reconciling backend servers for %s", ctx)
	}

	return reconcile.Result{}, nil
}

func (r haproxylbReconciler) BackEndointsForCluster(ctx *context.HAProxyLoadBalancerContext) ([]corev1.EndpointAddress, error) {
	// Get the CAPI Machine resources for the cluster.
	machineList := &clusterv1.MachineList{}
	if err := ctx.Client.List(
		ctx, machineList,
		ctrlclient.InNamespace(ctx.Cluster.Namespace),
		ctrlclient.MatchingLabels(
			map[string]string{
				clusterv1.ClusterLabelName: ctx.Cluster.Name,
			},
		)); err != nil {
		return nil, errors.Wrap(err, "failed to get machines for cluster")
	}

	// Get the control plane machines.
	controlPlaneMachines := clusterutilv1.GetControlPlaneMachinesFromList(machineList)
	endpoints := make([]corev1.EndpointAddress, 0)
	for _, machine := range controlPlaneMachines {
		machineEndpoints := make([]corev1.EndpointAddress, 0)
		for i, addr := range machine.Status.Addresses {
			if addr.Type == clusterv1.MachineExternalIP {
				endpoint := corev1.EndpointAddress{
					NodeName: pointer.StringPtr(fmt.Sprintf("%s-%d", machine.Name, i)),
					IP:       addr.Address,
				}
				machineEndpoints = append(machineEndpoints, endpoint)
			}
		}
		endpoints = append(endpoints, machineEndpoints...)
	}
	return endpoints, nil
}

func (r haproxylbReconciler) reconcileLoadBalancerConfiguration(ctx *context.HAProxyLoadBalancerContext) error {

	// Get the Secret with the HAPI config.
	secret, err := haproxy.GetConfigSecret(
		ctx, ctx.Client, ctx.HAProxyLoadBalancer.Namespace, ctx.HAProxyLoadBalancer.Name)
	if err != nil {
		return errors.Wrapf(err, "failed to get config secret for %s", ctx)
	}

	dataplaneConfig, err := haproxy.LoadDataplaneConfig(secret.Data[haproxy.SecretDataKey])
	if err != nil {
		return errors.Wrapf(err, "failed to rehydrate dataplane client config %s", ctx)
	}

	// Create a HAPI client.
	client, err := haproxy.ClientFromHAPIConfig(dataplaneConfig)
	if err != nil {
		return errors.Wrapf(err, "failed to get hapi client for %s", ctx)
	}

	// Get the current configuration version.
	global, _, err := client.GlobalApi.GetGlobal(ctx, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to get hapi global config for %s", ctx)
	}

	transaction, _, err := client.TransactionsApi.StartTransaction(ctx, global.Version)
	if err != nil {
		return errors.Wrapf(err, "failed to create hapi transaction for %s", ctx)
	}
	transactionID := optional.NewString(transaction.Id)

	backends, err := r.BackEndointsForCluster(ctx)
	if err != nil {
		return errors.Wrapf(err, "couldn't fetch endpoints for cluster")
	}

	renderConfig := haproxy.NewRenderConfiguration().
		WithDataPlaneConfig(dataplaneConfig).
		WithAddresses(backends)
	haProxyConfig, err := renderConfig.RenderHAProxyConfiguration()

	if err != nil {
		return errors.Wrapf(err, "unable to render new configuration")
	}

	resp, _, err := client.ConfigurationApi.GetHAProxyConfiguration(ctx, &hapi.GetHAProxyConfigurationOpts{
		TransactionId: transactionID,
	})

	if err != nil {
		return errors.Wrapf(err, "failed to get haproxy configuration for %s", ctx)
	}

	originalConfig := resp.Data

	// commit new configuration, ending the transaction
	if originalConfig != haProxyConfig {
		_, _, err := client.ConfigurationApi.PostHAProxyConfiguration(ctx, haProxyConfig, &hapi.PostHAProxyConfigurationOpts{
			Version: optional.NewInt32(transaction.Version),
		})
		if err != nil {
			return errors.Wrapf(err, "unable to post new configuration for %s", ctx)
		}
	}

	ctx.Logger.Info("reconciled load balancer backend servers")
	return nil
}

func (r haproxylbReconciler) reconcileVM(ctx *context.HAProxyLoadBalancerContext) (*unstructured.Unstructured, error) {
	// TODO(akutz) Determine the version of vSphere.
	vm, err := r.reconcileVMPre7(ctx)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	// Convert the VM resource to unstructured data.
	vmData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return nil, errors.Wrapf(err,
			"failed to convert %s to unstructured data",
			vm.GetObjectKind().GroupVersionKind())
	}

	vmObj := &unstructured.Unstructured{Object: vmData}
	vmObj.SetGroupVersionKind(vm.GetObjectKind().GroupVersionKind())
	vmObj.SetAPIVersion(vm.GetObjectKind().GroupVersionKind().GroupVersion().String())
	vmObj.SetKind(vm.GetObjectKind().GroupVersionKind().Kind)
	return vmObj, nil
}

func (r haproxylbReconciler) reconcileVMPre7(ctx *context.HAProxyLoadBalancerContext) (runtime.Object, error) {
	// Create or update the VSphereVM resource.
	vm := &infrav1.VSphereVM{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.HAProxyLoadBalancer.Namespace,
			Name:      ctx.HAProxyLoadBalancer.Name + "-lb",
		},
	}
	mutateFn := func() (err error) {
		// Ensure the HAProxyLoadBalancer is marked as an owner of the VSphereVM.
		if err := ctrlutil.SetControllerReference(ctx.HAProxyLoadBalancer, vm, r.Scheme); err != nil {
			return errors.Wrapf(
				err,
				"failed to set controller reference on HAProxyLoadBalancer %s %s/%s",
				ctx.HAProxyLoadBalancer.GroupVersionKind(),
				ctx.HAProxyLoadBalancer.GetNamespace(),
				ctx.HAProxyLoadBalancer.GetName())
		}

		// Initialize the VSphereVM's labels map if it is nil.
		if vm.Labels == nil {
			vm.Labels = map[string]string{}
		}

		// Ensure the VSphereVM has a label that can be used when searching for
		// resources associated with the target cluster.
		vm.Labels[clusterv1.ClusterLabelName] = ctx.Cluster.Name

		// Indicate where the VSphereVM should find its bootstrap data.
		vm.Spec.BootstrapRef = &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  ctx.HAProxyLoadBalancer.Namespace,
			Name:       haproxy.NameForBootstrapSecret(ctx.HAProxyLoadBalancer.Name),
		}

		// Copy the HAProxyLoadBalancer's VM clone spec into the VSphereVM's
		// clone spec.
		ctx.HAProxyLoadBalancer.Spec.VirtualMachineConfiguration.DeepCopyInto(&vm.Spec.VirtualMachineCloneSpec)
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

func (r haproxylbReconciler) reconcileNetwork(ctx *context.HAProxyLoadBalancerContext, vm *unstructured.Unstructured) (bool, error) {
	var (
		newAddr string
		oldAddr = ctx.HAProxyLoadBalancer.Status.Address
	)

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

	switch {
	case newAddr == "":
		ctx.Logger.Info("waiting on IP address")
		return false, nil
	case ctx.HAProxyLoadBalancer.Status.Address == "":
		ctx.HAProxyLoadBalancer.Status.Address = newAddr
		ctx.Logger.Info("initialized IP address", newAddr)
	case newAddr != ctx.HAProxyLoadBalancer.Status.Address:
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

func (r *haproxylbReconciler) reconcileClusterToHAProxyLoadBalancers(a handler.MapObject) []reconcile.Request {
	requests := []reconcile.Request{}
	lbs := &infrav1.HAProxyLoadBalancerList{}
	err := r.Client.List(goctx.Background(),
		lbs,
		ctrlclient.InNamespace(a.Meta.GetNamespace()),
		ctrlclient.MatchingLabels(
			map[string]string{
				clusterv1.ClusterLabelName: a.Meta.GetName(),
			},
		))
	if err != nil {
		return requests
	}
	for _, lb := range lbs.Items {
		r := reconcile.Request{
			NamespacedName: apitypes.NamespacedName{
				Name:      lb.Name,
				Namespace: lb.Namespace,
			},
		}
		requests = append(requests, r)
	}
	return requests
}
