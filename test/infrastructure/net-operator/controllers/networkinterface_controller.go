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
	"time"

	"github.com/pkg/errors"
	netopv1alpha1 "github.com/vmware-tanzu/net-operator-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/cluster-api-provider-vsphere/test/framework/vmoperator"
)

type NetworkInterfaceReconciler struct {
	Client            client.Client
	EnableKeepAlive   bool
	KeepAliveDuration time.Duration

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=netoperator.vmware.com,resources=networkinterfaces,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=netoperator.vmware.com,resources=networkinterfaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create

func (r *NetworkInterfaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling NetworkInterface")

	// Fetch the NetworkInterface instance
	networkInterface := &netopv1alpha1.NetworkInterface{}
	if err := r.Client.Get(ctx, req.NamespacedName, networkInterface); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(networkInterface, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the NetworkInterface object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, networkInterface); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	if networkInterface.Status.NetworkID == "" {
		s, err := vmoperator.GetVCenterSession(ctx, r.Client, r.EnableKeepAlive, r.KeepAliveDuration)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to get vcenter session")
		}

		distributedPortGroupName, err := vmoperator.GetDistributedPortGroup(ctx, r.Client)
		if err != nil {
			return reconcile.Result{}, err
		}

		distributedPortGroup, err := s.Finder.Network(ctx, distributedPortGroupName)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to get DistributedPortGroup %s", distributedPortGroupName)
		}

		networkInterface.Status.NetworkID = distributedPortGroup.Reference().Value
		networkInterface.Status.Conditions = []netopv1alpha1.NetworkInterfaceCondition{
			{
				Type:   netopv1alpha1.NetworkInterfaceReady,
				Status: corev1.ConditionTrue,
			},
		}

		// NOTE: we are not setting networkInterface.Status.IPConfigs because we are using dhcp to assign ip in supervisor tests (or the vmIP reconciler with vcsim).

		log.Info("Reconciling NetworkInterface status simulating successful net-operator reconcile")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *NetworkInterfaceReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&netopv1alpha1.NetworkInterface{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	return nil
}
