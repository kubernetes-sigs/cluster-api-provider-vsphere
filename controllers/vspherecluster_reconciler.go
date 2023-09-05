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

//nolint:nestif
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
const legacyIdentityFinalizer string = "identity/infrastructure.cluster.x-k8s.io"

type clusterReconciler struct {
	*capvcontext.ControllerContext

	clusterModuleReconciler Reconciler
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r clusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Get the VSphereCluster resource for this request.
	vsphereCluster := &infrav1.VSphereCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, vsphereCluster); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.V(4).Info("VSphereCluster not found, won't reconcile", "key", req.NamespacedName)
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
	clusterContext := &capvcontext.ClusterContext{
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

	if err := setOwnerRefsOnVsphereMachines(ctx, clusterContext); err != nil {
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

func (r clusterReconciler) reconcileDelete(ctx context.Context, clusterCtx *capvcontext.ClusterContext) (reconcile.Result, error) {
	clusterCtx.Logger.Info("Reconciling VSphereCluster delete")

	vsphereMachines, err := infrautilv1.GetVSphereMachinesInCluster(ctx, clusterCtx.Client, clusterCtx.Cluster.Namespace, clusterCtx.Cluster.Name)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"unable to list VSphereMachines part of VSphereCluster %s/%s", clusterCtx.VSphereCluster.Namespace, clusterCtx.VSphereCluster.Name)
	}

	machineDeletionCount := 0
	var deletionErrors []error
	for _, vsphereMachine := range vsphereMachines {
		// If the VSphereMachine is not owned by the CAPI Machine object because the machine object was deleted
		// before setting the owner references, then proceed with the deletion of the VSphereMachine object.
		// This is required until CAPI has a solution for https://github.com/kubernetes-sigs/cluster-api/issues/5483
		if !clusterutilv1.IsOwnedByObject(vsphereMachine, clusterCtx.VSphereCluster) || len(vsphereMachine.OwnerReferences) != 1 {
			continue
		}
		machineDeletionCount++
		// Remove the finalizer since VM creation wouldn't proceed
		r.Logger.Info("Removing finalizer from VSphereMachine", "namespace", vsphereMachine.Namespace, "name", vsphereMachine.Name)
		ctrlutil.RemoveFinalizer(vsphereMachine, infrav1.MachineFinalizer)
		if err := r.Client.Update(ctx, vsphereMachine); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.Client.Delete(ctx, vsphereMachine); err != nil && !apierrors.IsNotFound(err) {
			clusterCtx.Logger.Error(err, "Failed to delete for VSphereMachine", "namespace", vsphereMachine.Namespace, "name", vsphereMachine.Name)
			deletionErrors = append(deletionErrors, err)
		}
	}
	if len(deletionErrors) > 0 {
		return reconcile.Result{}, kerrors.NewAggregate(deletionErrors)
	}

	if len(vsphereMachines)-machineDeletionCount > 0 {
		clusterCtx.Logger.Info("Waiting for VSphereMachines to be deleted", "count", len(vsphereMachines)-machineDeletionCount)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// The cluster module info needs to be reconciled before the secret deletion
	// since it needs access to the vCenter instance to be able to perform LCM operations
	// on the cluster modules.
	affinityReconcileResult, err := r.reconcileClusterModules(clusterCtx)
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
		err := clusterCtx.Client.Get(ctx, secretKey, secret)
		if err != nil {
			if apierrors.IsNotFound(err) {
				ctrlutil.RemoveFinalizer(clusterCtx.VSphereCluster, infrav1.ClusterFinalizer)
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}
		r.Logger.Info(fmt.Sprintf("Removing finalizer from Secret %s/%s having finalizers %v", secret.Namespace, secret.Name, secret.Finalizers))
		ctrlutil.RemoveFinalizer(secret, infrav1.SecretIdentitySetFinalizer)

		// Check if the old finalizer(from v0.7) is present, if yes, delete it
		// For more context, please refer: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues/1482
		if ctrlutil.ContainsFinalizer(secret, legacyIdentityFinalizer) {
			ctrlutil.RemoveFinalizer(secret, legacyIdentityFinalizer)
		}
		if err := clusterCtx.Client.Update(ctx, secret); err != nil {
			return reconcile.Result{}, err
		}
		if err := clusterCtx.Client.Delete(ctx, secret); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Cluster is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(clusterCtx.VSphereCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

func (r clusterReconciler) reconcileNormal(ctx context.Context, clusterCtx *capvcontext.ClusterContext) (reconcile.Result, error) {
	clusterCtx.Logger.Info("Reconciling VSphereCluster")

	ok, err := r.reconcileDeploymentZones(ctx, clusterCtx)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !ok {
		clusterCtx.Logger.Info("waiting for failure domains to be reconciled")
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
		clusterCtx.Logger.Error(err, "could not reconcile vCenter version")
	}

	affinityReconcileResult, err := r.reconcileClusterModules(clusterCtx)
	if err != nil {
		conditions.MarkFalse(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition, infrav1.ClusterModuleSetupFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
		return affinityReconcileResult, err
	}

	clusterCtx.VSphereCluster.Status.Ready = true

	// Ensure the VSphereCluster is reconciled when the API server first comes online.
	// A reconcile event will only be triggered if the Cluster is not marked as
	// ControlPlaneInitialized.
	r.reconcileVSphereClusterWhenAPIServerIsOnline(clusterCtx)
	if clusterCtx.VSphereCluster.Spec.ControlPlaneEndpoint.IsZero() {
		clusterCtx.Logger.Info("control plane endpoint is not reconciled")
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

func (r clusterReconciler) reconcileIdentitySecret(ctx context.Context, clusterCtx *capvcontext.ClusterContext) error {
	vsphereCluster := clusterCtx.VSphereCluster
	if identity.IsSecretIdentity(vsphereCluster) {
		secret := &corev1.Secret{}
		secretKey := client.ObjectKey{
			Namespace: vsphereCluster.Namespace,
			Name:      vsphereCluster.Spec.IdentityRef.Name,
		}
		err := clusterCtx.Client.Get(ctx, secretKey, secret)
		if err != nil {
			return err
		}

		// check if cluster is already an owner
		if !clusterutilv1.IsOwnedByObject(secret, vsphereCluster) {
			ownerReferences := secret.GetOwnerReferences()
			if identity.IsOwnedByIdentityOrCluster(ownerReferences) {
				return fmt.Errorf("another cluster has set the OwnerRef for secret: %s/%s", secret.Namespace, secret.Name)
			}
			ownerReferences = append(ownerReferences, metav1.OwnerReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       vsphereCluster.Kind,
				Name:       vsphereCluster.Name,
				UID:        vsphereCluster.UID,
			})
			secret.SetOwnerReferences(ownerReferences)
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

func (r clusterReconciler) reconcileVCenterConnectivity(ctx context.Context, clusterCtx *capvcontext.ClusterContext) (*session.Session, error) {
	params := session.NewParams().
		WithServer(clusterCtx.VSphereCluster.Spec.Server).
		WithThumbprint(clusterCtx.VSphereCluster.Spec.Thumbprint).
		WithFeatures(session.Feature{
			EnableKeepAlive:   r.EnableKeepAlive,
			KeepAliveDuration: r.KeepAliveDuration,
		})

	if clusterCtx.VSphereCluster.Spec.IdentityRef != nil {
		creds, err := identity.GetCredentials(ctx, r.Client, clusterCtx.VSphereCluster, r.Namespace)
		if err != nil {
			return nil, err
		}

		params = params.WithUserInfo(creds.Username, creds.Password)
		return session.GetOrCreate(ctx, params)
	}

	params = params.WithUserInfo(clusterCtx.Username, clusterCtx.Password)
	return session.GetOrCreate(clusterCtx, params)
}

func (r clusterReconciler) reconcileVCenterVersion(clusterCtx *capvcontext.ClusterContext, s *session.Session) error {
	version, err := s.GetVersion()
	if err != nil {
		return err
	}
	clusterCtx.VSphereCluster.Status.VCenterVersion = version
	return nil
}

func (r clusterReconciler) reconcileDeploymentZones(ctx context.Context, clusterCtx *capvcontext.ClusterContext) (bool, error) {
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

func (r clusterReconciler) reconcileVSphereClusterWhenAPIServerIsOnline(clusterCtx *capvcontext.ClusterContext) {
	if conditions.IsTrue(clusterCtx.Cluster, clusterv1.ControlPlaneInitializedCondition) {
		clusterCtx.Logger.Info("skipping reconcile when API server is online",
			"reason", "controlPlaneInitialized")
		return
	}
	apiServerTriggersMu.Lock()
	defer apiServerTriggersMu.Unlock()
	if _, ok := apiServerTriggers[clusterCtx.Cluster.UID]; ok {
		clusterCtx.Logger.Info("skipping reconcile when API server is online",
			"reason", "alreadyPolling")
		return
	}
	apiServerTriggers[clusterCtx.Cluster.UID] = struct{}{}
	go func() {
		ctx := context.Background()
		// Block until the target API server is online.
		clusterCtx.Logger.Info("start polling API server for online check")
		wait.PollUntilContextCancel(ctx, time.Second*1, true, func(context.Context) (bool, error) { return r.isAPIServerOnline(ctx, clusterCtx), nil }) //nolint:errcheck
		clusterCtx.Logger.Info("stop polling API server for online check")
		clusterCtx.Logger.Info("triggering GenericEvent", "reason", "api-server-online")
		eventChannel := clusterCtx.GetGenericEventChannelFor(clusterCtx.VSphereCluster.GetObjectKind().GroupVersionKind())
		eventChannel <- event.GenericEvent{
			Object: clusterCtx.VSphereCluster,
		}

		// Once the control plane has been marked as initialized it is safe to
		// remove the key from the map that prevents multiple goroutines from
		// polling the API server to see if it is online.
		clusterCtx.Logger.Info("start polling for control plane initialized")
		wait.PollUntilContextCancel(ctx, time.Second*1, true, func(context.Context) (bool, error) { return r.isControlPlaneInitialized(ctx, clusterCtx), nil }) //nolint:errcheck
		clusterCtx.Logger.Info("stop polling for control plane initialized")
		apiServerTriggersMu.Lock()
		delete(apiServerTriggers, clusterCtx.Cluster.UID)
		apiServerTriggersMu.Unlock()
	}()
}

func (r clusterReconciler) isAPIServerOnline(ctx context.Context, clusterCtx *capvcontext.ClusterContext) bool {
	if kubeClient, err := infrautilv1.NewKubeClient(ctx, clusterCtx.Client, clusterCtx.Cluster); err == nil {
		if _, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
			// The target cluster is online. To make sure the correct control
			// plane endpoint information is logged, it is necessary to fetch
			// an up-to-date Cluster resource. If this fails, then set the
			// control plane endpoint information to the values from the
			// VSphereCluster resource, as it must have the correct information
			// if the API server is online.
			cluster := &clusterv1.Cluster{}
			clusterKey := client.ObjectKey{Namespace: clusterCtx.Cluster.Namespace, Name: clusterCtx.Cluster.Name}
			if err := clusterCtx.Client.Get(ctx, clusterKey, cluster); err != nil {
				cluster = clusterCtx.Cluster.DeepCopy()
				cluster.Spec.ControlPlaneEndpoint.Host = clusterCtx.VSphereCluster.Spec.ControlPlaneEndpoint.Host
				cluster.Spec.ControlPlaneEndpoint.Port = clusterCtx.VSphereCluster.Spec.ControlPlaneEndpoint.Port
				clusterCtx.Logger.Error(err, "failed to get updated cluster object while checking if API server is online")
			}
			clusterCtx.Logger.Info(
				"API server is online",
				"controlPlaneEndpoint", cluster.Spec.ControlPlaneEndpoint.String())
			return true
		}
	}
	return false
}

func (r clusterReconciler) isControlPlaneInitialized(ctx context.Context, clusterCtx *capvcontext.ClusterContext) bool {
	cluster := &clusterv1.Cluster{}
	clusterKey := client.ObjectKey{Namespace: clusterCtx.Cluster.Namespace, Name: clusterCtx.Cluster.Name}
	if err := clusterCtx.Client.Get(ctx, clusterKey, cluster); err != nil {
		if !apierrors.IsNotFound(err) {
			clusterCtx.Logger.Error(err, "failed to get updated cluster object while checking if control plane is initialized")
			return false
		}
		clusterCtx.Logger.Info("exiting early because cluster no longer exists")
		return true
	}
	return conditions.IsTrue(clusterCtx.Cluster, clusterv1.ControlPlaneInitializedCondition)
}

func setOwnerRefsOnVsphereMachines(ctx context.Context, clusterCtx *capvcontext.ClusterContext) error {
	vsphereMachines, err := infrautilv1.GetVSphereMachinesInCluster(ctx, clusterCtx.Client, clusterCtx.Cluster.Namespace, clusterCtx.Cluster.Name)
	if err != nil {
		return errors.Wrapf(err,
			"unable to list VSphereMachines part of VSphereCluster %s/%s", clusterCtx.VSphereCluster.Namespace, clusterCtx.VSphereCluster.Name)
	}

	var patchErrors []error
	for _, vsphereMachine := range vsphereMachines {
		patchHelper, err := patch.NewHelper(vsphereMachine, clusterCtx.Client)
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

// TODO: zhanggbj: Add golang context after refactoring ClusterModule controller and reconciler.
func (r clusterReconciler) reconcileClusterModules(clusterCtx *capvcontext.ClusterContext) (reconcile.Result, error) {
	if feature.Gates.Enabled(feature.NodeAntiAffinity) {
		return r.clusterModuleReconciler.Reconcile(clusterCtx)
	}
	return reconcile.Result{}, nil
}

// controlPlaneMachineToCluster is a handler.ToRequestsFunc to be used
// to enqueue requests for reconciliation for VSphereCluster to update
// its status.apiEndpoints field.
func (r clusterReconciler) controlPlaneMachineToCluster(ctx context.Context, o client.Object) []ctrl.Request {
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
	cluster, err := clusterutilv1.GetClusterFromMetadata(ctx, r.Client, vsphereMachine.ObjectMeta)
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
	if err := r.Client.Get(ctx, vsphereClusterKey, vsphereCluster); err != nil {
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

func (r clusterReconciler) deploymentZoneToCluster(ctx context.Context, o client.Object) []ctrl.Request {
	var requests []ctrl.Request
	obj, ok := o.(*infrav1.VSphereDeploymentZone)
	if !ok {
		r.Logger.Error(nil, fmt.Sprintf("expected an infrav1.VSphereDeploymentZone but got a %T", o))
		return nil
	}

	var clusterList infrav1.VSphereClusterList
	err := r.Client.List(ctx, &clusterList)
	if err != nil {
		r.Logger.Error(err, "unable to list clusters")
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
