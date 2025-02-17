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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/utils/ptr"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

type diskSpecInput struct {
	InfraClients
	global      GlobalInput
	specName    string
	namespace   string
	clusterName string
}

var _ = Describe("Ensure govmomi mode is able to add additional disks to VMs", func() {
	const specName = "multi-disk"
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.QuickStartSpec(ctx, func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				Flavor:                ptr.To(testSpecificSettingsGetter().FlavorForMode("multi-disk")),
				PostNamespaceCreated:  testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				PostMachinesProvisioned: func(_ framework.ClusterProxy, namespace, clusterName string) {
					dsi := diskSpecInput{
						specName:    specName,
						namespace:   namespace,
						clusterName: clusterName,
						InfraClients: InfraClients{
							Client:     vsphereClient,
							RestClient: restClient,
							Finder:     vsphereFinder,
						},
						global: GlobalInput{
							BootstrapClusterProxy: bootstrapClusterProxy,
							ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
							E2EConfig:             e2eConfig,
							ArtifactFolder:        artifactFolder,
						},
					}
					verifyDisks(ctx, dsi)
				},
				ControlPlaneMachineCount: ptr.To[int64](1),
				WorkerMachineCount:       ptr.To[int64](1),
			}
		})
	})
})

func verifyDisks(ctx context.Context, input diskSpecInput) {
	Byf("Fetching the VSphereVM objects for the cluster %s", input.clusterName)
	vms := getVSphereVMsForCluster(input.clusterName, input.namespace)

	By("Verifying the disks attached to the VMs")
	for _, vm := range vms.Items {
		// vSphere machine object should have the data disks configured. We will add +1 to the count since the os image
		// needs to be included for comparison.
		Byf("VM %s Spec has %d DataDisk(s) defined", vm.Name, len(vm.Spec.DataDisks))
		diskCount := 1 + len(vm.Spec.DataDisks)
		Expect(diskCount).ToNot(Equal(1), "Total disk count should be larger than 1 for this test")

		vmObj, err := input.Finder.VirtualMachine(ctx, vm.Name)
		Expect(err).NotTo(HaveOccurred())

		devices, err := vmObj.Device(ctx)
		Expect(err).NotTo(HaveOccurred())

		// We expect control plane VMs to have 3 disks, and the compute VMs will have 2.
		disks := devices.SelectByType((*types.VirtualDisk)(nil))
		Expect(disks).To(HaveLen(diskCount), fmt.Sprintf("Disk count of VM should be %d", diskCount))

		// Check each disk to see if its provisioning type matches the expected
		for diskIndex, disk := range disks {
			// Skip first disk since it is the OS
			if diskIndex == 0 {
				continue
			}

			// Get the backing info and perform check
			diskConfig := vm.Spec.DataDisks[diskIndex-1]
			backingInfo := disk.GetVirtualDevice().Backing.(*types.VirtualDiskFlatVer2BackingInfo)
			By(fmt.Sprintf("Checking provision type \"%v\"", diskConfig.ProvisioningMode))
			switch diskConfig.ProvisioningMode {
			case infrav1.ThinProvisioningMode:
				Expect(backingInfo.ThinProvisioned).To(Equal(types.NewBool(true)), "ThinProvisioned should be true for resulting disk when data disk provisionType is ThinProvisioned")
				Expect(backingInfo.EagerlyScrub).To(Equal(types.NewBool(false)), "EagerlyScrub should be false for resulting disk when data disk provisionType is ThinProvisioned")
			case infrav1.ThickProvisioningMode:
				Expect(backingInfo.ThinProvisioned).To(Equal(types.NewBool(false)), "ThinProvisioned should be false for resulting disk when data disk provisionType is ThickProvisioned")
				Expect(backingInfo.EagerlyScrub).To(Equal(types.NewBool(false)), "EagerlyScrub should be false for resulting disk when data disk provisionType is ThickProvisioned")
			case infrav1.EagerlyZeroedProvisioningMode:
				Expect(backingInfo.ThinProvisioned).To(Equal(types.NewBool(false)), "ThinProvisioned should be false for resulting disk when data disk provisionType is EagerlyZeroed")
				Expect(backingInfo.EagerlyScrub).To(Equal(types.NewBool(true)), "EagerlyScrub should be true for resulting disk when data disk provisionType is EagerlyZeroed")
			default:
				// Currently, the settings for default behavior of new disks during clone can come from templates settings,
				// the default storage policy, the clone's settings or the datastore configuration.
			}

			// Check disk size
			Expect((disk.(*types.VirtualDisk)).CapacityInKB).To(Equal(int64(diskConfig.SizeGiB*1024*1024)), "Resulting disk size should match the size configured for that data disk")
		}
	}
}
