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
	"sync"
	"time"

	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
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
)

var (
	defaultAPIEndpointPort    = int32(6443)
	clusterControlledType     = &infrav1.VSphereCluster{}
	clusterControlledTypeName = reflect.TypeOf(clusterControlledType).Elem().Name()
	clusterControlledTypeGVK  = infrav1.GroupVersion.WithKind(clusterControlledTypeName)
)

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusteridentities,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// AddClusterControllerToManager adds the cluster controller to the provided
// manager.
func AddClusterControllerToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {

	var (
		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(clusterControlledTypeName))
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
		For(clusterControlledType).
		// Watch the CAPI resource that owns this infrastructure resource.
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(clusterutilv1.ClusterToInfrastructureMapFunc(clusterControlledTypeGVK)),
		).
		// Watch the infrastructure machine resources that belong to the control
		// plane. This controller needs to reconcile the infrastructure cluster
		// once a control plane machine has an IP address.
		Watches(
			&source.Kind{Type: &infrav1.VSphereMachine{}},
			handler.EnqueueRequestsFromMapFunc(reconciler.controlPlaneMachineToCluster),
		).
		// Watch the load balancer resource that may be used to provide HA to
		// the VSphereCluster control plane.
		// TODO(akutz) Figure out how to watch LB resources without requiring
		//             their types ahead of time.
		//             Please see https://github.com/kubernetes-sigs/cluster-api/blob/84cd362e493f5edb7b16219d8134a008efb01dac/controllers/cluster_controller_phases.go#L107-L119
		//             for an example of external watchers.
		Watches(
			&source.Kind{Type: &infrav1.HAProxyLoadBalancer{}},
			handler.EnqueueRequestsFromMapFunc(reconciler.loadBalancerToCluster),
		).
		// Watch a GenericEvent channel for the controlled resource.
		//
		// This is useful when there are events outside of Kubernetes that
		// should cause a resource to be synchronized, such as a goroutine
		// waiting on some asynchronous, external task to complete.
		Watches(
			&source.Channel{Source: ctx.GetGenericEventChannelFor(clusterControlledTypeGVK)},
			&handler.EnqueueRequestForObject{},
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles}).
		Complete(reconciler)
}

type clusterReconciler struct {
	*context.ControllerContext
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r clusterReconciler) Reconcile(ctx goctx.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {

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
	if annotations.IsPaused(cluster, vsphereCluster) {
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

	// ccm and csi needs the control plane endpoint (which is removed when the VSphereCluster is)
	conditions.MarkFalse(ctx.VSphereCluster, infrav1.CCMAvailableCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
	conditions.MarkFalse(ctx.VSphereCluster, infrav1.CSIAvailableCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")

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

	if len(vsphereMachines) > 0 {
		ctx.Logger.Info("Waiting for VSphereMachines to be deleted", "count", len(vsphereMachines))
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if len(haproxyLoadbalancers.Items) > 0 {
		conditions.MarkFalse(ctx.VSphereCluster, infrav1.LoadBalancerAvailableCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
		for _, lb := range haproxyLoadbalancers.Items {
			if err := r.Client.Delete(ctx, lb.DeepCopy()); err != nil && !apierrors.IsNotFound(err) {
				conditions.MarkFalse(ctx.VSphereCluster, infrav1.LoadBalancerAvailableCondition, "DeletionFailed", clusterv1.ConditionSeverityWarning, "")
				return reconcile.Result{}, err
			}
		}
		ctx.Logger.Info("Waiting for HAProxyLoadBalancer to be deleted", "count", len(haproxyLoadbalancers.Items))
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
	conditions.MarkFalse(ctx.VSphereCluster, infrav1.LoadBalancerAvailableCondition, clusterv1.DeletedReason, clusterv1.ConditionSeverityInfo, "")

	// Remove finalizer on Identity Secret
	if identity.IsSecretIdentity(ctx.VSphereCluster) {
		secret := &apiv1.Secret{}
		secretKey := client.ObjectKey{
			Namespace: ctx.VSphereCluster.Namespace,
			Name:      ctx.VSphereCluster.Spec.IdentityRef.Name,
		}
		err := ctx.Client.Get(ctx, secretKey, secret)
		if err != nil {
			if apierrors.IsNotFound(err) {
				ctrlutil.RemoveFinalizer(ctx.VSphereCluster, infrav1.ClusterFinalizer)
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}
		r.Logger.Info(fmt.Sprintf("Removing finalizer form Secret %s/%s", secret.Namespace, secret.Name))
		ctrlutil.RemoveFinalizer(secret, infrav1.SecretIdentitySetFinalizer)
		if err := ctx.Client.Update(ctx, secret); err != nil {
			return reconcile.Result{}, err
		}
		if err := ctx.Client.Delete(ctx, secret); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Cluster is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(ctx.VSphereCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

func (r clusterReconciler) reconcileNormal(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("Reconciling VSphereCluster")

	// If the VSphereCluster doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.VSphereCluster, infrav1.ClusterFinalizer)

	if err := r.reconcileIdentitySecret(ctx); err != nil {
		conditions.MarkFalse(ctx.VSphereCluster, infrav1.VCenterAvailableCondition, infrav1.VCenterUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, err
	}

	if cloudProviderConfigurationAvailable(ctx) {
		if err := r.reconcileVCenterConnectivity(ctx); err != nil {
			conditions.MarkFalse(ctx.VSphereCluster, infrav1.VCenterAvailableCondition, infrav1.VCenterUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
			return reconcile.Result{}, errors.Wrapf(err,
				"unexpected error while probing vcenter for %s", ctx)
		}
		conditions.MarkTrue(ctx.VSphereCluster, infrav1.VCenterAvailableCondition)
	}

	// Reconcile the VSphereCluster's load balancer.
	if ok, err := r.reconcileLoadBalancer(ctx); !ok {
		if err != nil {
			conditions.MarkFalse(ctx.VSphereCluster, infrav1.LoadBalancerAvailableCondition, infrav1.LoadBalancerProvisioningFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return reconcile.Result{}, errors.Wrapf(err,
				"unexpected error while reconciling load balancer for %s", ctx)
		}
		ctx.Logger.Info("load balancer is not reconciled")
		conditions.MarkFalse(ctx.VSphereCluster, infrav1.LoadBalancerAvailableCondition, infrav1.LoadBalancerProvisioningReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Reconcile the VSphereCluster resource's ready state.
	conditions.MarkTrue(ctx.VSphereCluster, infrav1.LoadBalancerAvailableCondition)
	ctx.VSphereCluster.Status.Ready = true

	// Ensure the VSphereCluster is reconciled when the API server first comes online.
	// A reconcile event will only be triggered if the Cluster is not marked as
	// ControlPlaneInitialized.
	r.reconcileVSphereClusterWhenAPIServerIsOnline(ctx)
	if ctx.VSphereCluster.Spec.ControlPlaneEndpoint.IsZero() {
		ctx.Logger.Info("control plane endpoint is not reconciled")
		return reconcile.Result{}, nil
	}

	// If the cluster is deleted, that's mean that the workload cluster is being deleted and so the CCM/CSI instances
	if !ctx.Cluster.DeletionTimestamp.IsZero() {
		conditions.MarkFalse(ctx.VSphereCluster, infrav1.CCMAvailableCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
		conditions.MarkFalse(ctx.VSphereCluster, infrav1.CSIAvailableCondition, clusterv1.DeletingReason, clusterv1.ConditionSeverityInfo, "")
		return reconcile.Result{}, nil
	}

	// Wait until the API server is online and accessible.
	if !r.isAPIServerOnline(ctx) {
		return reconcile.Result{}, nil
	}
	if cloudProviderConfigurationAvailable(ctx) {
		// Create the cloud config secret for the target cluster.
		if err := r.reconcileCloudConfigSecret(ctx); err != nil {
			conditions.MarkFalse(ctx.VSphereCluster, infrav1.CCMAvailableCondition, infrav1.CCMProvisioningFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return reconcile.Result{}, errors.Wrapf(err,
				"failed to reconcile cloud config secret for VSphereCluster %s/%s",
				ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
		}

		// Create the external cloud provider addons
		if err := r.reconcileCloudProvider(ctx); err != nil {
			conditions.MarkFalse(ctx.VSphereCluster, infrav1.CCMAvailableCondition, infrav1.CCMProvisioningFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return reconcile.Result{}, errors.Wrapf(err,
				"failed to reconcile cloud provider for VSphereCluster %s/%s",
				ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
		}

		conditions.MarkTrue(ctx.VSphereCluster, infrav1.CCMAvailableCondition)

		// Create the vSphere CSI Driver addons
		if err := r.reconcileStorageProvider(ctx); err != nil {
			conditions.MarkFalse(ctx.VSphereCluster, infrav1.CSIAvailableCondition, infrav1.CSIProvisioningFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return reconcile.Result{}, errors.Wrapf(err,
				"failed to reconcile CSI Driver for VSphereCluster %s/%s",
				ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
		}
	}

	conditions.MarkTrue(ctx.VSphereCluster, infrav1.CSIAvailableCondition)

	return reconcile.Result{}, nil
}

func cloudProviderConfigurationAvailable(ctx *context.ClusterContext) bool {
	return !reflect.DeepEqual(ctx.VSphereCluster.Spec.CloudProviderConfiguration, infrav1.CPIConfig{})
}

func (r clusterReconciler) reconcileIdentitySecret(ctx *context.ClusterContext) error {
	vsphereCluster := ctx.VSphereCluster
	if identity.IsSecretIdentity(vsphereCluster) {
		secret := &apiv1.Secret{}
		secretKey := client.ObjectKey{
			Namespace: vsphereCluster.Namespace,
			Name:      vsphereCluster.Spec.IdentityRef.Name,
		}
		err := ctx.Client.Get(ctx, secretKey, secret)
		if err != nil {
			return err
		}

		// check if cluster is already an owner
		if !clusterutilv1.IsOwnedByObject(secret, vsphereCluster) {
			if len(secret.GetOwnerReferences()) > 0 {
				return fmt.Errorf("another cluster has set the OwnerRef for secret: %s/%s", secret.Namespace, secret.Name)
			}

			secret.SetOwnerReferences([]metav1.OwnerReference{{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       vsphereCluster.Kind,
				Name:       vsphereCluster.Name,
				UID:        vsphereCluster.UID,
			}})
		}

		if !ctrlutil.ContainsFinalizer(secret, infrav1.SecretIdentitySetFinalizer) {
			ctrlutil.AddFinalizer(secret, infrav1.SecretIdentitySetFinalizer)
		}
		err = r.Client.Update(ctx, secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r clusterReconciler) reconcileVCenterConnectivity(ctx *context.ClusterContext) error {
	params := session.NewParams().
		WithServer(ctx.VSphereCluster.Spec.Server).
		WithDatacenter(ctx.VSphereCluster.Spec.CloudProviderConfiguration.Workspace.Datacenter).
		WithThumbprint(ctx.VSphereCluster.Spec.Thumbprint).
		WithFeatures(session.Feature{
			EnableKeepAlive:   r.EnableKeepAlive,
			KeepAliveDuration: r.KeepAliveDuration,
		})

	if ctx.VSphereCluster.Spec.IdentityRef != nil {
		creds, err := identity.GetCredentials(ctx, r.Client, ctx.VSphereCluster, r.Namespace)
		if err != nil {
			return err
		}

		params = params.WithUserInfo(creds.Username, creds.Password)
		_, err = session.GetOrCreate(ctx, params)
		return err
	}

	params = params.WithUserInfo(ctx.Username, ctx.Password)
	_, err := session.GetOrCreate(ctx,
		params)
	return err
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

var (
	// apiServerTriggers is used to prevent multiple goroutines for a single
	// Cluster that poll to see if the target API server is online.
	apiServerTriggers   = map[types.UID]struct{}{}
	apiServerTriggersMu sync.Mutex
)

func (r clusterReconciler) reconcileVSphereClusterWhenAPIServerIsOnline(ctx *context.ClusterContext) {
	if conditions.IsTrue(ctx.Cluster, clusterv1.ControlPlaneInitializedCondition) {
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
		if _, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
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
	return conditions.IsTrue(ctx.Cluster, clusterv1.ControlPlaneInitializedCondition)
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
	if _, err := targetClusterClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(ctx, serviceAccount, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	cloudConfigData, err := ctx.VSphereCluster.Spec.CloudProviderConfiguration.MarshalINI()
	if err != nil {
		return err
	}

	cloudConfigMap := cloudprovider.CloudControllerManagerConfigMap(string(cloudConfigData))
	if _, err := targetClusterClient.CoreV1().ConfigMaps(cloudConfigMap.Namespace).Create(ctx, cloudConfigMap, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonSet := cloudprovider.CloudControllerManagerDaemonSet(controllerImage, cloudproviderConfig.MarshalCloudProviderArgs())
	if _, err := targetClusterClient.AppsV1().DaemonSets(daemonSet.Namespace).Create(ctx, daemonSet, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	service := cloudprovider.CloudControllerManagerService()
	if _, err := targetClusterClient.CoreV1().Services(daemonSet.Namespace).Create(ctx, service, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRole := cloudprovider.CloudControllerManagerClusterRole()
	if _, err := targetClusterClient.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRoleBinding := cloudprovider.CloudControllerManagerClusterRoleBinding()
	if _, err := targetClusterClient.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	roleBinding := cloudprovider.CloudControllerManagerRoleBinding()
	if _, err := targetClusterClient.RbacV1().RoleBindings(roleBinding.Namespace).Create(ctx, roleBinding, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
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
	if _, err := targetClusterClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(ctx, serviceAccount, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	configMap := cloudprovider.CSIFeatureStatesConfigMap()
	if _, err := targetClusterClient.CoreV1().ConfigMaps(configMap.Namespace).Create(ctx, configMap, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRole := cloudprovider.CSIControllerClusterRole()
	if _, err := targetClusterClient.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRoleBinding := cloudprovider.CSIControllerClusterRoleBinding()
	if _, err := targetClusterClient.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// we have to marshal a separate INI file for CSI since it does not
	// support Secrets for vCenter credentials yet.
	cloudConfig, err := cloudprovider.ConfigForCSI(*ctx.VSphereCluster, *ctx.Cluster, ctx.Username, ctx.Password).MarshalINI()
	if err != nil {
		return err
	}

	cloudConfigSecret := cloudprovider.CSICloudConfigSecret(string(cloudConfig))
	if _, err := targetClusterClient.CoreV1().Secrets(cloudConfigSecret.Namespace).Create(ctx, cloudConfigSecret, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	csiDriver := cloudprovider.CSIDriver()
	if _, err := targetClusterClient.StorageV1beta1().CSIDrivers().Create(ctx, csiDriver, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonSet := cloudprovider.VSphereCSINodeDaemonSet(ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Storage)
	if _, err := targetClusterClient.AppsV1().DaemonSets(daemonSet.Namespace).Create(ctx, daemonSet, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// check if CSI is already deployed
	_, err = targetClusterClient.AppsV1().StatefulSets(cloudprovider.CSINamespace).Get(ctx, cloudprovider.CSIControllerName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	// this is a new cluster deploy the latest csi
	if apierrors.IsNotFound(err) {
		deployment := cloudprovider.CSIControllerDeployment(ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Storage)
		if _, err := targetClusterClient.AppsV1().Deployments(deployment.Namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
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
	if _, err := targetClusterClient.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
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
func (r clusterReconciler) controlPlaneMachineToCluster(o client.Object) []ctrl.Request {
	vsphereMachine, ok := o.(*infrav1.VSphereMachine)
	if !ok {
		r.Logger.Error(nil, fmt.Sprintf("expected a VSphereMachine but got a %T", o))
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

	if conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition) {
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
func (r clusterReconciler) loadBalancerToCluster(o client.Object) []ctrl.Request {
	obj, ok := o.(metav1.Object)
	if !ok {
		r.Logger.Error(nil, fmt.Sprintf("expected an metav1.Object but got a %T", o))
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
