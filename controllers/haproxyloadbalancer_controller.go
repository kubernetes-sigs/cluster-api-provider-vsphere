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
	"regexp"
	"strings"
	"time"

	"github.com/antihax/optional"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/utils/net"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/collections"
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

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	hapi "sigs.k8s.io/cluster-api-provider-vsphere/contrib/haproxy/openapi"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/haproxy"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

var (
	haproxyControlledType     = &infrav1.HAProxyLoadBalancer{}
	haproxyControlledTypeName = reflect.TypeOf(haproxyControlledType).Elem().Name()
	haproxyControlledTypeGVK  = infrav1.GroupVersion.WithKind(haproxyControlledTypeName)
	trimmer                   = regexp.MustCompile(`(?m)[\t ]?(.*)[\t ]$`)
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
		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(haproxyControlledTypeName))
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
		For(haproxyControlledType).
		// Watch any VSphereVM resources owned by the controlled type.
		Watches(
			&source.Kind{Type: &infrav1.VSphereVM{}},
			&handler.EnqueueRequestForOwner{OwnerType: haproxyControlledType, IsController: false},
		).
		// Watch the CAPI machines that are members of the control plane which
		// this HAProxyLoadBalancer servies.
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(reconciler.controlPlaneMachineToHAProxyLoadBalancer),
		).
		// Watch a GenericEvent channel for the controlled resource.
		//
		// This is useful when there are events outside of Kubernetes that
		// should cause a resource to be synchronized, such as a goroutine
		// waiting on some asynchronous, external task to complete.
		Watches(
			&source.Channel{Source: ctx.GetGenericEventChannelFor(haproxyControlledTypeGVK)},
			&handler.EnqueueRequestForObject{},
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles}).
		Build(reconciler)
	if err != nil {
		return err
	}
	err = controller.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(reconciler.reconcileClusterToHAProxyLoadBalancers),
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

type haproxylbReconciler struct {
	*context.ControllerContext
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r haproxylbReconciler) Reconcile(_ goctx.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	logger := r.Logger.WithValues("namespace", req.Namespace, "name", req.Name)

	logger.Info("Starting reconciliation")

	// Get the HAProxyLoadBalancer resource for this request.
	haproxylb := &infrav1.HAProxyLoadBalancer{}
	if err := r.Client.Get(r, req.NamespacedName, haproxylb); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "Won't reconcile")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(haproxylb, r.Client)
	if err != nil {
		logger.Error(err, "Failed to init patch helper")
		return ctrl.Result{}, err
	}

	// Create the HAProxyLoadBalancer context for this request.
	ctx := &context.HAProxyLoadBalancerContext{
		ControllerContext:   r.ControllerContext,
		HAProxyLoadBalancer: haproxylb,
		Logger:              logger.WithValues("kind", haproxyControlledTypeGVK.Kind, "api-version", haproxyControlledTypeGVK.Version),
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
			ctx.Logger.Error(err, "Patch failed")
		}
	}()

	cluster, err := clusterutilv1.GetClusterFromMetadata(r.Context, r.Client, haproxylb.ObjectMeta)
	if err == nil {
		if annotations.IsPaused(cluster, haproxylb) {
			ctx.Logger.V(4).Info("Linked cluster is paused")
			return ctrl.Result{}, nil
		}
		ctx.Cluster = cluster
	}
	// Handle deleted haproxyloadbalancers
	if !haproxylb.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx)
	}

	// check if we got the cluster as it is needed for reconcileNormal
	if ctx.Cluster == nil {
		return ctrl.Result{}, err
	}

	// Handle non-deleted haproxyloadbalancers
	return r.reconcileNormal(ctx)
}

func (r haproxylbReconciler) reconcileDelete(ctx *context.HAProxyLoadBalancerContext) (ctrl.Result, error) {
	ctx.Logger.Info("Handling deleted HAProxyLoadBalancer")

	if err := r.reconcileDeleteVM(ctx); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.reconcileDeleteSecrets(ctx); err != nil {
				ctx.Logger.Error(err, "Error deleting secrets")
				return ctrl.Result{}, err
			}

			// The VM is deleted so remove the finalizer.
			ctrlutil.RemoveFinalizer(ctx.HAProxyLoadBalancer, infrav1.HAProxyLoadBalancerFinalizer)
		}
		return ctrl.Result{}, err
	}

	ctx.Logger.Info("Waiting for VSphereVM to be deleted")
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
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
	vmKey := types.NamespacedName{
		Namespace: ctx.HAProxyLoadBalancer.Namespace,
		Name:      ctx.HAProxyLoadBalancer.Name + "-lb",
	}

	// Attempt to find the associated VSphereVM resource.
	if err := ctx.Client.Get(ctx, vmKey, vm); err != nil {
		return err
	} else if vm.GetDeletionTimestamp().IsZero() {
		// If the VSphereVM was found and it's not already enqueued for
		// deletion, go ahead and attempt to delete it.
		if err := ctx.Client.Delete(ctx, vm); err != nil {
			return err
		}

		// Go ahead and return here since the deletion of the VSphereVM resource
		// will trigger a new reconcile for this HAProxyLoadBalancer resource.
		return nil
	}

	return nil
}

func (r haproxylbReconciler) reconcileNormal(ctx *context.HAProxyLoadBalancerContext) (ctrl.Result, error) {
	// If the HAProxyLoadBalancer doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.HAProxyLoadBalancer, infrav1.HAProxyLoadBalancerFinalizer)

	if !ctx.HAProxyLoadBalancer.Status.Ready {
		// Create the HAProxyLoadBalancer's signing certificate/key pair secret.
		ctx.Logger.Info("Generating certificates")
		if err := haproxy.CreateCASecret(ctx, ctx.Client, ctx.Cluster, ctx.HAProxyLoadBalancer); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				ctx.Logger.Error(err, "Failed to create signing certificate/key pair secret")
				return ctrl.Result{}, err
			}
			// Create the HAProxyLoadBalancer's bootstrap data secret.
			if err := haproxy.CreateBootstrapSecret(ctx, ctx.Client, ctx.Cluster, ctx.HAProxyLoadBalancer); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					ctx.Logger.Error(err, "Failed to create bootstrap secret")
					return ctrl.Result{}, err
				}
			}
		}
	}

	// Reconcile the load balancer VM.
	vm, err := r.reconcileVM(ctx)
	if err != nil {
		ctx.Logger.Error(err, "Unexpected error reconciling vm")
		return ctrl.Result{}, err
	}

	if !ctx.HAProxyLoadBalancer.Status.Ready {
		ctx.Logger.Info("HAProxy LoadBalancer not ready, reconciling network")
		// Reconcile the HAProxyLoadBalancer's address.
		if ok, err := r.reconcileNetwork(ctx, vm); !ok {
			if err != nil {
				ctx.Logger.Error(err, "Unexpected error while reconciling network")
				return ctrl.Result{}, err
			}
			ctx.Logger.Info("Network is not reconciled, requeing in 10 seconds")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		// Create the HAProxyLoadBalancer's API config secret.
		if err := haproxy.CreateConfigSecret(ctx, ctx.Client, ctx.Cluster, ctx.HAProxyLoadBalancer); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				ctx.Logger.Error(err, "Failed to create API config secret")
				return ctrl.Result{}, err
			}
		}

		// Mark the load balancer as ready.
		ctx.HAProxyLoadBalancer.Status.Ready = true
		ctx.Logger.Info("HAProxyLoadBalancer is ready")
	}

	// Reconcile the HAProxyLoadBalancer's backen!d servers.
	if err := r.reconcileLoadBalancerConfiguration(ctx); err != nil {
		ctx.Logger.Error(err, "Requeing after 10 seconds")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

func (r haproxylbReconciler) BackEndpointsForCluster(ctx *context.HAProxyLoadBalancerContext) ([]corev1.EndpointAddress, error) {
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
		return nil, errors.Wrap(err, "Failed to get machines for cluster")
	}

	// Get the control plane machines.
	controlPlaneMachines := collections.FromMachineList(machineList).Filter(collections.ControlPlaneMachines(ctx.Cluster.Name))
	endpoints := make([]corev1.EndpointAddress, 0)
	for _, machine := range controlPlaneMachines {

		// check if machine has joined the cluster before adding it to the list of backends
		if conditions.IsTrue(ctx.Cluster, clusterv1.ControlPlaneInitializedCondition) {
			if machine.Status.NodeRef == nil ||
				machine.Status.FailureReason != nil ||
				machine.Status.FailureMessage != nil {
				continue
			}
		}

		machineEndpoints := make([]corev1.EndpointAddress, 0)
		for i, addr := range machine.Status.Addresses {
			if addr.Type == clusterv1.MachineExternalIP {
				// TODO(frapposelli): Remove this check once HAproxy fully supports IPv6 - issue #859
				if utilnet.IsIPv6String(addr.Address) {
					continue
				}
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
		return errors.Wrap(err, "Failed to get HAproxy dataplane config secret")
	}

	dataplaneConfig, err := haproxy.LoadDataplaneConfig(secret.Data[haproxy.SecretDataKey])
	if err != nil {
		return errors.Wrap(err, "Failed to rehydrate HAProxy dataplane client config")
	}

	// Create a HAPI client.
	client, err := haproxy.ClientFromHAPIConfig(dataplaneConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to get HAProxy dataplane client")
	}

	// Get the current configuration version.
	global, _, err := client.GlobalApi.GetGlobal(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to get HAProxy dataplane global config")
	}

	// Attempt to close existing transactions in case controller had previously crashed
	// TODO: When migrating to Dataplane APIv2 and no longer string templating,
	// this should switch to tracking the existing transaction ID in the spec or status
	// for re-entrancy, allowing other dataplane users to do configuration changes.
	if err := r.closeAllTransactions(ctx, client); err != nil {
		// only log error when closing existing transactions
		ctx.Logger.Error(err, "error closing transactions")
	}

	transaction, _, err := client.TransactionsApi.StartTransaction(ctx, global.Version)

	if err != nil {
		return errors.Wrap(err, "Failed to create HAProxy dataplane transaction")
	}
	transactionID := optional.NewString(transaction.Id)

	defer func() {
		if _, _, err := client.TransactionsApi.CommitTransaction(ctx, transaction.Id, nil); err != nil {
			ctx.Logger.Error(err, "Failed to commit HAProxy dataplane transaction")
		}
	}()

	backends, err := r.BackEndpointsForCluster(ctx)
	if err != nil {
		return errors.Wrap(err, "Couldn't fetch endpoints for cluster")
	}

	if len(backends) == 0 {
		return errors.Wrap(err, "No backends found, skipping reconfiguration")
	}

	renderConfig := haproxy.NewRenderConfiguration().
		WithDataPlaneConfig(dataplaneConfig).
		WithAddresses(backends)
	haProxyConfig, err := renderConfig.RenderHAProxyConfiguration()

	if err != nil {
		return errors.Wrap(err, "Unable to render new configuration")
	}

	resp, _, err := client.ConfigurationApi.GetHAProxyConfiguration(ctx, &hapi.GetHAProxyConfigurationOpts{
		TransactionId: transactionID,
	})

	if err != nil {
		return errors.Wrap(err, "Failed to get haproxy configuration")
	}

	originalConfig := trimmer.ReplaceAllString(resp.Data, "$1")
	comparisonConfig := trimmer.ReplaceAllString(haProxyConfig, "$1")

	// commit new configuration, ending the transaction
	if originalConfig != comparisonConfig {
		ctx.Logger.Info("HAProxy Configuration changed, reloading")
		_, _, err := client.ConfigurationApi.PostHAProxyConfiguration(ctx, haProxyConfig, &hapi.PostHAProxyConfigurationOpts{
			Version: optional.NewInt32(transaction.Version),
		})
		if err != nil {
			return errors.Wrap(err, "Unable to post new configuration")
		}
	} else {
		ctx.Logger.Info("No change in HAProxy configration, skipping reconciliation.")
	}

	ctx.Logger.Info("Reconciled load balancer backend servers")
	return nil
}

func (r haproxylbReconciler) closeAllTransactions(ctx *context.HAProxyLoadBalancerContext, client *hapi.APIClient) error {
	transactions, _, err := client.TransactionsApi.GetTransactions(ctx, nil)
	if err != nil {
		return err
	}
	for _, t := range transactions {
		if _, _, err := client.TransactionsApi.CommitTransaction(ctx, t.Id, nil); err != nil {
			return err
		}
	}

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
			"Failed to convert %s to unstructured data",
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
	ctx.Logger = ctx.Logger.WithValues("vm-name", vm.Name)
	mutateFn := func() (err error) {
		// Ensure the HAProxyLoadBalancer is marked as an owner of the VSphereVM.
		if err := ctrlutil.SetControllerReference(ctx.HAProxyLoadBalancer, vm, r.Scheme); err != nil {
			return errors.Wrapf(err, "Failed to set controller reference on HAProxyLoadBalancer")
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

		objectKey := ctrlclient.ObjectKeyFromObject(vm)
		existingVM := &infrav1.VSphereVM{}
		if err := r.Client.Get(ctx, objectKey, existingVM); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		vm.Spec.BiosUUID = existingVM.Spec.BiosUUID
		return nil
	}
	if _, err := ctrlutil.CreateOrUpdate(ctx, ctx.Client, vm, mutateFn); err != nil {
		if apierrors.IsAlreadyExists(err) {
			ctx.Logger.Info("VSphereVM already exists")
			return nil, err
		}
		ctx.Logger.Error(err, "Failed to CreateOrUpdate VSphereVM")
		return nil, err
	}

	return vm, nil
}

func (r haproxylbReconciler) reconcileNetwork(ctx *context.HAProxyLoadBalancerContext, vm *unstructured.Unstructured) (bool, error) {
	var (
		newAddr string
		oldAddr = ctx.HAProxyLoadBalancer.Status.Address
	)

	ctx.Logger = ctx.Logger.WithValues("old-ip-address", oldAddr, "vm-api-version", vm.GetAPIVersion(), "vm-kind", vm.GetKind(), "vm-name", vm.GetName())

	// Otherwise the IP for the load balancer is obtained from the VM's
	// status.addresses field.
	addresses, ok, err := unstructured.NestedStringSlice(vm.Object, "status", "addresses")
	if !ok {
		if err != nil {
			return false, errors.Wrapf(err,
				"Unexpected error getting status.addresses from VM %s %s/%s for %s",
				vm.GroupVersionKind(),
				vm.GetNamespace(),
				vm.GetName(),
				ctx)
		}
		ctx.Logger.Info("waiting on vm for ip address")
		return false, nil
	}
	for _, addr := range addresses {
		if addr == "" {
			continue
		}
		newAddr = addr
		ctx.Logger = ctx.Logger.WithValues("ip-address", newAddr)
		ctx.Logger.Info("Discovered IP address from VM")
		break
	}

	switch {
	case newAddr == "":
		ctx.Logger.Info("Waiting on IP address")
		return false, nil
	case ctx.HAProxyLoadBalancer.Status.Address == "":
		ctx.HAProxyLoadBalancer.Status.Address = newAddr
		ctx.Logger.Info("Initialized IP address")
	case newAddr != ctx.HAProxyLoadBalancer.Status.Address:
		ctx.HAProxyLoadBalancer.Status.Address = newAddr
		ctx.Logger.Info("Updated IP address")
	}

	return true, nil
}

// controlPlaneMachineToHAProxyLoadBalancer is a handler.ToRequestsFunc to be
// used to trigger reconcile events for an HAProxyLoadBalancer when a CAPI
// Machine is reconciled and it has IP addresses and is a member of the same
// control plane that the HAProxyLoadBalancer services.
func (r haproxylbReconciler) controlPlaneMachineToHAProxyLoadBalancer(o ctrlclient.Object) []ctrl.Request {
	machine, ok := o.(*clusterv1.Machine)
	if !ok {
		r.Logger.Error(errors.New("invalid type"),
			"Expected to receive a CAPI Machine resource",
			"expected-type", "Machine",
			"actual-type", fmt.Sprintf("%T", o))
		return nil
	}
	if !infrautilv1.IsControlPlaneMachine(machine) {
		return nil
	}
	if len(machine.Status.Addresses) == 0 {
		return nil
	}

	logger := r.Logger.WithValues(
		"machine-api-version", machine.APIVersion,
		"machine-kind", machine.Kind,
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name)

	cluster, err := clusterutilv1.GetClusterFromMetadata(r, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Error(err, "Machine is missing cluster label or cluster does not exist")
		return nil
	}

	if cluster.Spec.InfrastructureRef == nil {
		return nil
	}

	// The infraClusterRef may not specify the namespace as it's assumed to be
	// in the same namespace as the Cluster. When the namespace is empty, set it
	// to the same namespace as the Cluster.
	infraClusterRef := cluster.Spec.InfrastructureRef

	// Since HAProxyLoadBalancer is now deprecated, only reconcile for vSphereClusters
	if infraClusterRef.Kind != clusterControlledTypeName {
		return nil
	}

	if infraClusterRef.Namespace == "" {
		infraClusterRef.Namespace = cluster.Namespace
	}

	infraClusterKey := ctrlclient.ObjectKey{
		Namespace: infraClusterRef.Namespace,
		Name:      infraClusterRef.Name,
	}
	infraCluster := &unstructured.Unstructured{Object: map[string]interface{}{}}
	infraCluster.SetAPIVersion(infraClusterRef.APIVersion)
	infraCluster.SetKind(infraClusterRef.Kind)
	logger = logger.WithValues(
		"infra-cluster-api-version", infraClusterRef.APIVersion,
		"infra-cluster-kind", infraClusterRef.Kind,
		"infra-cluster-namespace", infraClusterRef.Namespace,
		"infra-cluster-name", infraClusterRef.Name,
	)
	if err := r.Client.Get(r, infraClusterKey, infraCluster); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "Waiting on infrastructure cluster")
			return nil
		}
		logger.Error(err, "Unexpected error while waiting on infrastructure cluster")
		return nil
	}

	loadBalancerRef := &corev1.ObjectReference{}
	if err := clusterutilv1.UnstructuredUnmarshalField(infraCluster, loadBalancerRef, "spec", "loadBalancerRef"); err != nil {
		if err != clusterutilv1.ErrUnstructuredFieldNotFound {
			r.Logger.Error(err, "Unexpected error getting infrastructure cluster's spec.loadBalancerRef")
		}
		return nil
	}

	// The loadBalancerRef may not specify the namespace as it's assumed to be
	// in the same namespace as the Cluster. When the namespace is empty, set it
	// to the same namespace as the Cluster.
	if loadBalancerRef.Namespace == "" {
		loadBalancerRef.Namespace = cluster.Namespace
	}

	logger = logger.WithValues(
		"load-balancer-ref-api-version", loadBalancerRef.APIVersion,
		"load-balancer-ref-kind", loadBalancerRef.Kind,
		"load-balancer-ref-namespace", loadBalancerRef.Namespace,
		"load-balancer-ref-name", loadBalancerRef.Name,
	)

	if loadBalancerRef.Name == "" {
		logger.Error(err, "Infrastructure cluster's spec.loadBalancerRef.Name is empty")
		return nil
	}

	if loadBalancerRef.Kind != haproxyControlledTypeName {
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

func (r *haproxylbReconciler) reconcileClusterToHAProxyLoadBalancers(a ctrlclient.Object) []reconcile.Request {
	requests := []reconcile.Request{}
	lbs := &infrav1.HAProxyLoadBalancerList{}
	err := r.Client.List(goctx.Background(),
		lbs,
		ctrlclient.InNamespace(a.GetNamespace()),
		ctrlclient.MatchingLabels(
			map[string]string{
				clusterv1.ClusterLabelName: a.GetName(),
			},
		))
	if err != nil {
		return requests
	}
	for _, lb := range lbs.Items {
		r := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      lb.Name,
				Namespace: lb.Namespace,
			},
		}
		requests = append(requests, r)
	}
	return requests
}
