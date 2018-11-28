/*
Copyright 2018 The Kubernetes Authors.

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

package integration

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/integration/utils"
)

// Run this suite with the env var TEST_KUBECONFIG defined with the path of
// the kubeconfig for the desired cluster.
var _ = Describe("Create single master cluster", func() {
	Context("single master cluster", func() {
		var ctools *utils.ClusterTools
		var cluster, namespace string
		var err error

		It("Suite preconditions", func() {
			cluster = os.Getenv(ClusterNameEnv)
			Expect(cluster).ShouldNot(BeEmpty())

			namespace = os.Getenv(ClusterNameSpaceEnv)
			Expect(namespace).ShouldNot(BeEmpty())

			ctools, err = utils.NewClusterToolsFromEnv()
			Expect(err).Should(BeNil())
		})

		It("have Cluster API pods running on target cluster", func() {
			if ctools == nil {
				Skip("Unable to proceed with test because unable to create clients for clusters")
			}

			e := ctools.PodExist(VsphereProviderPodName, VsphereProviderNamespace)
			Expect(e).Should(BeTrue())
			e = ctools.PodExist(ClusterApiPodName, ClusterApiNamespace)
			Expect(e).Should(BeTrue())

			e = ctools.PodRunning(VsphereProviderPodName, VsphereProviderNamespace)
			Expect(e).Should(BeTrue())
			e = ctools.PodRunning(ClusterApiPodName, ClusterApiNamespace)
			Expect(e).Should(BeTrue())
		})

		It("have a cluster", func() {
			if ctools == nil || namespace == "" || cluster == "" {
				Skip("Unable to proceed with test because unable to create clients for clusters")
			}

			e := ctools.ClusterExist(cluster, namespace)
			Expect(e).ShouldNot(BeNil())
		})

		It("have 1 machines, 0 machineset, 0 machinedeployment", func() {
			if ctools == nil || namespace == "" {
				Skip("Unable to proceed with test because unable to create clients for clusters")
			}

			count := ctools.MachineCount(namespace)
			Expect(count).Should(Equal(1))

			count = ctools.MachineSetsCount(namespace)
			Expect(count).Should(BeZero())

			count = ctools.MachineDeploymentsCount(namespace)
			Expect(count).Should(BeZero())
		})

		It("All machines have IP Addresses", func() {
			if ctools == nil || namespace == "" {
				Skip("Unable to proceed with test because unable to create clients for clusters")
			}

			Expect(ctools.AllMachinesHaveIp(namespace)).Should(BeTrue())
		})
	})
})
