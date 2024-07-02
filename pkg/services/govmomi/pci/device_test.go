/*
Copyright 2023 The Kubernetes Authors.

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

package pci

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

func Test_CalculateDevicesToBeAdded(t *testing.T) {
	type input struct {
		name                      string
		expectedLen               int
		existingDeviceSpecIndexes []int
		pciDeviceSpecs            []infrav1.PCIDeviceSpec
		assertFunc                func(g *gomega.WithT, actual []infrav1.PCIDeviceSpec)
	}

	testFunc := func(t *testing.T, i input) {
		t.Helper()
		t.Run(i.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			simulator.Run(func(ctx context.Context, client *vim25.Client) error {
				finder := find.NewFinder(client)
				vm, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
				if err != nil {
					return err
				}

				if len(i.existingDeviceSpecIndexes) > 0 {
					existingDevices := []infrav1.PCIDeviceSpec{}
					for _, idx := range i.existingDeviceSpecIndexes {
						existingDevices = append(existingDevices, i.pciDeviceSpecs[idx])
					}
					g.Expect(vm.AddDevice(ctx,
						ConstructDeviceSpecs(existingDevices)...)).ToNot(gomega.HaveOccurred())
				}
				toBeAdded, err := CalculateDevicesToBeAdded(ctx, vm, i.pciDeviceSpecs)
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(toBeAdded).To(gomega.HaveLen(i.expectedLen))
				if i.assertFunc != nil {
					i.assertFunc(g, toBeAdded)
				}
				return nil
			})
		})
	}

	t.Run("when no PCI devices exist on the VM", func(t *testing.T) {
		inputs := []input{
			{
				name:        "when adding a single PCI device of each type",
				expectedLen: 3,
				pciDeviceSpecs: []infrav1.PCIDeviceSpec{
					{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
					{DeviceID: ptr.To[int32](4321), VendorID: ptr.To[int32](8765)},
					{VGPUProfile: "grid_t4-1a"},
				},
				assertFunc: func(g *gomega.WithT, actual []infrav1.PCIDeviceSpec) {
					g.Expect(*actual[0].DeviceID).To(gomega.Equal(int32(1234)))
					g.Expect(*actual[0].VendorID).To(gomega.Equal(int32(5678)))
					g.Expect(*actual[1].DeviceID).To(gomega.Equal(int32(4321)))
					g.Expect(*actual[1].VendorID).To(gomega.Equal(int32(8765)))
					g.Expect(actual[2].VGPUProfile).To(gomega.Equal("grid_t4-1a"))
				},
			},
			{
				name:        "when adding multiple PCI devices of a type",
				expectedLen: 4,
				pciDeviceSpecs: []infrav1.PCIDeviceSpec{
					{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
					{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
					{VGPUProfile: "grid_t4-1a"},
					{VGPUProfile: "grid_t4-1a"},
				},
				assertFunc: func(g *gomega.WithT, actual []infrav1.PCIDeviceSpec) {
					g.Expect(*actual[0].DeviceID).To(gomega.Equal(int32(1234)))
					g.Expect(*actual[0].VendorID).To(gomega.Equal(int32(5678)))
					g.Expect(*actual[1].DeviceID).To(gomega.Equal(int32(1234)))
					g.Expect(*actual[1].VendorID).To(gomega.Equal(int32(5678)))
					g.Expect(actual[2].VGPUProfile).To(gomega.Equal("grid_t4-1a"))
					g.Expect(actual[3].VGPUProfile).To(gomega.Equal("grid_t4-1a"))
				},
			},
		}
		for _, tt := range inputs {
			testFunc(t, tt)
		}
	})

	t.Run("when all PCI devices exist on the VM", func(t *testing.T) {
		inputs := []input{
			{
				name:        "when adding a single PCI device of each type",
				expectedLen: 0,
				pciDeviceSpecs: []infrav1.PCIDeviceSpec{
					{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
					{DeviceID: ptr.To[int32](4321), VendorID: ptr.To[int32](8765)},
					{VGPUProfile: "grid_t4-1a"},
				},
				existingDeviceSpecIndexes: []int{0, 1, 2},
			},
			{
				name:        "when adding multiple PCI devices of a type",
				expectedLen: 0,
				pciDeviceSpecs: []infrav1.PCIDeviceSpec{
					{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
					{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
					{VGPUProfile: "grid_t4-1a"},
					{VGPUProfile: "grid_t4-1a"},
				},
				existingDeviceSpecIndexes: []int{0, 1, 2, 3},
			},
		}
		for _, tt := range inputs {
			testFunc(t, tt)
		}
	})

	t.Run("when some PCI devices exist on the VM", func(t *testing.T) {
		inputs := []input{
			{
				name:        "when adding a single PCI device of each type",
				expectedLen: 2,
				pciDeviceSpecs: []infrav1.PCIDeviceSpec{
					{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
					{DeviceID: ptr.To[int32](4321), VendorID: ptr.To[int32](8765)},
					{VGPUProfile: "grid_t4-1a"},
				},
				existingDeviceSpecIndexes: []int{0},
				assertFunc: func(g *gomega.WithT, actual []infrav1.PCIDeviceSpec) {
					g.Expect(*actual[0].DeviceID).To(gomega.Equal(int32(4321)))
					g.Expect(*actual[0].VendorID).To(gomega.Equal(int32(8765)))
					g.Expect(actual[1].VGPUProfile).To(gomega.Equal("grid_t4-1a"))
				},
			},
			{
				name:        "when adding multiple PCI devices of a type",
				expectedLen: 3,
				pciDeviceSpecs: []infrav1.PCIDeviceSpec{
					{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
					{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
					{DeviceID: ptr.To[int32](4321), VendorID: ptr.To[int32](8765)},
					{VGPUProfile: "grid_t4-1a"},
				},
				existingDeviceSpecIndexes: []int{0},
				assertFunc: func(g *gomega.WithT, actual []infrav1.PCIDeviceSpec) {
					g.Expect(*actual[0].DeviceID).To(gomega.Equal(int32(1234)))
					g.Expect(*actual[0].VendorID).To(gomega.Equal(int32(5678)))
					g.Expect(*actual[1].DeviceID).To(gomega.Equal(int32(4321)))
					g.Expect(*actual[1].VendorID).To(gomega.Equal(int32(8765)))
					g.Expect(actual[2].VGPUProfile).To(gomega.Equal("grid_t4-1a"))
				},
			},
		}
		for _, tt := range inputs {
			testFunc(t, tt)
		}
	})
}
