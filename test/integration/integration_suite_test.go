package integration

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	ClusterApiPodName			= "cluster-api-controller-manager-0"
	ClusterApiNamespace			= "cluster-api-system"
	VsphereProviderPodName		= "vsphere-provider-controller-manager-0"
	VsphereProviderNamespace	= "vsphere-provider-system"

	ClusterNameEnv				= "TEST_CLUSTERNAME"
	ClusterNameSpaceEnv			= "TEST_CLUSTERNAMESPACE"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deployment Integration Suite")
}
