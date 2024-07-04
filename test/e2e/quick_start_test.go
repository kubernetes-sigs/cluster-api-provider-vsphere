/*
Copyright 2020 The Kubernetes Authors.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var _ = Describe("Cluster Creation using Cluster API quick-start test [vcsim] [supervisor]", func() {
	const specName = "quick-start" // copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.QuickStartSpec(ctx, func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				Flavor:                ptr.To(testSpecificSettingsGetter().FlavorForMode(clusterctl.DefaultFlavor)),
				PostNamespaceCreated:  testSpecificSettingsGetter().PostNamespaceCreatedFunc,
			}
		})
	})
})

var _ = Describe("ClusterClass Creation using Cluster API quick-start test [vcsim] [supervisor] [PR-Blocking] [ClusterClass]", func() {
	const specName = "quick-start-cluster-class" // prefix (quick-start) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.QuickStartSpec(ctx, func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:               e2eConfig,
				ClusterctlConfigPath:    testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:   bootstrapClusterProxy,
				ArtifactFolder:          artifactFolder,
				SkipCleanup:             skipCleanup,
				Flavor:                  ptr.To(testSpecificSettingsGetter().FlavorForMode("topology")),
				PostNamespaceCreated:    testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				PostMachinesProvisioned: checkAllPodsReady,
			}
		})
	})
})

var _ = Describe("Cluster creation with [Ignition] bootstrap [PR-Blocking]", func() {
	const specName = "quick-start-ignition" // prefix (quick-start) copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.QuickStartSpec(ctx, func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				Flavor:                ptr.To(testSpecificSettingsGetter().FlavorForMode("ignition")),
				PostNamespaceCreated:  testSpecificSettingsGetter().PostNamespaceCreatedFunc,
			}
		})
	})
})

func checkAllPodsReady(managementClusterProxy framework.ClusterProxy, workloadClusterNamespace, workloadClusterName string) {
	if testTarget == VCSimTestTarget {
		return
	}

	wlProxy := managementClusterProxy.GetWorkloadCluster(ctx, workloadClusterNamespace, workloadClusterName)
	wlClient := wlProxy.GetClient()

	pods := &corev1.PodList{}
	Eventually(func() error {
		if err := wlClient.List(ctx, pods); err != nil {
			return err
		}

		errs := []error{}

		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				errs = append(errs, fmt.Errorf("pod %s not in phase running", klog.KObj(&pod)))
			}
			for _, cond := range pod.Status.Conditions {
				if cond.Type != corev1.PodReady {
					continue
				}
				if cond.Status != corev1.ConditionTrue {
					errs = append(errs, fmt.Errorf("pod %s not ready: reason=%q message=%q", klog.KObj(&pod), cond.Reason, cond.Message))
				}
				break
			}
		}

		return kerrors.NewAggregate(errs)
	}, time.Minute*5, time.Second).Should(Succeed())
}
