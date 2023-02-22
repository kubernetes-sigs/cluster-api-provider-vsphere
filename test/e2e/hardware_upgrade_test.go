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
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	capiutil "sigs.k8s.io/cluster-api/util"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

type HardwareUpgradeSpecInput struct {
	InfraClients
	Global    GlobalInput
	Namespace *corev1.Namespace
	Template  string
	ToVersion string
}

var _ = Describe("Hardware version upgrade", func() {
	var (
		namespace *corev1.Namespace
	)

	BeforeEach(func() {
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
		namespace = setupSpecNamespace("hw-upgrade-e2e")
	})

	AfterEach(func() {
		cleanupSpecNamespace(namespace)
	})

	It("creates a cluster with VM hardware versions upgraded", func() {
		Expect(e2eConfig.GetVariable("VSPHERE_TEMPLATE")).NotTo(BeEmpty())

		VerifyHardwareUpgrade(ctx, HardwareUpgradeSpecInput{
			Namespace: namespace,
			Template:  e2eConfig.GetVariable("VSPHERE_TEMPLATE"),
			ToVersion: "vmx-17",
			InfraClients: InfraClients{
				Client:     vsphereClient,
				RestClient: restClient,
				Finder:     vsphereFinder,
			},
			Global: GlobalInput{
				BootstrapClusterProxy: bootstrapClusterProxy,
				ClusterctlConfigPath:  clusterctlConfigPath,
				E2EConfig:             e2eConfig,
				ArtifactFolder:        artifactFolder,
			},
		})
	})
})

func VerifyHardwareUpgrade(ctx context.Context, input HardwareUpgradeSpecInput) {
	var (
		specName         = "hw-upgrade"
		namespace        = input.Namespace
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	)

	Byf("Getting the hardware version of the OVA %s", input.Template)
	fromVersion := getHardwareVersion(ctx, input.Finder, input.Template)

	clusterName := fmt.Sprintf("%s-%s", specName, capiutil.RandomString(6))
	By("Creating a cluster")
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

	By("Verifying the hardware version of the cluster VMs")
	for _, vm := range vms.Items {
		vmObj, err := input.Finder.VirtualMachine(ctx, vm.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(getHardwareVersionFromObj(vmObj)).To(Equal(input.ToVersion))

		upgraded, err := util.LessThan(fromVersion, input.ToVersion)
		Expect(err).NotTo(HaveOccurred())
		Expect(upgraded).To(BeTrue())
	}
}

func getHardwareVersion(ctx context.Context, finder *find.Finder, template string) string {
	templateObj, err := finder.VirtualMachine(ctx, template)
	Expect(err).ToNot(HaveOccurred(), "expected to find template")
	return getHardwareVersionFromObj(templateObj)
}

func getHardwareVersionFromObj(vmObj *object.VirtualMachine) string {
	var virtualMachine mo.VirtualMachine
	Expect(vmObj.Properties(ctx, vmObj.Reference(), []string{"config.version"}, &virtualMachine)).To(Succeed())
	Expect(virtualMachine.Config.Version).ToNot(BeEmpty())
	return virtualMachine.Config.Version
}
