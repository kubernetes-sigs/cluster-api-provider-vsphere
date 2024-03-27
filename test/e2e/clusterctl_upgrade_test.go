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
	. "github.com/onsi/ginkgo/v2"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
)

var _ = Describe("When testing clusterctl upgrades using ClusterClass (CAPV 1.9=>current, CAPI 1.6=>1.7) [ClusterClass]", func() {
	const specName = "clusterctl-upgrade-1.9-current" // prefix (clusterctl-upgrade) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ClusterctlUpgradeSpec(ctx, func() capi_e2e.ClusterctlUpgradeSpecInput {
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                         e2eConfig,
				ClusterctlConfigPath:              testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:             bootstrapClusterProxy,
				ArtifactFolder:                    artifactFolder,
				SkipCleanup:                       skipCleanup,
				MgmtFlavor:                        testSpecificSettingsGetter().FlavorForMode("remote-management"),
				PostNamespaceCreated:              testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				InitWithBinary:                    "https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.6.1/clusterctl-{OS}-{ARCH}",
				InitWithCoreProvider:              "cluster-api:v1.6.1",
				InitWithBootstrapProviders:        []string{"kubeadm:v1.6.1"},
				InitWithControlPlaneProviders:     []string{"kubeadm:v1.6.1"},
				InitWithInfrastructureProviders:   []string{"vsphere:v1.9.0"},
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
			return capi_e2e.ClusterctlUpgradeSpecInput{
				E2EConfig:                         e2eConfig,
				ClusterctlConfigPath:              testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:             bootstrapClusterProxy,
				ArtifactFolder:                    artifactFolder,
				SkipCleanup:                       skipCleanup,
				MgmtFlavor:                        testSpecificSettingsGetter().FlavorForMode("remote-management"),
				PostNamespaceCreated:              testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				InitWithBinary:                    "https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.5.4/clusterctl-{OS}-{ARCH}",
				InitWithCoreProvider:              "cluster-api:v1.5.4",
				InitWithBootstrapProviders:        []string{"kubeadm:v1.5.4"},
				InitWithControlPlaneProviders:     []string{"kubeadm:v1.5.4"},
				InitWithInfrastructureProviders:   []string{"vsphere:v1.8.4"},
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
