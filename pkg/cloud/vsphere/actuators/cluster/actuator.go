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
	"github.com/pkg/errors"

	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clientv1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/actuators"
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
		Cluster:               cluster,
		Client:                a.client,
		CoreClient:            a.coreClient,
		Logger:                klogr.New().WithName("[cluster-actuator]"),
		GetControlPlaneStatus: kubeclient.GetControlPlaneStatus,
	})
	if err != nil {
		return err
	}

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Reconcile", opErr)
	}()

	ctx.Logger.V(6).Info("reconciling cluster")

	// Ensure the PKI config is present or generated and then set the updated
	// clusterConfig back onto the cluster.
	if err := certificates.ReconcileCertificates(ctx); err != nil {
		return errors.Wrapf(err, "unable to reconcile certs while reconciling cluster %q", ctx)
	}

	return nil
}

// Delete will delete any cluster level resources for the cluster.
func (a *Actuator) Delete(cluster *clusterv1.Cluster) (opErr error) {
	ctx, err := context.NewClusterContext(&context.ClusterContextParams{
		Cluster:               cluster,
		Client:                a.client,
		CoreClient:            a.coreClient,
		GetControlPlaneStatus: kubeclient.GetControlPlaneStatus,
	})
	if err != nil {
		return err
	}

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Delete", opErr)
	}()

	ctx.Logger.V(2).Info("deleting cluster")

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
	machineContext, err := context.NewMachineContextFromClusterContext(clusterContext, machine)
	if err != nil {
		return "", err
	}
	return kubeclient.GetKubeConfig(machineContext)
}
