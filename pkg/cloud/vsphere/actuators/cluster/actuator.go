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
	"net"
	"strconv"

	"github.com/pkg/errors"

	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clientv1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	clusterErr "sigs.k8s.io/cluster-api/pkg/controller/error"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/actuators"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/certificates"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeclient"
)

// Actuator is responsible for maintaining the Cluster objects.
type Actuator struct {
	client     clientv1.ClusterV1alpha1Interface
	coreClient corev1.CoreV1Interface
}

// NewActuator returns a new instance of Actuator.
func NewActuator(
	client clientv1.ClusterV1alpha1Interface,
	coreClient corev1.CoreV1Interface) *Actuator {

	return &Actuator{
		client:     client,
		coreClient: coreClient,
	}
}

// Reconcile will create or update the cluster
func (a *Actuator) Reconcile(cluster *clusterv1.Cluster) (opErr error) {
	ctx, err := context.NewClusterContext(&context.ClusterContextParams{
		Cluster:    cluster,
		Client:     a.client,
		CoreClient: a.coreClient,
		Logger:     klogr.New().WithName("[cluster-actuator]"),
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

	if err := a.reconcileReadyState(ctx); err != nil {
		return err
	}

	// reconcileKubeConfig can fail because control plane endpoints
	// aren't ready so return requeue error
	if err := a.reconcileKubeConfig(ctx); err != nil {
		return errors.Wrapf(&clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue},
			"error reconciling kubeconfig secret for cluster %q", ctx)
	}

	return nil
}

// Delete will delete any cluster level resources for the cluster.
func (a *Actuator) Delete(cluster *clusterv1.Cluster) (opErr error) {
	ctx, err := context.NewClusterContext(&context.ClusterContextParams{
		Cluster:    cluster,
		Client:     a.client,
		CoreClient: a.coreClient,
	})
	if err != nil {
		return err
	}

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Delete", opErr)
	}()

	ctx.Logger.V(2).Info("deleting cluster")

	// if deleteKubeconfig fails, return requeue error so kubeconfig
	// secret is properly cleaned up
	if err := a.deleteKubeConfig(ctx); err != nil {
		return errors.Wrapf(&clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue},
			"error deleting kubeconfig secret for cluster %q", ctx)
	}

	return nil
}

// GetIP returns the control plane endpoint for the cluster.
func (a *Actuator) GetIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	clusterContext, err := context.NewClusterContext(&context.ClusterContextParams{
		Cluster:    cluster,
		Client:     a.client,
		CoreClient: a.coreClient,
	})
	if err != nil {
		return "", err
	}
	machineContext, err := context.NewMachineContextFromClusterContext(clusterContext, machine)
	if err != nil {
		return "", err
	}
	return machineContext.ControlPlaneEndpoint()
}

// GetKubeConfig returns the contents of a Kubernetes configuration file that
// may be used to access the cluster.
func (a *Actuator) GetKubeConfig(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	clusterContext, err := context.NewClusterContext(&context.ClusterContextParams{
		Cluster:    cluster,
		Client:     a.client,
		CoreClient: a.coreClient,
	})
	if err != nil {
		return "", err
	}

	if machine == nil {
		return kubeclient.GetKubeConfig(clusterContext)
	}

	machineContext, err := context.NewMachineContextFromClusterContext(clusterContext, machine)
	if err != nil {
		return "", err
	}

	return kubeclient.GetKubeConfig(machineContext)
}

func (a *Actuator) reconcilePKI(ctx *context.ClusterContext) error {
	if err := certificates.ReconcileCertificates(ctx); err != nil {
		return errors.Wrapf(err, "unable to reconcile certs while reconciling cluster %q", ctx)
	}
	return nil
}

func (a *Actuator) reconcileReadyState(ctx *context.ClusterContext) error {
	online, controlPlaneEndpoint, err := kubeclient.GetControlPlaneStatus(ctx)
	if err != nil {
		// This may or may not contain RequeueError. If it does then the deferred
		// PathAndHandleError will take care of requeueing things.
		return err
	}
	if !online {
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}
	host, szPort, err := net.SplitHostPort(controlPlaneEndpoint)
	if err != nil {
		return errors.Wrapf(err, "unable to get host/port for control plane endpoint %q for %q", controlPlaneEndpoint, ctx)
	}
	port, err := strconv.Atoi(szPort)
	if err != nil {
		return errors.Wrapf(err, "unable to get parse host and port for control plane endpoint %q for %q", controlPlaneEndpoint, ctx)
	}
	if len(ctx.Cluster.Status.APIEndpoints) == 0 || (ctx.Cluster.Status.APIEndpoints[0].Host != host && ctx.Cluster.Status.APIEndpoints[0].Port != port) {
		ctx.Cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{
			{
				Host: host,
				Port: port,
			},
		}
	}
	if ctx.ClusterConfig.ClusterConfiguration.ControlPlaneEndpoint == "" {
		ctx.ClusterConfig.ClusterConfiguration.ControlPlaneEndpoint = controlPlaneEndpoint
		ctx.Logger.V(6).Info("stored control plane endpoint in kubeadm cluster config", "control-plane-endpoint", controlPlaneEndpoint)
	}
	ctx.ClusterStatus.Ready = true
	if ctx.Cluster.Annotations == nil {
		ctx.Cluster.Annotations = map[string]string{}
	}
	ctx.Cluster.Annotations[constants.ReadyAnnotationLabel] = "true"
	ctx.Logger.V(6).Info("cluster is ready")

	return nil
}

// reconcileKubeConfig creates a Kubernetes Secret holding the kubeconfig
// for a cluster if the secret does not already exist
func (a *Actuator) reconcileKubeConfig(ctx *context.ClusterContext) error {
	cluster := ctx.Cluster

	kubeConfig, err := a.GetKubeConfig(cluster, nil)
	if err != nil {
		return errors.Wrap(err, "error generating kubeconfig")
	}

	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-kubeconfig", cluster.Name),
		},
		StringData: map[string]string{
			"value": kubeConfig,
		},
	}

	if _, err := a.coreClient.Secrets(cluster.Namespace).Create(secret); err != nil &&
		!apierrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "error creating kubeconfig secret")
	}

	return nil
}

func (a *Actuator) deleteKubeConfig(ctx *context.ClusterContext) error {
	secretName := fmt.Sprintf("%s-kubeconfig", ctx.Cluster.Name)
	return a.coreClient.Secrets(ctx.Cluster.Namespace).Delete(secretName, &metav1.DeleteOptions{})
}
