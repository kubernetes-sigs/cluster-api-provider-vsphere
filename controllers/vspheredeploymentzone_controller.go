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

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheredeploymentzones,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheredeploymentzones/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspherefailuredomains,verbs=get;list;watch;create;update;patch;delete

// AddVSphereDeploymentZoneControllerToManager adds the VSphereDeploymentZone controller to the provided manager.
func AddVSphereDeploymentZoneControllerToManager(controllerCtx *capvcontext.ControllerManagerContext, mgr manager.Manager, options controller.Options) error {
	var (
		controlledType     = &infrav1.VSphereDeploymentZone{}
		controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()
		controlledTypeGVK  = infrav1.GroupVersion.WithKind(controlledTypeName)

		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(controlledTypeName))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", controllerCtx.Namespace, controllerCtx.Name, controllerNameShort)
	)

	// Build the controller context.
	controllerContext := &capvcontext.ControllerContext{
		ControllerManagerContext: controllerCtx,
		Name:                     controllerNameShort,
		Recorder:                 record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		Logger:                   controllerCtx.Logger.WithName(controllerNameShort),
	}
	reconciler := vsphereDeploymentZoneReconciler{ControllerContext: controllerContext}

	return ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledType).
		WithOptions(options).
		Watches(
			&infrav1.VSphereFailureDomain{},
			handler.EnqueueRequestsFromMapFunc(reconciler.failureDomainsToDeploymentZones)).
		// Watch a GenericEvent channel for the controlled resource.
		// This is useful when there are events outside of Kubernetes that
		// should cause a resource to be synchronized, such as a goroutine
		// waiting on some asynchronous, external task to complete.
		WatchesRawSource(
			&source.Channel{Source: controllerCtx.GetGenericEventChannelFor(controlledTypeGVK)},
			&handler.EnqueueRequestForObject{},
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(controllerCtx), controllerCtx.WatchFilterValue)).
		Complete(reconciler)
}

type vsphereDeploymentZoneReconciler struct {
	*capvcontext.ControllerContext
}

func (r vsphereDeploymentZoneReconciler) Reconcile(ctx context.Context, request reconcile.Request) (_ reconcile.Result, reterr error) {
	logr := r.Logger.WithValues("vspheredeploymentzone", request.Name)
	// Fetch the VSphereDeploymentZone for this request.
	vsphereDeploymentZone := &infrav1.VSphereDeploymentZone{}
	if err := r.Client.Get(ctx, request.NamespacedName, vsphereDeploymentZone); err != nil {
		if apierrors.IsNotFound(err) {
			logr.V(4).Info("VSphereDeploymentZone not found, won't reconcile", "key", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	failureDomain := &infrav1.VSphereFailureDomain{}
	failureDomainKey := client.ObjectKey{Name: vsphereDeploymentZone.Spec.FailureDomain}
	if err := r.Client.Get(ctx, failureDomainKey, failureDomain); err != nil {
		if apierrors.IsNotFound(err) {
			logr.V(4).Info("Failure Domain not found, won't reconcile", "key", failureDomainKey)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	patchHelper, err := patch.NewHelper(vsphereDeploymentZone, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			vsphereDeploymentZone.GroupVersionKind(),
			vsphereDeploymentZone.Namespace,
			vsphereDeploymentZone.Name)
	}

	vsphereDeploymentZoneContext := &capvcontext.VSphereDeploymentZoneContext{
		ControllerContext:     r.ControllerContext,
		VSphereDeploymentZone: vsphereDeploymentZone,
		VSphereFailureDomain:  failureDomain,
		Logger:                logr,
		PatchHelper:           patchHelper,
	}
	defer func() {
		if err := vsphereDeploymentZoneContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			logr.Error(err, "patch failed", "vsphereDeploymentZone", vsphereDeploymentZoneContext.String())
		}
	}()

	if !vsphereDeploymentZone.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(vsphereDeploymentZoneContext)
	}

	// If the VSphereDeploymentZone doesn't have our finalizer, add it.
	// Requeue immediately after adding finalizer to avoid the race condition between init and delete
	if !ctrlutil.ContainsFinalizer(vsphereDeploymentZone, infrav1.DeploymentZoneFinalizer) {
		ctrlutil.AddFinalizer(vsphereDeploymentZone, infrav1.DeploymentZoneFinalizer)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, r.reconcileNormal(vsphereDeploymentZoneContext)
}

func (r vsphereDeploymentZoneReconciler) reconcileNormal(deploymentZoneCtx *capvcontext.VSphereDeploymentZoneContext) error {
	authSession, err := r.getVCenterSession(deploymentZoneCtx)
	if err != nil {
		deploymentZoneCtx.Logger.V(4).Error(err, "unable to create session")
		conditions.MarkFalse(deploymentZoneCtx.VSphereDeploymentZone, infrav1.VCenterAvailableCondition, infrav1.VCenterUnreachableReason, clusterv1.ConditionSeverityError, err.Error())
		deploymentZoneCtx.VSphereDeploymentZone.Status.Ready = pointer.Bool(false)
		return errors.Wrapf(err, "unable to create auth session")
	}
	deploymentZoneCtx.AuthSession = authSession
	conditions.MarkTrue(deploymentZoneCtx.VSphereDeploymentZone, infrav1.VCenterAvailableCondition)

	if err := r.reconcilePlacementConstraint(deploymentZoneCtx); err != nil {
		deploymentZoneCtx.VSphereDeploymentZone.Status.Ready = pointer.Bool(false)
		return errors.Wrap(err, "placement constraint is misconfigured")
	}
	conditions.MarkTrue(deploymentZoneCtx.VSphereDeploymentZone, infrav1.PlacementConstraintMetCondition)

	// reconcile the failure domain
	if err := r.reconcileFailureDomain(deploymentZoneCtx); err != nil {
		deploymentZoneCtx.Logger.V(4).Error(err, "failed to reconcile failure domain", "failureDomain", deploymentZoneCtx.VSphereDeploymentZone.Spec.FailureDomain)
		deploymentZoneCtx.VSphereDeploymentZone.Status.Ready = pointer.Bool(false)
		return errors.Wrapf(err, "failed to reconcile failure domain")
	}
	conditions.MarkTrue(deploymentZoneCtx.VSphereDeploymentZone, infrav1.VSphereFailureDomainValidatedCondition)

	// Ensure the VSphereDeploymentZone is marked as an owner of the VSphereFailureDomain.
	if !clusterutilv1.HasOwnerRef(deploymentZoneCtx.VSphereFailureDomain.GetOwnerReferences(), metav1.OwnerReference{
		APIVersion: infrav1.GroupVersion.String(),
		Kind:       "VSphereDeploymentZone",
		Name:       deploymentZoneCtx.VSphereDeploymentZone.Name,
	}) {
		if err := updateOwnerReferences(deploymentZoneCtx, deploymentZoneCtx.VSphereFailureDomain, r.Client, func() []metav1.OwnerReference {
			return append(deploymentZoneCtx.VSphereFailureDomain.OwnerReferences, metav1.OwnerReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       deploymentZoneCtx.VSphereDeploymentZone.Kind,
				Name:       deploymentZoneCtx.VSphereDeploymentZone.Name,
				UID:        deploymentZoneCtx.VSphereDeploymentZone.UID,
			})
		}); err != nil {
			return err
		}
	}

	deploymentZoneCtx.VSphereDeploymentZone.Status.Ready = pointer.Bool(true)
	return nil
}

func (r vsphereDeploymentZoneReconciler) reconcilePlacementConstraint(deploymentZoneCtx *capvcontext.VSphereDeploymentZoneContext) error {
	placementConstraint := deploymentZoneCtx.VSphereDeploymentZone.Spec.PlacementConstraint

	if resourcePool := placementConstraint.ResourcePool; resourcePool != "" {
		if _, err := deploymentZoneCtx.AuthSession.Finder.ResourcePool(deploymentZoneCtx, resourcePool); err != nil {
			deploymentZoneCtx.Logger.V(4).Error(err, "unable to find resource pool", "name", resourcePool)
			conditions.MarkFalse(deploymentZoneCtx.VSphereDeploymentZone, infrav1.PlacementConstraintMetCondition, infrav1.ResourcePoolNotFoundReason, clusterv1.ConditionSeverityError, "resource pool %s is misconfigured", resourcePool)
			return errors.Wrapf(err, "unable to find resource pool %s", resourcePool)
		}
	}

	if folder := placementConstraint.Folder; folder != "" {
		if _, err := deploymentZoneCtx.AuthSession.Finder.Folder(deploymentZoneCtx, placementConstraint.Folder); err != nil {
			deploymentZoneCtx.Logger.V(4).Error(err, "unable to find folder", "name", folder)
			conditions.MarkFalse(deploymentZoneCtx.VSphereDeploymentZone, infrav1.PlacementConstraintMetCondition, infrav1.FolderNotFoundReason, clusterv1.ConditionSeverityError, "datastore %s is misconfigured", folder)
			return errors.Wrapf(err, "unable to find folder %s", folder)
		}
	}
	return nil
}

func (r vsphereDeploymentZoneReconciler) getVCenterSession(deploymentZoneCtx *capvcontext.VSphereDeploymentZoneContext) (*session.Session, error) {
	params := session.NewParams().
		WithServer(deploymentZoneCtx.VSphereDeploymentZone.Spec.Server).
		WithDatacenter(deploymentZoneCtx.VSphereFailureDomain.Spec.Topology.Datacenter).
		WithUserInfo(r.ControllerContext.Username, r.ControllerContext.Password).
		WithFeatures(session.Feature{
			EnableKeepAlive:   r.EnableKeepAlive,
			KeepAliveDuration: r.KeepAliveDuration,
		})

	clusterList := &infrav1.VSphereClusterList{}
	if err := r.Client.List(deploymentZoneCtx, clusterList); err != nil {
		return nil, err
	}

	for _, vsphereCluster := range clusterList.Items {
		if deploymentZoneCtx.VSphereDeploymentZone.Spec.Server != vsphereCluster.Spec.Server || vsphereCluster.Spec.IdentityRef == nil {
			continue
		}
		logger := deploymentZoneCtx.Logger.WithValues("cluster", vsphereCluster.Name)
		params = params.WithThumbprint(vsphereCluster.Spec.Thumbprint)
		clust := vsphereCluster
		creds, err := identity.GetCredentials(deploymentZoneCtx, r.Client, &clust, r.Namespace)
		if err != nil {
			logger.Error(err, "error retrieving credentials from IdentityRef")
			continue
		}
		logger.Info("using server credentials to create the authenticated session")
		params = params.WithUserInfo(creds.Username, creds.Password)
		return session.GetOrCreate(r.Context,
			params)
	}

	// Fallback to using credentials provided to the manager
	return session.GetOrCreate(r.Context,
		params)
}

func (r vsphereDeploymentZoneReconciler) reconcileDelete(deploymentZoneCtx *capvcontext.VSphereDeploymentZoneContext) error {
	r.Logger.Info("Deleting VSphereDeploymentZone")

	machines := &clusterv1.MachineList{}
	if err := r.Client.List(deploymentZoneCtx, machines); err != nil {
		r.Logger.Error(err, "unable to list machines")
		return errors.Wrapf(err, "unable to list machines")
	}

	machinesUsingDeploymentZone := collections.FromMachineList(machines).Filter(collections.ActiveMachines, func(machine *clusterv1.Machine) bool {
		if machine.Spec.FailureDomain != nil {
			return *machine.Spec.FailureDomain == deploymentZoneCtx.VSphereDeploymentZone.Name
		}
		return false
	})
	if len(machinesUsingDeploymentZone) > 0 {
		machineNamesStr := util.MachinesAsString(machinesUsingDeploymentZone.SortedByCreationTimestamp())
		err := errors.Errorf("%s is currently in use by machines: %s", deploymentZoneCtx.VSphereDeploymentZone.Name, machineNamesStr)
		r.Logger.Error(err, "Error deleting VSphereDeploymentZone", "name", deploymentZoneCtx.VSphereDeploymentZone.Name)
		return err
	}

	if err := updateOwnerReferences(deploymentZoneCtx, deploymentZoneCtx.VSphereFailureDomain, r.Client, func() []metav1.OwnerReference {
		return clusterutilv1.RemoveOwnerRef(deploymentZoneCtx.VSphereFailureDomain.OwnerReferences, metav1.OwnerReference{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       deploymentZoneCtx.VSphereDeploymentZone.Kind,
			Name:       deploymentZoneCtx.VSphereDeploymentZone.Name,
		})
	}); err != nil {
		return err
	}

	if len(deploymentZoneCtx.VSphereFailureDomain.OwnerReferences) == 0 {
		deploymentZoneCtx.Logger.Info("deleting vsphereFailureDomain", "name", deploymentZoneCtx.VSphereFailureDomain.Name)
		if err := r.Client.Delete(deploymentZoneCtx, deploymentZoneCtx.VSphereFailureDomain); err != nil && !apierrors.IsNotFound(err) {
			deploymentZoneCtx.Logger.Error(err, "failed to delete related %s %s", deploymentZoneCtx.VSphereFailureDomain.GroupVersionKind(), deploymentZoneCtx.VSphereFailureDomain.Name)
		}
	}

	ctrlutil.RemoveFinalizer(deploymentZoneCtx.VSphereDeploymentZone, infrav1.DeploymentZoneFinalizer)

	return nil
}

// updateOwnerReferences uses the ownerRef function to calculate the owner references
// to be set on the object and patches the object.
func updateOwnerReferences(ctx context.Context, obj client.Object, client client.Client, ownerRefFunc func() []metav1.OwnerReference) error {
	patchHelper, err := patch.NewHelper(obj, client)
	if err != nil {
		return errors.Wrapf(err, "failed to init patch helper for %s %s",
			obj.GetObjectKind(),
			obj.GetName())
	}

	obj.SetOwnerReferences(ownerRefFunc())
	if err := patchHelper.Patch(ctx, obj); err != nil {
		return errors.Wrapf(err, "failed to patch object %s %s",
			obj.GetObjectKind(),
			obj.GetName())
	}
	return nil
}

func (r vsphereDeploymentZoneReconciler) failureDomainsToDeploymentZones(ctx context.Context, a client.Object) []reconcile.Request {
	failureDomain, ok := a.(*infrav1.VSphereFailureDomain)
	if !ok {
		r.Logger.Error(nil, fmt.Sprintf("expected a VSphereFailureDomain but got a %T", a))
		return nil
	}

	var zones infrav1.VSphereDeploymentZoneList
	if err := r.Client.List(ctx, &zones); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, zone := range zones.Items {
		if zone.Spec.FailureDomain == failureDomain.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: zone.Name,
				},
			})
		}
	}
	return requests
}
