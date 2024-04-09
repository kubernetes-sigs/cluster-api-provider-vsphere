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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/kubernetesversions"
)

var _ = Describe("When testing K8S conformance [supervisor] [Conformance] [K8s-Install]", func() {
	// Note: This installs a cluster based on KUBERNETES_VERSION and runs conformance tests.
	const specName = "k8s-conformance" // copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.K8SConformanceSpec(ctx, func() capi_e2e.K8SConformanceSpecInput {
			return capi_e2e.K8SConformanceSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				Flavor:                testSpecificSettingsGetter().FlavorForMode("conformance"),
				PostNamespaceCreated:  testSpecificSettingsGetter().PostNamespaceCreatedFunc,
			}
		})
	})
})

var _ = Describe("When testing K8S conformance with K8S latest ci [supervisor] [Conformance] [K8s-Install-ci-latest]", func() {
	// Note: This installs a cluster based on KUBERNETES_VERSION_LATEST_CI and runs conformance tests.
	// Note: We are resolving KUBERNETES_VERSION_LATEST_CI and then setting the resolved version as
	// KUBERNETES_VERSION env var. This only works without side effects on other tests because we are
	// running this test in its separate job.
	const specName = "k8s-conformance-ci-latest" // prefix copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.K8SConformanceSpec(ctx, func() capi_e2e.K8SConformanceSpecInput {
			kubernetesVersion, err := kubernetesversions.ResolveVersion(ctx, e2eConfig.GetVariable("KUBERNETES_VERSION_LATEST_CI"))
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Setenv("KUBERNETES_VERSION", kubernetesVersion)).To(Succeed())
			return capi_e2e.K8SConformanceSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				// Note: install-on-bootstrap will install Kubernetes on bootstrap if the correct Kubernetes version
				// cannot be detected. This is required to install versions we don't have images for (e.g. ci/latest-1.30).
				Flavor:               testSpecificSettingsGetter().FlavorForMode("install-on-bootstrap"),
				PostNamespaceCreated: testSpecificSettingsGetter().PostNamespaceCreatedFunc,
			}
		})
	})
})
