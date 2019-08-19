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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api/pkg/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

const (
	controllerName  = "vspherecluster-controller"
	apiEndpointPort = 6443
)

// VSphereClusterReconciler reconciles a VSphereCluster object
type VSphereClusterReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Log      logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters/status,verbs=get;update;patch

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *VSphereClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	parentContext := goctx.Background()

	logger := r.Log.WithName(controllerName).
		WithName(fmt.Sprintf("namespace=%s", req.Namespace)).
		WithName(fmt.Sprintf("vsphereCluster=%s", req.Name))

	// Fetch the VSphereCluster instance
	vsphereCluster := &infrav1.VSphereCluster{}
	err := r.Get(parentContext, req.NamespacedName, vsphereCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	logger = logger.WithName(vsphereCluster.APIVersion)

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(parentContext, r.Client, vsphereCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		logger.Info("Waiting for Cluster Controller to set OwnerRef on VSphereCluster")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	logger = logger.WithName(fmt.Sprintf("cluster=%s", cluster.Name))

	// Create the context.
	ctx, err := context.NewClusterContext(&context.ClusterContextParams{
		Context:        parentContext,
		Cluster:        cluster,
		VSphereCluster: vsphereCluster,
		Client:         r.Client,
		Logger:         logger,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create context: %+v", err)
	}

	// Always close the context when exiting this function so we can persist any VSphereCluster changes.
	defer func() {
		if err := ctx.Patch(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !vsphereCluster.DeletionTimestamp.IsZero() {
		return reconcileDelete(ctx)
	}

	// Handle non-deleted clusters
	return reconcileNormal(ctx)
}

func reconcileDelete(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("Reconciling VSphereCluster delete")

	// Cluster is deleted so remove the finalizer.
	ctx.VSphereCluster.Finalizers = util.Filter(ctx.VSphereCluster.Finalizers, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

func reconcileNormal(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("Reconciling VSphereCluster")

	vsphereCluster := ctx.VSphereCluster

	// If the VSphereCluster doesn't have our finalizer, add it.
	if !util.Contains(vsphereCluster.Finalizers, infrav1.ClusterFinalizer) {
		vsphereCluster.Finalizers = append(vsphereCluster.Finalizers, infrav1.ClusterFinalizer)
	}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull them
	vsphereCluster.Status.APIEndpoints = []infrav1.APIEndpoint{
		{
			Host: "", // vsphereCluster.Status.Network.APIServerELB.DNSName,
			Port: 0,
		},
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	vsphereCluster.Status.Ready = true

	return reconcile.Result{}, nil
}

// SetupWithManager adds this controller to the provided manager.
func (r *VSphereClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.VSphereCluster{}).
		Complete(r)
}
