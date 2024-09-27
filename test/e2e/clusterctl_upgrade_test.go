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
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/test/framework/kubernetesversions"

	"sigs.k8s.io/cluster-api-provider-vsphere/test/framework/vcsim"
)

var (
	clusterctlDownloadURL   = "https://github.com/kubernetes-sigs/cluster-api/releases/download/v%s/clusterctl-{OS}-{ARCH}"
	providerCAPIPrefix      = "cluster-api:v%s"
	providerKubeadmPrefix   = "kubeadm:v%s"
	providerVSpherePrefix   = "vsphere:v%s"
	capiReleaseMarkerPrefix = "go://sigs.k8s.io/cluster-api@v%s"
	capvReleaseMarkerPrefix = "go://sigs.k8s.io/cluster-api-provider-vsphere@v%s"
)

// Note: This test should be changed during "prepare main branch", it should test CAPV n-1 => current (and then corresponding CAPI versions if already available).
var _ = Describe("When testing clusterctl upgrades using ClusterClass (CAPV 1.11=>current, CAPI 1.8=>1.9) on K8S latest ci mgmt cluster [vcsim] [supervisor] [ClusterClass]", func() {
	const specName = "clusterctl-upgrade-1.11-current-latest-ci" // prefix (clusterctl-upgrade) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
			capiVersion := "1.8"
			capiStableRelease, err := getStableReleaseOfMinor(ctx, capiReleaseMarkerPrefix, capiVersion)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capiVersion)
			capvVersion := "1.11"
			capvStableRelease, err := getStableReleaseOfMinor(ctx, capvReleaseMarkerPrefix, capvVersion)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capvVersion)
			initKubernetesVersion, err := kubernetesversions.ResolveVersion(ctx, e2eConfig.GetVariable("KUBERNETES_VERSION_LATEST_CI"))
			Expect(err).ToNot(HaveOccurred())
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                         e2eConfig,
				ClusterctlConfigPath:              testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:             bootstrapClusterProxy,
				ArtifactFolder:                    artifactFolder,
				SkipCleanup:                       skipCleanup,
				MgmtFlavor:                        testSpecificSettingsGetter().FlavorForMode("topology"),
				PostNamespaceCreated:              testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				InitWithBinary:                    fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
				InitWithCoreProvider:              fmt.Sprintf(providerCAPIPrefix, capiStableRelease),
				InitWithBootstrapProviders:        []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
				InitWithControlPlaneProviders:     []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
				InitWithInfrastructureProviders:   []string{fmt.Sprintf(providerVSpherePrefix, capvStableRelease)},
				InitWithRuntimeExtensionProviders: testSpecificSettingsGetter().RuntimeExtensionProviders,
				InitWithIPAMProviders:             []string{},
				// InitWithKubernetesVersion should be the highest kubernetes version supported by the init Cluster API version.
				// This is to guarantee that both, the old and new CAPI version, support the defined version.
				// Ensure all Kubernetes versions used here are covered in patch-vsphere-template.yaml
				InitWithKubernetesVersion: initKubernetesVersion,
				WorkloadKubernetesVersion: "v1.31.0",
				WorkloadFlavor:            testSpecificSettingsGetter().FlavorForMode("workload"),
				// We are using a separate management cluster. For running in VCSim we also have to pass WithAdditionalVCSimServer
				// below otherwise there will be no VCSim instance created in the management cluster.
				UseKindForManagementCluster:              true,
				KindManagementClusterNewClusterProxyFunc: kindManagementClusterNewClusterProxyFunc,
			}
		})
	},
		WithIP("WORKLOAD_CONTROL_PLANE_ENDPOINT_IP"),
		// This is required because we are using a separate management cluster with kind by passing `UseKindForManagementCluster` above.
		WithAdditionalVCSimServer(true),
	)
})

// Note: This test should be changed during "prepare main branch", it should test CAPV n-1 => current (and then corresponding CAPI versions if already available).
var _ = Describe("When testing clusterctl upgrades using ClusterClass (CAPV 1.11=>current, CAPI 1.8=>1.9) [vcsim] [supervisor] [ClusterClass]", func() {
	const specName = "clusterctl-upgrade-1.11-current" // prefix (clusterctl-upgrade) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
			capiVersion := "1.8"
			capiStableRelease, err := getStableReleaseOfMinor(ctx, capiReleaseMarkerPrefix, capiVersion)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capiVersion)
			capvVersion := "1.11"
			capvStableRelease, err := getStableReleaseOfMinor(ctx, capvReleaseMarkerPrefix, capvVersion)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capvVersion)
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                         e2eConfig,
				ClusterctlConfigPath:              testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:             bootstrapClusterProxy,
				ArtifactFolder:                    artifactFolder,
				SkipCleanup:                       skipCleanup,
				MgmtFlavor:                        testSpecificSettingsGetter().FlavorForMode("topology"),
				PostNamespaceCreated:              testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				InitWithBinary:                    fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
				InitWithCoreProvider:              fmt.Sprintf(providerCAPIPrefix, capiStableRelease),
				InitWithBootstrapProviders:        []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
				InitWithControlPlaneProviders:     []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
				InitWithInfrastructureProviders:   []string{fmt.Sprintf(providerVSpherePrefix, capvStableRelease)},
				InitWithRuntimeExtensionProviders: testSpecificSettingsGetter().RuntimeExtensionProviders,
				InitWithIPAMProviders:             []string{},
				// InitWithKubernetesVersion should be the highest kubernetes version supported by the init Cluster API version.
				// This is to guarantee that both, the old and new CAPI version, support the defined version.
				// Ensure all Kubernetes versions used here are covered in patch-vsphere-template.yaml
				InitWithKubernetesVersion: "v1.31.0",
				WorkloadKubernetesVersion: "v1.31.0",
				WorkloadFlavor:            testSpecificSettingsGetter().FlavorForMode("workload"),
				// We are using a separate management cluster. For running in VCSim we also have to pass WithAdditionalVCSimServer
				// below otherwise there will be no VCSim instance created in the management cluster.
				UseKindForManagementCluster:              true,
				KindManagementClusterNewClusterProxyFunc: kindManagementClusterNewClusterProxyFunc,
			}
		})
	},
		WithIP("WORKLOAD_CONTROL_PLANE_ENDPOINT_IP"),
		// This is required because we are using a separate management cluster with kind by passing `UseKindForManagementCluster` above.
		WithAdditionalVCSimServer(true),
	)
})

// Note: This test should be changed during "prepare main branch", it should test CAPV n-2 => current (and then corresponding CAPI versions if already available).
var _ = Describe("When testing clusterctl upgrades using ClusterClass (CAPV 1.10=>current, CAPI 1.7=>1.9) [vcsim] [supervisor] [ClusterClass]", func() {
	const specName = "clusterctl-upgrade-1.10-current" // prefix (clusterctl-upgrade) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
			capiVersion := "1.7"
			capiStableRelease, err := getStableReleaseOfMinor(ctx, capiReleaseMarkerPrefix, capiVersion)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capiVersion)
			capvVersion := "1.10"
			capvStableRelease, err := getStableReleaseOfMinor(ctx, capvReleaseMarkerPrefix, capvVersion)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capvVersion)
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                         e2eConfig,
				ClusterctlConfigPath:              testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:             bootstrapClusterProxy,
				ArtifactFolder:                    artifactFolder,
				SkipCleanup:                       skipCleanup,
				MgmtFlavor:                        testSpecificSettingsGetter().FlavorForMode("topology"),
				PostNamespaceCreated:              testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				InitWithBinary:                    fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
				InitWithCoreProvider:              fmt.Sprintf(providerCAPIPrefix, capiStableRelease),
				InitWithBootstrapProviders:        []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
				InitWithControlPlaneProviders:     []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
				InitWithInfrastructureProviders:   []string{fmt.Sprintf(providerVSpherePrefix, capvStableRelease)},
				InitWithRuntimeExtensionProviders: testSpecificSettingsGetter().RuntimeExtensionProviders,
				InitWithIPAMProviders:             []string{},
				// InitWithKubernetesVersion should be the highest kubernetes version supported by the init Cluster API version.
				// This is to guarantee that both, the old and new CAPI version, support the defined version.
				// Ensure all Kubernetes versions used here are covered in patch-vsphere-template.yaml
				InitWithKubernetesVersion: "v1.30.0",
				WorkloadKubernetesVersion: "v1.30.0",
				WorkloadFlavor:            testSpecificSettingsGetter().FlavorForMode("workload"),
				// We are using a separate management cluster. For running in VCSim we also have to pass WithAdditionalVCSimServer
				// below otherwise there will be no VCSim instance created in the management cluster.
				UseKindForManagementCluster:              true,
				KindManagementClusterNewClusterProxyFunc: kindManagementClusterNewClusterProxyFunc,
			}
		})
	},
		WithIP("WORKLOAD_CONTROL_PLANE_ENDPOINT_IP"),
		// This is required because we are using a separate management cluster with kind by passing `UseKindForManagementCluster` above.
		WithAdditionalVCSimServer(true),
	)
})

// Note: This test should be changed during "prepare main branch", it should test CAPV n-3 => current (and then corresponding CAPI versions if already available).
var _ = Describe("When testing clusterctl upgrades using ClusterClass (CAPV 1.9=>current, CAPI 1.6=>1.9) [vcsim] [supervisor] [ClusterClass]", func() {
	const specName = "clusterctl-upgrade-1.9-current" // prefix (clusterctl-upgrade) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
			capiVersion := "1.6"
			capiStableRelease, err := getStableReleaseOfMinor(ctx, capiReleaseMarkerPrefix, capiVersion)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capiVersion)
			capvVersion := "1.9"
			capvStableRelease, err := getStableReleaseOfMinor(ctx, capvReleaseMarkerPrefix, capvVersion)
			Expect(err).ToNot(HaveOccurred(), "Failed to get stable version for minor release : %s", capvVersion)
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                         e2eConfig,
				ClusterctlConfigPath:              testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:             bootstrapClusterProxy,
				ArtifactFolder:                    artifactFolder,
				SkipCleanup:                       skipCleanup,
				MgmtFlavor:                        testSpecificSettingsGetter().FlavorForMode("topology"),
				PostNamespaceCreated:              testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				InitWithBinary:                    fmt.Sprintf(clusterctlDownloadURL, capiStableRelease),
				InitWithCoreProvider:              fmt.Sprintf(providerCAPIPrefix, capiStableRelease),
				InitWithBootstrapProviders:        []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
				InitWithControlPlaneProviders:     []string{fmt.Sprintf(providerKubeadmPrefix, capiStableRelease)},
				InitWithInfrastructureProviders:   []string{fmt.Sprintf(providerVSpherePrefix, capvStableRelease)},
				InitWithRuntimeExtensionProviders: testSpecificSettingsGetter().RuntimeExtensionProviders,
				InitWithIPAMProviders:             []string{},
				// InitWithKubernetesVersion should be the highest kubernetes version supported by the init Cluster API version.
				// This is to guarantee that both, the old and new CAPI version, support the defined version.
				// Ensure all Kubernetes versions used here are covered in patch-vsphere-template.yaml
				InitWithKubernetesVersion: "v1.29.0",
				WorkloadKubernetesVersion: "v1.29.0",
				WorkloadFlavor:            testSpecificSettingsGetter().FlavorForMode("workload"),
				// We are using a separate management cluster. For running in VCSim we also have to pass WithAdditionalVCSimServer
				// below otherwise there will be no VCSim instance created in the management cluster.
				UseKindForManagementCluster:              true,
				KindManagementClusterNewClusterProxyFunc: kindManagementClusterNewClusterProxyFunc,
			}
		})
	},
		WithIP("WORKLOAD_CONTROL_PLANE_ENDPOINT_IP"),
		// This is required because we are using a separate management cluster with kind by passing `UseKindForManagementCluster` above.
		WithAdditionalVCSimServer(true),
	)
})

// getStableReleaseOfMinor returns the latest stable version of minorRelease.
func getStableReleaseOfMinor(ctx context.Context, releaseMarkerPrefix, minorRelease string) (string, error) {
	releaseMarker := fmt.Sprintf(releaseMarkerPrefix, minorRelease)
	return clusterctl.ResolveRelease(ctx, releaseMarker)
}

func kindManagementClusterNewClusterProxyFunc(name string, kubeconfigPath string) framework.ClusterProxy {
	if testTarget == VCSimTestTarget {
		return vcsim.NewClusterProxy(name, kubeconfigPath, initScheme())
	}
	return framework.NewClusterProxy(name, kubeconfigPath, initScheme())
}
