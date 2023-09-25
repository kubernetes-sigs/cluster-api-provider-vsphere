/*
Copyright 2020 The Kubernetes Authors.

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

// Package e2e contains end to end test code and utils.
package e2e

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vapi/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	KubernetesVersion = "KUBERNETES_VERSION"
)

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

type InfraClients struct {
	Client     *govmomi.Client
	RestClient *rest.Client
	Finder     *find.Finder
}

type GlobalInput struct {
	ArtifactFolder        string
	ClusterctlConfigPath  string
	BootstrapClusterProxy framework.ClusterProxy
	E2EConfig             *clusterctl.E2EConfig
}

func defaultConfigCluster(clusterName, namespace, flavor string, controlPlaneNodeCount, workerNodeCount int64,
	input GlobalInput) clusterctl.ConfigClusterInput {
	configClusterInput := clusterctl.ConfigClusterInput{
		LogFolder:                filepath.Join(input.ArtifactFolder, "clusters", input.BootstrapClusterProxy.GetName()),
		ClusterctlConfigPath:     input.ClusterctlConfigPath,
		KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
		InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
		Flavor:                   clusterctl.DefaultFlavor,
		Namespace:                namespace,
		ClusterName:              clusterName,
		KubernetesVersion:        input.E2EConfig.GetVariable(KubernetesVersion),
		ControlPlaneMachineCount: pointer.Int64(controlPlaneNodeCount),
		WorkerMachineCount:       pointer.Int64(workerNodeCount),
	}
	if flavor != "" {
		configClusterInput.Flavor = flavor
	}
	return configClusterInput
}
