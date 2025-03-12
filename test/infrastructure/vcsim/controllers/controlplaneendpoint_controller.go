/*
Copyright 2024 The Kubernetes Authors.

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

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	"sigs.k8s.io/cluster-api/util/finalizers"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type ControlPlaneEndpointReconciler struct {
	Client client.Client

	InMemoryManager inmemoryruntime.Manager
	APIServerMux    *inmemoryserver.WorkloadClustersMux
	PodIP           string

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=vcsim.infrastructure.cluster.x-k8s.io,resources=controlplaneendpoints,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=vcsim.infrastructure.cluster.x-k8s.io,resources=controlplaneendpoints/status,verbs=get;update;patch

func (r *ControlPlaneEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Fetch the ControlPlaneEndpoint instance
	controlPlaneEndpoint := &vcsimv1.ControlPlaneEndpoint{}
	if err := r.Client.Get(ctx, req.NamespacedName, controlPlaneEndpoint); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Add finalizer first if not set to avoid the race condition between init and delete.
	if finalizerAdded, err := finalizers.EnsureFinalizer(ctx, r.Client, controlPlaneEndpoint, vcsimv1.ControlPlaneEndpointFinalizer); err != nil || finalizerAdded {
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(controlPlaneEndpoint, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the controlPlaneEndpoint object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, controlPlaneEndpoint); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deleted machines
	if !controlPlaneEndpoint.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, controlPlaneEndpoint)
	}

	// Handle non-deleted machines
	return ctrl.Result{}, r.reconcileNormal(ctx, controlPlaneEndpoint)
}

func (r *ControlPlaneEndpointReconciler) reconcileNormal(ctx context.Context, controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling VCSim ControlPlaneEndpoint")

	// Initialize a listener for the workload cluster.
	// IMPORTANT: The fact that both the listener and the resourceGroup for a workload cluster have
	// the same name is used as assumptions in other part of the implementation.
	listenerName := klog.KObj(controlPlaneEndpoint).String()
	listener, err := r.APIServerMux.InitWorkloadClusterListener(listenerName)
	if err != nil {
		return errors.Wrapf(err, "failed to init the listener for the control plane endpoint")
	}

	controlPlaneEndpoint.Status.Host = r.PodIP // NOTE: we are replacing the listener ip with the pod ip so it will be accessible from other pods as well
	controlPlaneEndpoint.Status.Port = int32(listener.Port())

	return nil
}

func (r *ControlPlaneEndpointReconciler) reconcileDelete(ctx context.Context, controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling delete VCSim ControlPlaneEndpoint")
	listenerName := klog.KObj(controlPlaneEndpoint).String()

	// Delete the resource group hosting all the cloud resources belonging the workload cluster;
	if resourceGroup, err := r.APIServerMux.ResourceGroupByWorkloadCluster(listenerName); err == nil {
		r.InMemoryManager.DeleteResourceGroup(resourceGroup)
	}

	// Delete the listener for the workload cluster;
	if err := r.APIServerMux.DeleteWorkloadClusterListener(listenerName); err != nil {
		return errors.Wrapf(err, "failed to delete the listener for the control plane endpoint")
	}

	controllerutil.RemoveFinalizer(controlPlaneEndpoint, vcsimv1.ControlPlaneEndpointFinalizer)

	return nil
}

// SetupWithManager will add watches for this controller.
func (r *ControlPlaneEndpointReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	predicateLog := ctrl.LoggerFrom(ctx).WithValues("controller", "controlplaneendpoint")

	err := ctrl.NewControllerManagedBy(mgr).
		For(&vcsimv1.ControlPlaneEndpoint{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), predicateLog, r.WatchFilterValue)).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}
