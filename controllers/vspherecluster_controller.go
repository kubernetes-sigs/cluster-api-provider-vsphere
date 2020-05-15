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
	"sync"
	"time"

	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

var (
	defaultAPIEndpointPort = int32(6443)
)

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// AddClusterControllerToManager adds the cluster controller to the provided
// manager.
func AddClusterControllerToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {

	var (
		controlledType     = &infrav1.VSphereCluster{}
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

	reconciler := clusterReconciler{ControllerContext: controllerContext}

	return ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledType).
		// Watch the CAPI resource that owns this infrastructure resource.
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: clusterutilv1.ClusterToInfrastructureMapFunc(controlledTypeGVK),
			},
		).
		// Watch the infrastructure machine resources that belong to the control
		// plane. This controller needs to reconcile the infrastructure cluster
		// once a control plane machine has an IP address.
		Watches(
			&source.Kind{Type: &infrav1.VSphereMachine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(reconciler.controlPlaneMachineToCluster),
			},
		).
		// Watch the load balancer resource that may be used to provide HA to
		// the VSphereCluster control plane.
		// TODO(akutz) Figure out how to watch LB resources without requiring
		//             their types ahead of time.
		//             Please see https://github.com/kubernetes-sigs/cluster-api/blob/84cd362e493f5edb7b16219d8134a008efb01dac/controllers/cluster_controller_phases.go#L107-L119
		//             for an example of external watchers.
		Watches(
			&source.Kind{Type: &infrav1.HAProxyLoadBalancer{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(reconciler.loadBalancerToCluster),
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
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles}).
		Complete(reconciler)
}

type clusterReconciler struct {
	*context.ControllerContext
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r clusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {

	// Get the VSphereCluster resource for this request.
	vsphereCluster := &infrav1.VSphereCluster{}
	if err := r.Client.Get(r, req.NamespacedName, vsphereCluster); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.V(4).Info("VSphereCluster not found, won't reconcile", "key", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the CAPI Cluster.
	cluster, err := clusterutilv1.GetOwnerCluster(r, r.Client, vsphereCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		r.Logger.Info("Waiting for Cluster Controller to set OwnerRef on VSphereCluster")
		return reconcile.Result{}, nil
	}
	if clusterutilv1.IsPaused(cluster, vsphereCluster) {
		r.Logger.V(4).Info("VSphereCluster %s/%s linked to a cluster that is paused",
			vsphereCluster.Namespace, vsphereCluster.Name)
		return reconcile.Result{}, nil
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(vsphereCluster, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			vsphereCluster.GroupVersionKind(),
			vsphereCluster.Namespace,
			vsphereCluster.Name)
	}

	// Create the cluster context for this request.
	clusterContext := &context.ClusterContext{
		ControllerContext: r.ControllerContext,
		Cluster:           cluster,
		VSphereCluster:    vsphereCluster,
		Logger:            r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:       patchHelper,
	}

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		if err := clusterContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			clusterContext.Logger.Error(err, "patch failed", "cluster", clusterContext.String())
		}
	}()

	// Handle deleted clusters
	if !vsphereCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(clusterContext)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(clusterContext)
}

func (r clusterReconciler) reconcileDelete(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("Reconciling VSphereCluster delete")

	vsphereMachines, err := infrautilv1.GetVSphereMachinesInCluster(ctx, ctx.Client, ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"unable to list VSphereMachines part of VSphereCluster %s/%s", ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	haproxyLoadbalancers := infrav1.HAProxyLoadBalancerList{}

	err = r.Client.List(ctx, &haproxyLoadbalancers, client.MatchingLabels(
		map[string]string{
			clusterv1.ClusterLabelName: ctx.Cluster.Name,
		},
	))
	if err != nil {
		return reconcile.Result{}, err
	}
	if len(haproxyLoadbalancers.Items) > 0 {
		for _, lb := range haproxyLoadbalancers.Items {
			if err := r.Client.Delete(ctx, lb.DeepCopy()); err != nil && !apierrors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
		}

		ctx.Logger.Info("Waiting for HAProxyLoadBalancer to be deleted", "count", len(haproxyLoadbalancers.Items))
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if len(vsphereMachines) > 0 {
		ctx.Logger.Info("Waiting for VSphereMachines to be deleted", "count", len(vsphereMachines))
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Cluster is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(ctx.VSphereCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

func (r clusterReconciler) reconcileNormal(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("Reconciling VSphereCluster")

	// If the VSphereCluster doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.VSphereCluster, infrav1.ClusterFinalizer)

	// Reconcile the VSphereCluster's load balancer.
	if ok, err := r.reconcileLoadBalancer(ctx); !ok {
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err,
				"unexpected error while reconciling load balancer for %s", ctx)
		}
		ctx.Logger.Info("load balancer is not reconciled")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Reconcile the VSphereCluster resource's ready state.
	ctx.VSphereCluster.Status.Ready = true

	// Reconcile the VSphereCluster resource's control plane endpoint.
	if ok, err := r.reconcileControlPlaneEndpoint(ctx); !ok {
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err,
				"unexpected error while reconciling control plane endpoint for %s", ctx)
		}
		ctx.Logger.Info("control plane endpoint is not reconciled")
		return reconcile.Result{}, nil
	}

	// Wait until the API server is online and accessible.
	if !r.isAPIServerOnline(ctx) {
		return reconcile.Result{}, nil
	}

	// Create the cloud config secret for the target cluster.
	if err := r.reconcileCloudConfigSecret(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"failed to reconcile cloud config secret for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// Create the external cloud provider addons
	if err := r.reconcileCloudProvider(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"failed to reconcile cloud provider for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// Create the vSphere CSI Driver addons
	if err := r.reconcileStorageProvider(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"failed to reconcile CSI Driver for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	return reconcile.Result{}, nil
}

func (r clusterReconciler) reconcileLoadBalancer(ctx *context.ClusterContext) (bool, error) {

	if ctx.VSphereCluster.Spec.LoadBalancerRef == nil {
		ctx.Logger.Info("skipping load balancer reconciliation",
			"reason", "VSphereCluster.Spec.LoadBalancerRef is nil")
		return true, nil
	}

	if !ctx.Cluster.Spec.ControlPlaneEndpoint.IsZero() {
		ctx.Logger.Info("skipping load balancer reconciliation",
			"reason", "Cluster.Spec.ControlPlaneEndpoint is already set",
			"controlPlaneEndpoint", ctx.Cluster.Spec.ControlPlaneEndpoint.String())
		return true, nil
	}

	if !ctx.VSphereCluster.Spec.ControlPlaneEndpoint.IsZero() {
		ctx.Logger.Info("skipping load balancer reconciliation",
			"reason", "VSphereCluster.Spec.ControlPlaneEndpoint is already set",
			"controlPlaneEndpoint", ctx.VSphereCluster.Spec.ControlPlaneEndpoint.String())
		return true, nil
	}

	loadBalancerRef := ctx.VSphereCluster.Spec.LoadBalancerRef
	loadBalancer := &unstructured.Unstructured{}
	loadBalancer.SetKind(loadBalancerRef.Kind)
	loadBalancer.SetAPIVersion(loadBalancerRef.APIVersion)
	loadBalancerKey := types.NamespacedName{
		Namespace: ctx.VSphereCluster.GetNamespace(),
		Name:      loadBalancerRef.Name,
	}
	if err := ctx.Client.Get(ctx, loadBalancerKey, loadBalancer); err != nil {
		if apierrors.IsNotFound(err) {
			ctx.Logger.Info("resource specified by LoadBalancerRef not found",
				"load-balancer-gvk", loadBalancerRef.APIVersion,
				"load-balancer-namespace", ctx.VSphereCluster.GetNamespace(),
				"load-balancer-name", loadBalancerRef.Name)
			return false, nil
		}
		return false, err
	}

	vsphereClusterOwnerRef := metav1.OwnerReference{
		APIVersion: ctx.VSphereCluster.APIVersion,
		Kind:       ctx.VSphereCluster.Kind,
		Name:       ctx.VSphereCluster.Name,
		UID:        ctx.VSphereCluster.UID,
	}
	loadBalancerOwnerRefs := loadBalancer.GetOwnerReferences()
	if !clusterutilv1.HasOwnerRef(loadBalancerOwnerRefs, vsphereClusterOwnerRef) {
		loadBalancerPatchHelper, err := patch.NewHelper(loadBalancer, ctx.Client)
		if err != nil {
			return false, errors.Wrapf(err,
				"failed to create patch helper for load balancer %s %s/%s",
				loadBalancer.GroupVersionKind(),
				loadBalancer.GetNamespace(),
				loadBalancer.GetName())
		}
		if err := ctrlutil.SetControllerReference(ctx.VSphereCluster, loadBalancer, r.Scheme); err != nil {
			return false, errors.Wrapf(
				err,
				"failed to set controller reference on vSphereCluster %s %s/%s",
				ctx.VSphereCluster.GroupVersionKind(),
				ctx.VSphereCluster.GetNamespace(),
				ctx.VSphereCluster.GetName())
		}
		if err := loadBalancerPatchHelper.Patch(ctx, loadBalancer); err != nil {
			return false, errors.Wrapf(err,
				"failed to patch owner references for load balancer %s %s/%s",
				loadBalancer.GroupVersionKind(),
				loadBalancer.GetNamespace(),
				loadBalancer.GetName())
		}
		ctx.Logger.Info("the load balancer is now owned by the cluster",
			"load-balancer-gvk", loadBalancer.GroupVersionKind().String(),
			"load-balancer-namespace", loadBalancer.GetNamespace(),
			"load-balancer-name", loadBalancer.GetName(),
			"vspherecluster-gvk", ctx.VSphereCluster.GroupVersionKind().String(),
			"vspherecluster-namespace", ctx.VSphereCluster.GetNamespace(),
			"vspherecluster-name", ctx.VSphereCluster.GetName())
	}

	ready, ok, err := unstructured.NestedBool(loadBalancer.Object, "status", "ready")
	if !ok {
		if err != nil {
			return false, errors.Wrapf(err,
				"unexpected error when getting status.ready for load balancer %s %s/%s",
				loadBalancer.GroupVersionKind(),
				loadBalancer.GetNamespace(),
				loadBalancer.GetName())
		}
		ctx.Logger.Info("status.ready not found for load balancer",
			"load-balancer-gvk", loadBalancer.GroupVersionKind().String(),
			"load-balancer-namespace", loadBalancer.GetNamespace(),
			"load-balancer-name", loadBalancer.GetName())
		return false, nil
	}
	if !ready {
		ctx.Logger.Info("load balancer is not ready",
			"load-balancer-gvk", loadBalancer.GroupVersionKind().String(),
			"load-balancer-namespace", loadBalancer.GetNamespace(),
			"load-balancer-name", loadBalancer.GetName())
		return false, nil
	}

	address, ok, err := unstructured.NestedString(loadBalancer.Object, "status", "address")
	if !ok {
		if err != nil {
			return false, errors.Wrapf(err,
				"unexpected error when getting status.address for load balancer %s %s/%s",
				loadBalancer.GroupVersionKind(),
				loadBalancer.GetNamespace(),
				loadBalancer.GetName())
		}
		ctx.Logger.Info("status.address not found for load balancer",
			"load-balancer-gvk", loadBalancer.GroupVersionKind().String(),
			"load-balancer-namespace", loadBalancer.GetNamespace(),
			"load-balancer-name", loadBalancer.GetName())
		return false, nil
	}
	if address == "" {
		ctx.Logger.Info("load balancer address is empty",
			"load-balancer-gvk", loadBalancer.GroupVersionKind().String(),
			"load-balancer-namespace", loadBalancer.GetNamespace(),
			"load-balancer-name", loadBalancer.GetName())
		return false, nil
	}

	// Update the VSphereCluster.Spec.ControlPlaneEndpoint with the address
	// from the load balancer.
	// The control plane endpoint also requires a port, which is obtained
	// either from the VSphereCluster.Spec.ControlPlaneEndpoint.Port
	// or the default port is used.
	ctx.VSphereCluster.Spec.ControlPlaneEndpoint.Host = address
	if ctx.VSphereCluster.Spec.ControlPlaneEndpoint.Port == 0 {
		ctx.VSphereCluster.Spec.ControlPlaneEndpoint.Port = defaultAPIEndpointPort
	}
	ctx.Logger.Info("ControlPlaneEndpoint discovered via load balancer",
		"controlPlaneEndpoint", ctx.VSphereCluster.Spec.ControlPlaneEndpoint.String())

	return true, nil
}

func (r clusterReconciler) reconcileControlPlaneEndpoint(ctx *context.ClusterContext) (bool, error) {
	// Ensure the VSphereCluster is reconciled when the API server first comes online.
	// A reconcile event will only be triggered if the Cluster is not marked as
	// ControlPlaneInitialized.
	defer r.reconcileVSphereClusterWhenAPIServerIsOnline(ctx)

	// If the cluster already has a control plane endpoint set then there
	// is nothing to do.
	if !ctx.Cluster.Spec.ControlPlaneEndpoint.IsZero() {
		ctx.VSphereCluster.Spec.ControlPlaneEndpoint.Host = ctx.Cluster.Spec.ControlPlaneEndpoint.Host
		ctx.VSphereCluster.Spec.ControlPlaneEndpoint.Port = ctx.Cluster.Spec.ControlPlaneEndpoint.Port
		ctx.Logger.Info("skipping control plane endpoint reconciliation",
			"reason", "ControlPlaneEndpoint already set on Cluster",
			"controlPlaneEndpoint", ctx.Cluster.Spec.ControlPlaneEndpoint.String())
		return true, nil
	}

	if !ctx.VSphereCluster.Spec.ControlPlaneEndpoint.IsZero() {
		ctx.Logger.Info("skipping control plane endpoint reconciliation",
			"reason", "ControlPlaneEndpoint already set on VSphereCluster",
			"controlPlaneEndpoint", ctx.VSphereCluster.Spec.ControlPlaneEndpoint.String())
		return true, nil
	}

	// Get the CAPI Machine resources for the cluster.
	machines, err := infrautilv1.GetMachinesInCluster(ctx, ctx.Client, ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	if err != nil {
		return false, errors.Wrapf(err,
			"failed to get Machinces for Cluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// Iterate over the cluster's control plane CAPI machines.
	for _, machine := range clusterutilv1.GetControlPlaneMachines(machines) {

		// Only machines with bootstrap data will have an IP address.
		if machine.Spec.Bootstrap.DataSecretName == nil {
			ctx.Logger.V(4).Info(
				"skipping machine while looking for IP address",
				"machine-name", machine.Name,
				"skip-reason", "nilBootstrapData")
			continue
		}

		// Get the VSphereMachine for the CAPI Machine resource.
		vsphereMachine, err := infrautilv1.GetVSphereMachine(ctx, ctx.Client, machine.Namespace, machine.Name)
		if err != nil {
			return false, errors.Wrapf(err,
				"failed to get VSphereMachine for Machine %s/%s/%s",
				machine.GroupVersionKind(),
				machine.Namespace,
				machine.Name)
		}

		// Get the VSphereMachine's preferred IP address.
		ipAddr, err := infrautilv1.GetMachinePreferredIPAddress(vsphereMachine)
		if err != nil {
			if err == infrautilv1.ErrNoMachineIPAddr {
				continue
			}
			return false, errors.Wrapf(err,
				"failed to get preferred IP address for VSphereMachine %s %s/%s",
				vsphereMachine.GroupVersionKind(),
				vsphereMachine.Namespace,
				vsphereMachine.Name)
		}

		// Set the ControlPlaneEndpoint so the CAPI controller can read the
		// value into the analogous CAPI Cluster using an UnstructuredReader.
		ctx.VSphereCluster.Spec.ControlPlaneEndpoint.Host = ipAddr
		ctx.VSphereCluster.Spec.ControlPlaneEndpoint.Port = defaultAPIEndpointPort
		ctx.Logger.Info(
			"ControlPlaneEndpoin discovered via control plane machine",
			"controlPlaneEndpoint", ctx.VSphereCluster.Spec.ControlPlaneEndpoint)
		return true, nil
	}

	return false, errors.Errorf("unable to determine control plane endpoint for %s", ctx)
}

var (
	// apiServerTriggers is used to prevent multiple goroutines for a single
	// Cluster that poll to see if the target API server is online.
	apiServerTriggers   = map[types.UID]struct{}{}
	apiServerTriggersMu sync.Mutex
)

func (r clusterReconciler) reconcileVSphereClusterWhenAPIServerIsOnline(ctx *context.ClusterContext) {
	if ctx.Cluster.Status.ControlPlaneInitialized {
		ctx.Logger.Info("skipping reconcile when API server is online",
			"reason", "controlPlaneInitialized")
		return
	}
	apiServerTriggersMu.Lock()
	defer apiServerTriggersMu.Unlock()
	if _, ok := apiServerTriggers[ctx.Cluster.UID]; ok {
		ctx.Logger.Info("skipping reconcile when API server is online",
			"reason", "alreadyPolling")
		return
	}
	apiServerTriggers[ctx.Cluster.UID] = struct{}{}
	go func() {
		// Block until the target API server is online.
		ctx.Logger.Info("start polling API server for online check")
		wait.PollImmediateInfinite(time.Second*1, func() (bool, error) { return r.isAPIServerOnline(ctx), nil }) // nolint:errcheck
		ctx.Logger.Info("stop polling API server for online check")
		ctx.Logger.Info("triggering GenericEvent", "reason", "api-server-online")
		eventChannel := ctx.GetGenericEventChannelFor(ctx.VSphereCluster.GetObjectKind().GroupVersionKind())
		eventChannel <- event.GenericEvent{
			Meta:   ctx.VSphereCluster,
			Object: ctx.VSphereCluster,
		}

		// Once the control plane has been marked as initialized it is safe to
		// remove the key from the map that prevents multiple goroutines from
		// polling the API server to see if it is online.
		ctx.Logger.Info("start polling for control plane initialized")
		wait.PollImmediateInfinite(time.Second*1, func() (bool, error) { return r.isControlPlaneInitialized(ctx), nil }) // nolint:errcheck
		ctx.Logger.Info("stop polling for control plane initialized")
		apiServerTriggersMu.Lock()
		delete(apiServerTriggers, ctx.Cluster.UID)
		apiServerTriggersMu.Unlock()
	}()
}

func (r clusterReconciler) isAPIServerOnline(ctx *context.ClusterContext) bool {
	if kubeClient, err := infrautilv1.NewKubeClient(ctx, ctx.Client, ctx.Cluster); err == nil {
		if _, err := kubeClient.CoreV1().Nodes().List(metav1.ListOptions{}); err == nil {
			// The target cluster is online. To make sure the correct control
			// plane endpoint information is logged, it is necessary to fetch
			// an up-to-date Cluster resource. If this fails, then set the
			// control plane endpoint information to the values from the
			// VSphereCluster resource, as it must have the correct information
			// if the API server is online.
			cluster := &clusterv1.Cluster{}
			clusterKey := client.ObjectKey{Namespace: ctx.Cluster.Namespace, Name: ctx.Cluster.Name}
			if err := ctx.Client.Get(ctx, clusterKey, cluster); err != nil {
				cluster = ctx.Cluster.DeepCopy()
				cluster.Spec.ControlPlaneEndpoint.Host = ctx.VSphereCluster.Spec.ControlPlaneEndpoint.Host
				cluster.Spec.ControlPlaneEndpoint.Port = ctx.VSphereCluster.Spec.ControlPlaneEndpoint.Port
				ctx.Logger.Error(err, "failed to get updated cluster object while checking if API server is online")
			}
			ctx.Logger.Info(
				"API server is online",
				"controlPlaneEndpoint", cluster.Spec.ControlPlaneEndpoint.String())
			return true
		}
	}
	return false
}

func (r clusterReconciler) isControlPlaneInitialized(ctx *context.ClusterContext) bool {
	cluster := &clusterv1.Cluster{}
	clusterKey := client.ObjectKey{Namespace: ctx.Cluster.Namespace, Name: ctx.Cluster.Name}
	if err := ctx.Client.Get(ctx, clusterKey, cluster); err != nil {
		if !apierrors.IsNotFound(err) {
			ctx.Logger.Error(err, "failed to get updated cluster object while checking if control plane is initialized")
			return false
		}
		ctx.Logger.Info("exiting early because cluster no longer exists")
		return true
	}
	return cluster.Status.ControlPlaneInitialized
}

func (r clusterReconciler) reconcileCloudProvider(ctx *context.ClusterContext) error {
	// if the cloud provider image is not specified, then we do nothing
	cloudproviderConfig := ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Cloud
	if cloudproviderConfig == nil {
		ctx.Logger.V(2).Info(
			"cloud provider config was not specified in VSphereCluster, skipping reconciliation of the cloud provider integration",
		)
		return nil
	}

	if cloudproviderConfig.ControllerImage == "" {
		cloudproviderConfig.ControllerImage = cloudprovider.DefaultCPIControllerImage
	}

	ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Cloud = cloudproviderConfig
	controllerImage := cloudproviderConfig.ControllerImage

	targetClusterClient, err := infrautilv1.NewKubeClient(ctx, ctx.Client, ctx.Cluster)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get client for Cluster %s/%s",
			ctx.Cluster.Namespace, ctx.Cluster.Name)
	}

	serviceAccount := cloudprovider.CloudControllerManagerServiceAccount()
	if _, err := targetClusterClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(serviceAccount); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	cloudConfigData, err := ctx.VSphereCluster.Spec.CloudProviderConfiguration.MarshalINI()
	if err != nil {
		return err
	}

	cloudConfigMap := cloudprovider.CloudControllerManagerConfigMap(string(cloudConfigData))
	if _, err := targetClusterClient.CoreV1().ConfigMaps(cloudConfigMap.Namespace).Create(cloudConfigMap); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonSet := cloudprovider.CloudControllerManagerDaemonSet(controllerImage, cloudproviderConfig.MarshalCloudProviderArgs())
	if _, err := targetClusterClient.AppsV1().DaemonSets(daemonSet.Namespace).Create(daemonSet); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	service := cloudprovider.CloudControllerManagerService()
	if _, err := targetClusterClient.CoreV1().Services(daemonSet.Namespace).Create(service); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRole := cloudprovider.CloudControllerManagerClusterRole()
	if _, err := targetClusterClient.RbacV1().ClusterRoles().Create(clusterRole); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRoleBinding := cloudprovider.CloudControllerManagerClusterRoleBinding()
	if _, err := targetClusterClient.RbacV1().ClusterRoleBindings().Create(clusterRoleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	roleBinding := cloudprovider.CloudControllerManagerRoleBinding()
	if _, err := targetClusterClient.RbacV1().RoleBindings(roleBinding.Namespace).Create(roleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// nolint:gocognit
func (r clusterReconciler) reconcileStorageProvider(ctx *context.ClusterContext) error {
	// if storage config is not defined, assume we don't want CSI installed
	storageConfig := ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Storage
	if storageConfig == nil {
		ctx.Logger.V(2).Info(
			"storage config was not specified in VSphereCluster, skipping reconciliation of the CSI driver",
		)

		return nil
	}

	// if at least 1 field in the storage config is defined, assume CNS should be installed
	// and use default images when not defined
	if storageConfig.ControllerImage == "" {
		storageConfig.ControllerImage = cloudprovider.DefaultCSIControllerImage
	}

	if storageConfig.NodeDriverImage == "" {
		storageConfig.NodeDriverImage = cloudprovider.DefaultCSINodeDriverImage
	}

	if storageConfig.AttacherImage == "" {
		storageConfig.AttacherImage = cloudprovider.DefaultCSIAttacherImage
	}

	if storageConfig.ProvisionerImage == "" {
		storageConfig.ProvisionerImage = cloudprovider.DefaultCSIProvisionerImage
	}

	if storageConfig.MetadataSyncerImage == "" {
		storageConfig.MetadataSyncerImage = cloudprovider.DefaultCSIMetadataSyncerImage
	}

	if storageConfig.LivenessProbeImage == "" {
		storageConfig.LivenessProbeImage = cloudprovider.DefaultCSILivenessProbeImage
	}

	if storageConfig.RegistrarImage == "" {
		storageConfig.RegistrarImage = cloudprovider.DefaultCSIRegistrarImage
	}

	ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Storage = storageConfig

	targetClusterClient, err := infrautilv1.NewKubeClient(ctx, ctx.Client, ctx.Cluster)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get client for Cluster %s/%s",
			ctx.Cluster.Namespace, ctx.Cluster.Name)
	}

	serviceAccount := cloudprovider.CSIControllerServiceAccount()
	if _, err := targetClusterClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(serviceAccount); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRole := cloudprovider.CSIControllerClusterRole()
	if _, err := targetClusterClient.RbacV1().ClusterRoles().Create(clusterRole); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRoleBinding := cloudprovider.CSIControllerClusterRoleBinding()
	if _, err := targetClusterClient.RbacV1().ClusterRoleBindings().Create(clusterRoleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// we have to marshal a separate INI file for CSI since it does not
	// support Secrets for vCenter credentials yet.
	cloudConfig, err := cloudprovider.ConfigForCSI(ctx).MarshalINI()
	if err != nil {
		return err
	}

	cloudConfigSecret := cloudprovider.CSICloudConfigSecret(string(cloudConfig))
	if _, err := targetClusterClient.CoreV1().Secrets(cloudConfigSecret.Namespace).Create(cloudConfigSecret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	csiDriver := cloudprovider.CSIDriver()
	if _, err := targetClusterClient.StorageV1beta1().CSIDrivers().Create(csiDriver); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonSet := cloudprovider.VSphereCSINodeDaemonSet(ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Storage)
	if _, err := targetClusterClient.AppsV1().DaemonSets(daemonSet.Namespace).Create(daemonSet); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// check if CSI is already deployed
	_, err = targetClusterClient.AppsV1().StatefulSets(cloudprovider.CSINamespace).Get(cloudprovider.CSIControllerName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	// this is a new cluster deploy the latest csi
	if apierrors.IsNotFound(err) {
		deployment := cloudprovider.CSIControllerDeployment(ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Storage)
		if _, err := targetClusterClient.AppsV1().Deployments(deployment.Namespace).Create(deployment); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

// reconcileCloudConfigSecret ensures the cloud config secret is present in the
// target cluster
func (r clusterReconciler) reconcileCloudConfigSecret(ctx *context.ClusterContext) error {
	if len(ctx.VSphereCluster.Spec.CloudProviderConfiguration.VCenter) == 0 {
		return errors.Errorf(
			"no vCenters defined for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	targetClusterClient, err := infrautilv1.NewKubeClient(ctx, ctx.Client, ctx.Cluster)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get client for Cluster %s/%s",
			ctx.Cluster.Namespace, ctx.Cluster.Name)
	}

	credentials := map[string]string{}
	for server := range ctx.VSphereCluster.Spec.CloudProviderConfiguration.VCenter {
		credentials[fmt.Sprintf("%s.username", server)] = ctx.Username
		credentials[fmt.Sprintf("%s.password", server)] = ctx.Password
	}
	// Define the kubeconfig secret for the target cluster.
	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.VSphereCluster.Spec.CloudProviderConfiguration.Global.SecretNamespace,
			Name:      ctx.VSphereCluster.Spec.CloudProviderConfiguration.Global.SecretName,
		},
		Type:       apiv1.SecretTypeOpaque,
		StringData: credentials,
	}
	if _, err := targetClusterClient.CoreV1().Secrets(secret.Namespace).Create(secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return errors.Wrapf(
			err,
			"failed to create cloud provider secret for Cluster %s/%s",
			ctx.Cluster.Namespace, ctx.Cluster.Name)
	}

	ctx.Logger.Info("created cloud provider credential secret",
		"cluster-namespace", ctx.Cluster.Namespace,
		"cluster-name", ctx.Cluster.Name,
		"secret-name", secret.Name,
		"secret-namespace", secret.Namespace)

	return nil
}

// controlPlaneMachineToCluster is a handler.ToRequestsFunc to be used
// to enqueue requests for reconciliation for VSphereCluster to update
// its status.apiEndpoints field.
func (r clusterReconciler) controlPlaneMachineToCluster(o handler.MapObject) []ctrl.Request {
	vsphereMachine, ok := o.Object.(*infrav1.VSphereMachine)
	if !ok {
		r.Logger.Error(nil, fmt.Sprintf("expected a VSphereMachine but got a %T", o.Object))
		return nil
	}
	if !infrautilv1.IsControlPlaneMachine(vsphereMachine) {
		return nil
	}
	if len(vsphereMachine.Status.Addresses) == 0 {
		return nil
	}
	// Get the VSphereMachine's preferred IP address.
	if _, err := infrautilv1.GetMachinePreferredIPAddress(vsphereMachine); err != nil {
		if err == infrautilv1.ErrNoMachineIPAddr {
			return nil
		}
		r.Logger.Error(err, "failed to get preferred IP address for VSphereMachine",
			"namespace", vsphereMachine.Namespace, "name", vsphereMachine.Name)
		return nil
	}

	// Fetch the CAPI Cluster.
	cluster, err := clusterutilv1.GetClusterFromMetadata(r, r.Client, vsphereMachine.ObjectMeta)
	if err != nil {
		r.Logger.Error(err, "VSphereMachine is missing cluster label or cluster does not exist",
			"namespace", vsphereMachine.Namespace, "name", vsphereMachine.Name)
		return nil
	}

	if cluster.Status.ControlPlaneInitialized {
		return nil
	}

	if !cluster.Spec.ControlPlaneEndpoint.IsZero() {
		return nil
	}

	// Fetch the VSphereCluster
	vsphereCluster := &infrav1.VSphereCluster{}
	vsphereClusterKey := client.ObjectKey{
		Namespace: vsphereMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(r, vsphereClusterKey, vsphereCluster); err != nil {
		r.Logger.Error(err, "failed to get VSphereCluster",
			"namespace", vsphereClusterKey.Namespace, "name", vsphereClusterKey.Name)
		return nil
	}

	if !vsphereCluster.Spec.ControlPlaneEndpoint.IsZero() {
		return nil
	}

	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: vsphereClusterKey.Namespace,
			Name:      vsphereClusterKey.Name,
		},
	}}
}

// loadBalancerToCluster is a handler.ToRequestsFunc that triggers
// reconcile events for a VSphereCluster resource when a load balancer
// resource is reconciled.
func (r clusterReconciler) loadBalancerToCluster(o handler.MapObject) []ctrl.Request {
	obj, ok := o.Object.(metav1.Object)
	if !ok {
		r.Logger.Error(nil, fmt.Sprintf("expected an metav1.Object but got a %T", o.Object))
		return nil
	}

	var vsphereClusterRef *metav1.OwnerReference
	for _, ownerRef := range obj.GetOwnerReferences() {
		ownerRef := ownerRef
		if ownerRef.APIVersion == infrav1.GroupVersion.String() &&
			ownerRef.Kind == "VSphereCluster" {
			vsphereClusterRef = &ownerRef
		}
	}
	if vsphereClusterRef == nil {
		return nil
	}

	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      vsphereClusterRef.Name,
		},
	}}
}
