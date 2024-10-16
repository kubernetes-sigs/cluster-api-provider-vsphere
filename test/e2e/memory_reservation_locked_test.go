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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/cluster-api/util"
)

type MemoryReservationLockedSpecInput struct {
	InfraClients
	Global    GlobalInput
	SpecName  string
	Namespace *corev1.Namespace
	// Allows to inject a function to be run after test namespace is created.
	// If not specified, this is a no-op.
	PostNamespaceCreated func(managementClusterProxy framework.ClusterProxy, workloadClusterNamespace string)
}

var _ = Describe("Set memory reservation locked to max on VMs", func() {
	const specName = "memory-reservation-locked"
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		var (
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
			namespace = setupSpecNamespace(specName, testSpecificSettingsGetter().PostNamespaceCreatedFunc)
		})

		AfterEach(func() {
			cleanupSpecNamespace(namespace)
		})

		It("Creates a workload cluster whose VMs have memory reservation locked set to true", func() {
			VerfiyMemoryReservationLockToMax(ctx, MemoryReservationLockedSpecInput{
				SpecName:  specName,
				Namespace: namespace,
				InfraClients: InfraClients{
					Client:     vsphereClient,
					RestClient: restClient,
					Finder:     vsphereFinder,
				},
				Global: GlobalInput{
					BootstrapClusterProxy: bootstrapClusterProxy,
					ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
					E2EConfig:             e2eConfig,
					ArtifactFolder:        artifactFolder,
				},
			})
		})
	})
})

func VerfiyMemoryReservationLockToMax(ctx context.Context, input MemoryReservationLockedSpecInput) {
	var (
		specName         = input.SpecName
		namespace        = input.Namespace
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	)

	clusterName := fmt.Sprintf("%s-%s", specName, util.RandomString(6))
	By("Creating a workload cluster")
	configCluster := defaultConfigCluster(clusterName, namespace.Name, specName, 1, 1, input.Global)

	clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy:                 input.Global.BootstrapClusterProxy,
		ConfigCluster:                configCluster,
		WaitForClusterIntervals:      input.Global.E2EConfig.GetIntervals(specName, "wait-cluster"),
		WaitForControlPlaneIntervals: input.Global.E2EConfig.GetIntervals(specName, "wait-control-plane"),
		WaitForMachineDeployments:    input.Global.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
	}, clusterResources)

	Byf("Fetching the VSphereVM objects for the cluster %s", clusterName)
	vms := getVSphereVMsForCluster(clusterName, namespace.Name)

	By("Verifying memory reservation locked to max is set to true")
	for _, vm := range vms.Items {
		vmObj, err := input.Finder.VirtualMachine(ctx, vm.Name)
		Expect(err).ToNot(HaveOccurred(), "expected to find VM %s", vm.Name)
		Expect(getMemoryReservationLockedToMaxFromObj(vmObj)).To(Equal(true), "expected memory reservation locked to max to be set to true")
	}
}

func getMemoryReservationLockedToMaxFromObj(vmObj *object.VirtualMachine) *bool {
	var virtualMachine mo.VirtualMachine
	Expect(vmObj.Properties(ctx, vmObj.Reference(), []string{"config.memoryReservationLockedToMax"}, &virtualMachine)).To(Succeed())
	Expect(virtualMachine.Config.MemoryReservationLockedToMax).ToNot(BeEmpty())
	return virtualMachine.Config.MemoryReservationLockedToMax
}
