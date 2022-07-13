/*
Copyright 2022 The Kubernetes Authors.

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

package e2e

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/kubescape"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	capiutil "sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Running a security scanner", func() {
	var namespace *v1.Namespace

	BeforeEach(func() {
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
		namespace = setupSpecNamespace("capv-e2e")
	})

	AfterEach(func() {
		cleanupSpecNamespace(namespace)
	})
	It("should create the cluster with worker nodes and run kubescape scanner on top of it", func() {
		clusterName := fmt.Sprintf("cluster-%s", capiutil.RandomString(6))

		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   clusterctl.DefaultFlavor,
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
				ControlPlaneMachineCount: pointer.Int64Ptr(1),
				WorkerMachineCount:       pointer.Int64Ptr(1),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals("", "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals("", "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals("", "wait-worker-nodes"),
		}, &clusterctl.ApplyClusterTemplateAndWaitResult{})

		By("Running a security scanner")
		RunningKubescapeScanner(clusterName, namespace.Name)

	})
})

func RunningKubescapeScanner(clusterName, namespace string) {
	scanResult := kubescape.KubescapeSpec(ctx, func() KubescapeSpecInput {
		return KubescapeSpecInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			Namespace:             namespace,
			ClusterName:           clusterName,
			FailThreshold:         e2eConfig.GetVariable(SecurityScanFailThreshold),
			Container:             e2eConfig.GetVariable(SecurityScanContainer),
			SkipCleanup:           skipCleanup,
		}
	})

}
