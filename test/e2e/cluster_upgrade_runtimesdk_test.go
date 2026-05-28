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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blang/semver/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	clusterctlcluster "sigs.k8s.io/cluster-api/cmd/clusterctl/client/cluster"
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
					framework.ValidateResourceVersionStable(ctx, framework.ValidateResourceVersionStableInput{
						ClusterProxy:             proxy,
						Namespace:                namespace,
						OwnerGraphFilterFunction: TMPDropVSphereMachineAndFilterObjectsWithKindAndName(clusterName),
					})
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

var _ = Describe("When performing chained upgrades for workload cluster using ClusterClass in a different NS with RuntimeSDK [vcsim] [supervisor] [ClusterClass]", Label("ClusterClass"), func() {
	const specName = "k8s-upgrade-with-runtimesdk-chained" // prefix (k8s-upgrade-with-runtimesdk) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterUpgradeWithRuntimeSDKSpec(ctx, func() capi_e2e.ClusterUpgradeWithRuntimeSDKSpecInput {
			return capi_e2e.ClusterUpgradeWithRuntimeSDKSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				PostUpgrade: func(proxy framework.ClusterProxy, namespace, clusterName string) {
					// This check ensures that the resourceVersions are stable, i.e. it verifies there are no
					// continuous reconciles when everything should be stable.
					resourceVersionInput := framework.ValidateResourceVersionStableInput{
						ClusterProxy:             proxy,
						Namespace:                namespace,
						OwnerGraphFilterFunction: clusterctlcluster.FilterClusterObjectsWithNameFilter(clusterName),
						WaitToBecomeStable:       e2eConfig.GetIntervals(specName, "wait-resource-versions-become-stable"),
						WaitToRemainStable:       e2eConfig.GetIntervals(specName, "wait-resource-versions-remain-stable"),
					}
					framework.ValidateResourceVersionStable(ctx, resourceVersionInput)
				},
				// "topology-runtimesdk" is the same as the "topology" flavor but with an additional RuntimeExtension.
				Flavor:                                ptr.To(testSpecificSettingsGetter().FlavorForMode("topology-runtimesdk")),
				DeployClusterClassInSeparateNamespace: true,
				// Setting Kubernetes version from
				KubernetesVersionFrom: e2eConfig.MustGetVariable(KubernetesVersionChainedUpgradeFrom),
				// Build a list of Kubernetes version with at least one version in between KubernetesVersionChainedUpgradeFrom and KubernetesVersionUpgradeTo.
				// NOTE: this relies on the fact that CAPV maintainers are publishing a .0 image for each Kubernetes version.
				KubernetesVersions: getKubernetesVersions(e2eConfig.MustGetVariable(KubernetesVersionChainedUpgradeFrom), e2eConfig.MustGetVariable(KubernetesVersionUpgradeTo)),
				// The runtime extension gets deployed to the test-extension-system namespace and is exposed
				// by the test-extension-webhook-service.
				// The below values are used when creating the cluster-wide ExtensionConfig to refer
				// the actual service.
				ExtensionServiceNamespace: "capv-test-extension",
				ExtensionServiceName:      "capv-test-extension-webhook-service",
				ExtensionConfigName:       "k8s-chained-upgrade-with-runtimesdk-cross-ns",
				PostNamespaceCreated:      testSpecificSettingsGetter().PostNamespaceCreatedFunc,
			}
		})
	})
})

func getKubernetesVersions(from, to string) []string {
	fromMinor := semver.MustParse(strings.TrimPrefix(from, "v")).Minor
	toMinor := semver.MustParse(strings.TrimPrefix(to, "v")).Minor

	versions := []string{from}
	for i := fromMinor + 1; i <= toMinor; i++ {
		if i == toMinor {
			versions = append(versions, to)
			break
		}
		versions = append(versions, fmt.Sprintf("v1.%d.0", i))
	}
	return versions
}
