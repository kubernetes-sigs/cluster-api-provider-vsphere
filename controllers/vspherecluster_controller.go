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
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/cloudprovider"
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

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs;kubeadmconfigs/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=loadbalancers;loadbalancers/status,verbs=get;list;watch;

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *VSphereClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	logger := r.Log.WithName(controllerName)

	ctx, result, err := GetClusterContext(req, logger, r)
	if err != nil {
		if err == ErrNoOwnerCluster || err == ErrVsphereClusterNotFound {
			return result, nil
		}
		return result, err
	}
	vsphereCluster := ctx.VSphereCluster
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

	// TODO(akutz) Update this logic to include infrastructure prep such as:
	//   * Downloading OVAs into the content library for any machines that
	//     use them.
	//   * Create any load balancers for VMC on AWS, etc.
	loadBalancers, err := infrautilv1.GetClusterLoadBalancer(ctx, r.Client, ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	if err != nil {
		return ctrl.Result{}, err
	}
	ctx.VSphereCluster.Status.Ready = len(loadBalancers) == 0

	// If there was no APIEndpoint set append the loadbalancer endpoints
	if len(ctx.VSphereCluster.Status.APIEndpoints) == 0 {
		for _, loadBalancer := range loadBalancers {
			if loadBalancer.Status.APIEndpoint != (infrav1.APIEndpoint{}) {
				ctx.VSphereCluster.Status.Ready = true
				ctx.VSphereCluster.Status.APIEndpoints = append(ctx.VSphereCluster.Status.APIEndpoints, loadBalancer.Status.APIEndpoint)
			}
		}
	}
	ctx.Logger.V(6).Info("VSphereCluster is infrastructure-ready")

	// If the VSphereCluster doesn't have our finalizer, add it.
	if !clusterutilv1.Contains(ctx.VSphereCluster.Finalizers, infrav1.ClusterFinalizer) {
		ctx.VSphereCluster.Finalizers = append(ctx.VSphereCluster.Finalizers, infrav1.ClusterFinalizer)
		ctx.Logger.V(6).Info(
			"adding finalizer for VSphereCluster",
			"cluster-namespace", ctx.VSphereCluster.Namespace,
			"cluster-name", ctx.VSphereCluster.Name)
	}
	if len(loadBalancers) == 0 {
		// Update the VSphereCluster resource with its API enpoints.
		if err := r.reconcileAPIEndpoints(ctx); err != nil {
			return reconcile.Result{}, errors.Wrapf(err,
				"failed to reconcile API endpoints for VSphereCluster %s/%s",
				ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
		}
	}

	// Create the cloud config secret for the target cluster.
	if err := r.reconcileCloudConfigSecret(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"failed to reconcile cloud config secret for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// Create the external cloud provider addons
	if err := r.reconcileCloudProvider(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"failed to reconcile cloud provider for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// Create the vSphere CSI Driver addons
	if err := r.reconcileStorageProvider(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err,
			"failed to reconcile CSI Driver for VSphereCluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	return reconcile.Result{}, nil
}

func (r *VSphereClusterReconciler) reconcileAPIEndpoints(ctx *context.ClusterContext) error {
	// If the cluster already has API endpoints set then there is nothing to do.
	if len(ctx.VSphereCluster.Status.APIEndpoints) > 0 {
		ctx.Logger.V(6).Info("API endpoints already exist")
		return nil
	}

	// Get the CAPI Machine resources for the cluster.
	labels := map[string]string{clusterv1.MachineClusterLabelName: ctx.VSphereCluster.Name}
	machines, err := infrautilv1.GetMachinesWithSelector(ctx, ctx.Client, ctx.VSphereCluster.Namespace, labels)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get Machinces for Cluster %s/%s",
			ctx.VSphereCluster.Namespace, ctx.VSphereCluster.Name)
	}

	// Iterate over the cluster's control plane CAPI machines.
	for _, machine := range clusterutilv1.GetControlPlaneMachines(machines) {

		// Get the machine's associated KubeadmConfig resource to check if
		// there is a ControlPlaneEndpoint value assigned to the Init/Join
		// configuration.
		//
		// TODO(akutz) Assuming the Config type violates the separation model
		//             in CAPI v1a2. Please see https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues/555#issuecomment-529237211
		//             for more information.
		kubeadmConfig, err := infrautilv1.GetKubeadmConfigForMachine(ctx, ctx.Client, machine)
		if err != nil {
			return err
		}

		var apiEndpoint infrav1.APIEndpoint

		// If there is a ControlPlaneEndpoint set then use it.
		if cpe := kubeadmConfig.Spec.ClusterConfiguration.ControlPlaneEndpoint; cpe != "" {
			parsedAPIEndpoint, err := infrautilv1.GetAPIEndpointForControlPlaneEndpoint(cpe)
			if err != nil {
				return err
			}
			apiEndpoint = *parsedAPIEndpoint
			ctx.Logger.V(6).Info(
				"found API endpoint via KubeadmConfig",
				"host", apiEndpoint.Host, "port", apiEndpoint.Port)
		} else {
			// Only machines with bootstrap data will have an IP address.
			if machine.Spec.Bootstrap.Data == nil {
				ctx.Logger.V(6).Info(
					"skipping machine while looking for IP address",
					"machine-name", machine.Name,
					"skip-reason", "nilBootstrapData")
				continue
			}

			// Get the VSphereMachine for the CAPI Machine resource.
			vsphereMachine, err := infrautilv1.GetVSphereMachine(ctx, ctx.Client, machine.Namespace, machine.Name)
			if err != nil {
				return errors.Wrapf(err,
					"failed to get VSphereMachine for Machine %s/%s/%s",
					machine.Namespace, ctx.VSphereCluster.Name, machine.Name)
			}

			// Get the VSphereMachine's preferred IP address.
			ipAddr, err := infrautilv1.GetMachinePreferredIPAddress(vsphereMachine)
			if err != nil {
				if err == infrautilv1.ErrNoMachineIPAddr {
					continue
				}
				return errors.Wrapf(err,
					"failed to get preferred IP address for VSphereMachine %s/%s/%s",
					machine.Namespace, ctx.VSphereCluster.Name, vsphereMachine.Name)
			}

			apiEndpoint.Host = ipAddr
			apiEndpoint.Port = apiEndpointPort

			ctx.Logger.V(6).Info(
				"found API endpoint via control plane machine",
				"host", apiEndpoint.Host, "port", apiEndpoint.Port)
		}

		// Set APIEndpoints so the CAPI controller can read the API endpoints
		// for this VSphereCluster into the analogous CAPI Cluster using an
		// UnstructuredReader.
		ctx.VSphereCluster.Status.APIEndpoints = []infrav1.APIEndpoint{apiEndpoint}
		return nil
	}
	return infrautilv1.ErrNoMachineIPAddr
}

func (r *VSphereClusterReconciler) reconcileCloudProvider(ctx *context.ClusterContext) error {
	// if the cloud provider image is not specified, then we do nothing
	controllerImage := ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Cloud.ControllerImage
	if controllerImage == "" {
		return nil
	}

	targetClusterClient, err := infrautilv1.NewKubeClient(ctx, ctx.Client, ctx.Cluster)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get client for Cluster %s/%s",
			ctx.Cluster.Namespace, ctx.Cluster.Name)
	}

	serviceAccount := cloudprovider.CloudControllerManagerServiceAccount()
	if _, err := targetClusterClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(serviceAccount); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	cloudConfigData, err := ctx.VSphereCluster.Spec.CloudProviderConfiguration.MarshalINI()
	if err != nil {
		return err
	}

	cloudConfigMap := cloudprovider.CloudControllerManagerConfigMap(string(cloudConfigData))
	if _, err := targetClusterClient.CoreV1().ConfigMaps(cloudConfigMap.Namespace).Create(cloudConfigMap); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonSet := cloudprovider.CloudControllerManagerDaemonSet(controllerImage)
	if _, err := targetClusterClient.AppsV1().DaemonSets(daemonSet.Namespace).Create(daemonSet); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	service := cloudprovider.CloudControllerManagerService()
	if _, err := targetClusterClient.CoreV1().Services(daemonSet.Namespace).Create(service); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRole := cloudprovider.CloudControllerManagerClusterRole()
	if _, err := targetClusterClient.RbacV1().ClusterRoles().Create(clusterRole); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRoleBinding := cloudprovider.CloudControllerManagerClusterRoleBinding()
	if _, err := targetClusterClient.RbacV1().ClusterRoleBindings().Create(clusterRoleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	roleBinding := cloudprovider.CloudControllerManagerRoleBinding()
	if _, err := targetClusterClient.RbacV1().RoleBindings(roleBinding.Namespace).Create(roleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (r *VSphereClusterReconciler) reconcileStorageProvider(ctx *context.ClusterContext) error {
	targetClusterClient, err := infrautilv1.NewKubeClient(ctx, ctx.Client, ctx.Cluster)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get client for Cluster %s/%s",
			ctx.Cluster.Namespace, ctx.Cluster.Name)
	}

	serviceAccount := cloudprovider.CSIControllerServiceAccount()
	if _, err := targetClusterClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(serviceAccount); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRole := cloudprovider.CSIControllerClusterRole()
	if _, err := targetClusterClient.RbacV1().ClusterRoles().Create(clusterRole); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	clusterRoleBinding := cloudprovider.CSIControllerClusterRoleBinding()
	if _, err := targetClusterClient.RbacV1().ClusterRoleBindings().Create(clusterRoleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// we have to marshal a separate INI file for CSI since it does not
	// support Secrets for vCenter credentials yet.
	cloudConfig, err := cloudprovider.ConfigForCSI(ctx).MarshalINI()
	if err != nil {
		return err
	}

	cloudConfigSecret := cloudprovider.CSICloudConfigSecret(string(cloudConfig))
	if _, err := targetClusterClient.CoreV1().Secrets(cloudConfigSecret.Namespace).Create(cloudConfigSecret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	csiDriver := cloudprovider.CSIDriver()
	if _, err := targetClusterClient.StorageV1beta1().CSIDrivers().Create(csiDriver); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	daemonSet := cloudprovider.VSphereCSINodeDaemonSet(ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Storage)
	if _, err := targetClusterClient.AppsV1().DaemonSets(daemonSet.Namespace).Create(daemonSet); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	statefulSet := cloudprovider.CSIControllerStatefulSet(ctx.VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Storage)
	if _, err := targetClusterClient.AppsV1().StatefulSets(statefulSet.Namespace).Create(statefulSet); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
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
	if _, err := targetClusterClient.CoreV1().Secrets(secret.Namespace).Create(secret); err != nil {
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
