package integration

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/integration/utils"
)

var _ = Describe("environment clean", func() {
	Context("clean cluster", func() {
		var ctools *utils.ClusterTools
		var err error

		It("kubeconfig for target cluster available locally", func() {
			ctools, err = utils.NewClusterToolsFromEnv()
			Expect(err).Should(BeNil())
		})

		It("have Cluster API pods running on cluster", func() {
			if ctools == nil {
				Skip("Unable to proceed with test because unable to create clients for clusters")
			}

			e := ctools.PodExist(VsphereProviderPodName, VsphereProviderNamespace)
			Expect(e).Should(BeFalse())
			e = ctools.PodExist(ClusterApiPodName, ClusterApiNamespace)
			Expect(e).Should(BeFalse())

			e = ctools.PodRunning(VsphereProviderPodName, VsphereProviderNamespace)
			Expect(e).Should(BeFalse())
			e = ctools.PodRunning(ClusterApiPodName, ClusterApiNamespace)
			Expect(e).Should(BeFalse())
		})
	})
})
