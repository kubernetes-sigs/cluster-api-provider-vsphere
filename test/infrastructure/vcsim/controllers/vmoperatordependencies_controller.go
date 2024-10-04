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
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"sigs.k8s.io/cluster-api-provider-vsphere/test/framework/vmoperator"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type VMOperatorDependenciesReconciler struct {
	Client client.Client

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=vcsim.infrastructure.cluster.x-k8s.io,resources=vmoperatordependencies,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=vcsim.infrastructure.cluster.x-k8s.io,resources=vmoperatordependencies/status,verbs=get;update;patch

func (r *VMOperatorDependenciesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Fetch the VMOperatorDependencies instance
	vmOperatorDependencies := &vcsimv1.VMOperatorDependencies{}
	if err := r.Client.Get(ctx, req.NamespacedName, vmOperatorDependencies); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(vmOperatorDependencies, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the VMOperatorDependencies object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, vmOperatorDependencies); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deleted VMOperatorDependencies
	if !vmOperatorDependencies.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, vmOperatorDependencies)
	}

	// Handle non-deleted VMOperatorDependencies
	return ctrl.Result{}, r.reconcileNormal(ctx, vmOperatorDependencies)
}

func (r *VMOperatorDependenciesReconciler) reconcileNormal(ctx context.Context, vmOperatorDependencies *vcsimv1.VMOperatorDependencies) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling VCSim VMOperatorDependencies")

	err := vmoperator.ReconcileDependencies(ctx, r.Client, vmOperatorDependencies)
	if err != nil {
		vmOperatorDependencies.Status.Ready = false
		return err
	}

	vmOperatorDependencies.Status.Ready = true
	return nil
}

func (r *VMOperatorDependenciesReconciler) reconcileDelete(_ context.Context, _ *vcsimv1.VMOperatorDependencies) (ctrl.Result, error) {
	// TODO: cleanup dependencies
	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *VMOperatorDependenciesReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	predicateLog := ctrl.LoggerFrom(ctx).WithValues("controller", "vmoperatordependencies")

	err := ctrl.NewControllerManagedBy(mgr).
		For(&vcsimv1.VMOperatorDependencies{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), predicateLog, r.WatchFilterValue)).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}
