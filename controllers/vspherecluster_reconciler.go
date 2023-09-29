/*
Copyright 2021 The Kubernetes Authors.

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

// Package controllers contains controllers for CAPV objects.
package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// legacyIdentityFinalizer is deprecated and should be used only while upgrading the cluster
// from v1alpha3(v.0.7).
//
// Deprecated: legacyIdentityFinalizer will be removed in a future release.
const legacyIdentityFinalizer string = "identity/infrastructure.cluster.x-k8s.io"

type clusterReconciler struct {
	ControllerManagerContext *capvcontext.ControllerManagerContext
	Client                   client.Client

	clusterModuleReconciler Reconciler
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *clusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Get the VSphereCluster resource for this request.
	vsphereCluster := &infrav1.VSphereCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, vsphereCluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(4).Info("VSphereCluster not found, won't reconcile")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the CAPI Cluster.
	cluster, err := clusterutilv1.GetOwnerCluster(ctx, r.Client, vsphereCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on VSphereCluster")
		return reconcile.Result{}, nil
	}
	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	if annotations.IsPaused(cluster, vsphereCluster) {
		log.V(4).Info("VSphereCluster %s/%s linked to a cluster that is paused")
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
	clusterContext := &capvcontext.ClusterContext{
		Cluster:        cluster,
		VSphereCluster: vsphereCluster,
		PatchHelper:    patchHelper,
	}

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		if err := clusterContext.Patch(ctx); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	if err := r.setOwnerRefsOnVsphereMachines(ctx, clusterContext); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to set owner refs on VSphereMachine objects")
	}

	// Handle deleted clusters
	if !vsphereCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterContext)
	}

	// If the VSphereCluster doesn't have our finalizer, add it.
	// Requeue immediately after adding finalizer to avoid the race condition between init and delete
	if !ctrlutil.ContainsFinalizer(vsphereCluster, infrav1.ClusterFinalizer) {
		ctrlutil.AddFinalizer(vsphereCluster, infrav1.ClusterFinalizer)
		return reconcile.Result{}, nil
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, clusterContext)
}

func (r *clusterReconciler) reconcileDelete(ctx context.Context, clusterCtx *capvcontext.ClusterContext) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling VSphereCluster delete")

	vsphereMachines, err := infrautilv1.GetVSphereMachinesInCluster(ctx, r.Client, clusterCtx.Cluster.Namespace, clusterCtx.Cluster.Name)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"unable to list VSphereMachines part of VSphereCluster %s/%s", clusterCtx.VSphereCluster.Namespace, clusterCtx.VSphereCluster.Name)
	}

	machineDeletionCount := 0
	var deletionErrors []error
	for _, vsphereMachine := range vsphereMachines {
		// Note: We have to use := here to not overwrite log & ctx outside the for loop.
		log := log.WithValues("VSphereMachine", klog.KObj(vsphereMachine))
		ctx := ctrl.LoggerInto(ctx, log)

		// If the VSphereMachine is not owned by the CAPI Machine object because the machine object was deleted
		// before setting the owner references, then proceed with the deletion of the VSphereMachine object.
		// This is required until CAPI has a solution for https://github.com/kubernetes-sigs/cluster-api/issues/5483
		if !clusterutilv1.IsOwnedByObject(vsphereMachine, clusterCtx.VSphereCluster) || len(vsphereMachine.OwnerReferences) != 1 {
			continue
		}
		machineDeletionCount++
		// Remove the finalizer since VM creation wouldn't proceed
		log.Info("Removing finalizer from VSphereMachine")
		ctrlutil.RemoveFinalizer(vsphereMachine, infrav1.MachineFinalizer)
		if err := r.Client.Update(ctx, vsphereMachine); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.Client.Delete(ctx, vsphereMachine); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to delete for VSphereMachine")
			deletionErrors = append(deletionErrors, err)
		}
	}
	if len(deletionErrors) > 0 {
		return reconcile.Result{}, kerrors.NewAggregate(deletionErrors)
	}

	if len(vsphereMachines)-machineDeletionCount > 0 {
		log.Info("Waiting for VSphereMachines to be deleted", "count", len(vsphereMachines)-machineDeletionCount)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// The cluster module info needs to be reconciled before the secret deletion
	// since it needs access to the vCenter instance to be able to perform LCM operations
	// on the cluster modules.
	affinityReconcileResult, err := r.reconcileClusterModules(ctx, clusterCtx)
	if err != nil {
		return affinityReconcileResult, err
	}

	// Remove finalizer on Identity Secret
	if identity.IsSecretIdentity(clusterCtx.VSphereCluster) {
		secret := &corev1.Secret{}
		secretKey := client.ObjectKey{
			Namespace: clusterCtx.VSphereCluster.Namespace,
			Name:      clusterCtx.VSphereCluster.Spec.IdentityRef.Name,
		}
		if err := r.Client.Get(ctx, secretKey, secret); err != nil {
			if apierrors.IsNotFound(err) {
				ctrlutil.RemoveFinalizer(clusterCtx.VSphereCluster, infrav1.ClusterFinalizer)
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}
		log.Info(fmt.Sprintf("Removing finalizer from Secret %s/%s having finalizers %v", secret.Namespace, secret.Name, secret.Finalizers))
		ctrlutil.RemoveFinalizer(secret, infrav1.SecretIdentitySetFinalizer)

		// Check if the old finalizer(from v0.7) is present, if yes, delete it
		// For more context, please refer: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues/1482
		if ctrlutil.ContainsFinalizer(secret, legacyIdentityFinalizer) {
			ctrlutil.RemoveFinalizer(secret, legacyIdentityFinalizer)
		}
		if err := r.Client.Update(ctx, secret); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.Client.Delete(ctx, secret); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Cluster is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(clusterCtx.VSphereCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

func (r *clusterReconciler) reconcileNormal(ctx context.Context, clusterCtx *capvcontext.ClusterContext) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling VSphereCluster")

	ok, err := r.reconcileDeploymentZones(ctx, clusterCtx)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !ok {
		log.Info("Waiting for failure domains to be reconciled")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if err := r.reconcileIdentitySecret(ctx, clusterCtx); err != nil {
		conditions.MarkFalse(clusterCtx.VSphereCluster, infrav1.VCenterAvailableCondition, infrav1.VCenterUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, err
	}

	vcenterSession, err := r.reconcileVCenterConnectivity(ctx, clusterCtx)
	if err != nil {
		conditions.MarkFalse(clusterCtx.VSphereCluster, infrav1.VCenterAvailableCondition, infrav1.VCenterUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, errors.Wrapf(err,
			"unexpected error while probing vcenter for %s", clusterCtx)
	}
	conditions.MarkTrue(clusterCtx.VSphereCluster, infrav1.VCenterAvailableCondition)

	err = r.reconcileVCenterVersion(clusterCtx, vcenterSession)
	if err != nil || clusterCtx.VSphereCluster.Status.VCenterVersion == "" {
		conditions.MarkFalse(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition, infrav1.MissingVCenterVersionReason, clusterv1.ConditionSeverityWarning, "vCenter API version not set")
		log.Error(err, "could not reconcile vCenter version")
	}

	affinityReconcileResult, err := r.reconcileClusterModules(ctx, clusterCtx)
	if err != nil {
		conditions.MarkFalse(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition, infrav1.ClusterModuleSetupFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
		return affinityReconcileResult, err
	}

	clusterCtx.VSphereCluster.Status.Ready = true

	// Ensure the VSphereCluster is reconciled when the API server first comes online.
	// A reconcile event will only be triggered if the Cluster is not marked as
	// ControlPlaneInitialized.
	r.reconcileVSphereClusterWhenAPIServerIsOnline(ctx, clusterCtx)
	if clusterCtx.VSphereCluster.Spec.ControlPlaneEndpoint.IsZero() {
		log.Info("control plane endpoint is not reconciled")
		return reconcile.Result{}, nil
	}

	// If the cluster is deleted, that's mean that the workload cluster is being deleted and so the CCM/CSI instances
	if !clusterCtx.Cluster.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	// Wait until the API server is online and accessible.
	if !r.isAPIServerOnline(ctx, clusterCtx) {
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func (r *clusterReconciler) reconcileIdentitySecret(ctx context.Context, clusterCtx *capvcontext.ClusterContext) error {
	vsphereCluster := clusterCtx.VSphereCluster
	if !identity.IsSecretIdentity(vsphereCluster) {
		return nil
	}
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: vsphereCluster.Namespace,
		Name:      vsphereCluster.Spec.IdentityRef.Name,
	}
	err := r.Client.Get(ctx, secretKey, secret)
	if err != nil {
		return err
	}

	// If a different VSphereCluster is an owner return an error.
	if !clusterutilv1.IsOwnedByObject(secret, vsphereCluster) && identity.IsOwnedByIdentityOrCluster(secret.GetOwnerReferences()) {
		return fmt.Errorf("another cluster has set the OwnerRef for secret: %s/%s", secret.Namespace, secret.Name)
	}

	helper, err := patch.NewHelper(secret, r.Client)
	if err != nil {
		return err
	}

	// Ensure the VSphereCluster is an owner and that the APIVersion is up to date.
	secret.SetOwnerReferences(clusterutilv1.EnsureOwnerRef(secret.GetOwnerReferences(),
		metav1.OwnerReference{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       vsphereCluster.Kind,
			Name:       vsphereCluster.Name,
			UID:        vsphereCluster.UID,
		},
	))

	// Ensure the finalizer is added.
	if !ctrlutil.ContainsFinalizer(secret, infrav1.SecretIdentitySetFinalizer) {
		ctrlutil.AddFinalizer(secret, infrav1.SecretIdentitySetFinalizer)
	}
	err = helper.Patch(ctx, secret)
	if err != nil {
		return err
	}

	return nil
}

func (r *clusterReconciler) reconcileVCenterConnectivity(ctx context.Context, clusterCtx *capvcontext.ClusterContext) (*session.Session, error) {
	params := session.NewParams().
		WithServer(clusterCtx.VSphereCluster.Spec.Server).
		WithThumbprint(clusterCtx.VSphereCluster.Spec.Thumbprint).
		WithFeatures(session.Feature{
			EnableKeepAlive:   r.ControllerManagerContext.EnableKeepAlive,
			KeepAliveDuration: r.ControllerManagerContext.KeepAliveDuration,
		})

	if clusterCtx.VSphereCluster.Spec.IdentityRef != nil {
		creds, err := identity.GetCredentials(ctx, r.Client, clusterCtx.VSphereCluster, r.ControllerManagerContext.Namespace)
		if err != nil {
			return nil, err
		}

		params = params.WithUserInfo(creds.Username, creds.Password)
		return session.GetOrCreate(ctx, params)
	}

	params = params.WithUserInfo(r.ControllerManagerContext.Username, r.ControllerManagerContext.Password)
	return session.GetOrCreate(ctx, params)
}

func (r *clusterReconciler) reconcileVCenterVersion(clusterCtx *capvcontext.ClusterContext, s *session.Session) error {
	version, err := s.GetVersion()
	if err != nil {
		return err
	}
	clusterCtx.VSphereCluster.Status.VCenterVersion = version
	return nil
}

func (r *clusterReconciler) reconcileDeploymentZones(ctx context.Context, clusterCtx *capvcontext.ClusterContext) (bool, error) {
	// If there is no failure domain selector, we should simply skip it
	if clusterCtx.VSphereCluster.Spec.FailureDomainSelector == nil {
		return true, nil
	}

	var opts client.ListOptions
	var err error
	opts.LabelSelector, err = metav1.LabelSelectorAsSelector(clusterCtx.VSphereCluster.Spec.FailureDomainSelector)
	if err != nil {
		return false, errors.Wrapf(err, "zone label selector is misconfigured")
	}

	var deploymentZoneList infrav1.VSphereDeploymentZoneList
	err = r.Client.List(ctx, &deploymentZoneList, &opts)
	if err != nil {
		return false, errors.Wrap(err, "unable to list deployment zones")
	}

	readyNotReported, notReady := 0, 0
	failureDomains := clusterv1.FailureDomains{}
	for _, zone := range deploymentZoneList.Items {
		if zone.Spec.Server != clusterCtx.VSphereCluster.Spec.Server {
			continue
		}

		if zone.Status.Ready == nil {
			readyNotReported++
			failureDomains[zone.Name] = clusterv1.FailureDomainSpec{
				ControlPlane: pointer.BoolDeref(zone.Spec.ControlPlane, true),
			}
			continue
		}

		if *zone.Status.Ready {
			failureDomains[zone.Name] = clusterv1.FailureDomainSpec{
				ControlPlane: pointer.BoolDeref(zone.Spec.ControlPlane, true),
			}
			continue
		}
		notReady++
	}

	clusterCtx.VSphereCluster.Status.FailureDomains = failureDomains
	if readyNotReported > 0 {
		conditions.MarkFalse(clusterCtx.VSphereCluster, infrav1.FailureDomainsAvailableCondition, infrav1.WaitingForFailureDomainStatusReason, clusterv1.ConditionSeverityInfo, "waiting for failure domains to report ready status")
		return false, nil
	}

	if len(failureDomains) > 0 {
		if notReady > 0 {
			conditions.MarkFalse(clusterCtx.VSphereCluster, infrav1.FailureDomainsAvailableCondition, infrav1.FailureDomainsSkippedReason, clusterv1.ConditionSeverityInfo, "one or more failure domains are not ready")
		} else {
			conditions.MarkTrue(clusterCtx.VSphereCluster, infrav1.FailureDomainsAvailableCondition)
		}
	} else {
		// Remove the condition if failure domains do not exist
		conditions.Delete(clusterCtx.VSphereCluster, infrav1.FailureDomainsAvailableCondition)
	}
	return true, nil
}

var (
	// apiServerTriggers is used to prevent multiple goroutines for a single
	// Cluster that poll to see if the target API server is online.
	apiServerTriggers   = map[types.UID]struct{}{}
	apiServerTriggersMu sync.Mutex
)

func (r *clusterReconciler) reconcileVSphereClusterWhenAPIServerIsOnline(ctx context.Context, clusterCtx *capvcontext.ClusterContext) {
	log := ctrl.LoggerFrom(ctx)

	if conditions.IsTrue(clusterCtx.Cluster, clusterv1.ControlPlaneInitializedCondition) {
		log.Info("Skipping reconcile when API server is online",
			"reason", "controlPlaneInitialized")
		return
	}
	apiServerTriggersMu.Lock()
	defer apiServerTriggersMu.Unlock()
	if _, ok := apiServerTriggers[clusterCtx.Cluster.UID]; ok {
		log.Info("Skipping reconcile when API server is online",
			"reason", "alreadyPolling")
		return
	}
	apiServerTriggers[clusterCtx.Cluster.UID] = struct{}{}
	go func() {
		// Note: we have to use a new context here so the ctx in this go routine is not canceled
		// when the reconcile returns.
		ctx := ctrl.LoggerInto(context.Background(), log)

		// Block until the target API server is online.
		log.Info("Start polling API server for online check")
		// Ignore the error as the passed function never returns one.
		_ = wait.PollUntilContextCancel(ctx, time.Second*1, true, func(context.Context) (bool, error) { return r.isAPIServerOnline(ctx, clusterCtx), nil })
		log.Info("Stop polling API server for online check")
		log.Info("Triggering GenericEvent", "reason", "api-server-online")
		eventChannel := r.ControllerManagerContext.GetGenericEventChannelFor(clusterCtx.VSphereCluster.GetObjectKind().GroupVersionKind())
		eventChannel <- event.GenericEvent{
			Object: clusterCtx.VSphereCluster,
		}

		// Once the control plane has been marked as initialized it is safe to
		// remove the key from the map that prevents multiple goroutines from
		// polling the API server to see if it is online.
		log.Info("Start polling for control plane initialized")
		// Ignore the error as the passed function never returns one.
		_ = wait.PollUntilContextCancel(ctx, time.Second*1, true, func(context.Context) (bool, error) { return r.isControlPlaneInitialized(ctx, clusterCtx), nil })
		log.Info("Stop polling for control plane initialized")
		apiServerTriggersMu.Lock()
		delete(apiServerTriggers, clusterCtx.Cluster.UID)
		apiServerTriggersMu.Unlock()
	}()
}

func (r *clusterReconciler) isAPIServerOnline(ctx context.Context, clusterCtx *capvcontext.ClusterContext) bool {
	log := ctrl.LoggerFrom(ctx)

	if kubeClient, err := infrautilv1.NewKubeClient(ctx, r.Client, clusterCtx.Cluster); err == nil {
		if _, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
			// The target cluster is online. To make sure the correct control
			// plane endpoint information is logged, it is necessary to fetch
			// an up-to-date Cluster resource. If this fails, then set the
			// control plane endpoint information to the values from the
			// VSphereCluster resource, as it must have the correct information
			// if the API server is online.
			cluster := &clusterv1.Cluster{}
			clusterKey := client.ObjectKey{Namespace: clusterCtx.Cluster.Namespace, Name: clusterCtx.Cluster.Name}
			if err := r.Client.Get(ctx, clusterKey, cluster); err != nil {
				cluster = clusterCtx.Cluster.DeepCopy()
				cluster.Spec.ControlPlaneEndpoint.Host = clusterCtx.VSphereCluster.Spec.ControlPlaneEndpoint.Host
				cluster.Spec.ControlPlaneEndpoint.Port = clusterCtx.VSphereCluster.Spec.ControlPlaneEndpoint.Port
				log.Error(err, "failed to get updated cluster object while checking if API server is online")
			}
			log.Info(
				"API server is online",
				"controlPlaneEndpoint", cluster.Spec.ControlPlaneEndpoint.String())
			return true
		}
	}
	return false
}

func (r *clusterReconciler) isControlPlaneInitialized(ctx context.Context, clusterCtx *capvcontext.ClusterContext) bool {
	log := ctrl.LoggerFrom(ctx)

	cluster := &clusterv1.Cluster{}
	clusterKey := client.ObjectKey{Namespace: clusterCtx.Cluster.Namespace, Name: clusterCtx.Cluster.Name}
	if err := r.Client.Get(ctx, clusterKey, cluster); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get updated cluster object while checking if control plane is initialized")
			return false
		}
		log.Info("Exiting early because cluster no longer exists")
		return true
	}
	return conditions.IsTrue(clusterCtx.Cluster, clusterv1.ControlPlaneInitializedCondition)
}

func (r *clusterReconciler) setOwnerRefsOnVsphereMachines(ctx context.Context, clusterCtx *capvcontext.ClusterContext) error {
	vsphereMachines, err := infrautilv1.GetVSphereMachinesInCluster(ctx, r.Client, clusterCtx.Cluster.Namespace, clusterCtx.Cluster.Name)
	if err != nil {
		return errors.Wrapf(err,
			"unable to list VSphereMachines part of VSphereCluster %s/%s", clusterCtx.VSphereCluster.Namespace, clusterCtx.VSphereCluster.Name)
	}

	var patchErrors []error
	for _, vsphereMachine := range vsphereMachines {
		patchHelper, err := patch.NewHelper(vsphereMachine, r.Client)
		if err != nil {
			patchErrors = append(patchErrors, err)
			continue
		}

		vsphereMachine.SetOwnerReferences(clusterutilv1.EnsureOwnerRef(
			vsphereMachine.OwnerReferences,
			metav1.OwnerReference{
				APIVersion: clusterCtx.VSphereCluster.APIVersion,
				Kind:       clusterCtx.VSphereCluster.Kind,
				Name:       clusterCtx.VSphereCluster.Name,
				UID:        clusterCtx.VSphereCluster.UID,
			}))

		if err := patchHelper.Patch(ctx, vsphereMachine); err != nil {
			patchErrors = append(patchErrors, err)
		}
	}
	return kerrors.NewAggregate(patchErrors)
}

func (r *clusterReconciler) reconcileClusterModules(ctx context.Context, clusterCtx *capvcontext.ClusterContext) (reconcile.Result, error) {
	if feature.Gates.Enabled(feature.NodeAntiAffinity) {
		return r.clusterModuleReconciler.Reconcile(ctx, clusterCtx)
	}
	return reconcile.Result{}, nil
}

// controlPlaneMachineToCluster is a handler.ToRequestsFunc to be used
// to enqueue requests for reconciliation for VSphereCluster to update
// its status.apiEndpoints field.
func (r *clusterReconciler) controlPlaneMachineToCluster(ctx context.Context, o client.Object) []ctrl.Request {
	log := ctrl.LoggerFrom(ctx)

	vsphereMachine, ok := o.(*infrav1.VSphereMachine)
	if !ok {
		log.Error(nil, fmt.Sprintf("expected a VSphereMachine but got a %T", o))
		return nil
	}
	log = log.WithValues("VSphereMachine", klog.KObj(vsphereMachine))
	ctx = ctrl.LoggerInto(ctx, log)

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
		log.Error(err, "Failed to get preferred IP address for VSphereMachine")
		return nil
	}

	// Fetch the CAPI Cluster.
	cluster, err := clusterutilv1.GetClusterFromMetadata(ctx, r.Client, vsphereMachine.ObjectMeta)
	if err != nil {
		log.Error(err, "VSphereMachine is missing cluster label or cluster does not exist")
		return nil
	}
	log = log.WithValues("Cluster", klog.KObj(cluster), "VSphereCluster", klog.KRef(cluster.Namespace, cluster.Spec.InfrastructureRef.Name))
	ctx = ctrl.LoggerInto(ctx, log)

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
	if err := r.Client.Get(ctx, vsphereClusterKey, vsphereCluster); err != nil {
		log.Error(err, "Failed to get VSphereCluster")
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

func (r *clusterReconciler) deploymentZoneToCluster(ctx context.Context, o client.Object) []ctrl.Request {
	log := ctrl.LoggerFrom(ctx)

	var requests []ctrl.Request
	obj, ok := o.(*infrav1.VSphereDeploymentZone)
	if !ok {
		log.Error(nil, fmt.Sprintf("expected an infrav1.VSphereDeploymentZone but got a %T", o))
		return nil
	}

	var clusterList infrav1.VSphereClusterList
	err := r.Client.List(ctx, &clusterList)
	if err != nil {
		log.Error(err, "Unable to list clusters")
		return requests
	}

	for _, cluster := range clusterList.Items {
		if obj.Spec.Server == cluster.Spec.Server {
			r := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				},
			}
			requests = append(requests, r)
		}
	}
	return requests
}
