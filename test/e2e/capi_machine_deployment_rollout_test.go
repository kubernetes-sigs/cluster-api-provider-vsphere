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
	"context"

	. "github.com/onsi/ginkgo"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
)

var _ = Describe("ClusterAPI Machine Deployment Tests", func() {
	Context("Running the MachineDeployment rollout spec", func() {
		capi_e2e.MachineDeploymentRolloutSpec(context.TODO(), func() capi_e2e.MachineDeploymentRolloutSpecInput {
			return capi_e2e.MachineDeploymentRolloutSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  clusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
			}
		})
	})

})
