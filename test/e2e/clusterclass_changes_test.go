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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"

	"sigs.k8s.io/cluster-api-provider-vsphere/test/e2e/ipam"
)

var _ = Describe("When testing ClusterClass changes [ClusterClass]", func() {
	var (
		testSpecificClusterctlConfigPath string
		testSpecificIPAddressClaims      ipam.IPAddressClaims
	)
	BeforeEach(func() {
		testSpecificClusterctlConfigPath, testSpecificIPAddressClaims = ipamHelper.ClaimIPs(ctx, clusterctlConfigPath)
	})
	defer AfterEach(func() {
		Expect(ipamHelper.Cleanup(ctx, testSpecificIPAddressClaims)).To(Succeed())
	})

	capie2e.ClusterClassChangesSpec(ctx, func() capie2e.ClusterClassChangesSpecInput {
		return capie2e.ClusterClassChangesSpecInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  testSpecificClusterctlConfigPath,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ArtifactFolder:        artifactFolder,
			SkipCleanup:           skipCleanup,
			Flavor:                "topology",
			ModifyControlPlaneFields: map[string]interface{}{
				"spec.machineTemplate.nodeDrainTimeout": "10s",
			},
			ModifyMachineDeploymentBootstrapConfigTemplateFields: map[string]interface{}{
				"spec.template.spec.verbosity": int64(4),
			},
			ModifyMachineDeploymentInfrastructureMachineTemplateFields: map[string]interface{}{
				"spec.template.spec.numCPUs": int64(4),
			},
		}
	})
})
