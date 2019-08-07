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

package cluster

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/klogr"
	elb "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/aws"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clientv1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	clusterErr "sigs.k8s.io/cluster-api/pkg/controller/error"
	remotev1 "sigs.k8s.io/cluster-api/pkg/controller/remote"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/actuators"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/config"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/certificates"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeclient"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeconfig"
)

//+kubebuilder:rbac:groups=vsphere.cluster.k8s.io,resources=vsphereclusterproviderspecs;vsphereclusterproviderstatuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=,resources=secrets,verbs=create;get;list;watch;delete
//+kubebuilder:rbac:groups="",resources=nodes;events;configmaps,verbs=get;list;watch;create;update;patch;delete

// Actuator is responsible for maintaining the Cluster objects.
type Actuator struct {
	client           clientv1.ClusterV1alpha1Interface
	coreClient       corev1.CoreV1Interface
	controllerClient controllerClient.Client
}

// NewActuator returns a new instance of Actuator.
func NewActuator(
	client clientv1.ClusterV1alpha1Interface,
	coreClient corev1.CoreV1Interface,
	controllerClient controllerClient.Client) *Actuator {

	return &Actuator{
		client:           client,
		coreClient:       coreClient,
		controllerClient: controllerClient,
	}
}

// Reconcile will create or update the cluster
func (a *Actuator) Reconcile(cluster *clusterv1.Cluster) (opErr error) {
	ctx, err := context.NewClusterContext(&context.ClusterContextParams{
		Cluster:          cluster,
		Client:           a.client,
		CoreClient:       a.coreClient,
		ControllerClient: a.controllerClient,
		Logger:           klogr.New().WithName("[cluster-actuator]"),
	})
	if err != nil {
		return err
	}

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Reconcile", opErr)
	}()

	ctx.Logger.V(6).Info("reconciling cluster")

	if err := a.reconcilePKI(ctx); err != nil {
		return err
	}

	isVMwareCloud := ctx.ClusterConfig.VmwareCloud != nil
	if isVMwareCloud {
		if err = a.reconcileLoadBalancers(ctx); err != nil {
			return err
		}
	}

	if err := a.reconcileKubeConfig(ctx); err != nil {
		return err
	}

	if err := a.reconcileReadyState(ctx, isVMwareCloud); err != nil {
		return err
	}

	if err := a.reconcileCloudConfigSecret(ctx); err != nil {
		return err
	}

	return nil
}

// Delete will delete any cluster level resources for the cluster.
func (a *Actuator) Delete(cluster *clusterv1.Cluster) (opErr error) {
	ctx, err := context.NewClusterContext(&context.ClusterContextParams{
		Cluster:          cluster,
		Client:           a.client,
		CoreClient:       a.coreClient,
		ControllerClient: a.controllerClient,
	})
	if err != nil {
		return err
	}

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Delete", opErr)
	}()

	ctx.Logger.V(2).Info("deleting cluster")

	// Delete the kubeconfig secret for the target cluster.
	if err := a.deleteKubeConfigSecret(ctx); err != nil {
		return err
	}

	// Delete the control plane config map for the target cluster.
	if err := a.deleteControlPlaneConfigMap(ctx); err != nil {
		return err
	}

	// Delete the Load balancer if we are using VMC
	if ctx.ClusterConfig.VmwareCloud != nil {
		if err := a.deleteLoadBalancer(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (a *Actuator) reconcilePKI(ctx *context.ClusterContext) error {
	if err := certificates.ReconcileCertificates(ctx); err != nil {
		return errors.Wrapf(err, "unable to reconcile certs while reconciling cluster %q", ctx)
	}
	return nil
}

func (a *Actuator) reconcileReadyState(ctx *context.ClusterContext, isVMwareCloud bool) error {

	// Always recalculate the API Endpoints.
	ctx.Cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{}

	// Reset the cluster's ready status
	ctx.ClusterStatus.Ready = false

	// List the target cluster's nodes to verify the target cluster is online.
	client, err := remotev1.NewClusterClient(a.controllerClient, ctx.Cluster)
	if err != nil {
		ctx.Logger.Error(err, "unable to get client for target cluster")
		return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
	}
	coreClient, err := client.CoreV1()
	if err != nil {
		ctx.Logger.Error(err, "unable to get core client for target cluster")
		return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
	}
	if _, err := coreClient.Nodes().List(metav1.ListOptions{}); err != nil {
		ctx.Logger.Error(err, "unable to list nodes for target cluster")
		return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
	}
	if isVMwareCloud {
		// Get the RESTConfig in order to parse its Host to use as the control plane
		// endpoint to add to the Cluster's API endpoints.
		restConfig := client.RESTConfig()
		if restConfig == nil {
			ctx.Logger.Error(errors.New("restConfig == nil"), "error getting RESTConfig for kube client")
			return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
		}

		// Calculate the API endpoint for the cluster.
		controlPlaneEndpointURL, err := url.Parse(restConfig.Host)
		if err != nil {
			return errors.Wrapf(err, "unable to parse cluster's restConifg host value: %v", restConfig.Host)
		}

		// The API endpoint may just have a host.
		apiEndpoint := clusterv1.APIEndpoint{
			Host: controlPlaneEndpointURL.Hostname(),
		}

		// Check to see if there is also a port.
		if szPort := controlPlaneEndpointURL.Port(); szPort != "" {
			port, err := strconv.Atoi(szPort)
			if err != nil {
				return errors.Wrapf(err, "unable to get parse host and port for control plane endpoint %q for %q", controlPlaneEndpointURL.Host, ctx)
			}
			apiEndpoint.Port = port
		}

		// Update the API endpoints.
		ctx.Cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{apiEndpoint}
		ctx.Logger.V(6).Info("calculated API endpoint for target cluster", "api-endpoint-host", apiEndpoint.Host, "api-endpoint-port", apiEndpoint.Port)

		// Update the kubeadm control plane endpoint with the one from the kubeconfig.
		if ctx.ClusterConfig.ClusterConfiguration.ControlPlaneEndpoint != controlPlaneEndpointURL.Host {
			ctx.ClusterConfig.ClusterConfiguration.ControlPlaneEndpoint = controlPlaneEndpointURL.Host
			ctx.Logger.V(6).Info("stored control plane endpoint in kubeadm cluster config", "control-plane-endpoint", controlPlaneEndpointURL.Host)
		}
	}
	// Update the ready status.
	ctx.ClusterStatus.Ready = true

	ctx.Logger.V(6).Info("cluster is ready")
	return nil
}

// reconcileCloudConfigSecret ensures the cloud config secret is present in the
// target cluster.
func (a *Actuator) reconcileCloudConfigSecret(ctx *context.ClusterContext) error {
	client, err := kubeclient.New(ctx)
	if err != nil {
		ctx.Logger.Error(err, "target cluster is not ready")
		return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
	}
	if len(ctx.ClusterConfig.CloudProviderConfiguration.VCenter) == 0 {
		return errors.New("cloud provider configuration does not define any vCenters")
	}
	credentials := map[string]string{}
	for server := range ctx.ClusterConfig.CloudProviderConfiguration.VCenter {
		credentials[fmt.Sprintf("%s.username", server)] = ctx.User()
		credentials[fmt.Sprintf("%s.password", server)] = ctx.Pass()
	}
	// Define the kubeconfig secret for the target cluster.
	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.ClusterConfig.CloudProviderConfiguration.Global.SecretNamespace,
			Name:      ctx.ClusterConfig.CloudProviderConfiguration.Global.SecretName,
		},
		Type:       apiv1.SecretTypeOpaque,
		StringData: credentials,
	}
	if _, err := client.Secrets(secret.Namespace).Create(secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		ctx.Logger.Error(err, "unable to create cloud provider secret")
		return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
	}

	ctx.Logger.V(6).Info("created cloud provider credential secret",
		"secret-name", secret.Name,
		"secret-namespace", secret.Namespace)

	return nil
}

func (a *Actuator) deleteKubeConfigSecret(ctx *context.ClusterContext) error {
	if err := a.coreClient.Secrets(ctx.Cluster.Namespace).Delete(remotev1.KubeConfigSecretName(ctx.Cluster.Name), &metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsGone(err) && !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "error deleting kubeconfig secret for target cluster %q", ctx)
		}
	}
	return nil
}

func (a *Actuator) deleteControlPlaneConfigMap(ctx *context.ClusterContext) error {
	controlPlaneConfigMapName := actuators.GetNameOfControlPlaneConfigMap(ctx.Cluster.UID)
	if err := ctx.CoreClient.ConfigMaps(ctx.Cluster.Namespace).Delete(controlPlaneConfigMapName, &metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsGone(err) && !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "error deleting control plane config map %q for target cluster %q", controlPlaneConfigMapName, ctx)
		}
	}
	return nil
}

func (a *Actuator) deleteLoadBalancer(ctx *context.ClusterContext) error {
	elbSvc := elb.New(ctx.ClusterConfig.VmwareCloud.AwsProvider.Region)
	clusterName := ctx.ClusterName()

	if err := elbSvc.Delete(clusterName); err != nil {
		ctx.Logger.Error(err, "cannot delete load balancers")
		return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
	}
	return nil
}

func (a *Actuator) reconcileLoadBalancers(ctx *context.ClusterContext) error {

	ctx.Logger.V(2).Info("Reconciling load balancers")

	clusterName := ctx.ClusterName()
	controlPlaneMachines, err := ctx.GetControlPlaneMachines()
	if err != nil {
		ctx.Logger.Error(err, "error getting control plane machines")
		return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
	}
	controlPlaneIPs := []string{}
	for _, controlPlaneMachine := range controlPlaneMachines {
		for _, nodeAddress := range controlPlaneMachine.Status.Addresses {
			if nodeAddress.Type == apiv1.NodeExternalIP || nodeAddress.Type == apiv1.NodeInternalIP {
				controlPlaneIPs = append(controlPlaneIPs, nodeAddress.Address)
				break
			}
		}
	}

	awsProviderInfo := ctx.ClusterConfig.VmwareCloud.AwsProvider
	elbSvc := elb.New(awsProviderInfo.Region)
	vpcID := awsProviderInfo.VpcID
	subnets := awsProviderInfo.Subnets
	loadBalancerDNS, loadBalancerPort, err := elbSvc.Reconcile(vpcID, controlPlaneIPs, clusterName, subnets)
	if err != nil {
		ctx.Logger.Error(err, "cannot reconcile load balancer")
		return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
	}

	apiEndpoint := clusterv1.APIEndpoint{
		Host: loadBalancerDNS,
		Port: loadBalancerPort,
	}

	// Update the API endpoints.
	ctx.Cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{apiEndpoint}
	ctx.Logger.V(6).Info("calculated API endpoint for target cluster", "api-endpoint-host", apiEndpoint.Host, "api-endpoint-port", apiEndpoint.Port)
	controlPlaneEndpointURL := strings.Join([]string{apiEndpoint.Host, strconv.Itoa(apiEndpoint.Port)}, ":")
	// Update the kubeadm control plane endpoint with the one from the kubeconfig.
	if ctx.ClusterConfig.ClusterConfiguration.ControlPlaneEndpoint != controlPlaneEndpointURL {
		ctx.ClusterConfig.ClusterConfiguration.ControlPlaneEndpoint = controlPlaneEndpointURL
		ctx.Logger.V(6).Info("stored control plane endpoint in kubeadm cluster config", "control-plane-endpoint", controlPlaneEndpointURL)
	}

	return nil

}

// reconcileKubeConfig creates a secret on the management cluster with
// the kubeconfig for target cluster.
func (a *Actuator) reconcileKubeConfig(ctx *context.ClusterContext) error {

	// Get the control plane endpoint.
	controlPlaneEndpoint, err := ctx.ControlPlaneEndpoint()
	if err != nil {
		ctx.Logger.Error(err, "requeueing until control plane endpoint is available")
		return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
	}

	// Create a new kubeconfig for the target cluster.
	ctx.Logger.V(6).Info("generating kubeconfig secret", "controlPlaneEndpoint", controlPlaneEndpoint)
	kubeConfig, err := kubeconfig.New(ctx.Cluster.Name, controlPlaneEndpoint, ctx.ClusterConfig.CAKeyPair)
	if err != nil {
		return errors.Wrapf(err, "error generating kubeconfig for %q", ctx)
	}

	// Define the kubeconfig secret for the target cluster.
	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.Cluster.Namespace,
			Name:      remotev1.KubeConfigSecretName(ctx.Cluster.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: ctx.Cluster.APIVersion,
					Kind:       ctx.Cluster.Kind,
					Name:       ctx.Cluster.Name,
					UID:        ctx.Cluster.UID,
				},
			},
		},
		StringData: map[string]string{
			"value": kubeConfig,
		},
	}
	if exstingSecret, err := a.coreClient.Secrets(ctx.Cluster.Namespace).Get(secret.Name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			if _, err := a.coreClient.Secrets(ctx.Cluster.Namespace).Create(secret); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					ctx.Logger.Error(err, "error creating kubeconfig secret")
					return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
				}
				ctx.Logger.V(6).Info("kubeconfig secret already exists")
			} else {
				ctx.Logger.V(4).Info("created kubeconfig secret")
			}
		} else {
			ctx.Logger.Error(err, "cannot get the kubeconfig secret")
			return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
		}
	} else {
		secret.ResourceVersion = exstingSecret.ResourceVersion
		_, err = a.coreClient.Secrets(ctx.Cluster.Namespace).Update(secret)
		if err != nil {
			ctx.Logger.Error(err, "cannot update the existing kubeconfig")
			return &clusterErr.RequeueAfterError{RequeueAfter: config.DefaultRequeue}
		}
	}
	// Create the kubeconfig secret.
	return nil
}
