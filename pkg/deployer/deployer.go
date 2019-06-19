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

package deployer

import (
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeclient"
)

// Deployer satisfies the ProviderDeployer (https://github.com/kubernetes-sigs/cluster-api/blob/master/cmd/clusterctl/clusterdeployer/clusterdeployer.go) interface.
type Deployer struct{}

// GetIP returns the control plane endpoint for the cluster.
func (d Deployer) GetIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	clusterContext, err := context.NewClusterContext(&context.ClusterContextParams{Cluster: cluster})
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
func (d Deployer) GetKubeConfig(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	clusterContext, err := context.NewClusterContext(&context.ClusterContextParams{Cluster: cluster})
	if err != nil {
		return "", err
	}
	machineContext, err := context.NewMachineContextFromClusterContext(clusterContext, machine)
	if err != nil {
		return "", err
	}
	return kubeclient.GetKubeConfig(machineContext)
}
