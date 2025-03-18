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
	"path/filepath"

	"github.com/blang/semver/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
)

var _ = Describe("When upgrading a workload cluster using ClusterClass with RuntimeSDK [vcsim] [supervisor] [ClusterClass]", func() {
	const specName = "k8s-upgrade-with-runtimesdk" // aligned to CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterUpgradeWithRuntimeSDKSpec(ctx, func() capi_e2e.ClusterUpgradeWithRuntimeSDKSpecInput {
			version, err := semver.ParseTolerant(e2eConfig.MustGetVariable(capi_e2e.KubernetesVersionUpgradeFrom))
			Expect(err).ToNot(HaveOccurred(), "Invalid argument, KUBERNETES_VERSION_UPGRADE_FROM is not a valid version")
			if version.LT(semver.MustParse("1.24.0")) {
				Fail("This test only supports upgrades from Kubernetes >= v1.24.0")
			}

			return capi_e2e.ClusterUpgradeWithRuntimeSDKSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				PostUpgrade: func(proxy framework.ClusterProxy, namespace, clusterName string) {
					// Dump all Cluster API related resources to artifacts before checking for resource versions being stable.
					framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
						Lister:               proxy.GetClient(),
						KubeConfigPath:       proxy.GetKubeconfigPath(),
						ClusterctlConfigPath: clusterctlConfigPath,
						Namespace:            namespace,
						LogPath:              filepath.Join(artifactFolder, "clusters-beforeValidateResourceVersions", proxy.GetName(), "resources"),
					})

					// This check ensures that the resourceVersions are stable, i.e. it verifies there are no
					// continuous reconciles when everything should be stable.
					framework.ValidateResourceVersionStable(ctx, proxy, namespace, FilterObjectsWithKindAndName(clusterName))
				},
				// "topology-runtimesdk" is the same as the "topology" flavor but with an additional RuntimeExtension.
				Flavor:                    ptr.To(testSpecificSettingsGetter().FlavorForMode("topology-runtimesdk")),
				ExtensionServiceNamespace: "capv-test-extension",
				ExtensionServiceName:      "capv-test-extension-webhook-service",
				PostNamespaceCreated:      testSpecificSettingsGetter().PostNamespaceCreatedFunc,
			}
		})
	})
})
