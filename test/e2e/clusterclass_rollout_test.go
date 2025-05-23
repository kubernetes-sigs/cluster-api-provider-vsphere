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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/constants"
)

var _ = Describe("When testing ClusterClass rollouts [vcsim] [supervisor] [ClusterClass]", func() {
	const specName = "clusterclass-rollouts" // copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capie2e.ClusterClassRolloutSpec(ctx, func() capie2e.ClusterClassRolloutSpecInput {
			return capie2e.ClusterClassRolloutSpecInput{
				E2EConfig:                      e2eConfig,
				ClusterctlConfigPath:           testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:          bootstrapClusterProxy,
				ArtifactFolder:                 artifactFolder,
				SkipCleanup:                    skipCleanup,
				Flavor:                         testSpecificSettingsGetter().FlavorForMode("topology"),
				PostNamespaceCreated:           testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				InfrastructureProvider:         ptr.To("vsphere"),
				FilterMetadataBeforeValidation: filterMetadataBeforeValidation,
			}
		})
	})
})

func filterMetadataBeforeValidation(object client.Object) clusterv1.ObjectMeta {
	// CAPV adds an extra label node.cluster.x-k8s.io/esxi-host on Machine, we need to filter it out to pass the
	// clusterclass rollout test
	if machine, ok := object.(*clusterv1.Machine); ok {
		delete(machine.Labels, constants.ESXiHostInfoLabel)
		return clusterv1.ObjectMeta{Labels: machine.Labels, Annotations: machine.Annotations}
	}

	// If the object is not a Machine, just return the default labels and annotations of the object
	return clusterv1.ObjectMeta{Labels: object.GetLabels(), Annotations: object.GetAnnotations()}
}
