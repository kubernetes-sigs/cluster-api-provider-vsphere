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
	"sigs.k8s.io/cluster-api/test/framework"
)

var _ = Describe("When using the autoscaler with Cluster API using ClusterClass and scale to zero [supervisor] [ClusterClass]", func() {
	const specName = "autoscaler" // aligned to CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.AutoscalerSpec(ctx, func() capi_e2e.AutoscalerSpecInput {
			return capi_e2e.AutoscalerSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				Flavor:                ptr.To(testSpecificSettingsGetter().FlavorForMode("topology-autoscaler")),
				PostNamespaceCreated: func(managementClusterProxy framework.ClusterProxy, workloadClusterNamespace string) {
					if testMode == GovmomiTestMode {
						// This test is only implemented for supervisor
						Skip("This test is only implemented for supervisor")
					}
					testSpecificSettingsGetter().PostNamespaceCreatedFunc(managementClusterProxy, workloadClusterNamespace)
				},
				InfrastructureAPIGroup:            "vmware.infrastructure.cluster.x-k8s.io",
				InfrastructureMachineTemplateKind: "vspheremachinetemplates",
				AutoscalerVersion:                 "v1.31.0",
				ScaleToAndFromZero:                true,
				// We have no connectivity from the workload cluster to the kind management cluster in CI so we
				// can't deploy the autoscaler to the workload cluster.
				InstallOnManagementCluster: true,
			}
		})
	})
})
