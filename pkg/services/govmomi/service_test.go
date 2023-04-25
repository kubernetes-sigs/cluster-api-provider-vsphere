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

package govmomi

import (
	goctx "context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

func emptyVirtualMachineContext() *virtualMachineContext {
	return &virtualMachineContext{
		VMContext: context.VMContext{
			Logger: logr.Discard(),
			ControllerContext: &context.ControllerContext{
				ControllerManagerContext: &context.ControllerManagerContext{
					Context: goctx.TODO(),
				},
			},
		},
	}
}

func Test_reconcilePCIDevices(t *testing.T) {
	var vmCtx *virtualMachineContext
	var g *WithT
	var vms *VMService

	before := func() {
		vmCtx = emptyVirtualMachineContext()
		vmCtx.Client = fake.NewClientBuilder().Build()

		vms = &VMService{}
	}

	t.Run("when powered off VM has no PCI devices", func(t *testing.T) {
		g = NewWithT(t)
		before()

		simulator.Run(func(ctx goctx.Context, c *vim25.Client) error {
			finder := find.NewFinder(c)
			vm, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
			g.Expect(err).ToNot(HaveOccurred())
			_, err = vm.PowerOff(ctx)
			g.Expect(err).ToNot(HaveOccurred())

			vmCtx.Obj = vm
			vmCtx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
						PciDevices: []infrav1.PCIDeviceSpec{
							{DeviceID: pointer.Int32(1234), VendorID: pointer.Int32(5678)},
							{DeviceID: pointer.Int32(1234), VendorID: pointer.Int32(5678)},
						},
					},
				},
			}

			g.Expect(vms.reconcilePCIDevices(vmCtx)).ToNot(HaveOccurred())

			// get the VM's virtual device list
			devices, err := vm.Device(ctx)
			g.Expect(err).ToNot(HaveOccurred())
			// filter the device with the given backing info
			pciDevices := devices.SelectByBackingInfo(&types.VirtualPCIPassthroughDynamicBackingInfo{
				AllowedDevice: []types.VirtualPCIPassthroughAllowedDevice{
					{DeviceId: 1234, VendorId: 5678},
				}})
			g.Expect(pciDevices).To(HaveLen(2))
			return nil
		})
	})
}
