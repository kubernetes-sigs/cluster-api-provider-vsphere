/*
Copyright 2021 The Kubernetes Authors.

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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"

	vsphereframework "sigs.k8s.io/cluster-api-provider-vsphere/test/framework"
)

var (
	clusterctlDownloadURL   = "https://github.com/kubernetes-sigs/cluster-api/releases/download/v%s/clusterctl-{OS}-{ARCH}"
	providerCAPIPrefix      = "cluster-api:v%s"
	providerKubeadmPrefix   = "kubeadm:v%s"
	providerVSpherePrefix   = "vsphere:v%s"
	capiReleaseMarkerPrefix = "go://sigs.k8s.io/cluster-api@v%s"
	capvReleaseMarkerPrefix = "go://sigs.k8s.io/cluster-api-provider-vsphere@v%s"
)

var _ = Describe("When testing clusterctl upgrades using ClusterClass (CAPV 1.9=>current, CAPI 1.6=>1.7) [ClusterClass]", func() {
	const specName = "clusterctl-upgrade-1.9-current" // prefix (clusterctl-upgrade) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
			// Get CAPI v1.6 latest stable release
			capiVersion16 := "1.6"
			capiStableRelease16, err := getStableReleaseOfMinor(ctx, capiReleaseMarkerPrefix, capiVersion16)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capiVersion16)
			// Get CAPV v1.9 latest stable release
			capvVersion19 := "1.9"
			capvStableRelease19, err := getStableReleaseOfMinor(ctx, capvReleaseMarkerPrefix, capvVersion19)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capvVersion19)
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                         e2eConfig,
				ClusterctlConfigPath:              testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:             bootstrapClusterProxy,
				ArtifactFolder:                    artifactFolder,
				SkipCleanup:                       skipCleanup,
				MgmtFlavor:                        testSpecificSettingsGetter().FlavorForMode("topology"),
				PostNamespaceCreated:              testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				PreUpgrade:                        vsphereframework.LoadImagesFunc(ctx),
				InitWithBinary:                    fmt.Sprintf(clusterctlDownloadURL, capiStableRelease16),
				InitWithCoreProvider:              fmt.Sprintf(providerCAPIPrefix, capiStableRelease16),
				InitWithBootstrapProviders:        []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease16)},
				InitWithControlPlaneProviders:     []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease16)},
				InitWithInfrastructureProviders:   []string{fmt.Sprintf(providerVSpherePrefix, capvStableRelease19)},
				InitWithRuntimeExtensionProviders: []string{},
				InitWithIPAMProviders:             []string{},
				// InitWithKubernetesVersion should be the highest kubernetes version supported by the init Cluster API version.
				// This is to guarantee that both, the old and new CAPI version, support the defined version.
				// Ensure all Kubernetes versions used here are covered in patch-vsphere-template.yaml
				InitWithKubernetesVersion: "v1.29.0",
				WorkloadKubernetesVersion: "v1.29.0",
				WorkloadFlavor:            testSpecificSettingsGetter().FlavorForMode("workload"),
			}
		})
	}, WithIP("WORKLOAD_CONTROL_PLANE_ENDPOINT_IP"))
})

var _ = Describe("When testing clusterctl upgrades using ClusterClass (CAPV 1.8=>current, CAPI 1.5=>1.7) [ClusterClass]", func() {
	const specName = "clusterctl-upgrade-1.8-current" // prefix (clusterctl-upgrade) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
			// Get CAPI v1.5 latest stable release
			capiVersion15 := "1.5"
			capiStableRelease15, err := getStableReleaseOfMinor(ctx, capiReleaseMarkerPrefix, capiVersion15)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capiVersion15)
			// Get CAPV v1.8 latest stable release
			capvVersion18 := "1.8"
			capvStableRelease18, err := getStableReleaseOfMinor(ctx, capvReleaseMarkerPrefix, capvVersion18)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capvVersion18)
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                         e2eConfig,
				ClusterctlConfigPath:              testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:             bootstrapClusterProxy,
				ArtifactFolder:                    artifactFolder,
				SkipCleanup:                       skipCleanup,
				MgmtFlavor:                        testSpecificSettingsGetter().FlavorForMode("topology"),
				PostNamespaceCreated:              testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				PreUpgrade:                        vsphereframework.LoadImagesFunc(ctx),
				InitWithBinary:                    fmt.Sprintf(clusterctlDownloadURL, capiStableRelease15),
				InitWithCoreProvider:              fmt.Sprintf(providerCAPIPrefix, capiStableRelease15),
				InitWithBootstrapProviders:        []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease15)},
				InitWithControlPlaneProviders:     []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease15)},
				InitWithInfrastructureProviders:   []string{fmt.Sprintf(providerVSpherePrefix, capvStableRelease18)},
				InitWithRuntimeExtensionProviders: []string{},
				InitWithIPAMProviders:             []string{},
				// InitWithKubernetesVersion should be the highest kubernetes version supported by the init Cluster API version.
				// This is to guarantee that both, the old and new CAPI version, support the defined version.
				// Ensure all Kubernetes versions used here are covered in patch-vsphere-template.yaml
				InitWithKubernetesVersion: "v1.28.0",
				WorkloadKubernetesVersion: "v1.28.0",
				WorkloadFlavor:            testSpecificSettingsGetter().FlavorForMode("workload"),
			}
		})
	}, WithIP("WORKLOAD_CONTROL_PLANE_ENDPOINT_IP"))
})

// getStableReleaseOfMinor returns the latest stable version of minorRelease.
func getStableReleaseOfMinor(ctx context.Context, releaseMarkerPrefix, minorRelease string) (string, error) {
	releaseMarker := fmt.Sprintf(releaseMarkerPrefix, minorRelease)
	return clusterctl.ResolveRelease(ctx, releaseMarker)
}
