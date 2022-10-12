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
	"bytes"
	"fmt"
	"net"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

const (
	NodeIPAMPoolSubnet = "NODE_IPAM_POOL_SUBNET"
	NodeIPAMPoolStart  = "NODE_IPAM_POOL_START"
	NodeIPAMPoolEnd    = "NODE_IPAM_POOL_END"
)

var _ = Describe("Clusters using FromPools get assigned addresses from IPAM", func() {
	var namespace *v1.Namespace
	var start, end string

	BeforeEach(func() {
		start = e2eConfig.GetVariable(NodeIPAMPoolStart)
		Expect(net.ParseIP(start)).NotTo(BeNil())
		end = e2eConfig.GetVariable(NodeIPAMPoolEnd)
		Expect(net.ParseIP(end)).NotTo(BeNil())
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
		namespace = setupSpecNamespace("node-ipam")
	})

	AfterEach(func() {
		cleanupSpecNamespace(namespace)
	})

	It("should create a cluster successfully", func() {
		clusterName := fmt.Sprintf("cluster-%s", util.RandomString(6))

		By("creating a workload cluster")
		configCluster := ipamConfigCluster(clusterName, namespace.Name, 1, 1)

		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy:                 bootstrapClusterProxy,
			ConfigCluster:                configCluster,
			WaitForClusterIntervals:      e2eConfig.GetIntervals("", "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals("", "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals("", "wait-worker-nodes"),
		}, &clusterctl.ApplyClusterTemplateAndWaitResult{})

		By("Verifying that the cluster is using IPAM provided IP addresses")
		list := getVSphereVMsForCluster(clusterName, namespace.Name)
		Expect(list.Items).NotTo(BeEmpty())
		for _, vm := range list.Items {
			path := fmt.Sprintf("/%s/vm/%s/%s", vm.Spec.Datacenter, vm.Spec.Folder, vm.Name)
			vm, err := vsphereFinder.VirtualMachine(ctx, path)
			Expect(err).ShouldNot(HaveOccurred())
			ip, err := vm.WaitForIP(ctx, true)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(IPIsInRange(ip, start, end)).To(BeTrue(), fmt.Sprintf("Expected IP %q to be between %q and %q", ip, start, end))
		}
	})
})

func IPIsInRange(ip, rangeStart, rangeEnd string) bool {
	parsedIP := net.ParseIP(ip)
	Expect(parsedIP).NotTo(BeNil())

	parsedStart := net.ParseIP(rangeStart)
	Expect(parsedStart).NotTo(BeNil())

	parsedEnd := net.ParseIP(rangeEnd)
	Expect(parsedEnd).NotTo(BeNil())

	return bytes.Compare(parsedIP, parsedStart) >= 0 && bytes.Compare(parsedIP, parsedEnd) <= 0
}

func ipamConfigCluster(clusterName, namespace string, controlPlaneNodeCount, workerNodeCount int64) clusterctl.ConfigClusterInput {
	return clusterctl.ConfigClusterInput{
		LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
		ClusterctlConfigPath:     clusterctlConfigPath,
		KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
		InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
		Flavor:                   "ipam",
		Namespace:                namespace,
		ClusterName:              clusterName,
		KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
		ControlPlaneMachineCount: pointer.Int64Ptr(controlPlaneNodeCount),
		WorkerMachineCount:       pointer.Int64Ptr(workerNodeCount),
	}
}
