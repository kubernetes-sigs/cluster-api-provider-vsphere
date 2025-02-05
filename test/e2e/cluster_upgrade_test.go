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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/kubernetesversions"
)

var _ = Describe("When upgrading a workload cluster using ClusterClass and testing K8S conformance [supervisor] [Conformance] [K8s-Upgrade] [ClusterClass]", func() {
	// Note: This installs a cluster based on KUBERNETES_VERSION_UPGRADE_FROM and then upgrades to
	// KUBERNETES_VERSION_UPGRADE_TO and runs conformance tests.
	// Note: We are resolving KUBERNETES_VERSION_UPGRADE_FROM and KUBERNETES_VERSION_UPGRADE_TO and then setting
	// the resolved versions as env vars. This only works without side effects on other tests because we are
	// running this test in its separate job.
	const specName = "k8s-upgrade-and-conformance" // copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterUpgradeConformanceSpec(ctx, func() capi_e2e.ClusterUpgradeConformanceSpecInput {
			// The Kubernetes versions have to be resolved as they can be defined like this: stable-1.29, ci/latest-1.30.
			kubernetesVersionUpgradeFrom, err := kubernetesversions.ResolveVersion(ctx, e2eConfig.MustGetVariable("KUBERNETES_VERSION_UPGRADE_FROM"))
			Expect(err).NotTo(HaveOccurred())
			kubernetesVersionUpgradeTo, err := kubernetesversions.ResolveVersion(ctx, e2eConfig.MustGetVariable("KUBERNETES_VERSION_UPGRADE_TO"))
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Setenv("KUBERNETES_VERSION_UPGRADE_FROM", kubernetesVersionUpgradeFrom)).To(Succeed())
			Expect(os.Setenv("KUBERNETES_VERSION_UPGRADE_TO", kubernetesVersionUpgradeTo)).To(Succeed())
			return capi_e2e.ClusterUpgradeConformanceSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				WorkerMachineCount:    ptr.To[int64](5),
				Flavor:                ptr.To(testSpecificSettingsGetter().FlavorForMode("fast-rollout")),
				PostNamespaceCreated:  testSpecificSettingsGetter().PostNamespaceCreatedFunc,
			}
		})
	})
})

var _ = Describe("When upgrading a workload cluster using ClusterClass [vcsim] [supervisor] [ClusterClass]", func() {
	// Note: This installs a cluster based on KUBERNETES_VERSION_UPGRADE_FROM and then upgrades to
	// KUBERNETES_VERSION_UPGRADE_TO.
	const specName = "k8s-upgrade" // aligned to CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterUpgradeConformanceSpec(ctx, func() capi_e2e.ClusterUpgradeConformanceSpecInput {
			return capi_e2e.ClusterUpgradeConformanceSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				Flavor:                ptr.To(testSpecificSettingsGetter().FlavorForMode("topology")),
				PostNamespaceCreated:  testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				// This test is run in CI in parallel with other tests. To keep the test duration reasonable
				// the conformance tests are skipped.
				ControlPlaneMachineCount: ptr.To[int64](1),
				WorkerMachineCount:       ptr.To[int64](2),
				SkipConformanceTests:     true,
			}
		})
	})
})
