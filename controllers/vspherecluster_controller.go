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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/config"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/util"
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
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

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
	cluster, err := clusterutilv1.GetOwnerCluster(parentContext, r.Client, vsphereCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		logger.Info("Waiting for Cluster Controller to set OwnerRef on VSphereCluster")
		return reconcile.Result{RequeueAfter: config.DefaultRequeue}, nil
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
		return reconcile.Result{}, errors.Errorf("failed to create cluster context: %+v", err)
	}

	// Always close the context when exiting this function so we can persist any VSphereCluster changes.
	defer func() {
		if err := ctx.Patch(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !vsphereCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx)
}

func (r *VSphereClusterReconciler) reconcileDelete(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("Reconciling VSphereCluster delete")

	// Cluster is deleted so remove the finalizer.
	ctx.VSphereCluster.Finalizers = clusterutilv1.Filter(ctx.VSphereCluster.Finalizers, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

func (r *VSphereClusterReconciler) reconcileNormal(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("Reconciling VSphereCluster")

	ctx.VSphereCluster.Status.Ready = true

	// If the VSphereCluster doesn't have our finalizer, add it.
	if !clusterutilv1.Contains(ctx.VSphereCluster.Finalizers, infrav1.ClusterFinalizer) {
		ctx.VSphereCluster.Finalizers = append(ctx.VSphereCluster.Finalizers, infrav1.ClusterFinalizer)
		ctx.Logger.V(6).Info(
			"adding finalizer for VSphereCluster",
			"cluster-namespace", ctx.VSphereCluster.Namespace,
			"cluster-name", ctx.VSphereCluster.Name)
	}

	// Update the VSphereCluster resource with its API enpoints.
	if err := r.reconcileAPIEndpoints(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"failed to reconcile API endpoints for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// If the VSphereCluster resource has no API endpoints set then requeue
	// until an API endpoint for the cluster resource can be found.
	if len(ctx.VSphereCluster.Status.APIEndpoints) == 0 {
		return reconcile.Result{}, errors.Errorf(
			"no API endpoints found for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// Create the cloud config secret for the target cluster.
	if err := r.reconcileCloudConfigSecret(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"failed to reconcile cloud config secret for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// No errors, so mark us ready so the Cluster API Cluster Controller can pull it
	ctx.Logger.V(4).Info("VSphereCluster is ready")

	return reconcile.Result{}, nil
}

func (r *VSphereClusterReconciler) reconcileAPIEndpoints(ctx *context.ClusterContext) error {
	// If the cluster already has API endpoints set then there is nothing to do.
	if len(ctx.VSphereCluster.Status.APIEndpoints) > 0 {
		ctx.Logger.V(6).Info("API endpoints already exist")
		return nil
	}

	// Get the oldest control plane machine in the cluster.
	machine, err := infrautilv1.GetOldestControlPlaneMachine(
		ctx, ctx.Client, ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get oldest control plane machine for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// Get the oldest control plane machine's VSphereMachine resource.
	vsphereMachine, err := infrautilv1.GetVSphereMachine(
		ctx, ctx.Client, ctx.VSphereCluster.Namespace, machine.Name)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get VSphereMachine for Machine %s/%s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name, machine.Name)
	}

	// Get the machine's preferred IP address.
	ipAddr, err := infrautilv1.GetMachinePreferredIPAddress(vsphereMachine)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get preferred IP address for VSphereMachine %s/%s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name, vsphereMachine.Name)
	}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull them
	ctx.VSphereCluster.Status.APIEndpoints = []infrav1.APIEndpoint{
		{
			Host: ipAddr,
			Port: apiEndpointPort,
		},
	}

	return nil
}

// reconcileCloudConfigSecret ensures the cloud config secret is present in the
// target cluster
func (r *VSphereClusterReconciler) reconcileCloudConfigSecret(ctx *context.ClusterContext) error {
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
		credentials[fmt.Sprintf("%s.username", server)] = ctx.User()
		credentials[fmt.Sprintf("%s.password", server)] = ctx.Pass()
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
	if _, err := targetClusterClient.Secrets(secret.Namespace).Create(secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return errors.Wrapf(
			err,
			"failed to create cloud provider secret for Cluster %s/%s",
			ctx.Cluster.Namespace, ctx.Cluster.Name)
	}

	ctx.Logger.V(6).Info("created cloud provider credential secret",
		"cluster-namespace", ctx.Cluster.Namespace,
		"cluster-name", ctx.Cluster.Name,
		"secret-name", secret.Name,
		"secret-namespace", secret.Namespace)

	return nil
}

// SetupWithManager adds this controller to the provided manager.
func (r *VSphereClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.VSphereCluster{}).
		Complete(r)
}
