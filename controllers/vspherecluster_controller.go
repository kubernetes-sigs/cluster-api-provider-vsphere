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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

var (
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
	clusterToInfraFn := clusterutilv1.ClusterToInfrastructureMapFunc(clusterControlledTypeGVK)
	return ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(clusterControlledType).
		// Watch the CAPI resource that owns this infrastructure resource.
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				requests := clusterToInfraFn(o)
				if requests == nil {
					return nil
				}

				c := &infrav1.VSphereCluster{}
				if err := reconciler.Client.Get(ctx, requests[0].NamespacedName, c); err != nil {
					reconciler.Logger.V(4).Error(err, "Failed to get VSphereCluster")
					return nil
				}

				if annotations.IsExternallyManaged(c) {
					reconciler.Logger.V(4).Info("VSphereCluster is externally managed, skipping mapping.")
					return nil
				}
				return requests
			}),
		).

		// Watch the infrastructure machine resources that belong to the control
		// plane. This controller needs to reconcile the infrastructure cluster
		// once a control plane machine has an IP address.
		Watches(
			&source.Kind{Type: &infrav1.VSphereMachine{}},
			handler.EnqueueRequestsFromMapFunc(reconciler.controlPlaneMachineToCluster),
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
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(reconciler.Logger)).
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

	if len(vsphereMachines) > 0 {
		ctx.Logger.Info("Waiting for VSphereMachines to be deleted", "count", len(vsphereMachines))
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

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

	if err := r.reconcileVCenterConnectivity(ctx); err != nil {
		conditions.MarkFalse(ctx.VSphereCluster, infrav1.VCenterAvailableCondition, infrav1.VCenterUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, errors.Wrapf(err,
			"unexpected error while probing vcenter for %s", ctx)
	}
	conditions.MarkTrue(ctx.VSphereCluster, infrav1.VCenterAvailableCondition)
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

	conditions.MarkTrue(ctx.VSphereCluster, infrav1.CSIAvailableCondition)

	return reconcile.Result{}, nil
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
