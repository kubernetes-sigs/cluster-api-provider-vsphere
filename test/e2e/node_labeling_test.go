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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/cluster-api/util"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/constants"
)

type NodeLabelingSpecInput struct {
	InfraClients
	Global    GlobalInput
	SpecName  string
	Namespace *corev1.Namespace
}

var _ = Describe("Label nodes with ESXi host info", func() {
	const specName = "node-labeling"
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		var (
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
			namespace = setupSpecNamespace("node-labeling-e2e")
		})

		AfterEach(func() {
			cleanupSpecNamespace(namespace)
		})

		It("creates a workload cluster whose nodes have the ESXi host info", func() {
			VerifyNodeLabeling(ctx, NodeLabelingSpecInput{
				SpecName:  specName,
				Namespace: namespace,
				InfraClients: InfraClients{
					Client:     vsphereClient,
					RestClient: restClient,
					Finder:     vsphereFinder,
				},
				Global: GlobalInput{
					BootstrapClusterProxy: bootstrapClusterProxy,
					ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
					E2EConfig:             e2eConfig,
					ArtifactFolder:        artifactFolder,
				},
			})
		})
	})
})

func VerifyNodeLabeling(ctx context.Context, input NodeLabelingSpecInput) {
	var (
		specName         = input.SpecName
		namespace        = input.Namespace
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	)

	clusterName := fmt.Sprintf("%s-%s", specName, util.RandomString(6))
	By("creating a workload cluster")
	configCluster := defaultConfigCluster(clusterName, namespace.Name, "", 1, 1, input.Global)

	clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy:                 input.Global.BootstrapClusterProxy,
		ConfigCluster:                configCluster,
		WaitForClusterIntervals:      input.Global.E2EConfig.GetIntervals("", "wait-cluster"),
		WaitForControlPlaneIntervals: input.Global.E2EConfig.GetIntervals("", "wait-control-plane"),
		WaitForMachineDeployments:    input.Global.E2EConfig.GetIntervals("", "wait-worker-nodes"),
	}, clusterResources)
	workloadProxy := input.Global.BootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterResources.Cluster.Name)

	Byf("fetching the nodes of the workload cluster %s", clusterName)
	nodes := corev1.NodeList{}
	Expect(workloadProxy.GetClient().List(ctx, &nodes)).To(Succeed())
	nodeMap := map[string]*corev1.Node{}
	for _, node := range nodes.Items {
		nodeMap[node.Name] = node.DeepCopy()
	}

	Byf("fetching the VSphereVM objects for the workload cluster %s", clusterName)
	vms := getVSphereVMsForCluster(clusterName, namespace.Name)
	Expect(vms.Items).To(HaveLen(len(nodeMap)))

	By("verifying the ESXi host info label on the nodes")
	for _, vm := range vms.Items {
		// since the name of the VSphereVM is the name of the K8s node.
		Expect(nodeMap).To(HaveKey(vm.Name))
		labels := nodeMap[vm.Name].GetLabels()
		Expect(labels).To(HaveKey(constants.ESXiHostInfoLabel))
		Expect(labels).To(HaveKeyWithValue(constants.ESXiHostInfoLabel, vm.Status.Host))
	}

	By("verifying the ESXi host information from the virtual machines")
	for _, vm := range vms.Items {
		vmObj, err := input.Finder.VirtualMachine(ctx, vm.Name)
		Expect(err).ToNot(HaveOccurred())

		host, err := vmObj.HostSystem(ctx)
		Expect(err).ToNot(HaveOccurred())

		name, err := host.ObjectName(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(name).To(Equal(vm.Status.Host))
	}
}
