/*
Copyright 2024 The Kubernetes Authors.

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
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
)

var _ = Describe("ClusterClass Creation using Cluster API quick-start test and IPAM Provider [ClusterClass]", func() {
	const specName = "ipam-cluster-class"
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.QuickStartSpec(ctx, func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				SkipCleanup:              skipCleanup,
				Flavor:                   ptr.To(testSpecificSettingsGetter().FlavorForMode("ipam")),
				PostNamespaceCreated:     testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				ControlPlaneMachineCount: ptr.To[int64](1),
				WorkerMachineCount:       ptr.To[int64](1),
			}
		})
	},
		// Set the WithGateway option to write the gateway ip address to the variable.
		// This variable is required for creating the InClusterIPPool for the ipam provider.
		WithGateway("IPAM_GATEWAY"),
		// Claim two IPs from the CI's IPAM provider to use in the InClusterIPPool of
		// the ipam provider. The IPs then get claimed during provisioning to configure
		// static IPs for the control-plane and worker node.
		WithIP("IPAM_IP_1"), WithIP("IPAM_IP_2"))
})
