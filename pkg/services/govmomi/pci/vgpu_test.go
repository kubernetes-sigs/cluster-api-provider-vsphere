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

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

func Test_CalculateVGPUsToBeAdded(t *testing.T) {
	type input struct {
		name                      string
		expectedLen               int
		existingDeviceSpecIndexes []int
		vGPUDeviceSpecs           []infrav1.VGPUSpec
		assertFunc                func(g *gomega.WithT, actual []infrav1.VGPUSpec)
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
					existingDevices := []infrav1.VGPUSpec{}
					for _, idx := range i.existingDeviceSpecIndexes {
						existingDevices = append(existingDevices, i.vGPUDeviceSpecs[idx])
					}
					g.Expect(vm.AddDevice(ctx,
						ConstructDeviceSpecsVGPU(existingDevices)...)).ToNot(gomega.HaveOccurred())
				}
				toBeAdded, err := CalculateVGPUsToBeAdded(ctx, vm, i.vGPUDeviceSpecs)
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(toBeAdded).To(gomega.HaveLen(i.expectedLen))
				if i.assertFunc != nil {
					i.assertFunc(g, toBeAdded)
				}
				return nil
			})
		})
	}

	t.Run("when no vGPU devices exist on the VM", func(t *testing.T) {
		inputs := []input{
			{
				name:        "when adding a single vGPU device of each type",
				expectedLen: 2,
				vGPUDeviceSpecs: []infrav1.VGPUSpec{
					{ProfileName: "1234"}, {ProfileName: "4321"},
				},
				assertFunc: func(g *gomega.WithT, actual []infrav1.VGPUSpec) {
					g.Expect(actual[0].ProfileName).To(gomega.Equal("1234"))
					g.Expect(actual[1].ProfileName).To(gomega.Equal("4321"))
				},
			},
			{
				name:        "when adding multiple vGPU devices of a type",
				expectedLen: 2,
				vGPUDeviceSpecs: []infrav1.VGPUSpec{
					{ProfileName: "1234"}, {ProfileName: "1234"},
				},
				assertFunc: func(g *gomega.WithT, actual []infrav1.VGPUSpec) {
					g.Expect(actual[0].ProfileName).To(gomega.Equal("1234"))
					g.Expect(actual[1].ProfileName).To(gomega.Equal("1234"))
				},
			},
		}
		for _, tt := range inputs {
			testFunc(t, tt)
		}
	})

	t.Run("when all vGPU devices exist on the VM", func(t *testing.T) {
		inputs := []input{
			{
				name:        "when adding a single vGPU device of each type",
				expectedLen: 0,
				vGPUDeviceSpecs: []infrav1.VGPUSpec{
					{ProfileName: "1234"}, {ProfileName: "4321"},
				},
				existingDeviceSpecIndexes: []int{0, 1},
			},
			{
				name:        "when adding multiple vGPU devices of a type",
				expectedLen: 0,
				vGPUDeviceSpecs: []infrav1.VGPUSpec{
					{ProfileName: "1234"}, {ProfileName: "1234"},
				},
				existingDeviceSpecIndexes: []int{0, 1},
			},
		}
		for _, tt := range inputs {
			testFunc(t, tt)
		}
	})

	t.Run("when some vGPU devices exist on the VM", func(t *testing.T) {
		inputs := []input{
			{
				name:        "when adding a single vGPU device of each type",
				expectedLen: 1,
				vGPUDeviceSpecs: []infrav1.VGPUSpec{
					{ProfileName: "1234"}, {ProfileName: "4321"},
				},
				existingDeviceSpecIndexes: []int{0},
				assertFunc: func(g *gomega.WithT, actual []infrav1.VGPUSpec) {
					g.Expect(actual[0].ProfileName).To(gomega.Equal("4321"))
				},
			},
			{
				name:        "when adding multiple vGPU devices of a type",
				expectedLen: 2,
				vGPUDeviceSpecs: []infrav1.VGPUSpec{
					{ProfileName: "1234"},
					{ProfileName: "1234"},
					{ProfileName: "4321"},
				},
				existingDeviceSpecIndexes: []int{0},
				assertFunc: func(g *gomega.WithT, actual []infrav1.VGPUSpec) {
					g.Expect(actual[0].ProfileName).To(gomega.Equal("1234"))
					g.Expect(actual[1].ProfileName).To(gomega.Equal("4321"))
				},
			},
		}
		for _, tt := range inputs {
			testFunc(t, tt)
		}
	})
}
