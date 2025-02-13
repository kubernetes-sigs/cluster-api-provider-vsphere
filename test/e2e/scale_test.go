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
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
)

const (
	scaleClusterControlPlaneEndpointIPPlaceholder   = "scale-cluster-controlplane-endpoint-ip-placeholder"
	scaleClusterControlPlaneEndpointPortPlaceholder = "scale-cluster-controlplane-endpoint-port-placeholder"
	scaleClusterVSphereServerPlaceholder            = "scale-cluster-vsphere-server-placeholder"
	scaleClusterVSphereTLSThumbprintPlaceholder     = "scale-cluster-vsphere-tls-thumbprint-placeholder"
	scaleClusterNamePlaceholder                     = "scale-cluster-name-placeholder"
)

var _ = Describe("When testing the machinery for scale testing using vcsim provider [vcsim] [supervisor] [ClusterClass]", func() {
	const specName = "scale" // prefix must be uniq because this test re-writes the clusterctl config.
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.ScaleSpec(ctx, func() capi_e2e.ScaleSpecInput {
			// For supporting real environments we would need to properly cleanup the namespaces and namespace specific ip allocations.
			if testTarget != VCSimTestTarget {
				Skip("Test should only be run using vcsim provider")
			}

			return capi_e2e.ScaleSpecInput{
				E2EConfig:              e2eConfig,
				ClusterctlConfigPath:   testSpecificSettingsGetter().ClusterctlConfigPath,
				InfrastructureProvider: ptr.To(clusterctl.DefaultInfrastructureProvider),
				BootstrapClusterProxy:  bootstrapClusterProxy,
				ArtifactFolder:         artifactFolder,
				Flavor:                 ptr.To(testSpecificSettingsGetter().FlavorForMode("topology-runtimesdk")),
				SkipUpgrade:            false,
				SkipCleanup:            skipCleanup,
				ClusterClassName:       getVariableOrFallback(testSpecificSettingsGetter().Variables["CLUSTER_CLASS_NAME"], e2eConfig.MustGetVariable("CLUSTER_CLASS_NAME")),

				// ClusterCount can be overwritten via `CAPI_SCALE_CLUSTER_COUNT`.
				ClusterCount: ptr.To[int64](5),
				// Concurrency can be overwritten via `CAPI_SCALE_CONCURRENCY`.
				Concurrency: ptr.To[int64](5),
				// ControlPlaneMachineCount can be overwritten via `CAPI_SCALE_CONTROL_PLANE_MACHINE_COUNT`.
				ControlPlaneMachineCount: ptr.To[int64](1),
				// MachineDeploymentCount can be overwritten via `CAPI_SCALE_MACHINE_DEPLOYMENT_COUNT`.
				MachineDeploymentCount: ptr.To[int64](1),
				// WorkerPerMachineDeploymentCount can be overwritten via `CAPI_SCALE_WORKER_PER_MACHINE_DEPLOYMENT_COUNT`.
				WorkerPerMachineDeploymentCount: ptr.To[int64](1),
				// AdditionalClusterClassCount can be overwritten via `CAPI_SCALE_ADDITIONAL_CLUSTER_CLASS_COUNT`.
				AdditionalClusterClassCount: ptr.To[int64](4),
				// DeployClusterInSeparateNamespaces can be overwritten via `CAPI_SCALE_DEPLOY_CLUSTER_IN_SEPARATE_NAMESPACES`.
				DeployClusterInSeparateNamespaces: ptr.To(true),
				// UseCrossNamespaceClusterClass can be overwritten via `CAPI_SCALE_USE_CROSS_NAMESPACE_CLUSTER_CLASS`.
				UseCrossNamespaceClusterClass: ptr.To(false),

				// The runtime extension gets deployed to the test-extension-system namespace and is exposed
				// by the test-extension-webhook-service.
				// The below values are used when creating the cluster-wide ExtensionConfig to refer
				// the actual service.
				ExtensionServiceNamespace: "capv-test-extension",
				ExtensionServiceName:      "capv-test-extension-webhook-service",
				ExtensionConfigName:       "capv-scale-test-extension",

				PostNamespaceCreated: func(managementClusterProxy framework.ClusterProxy, workloadClusterNamespace string) {
					testSpecificSettingsGetter().PostNamespaceCreatedFunc(managementClusterProxy, workloadClusterNamespace)

					// Overwrite some variables with placeholders which get replaced later on in PostScaleClusterNamespaceCreated
					testSpecificVariables := map[string]string{
						"VSPHERE_SERVER":              scaleClusterVSphereServerPlaceholder,
						"VSPHERE_TLS_THUMBPRINT":      scaleClusterVSphereTLSThumbprintPlaceholder,
						"CONTROL_PLANE_ENDPOINT_IP":   scaleClusterControlPlaneEndpointIPPlaceholder,
						"CONTROL_PLANE_ENDPOINT_PORT": scaleClusterControlPlaneEndpointPortPlaceholder,
					}
					// Re-write the clusterctl config file and add the new variables created above.
					Byf("Writing a new clusterctl config to %s", testSpecificSettingsGetter().ClusterctlConfigPath)
					Expect(clusterctl.CopyAndAmendClusterctlConfig(ctx, clusterctl.CopyAndAmendClusterctlConfigInput{
						ClusterctlConfigPath: testSpecificSettingsGetter().ClusterctlConfigPath,
						OutputPath:           testSpecificSettingsGetter().ClusterctlConfigPath,
						Variables:            testSpecificVariables,
					})).To(Succeed())
				},

				PostScaleClusterNamespaceCreated: func(clusterProxy framework.ClusterProxy, clusterNamespace string, clusterName string, clusterClassYAML []byte, clusterTemplateYAML []byte) ([]byte, []byte) {
					// Run additional initialization required for the namespace.
					if testMode == SupervisorTestMode {
						switch testTarget {
						case VCenterTestTarget:
							setupNamespaceWithVMOperatorDependenciesVCenter(clusterProxy, clusterNamespace)
						case VCSimTestTarget:
							setupNamespaceWithVMOperatorDependenciesVCSim(clusterProxy, clusterNamespace)
						}
					}

					// Allocate IP addresses from the correct ip address manager.
					// For vSphere target we use IPAM by the inClusterAddressManager via Boskos.
					// For VCSim we use the ControlPlaneEndpoint CRD inside the managementClusterProxy.
					// Note: for vSphere target we would need to implement a proper cleanup mechanism. This test currently only runs on vcSim though.
					_, testSpecificIPAddressClaims, testSpecificVariables := allocateIPAddresses(clusterProxy, &setupOptions{})

					// Get variables required when running on VCSim like VSphere Server address, user, etc.
					if testTarget == VCSimTestTarget {
						addVCSimTestVariables(clusterProxy, fmt.Sprintf("scale-%s", clusterName), testSpecificIPAddressClaims, testSpecificVariables, false)
					}

					clusterTemplateYAML = bytes.Replace(clusterTemplateYAML, []byte(scaleClusterControlPlaneEndpointIPPlaceholder), []byte(testSpecificVariables["CONTROL_PLANE_ENDPOINT_IP"]), -1)
					clusterTemplateYAML = bytes.Replace(clusterTemplateYAML, []byte(scaleClusterControlPlaneEndpointPortPlaceholder), []byte(testSpecificVariables["CONTROL_PLANE_ENDPOINT_PORT"]), -1)
					clusterTemplateYAML = bytes.Replace(clusterTemplateYAML, []byte(scaleClusterVSphereServerPlaceholder), []byte(testSpecificVariables["VSPHERE_SERVER"]), -1)
					clusterTemplateYAML = bytes.Replace(clusterTemplateYAML, []byte(scaleClusterVSphereTLSThumbprintPlaceholder), []byte(testSpecificVariables["VSPHERE_TLS_THUMBPRINT"]), -1)

					if testMode == GovmomiTestMode {
						clusterClassYAML = bytes.Replace(clusterClassYAML, []byte(scaleClusterNamePlaceholder), []byte(clusterName), -1)
						clusterClassYAML = bytes.Replace(clusterClassYAML, []byte(scaleClusterVSphereServerPlaceholder), []byte(testSpecificVariables["VSPHERE_SERVER"]), -1)
						clusterClassYAML = bytes.Replace(clusterClassYAML, []byte(scaleClusterVSphereTLSThumbprintPlaceholder), []byte(testSpecificVariables["VSPHERE_TLS_THUMBPRINT"]), -1)
					}

					return clusterClassYAML, clusterTemplateYAML
				},
			}
		})
	})
})

func getVariableOrFallback(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
