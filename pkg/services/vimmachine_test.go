/*
Copyright 2021 The Kubernetes Authors.

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

package services

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
)

var _ = Describe("VimMachineService_GenerateOverrideFunc", func() {
	deplZone := func(suffix string) *infrav1.VSphereDeploymentZone {
		return &infrav1.VSphereDeploymentZone{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("zone-%s", suffix)},
			Spec: infrav1.VSphereDeploymentZoneSpec{
				Server:        fmt.Sprintf("server-%s", suffix),
				FailureDomain: fmt.Sprintf("fd-%s", suffix),
				ControlPlane:  pointer.Bool(true),
				PlacementConstraint: infrav1.PlacementConstraint{
					ResourcePool: fmt.Sprintf("rp-%s", suffix),
					Folder:       fmt.Sprintf("folder-%s", suffix),
				},
			},
		}
	}

	failureDomain := func(suffix string) *infrav1.VSphereFailureDomain {
		return &infrav1.VSphereFailureDomain{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("fd-%s", suffix)},
			Spec: infrav1.VSphereFailureDomainSpec{
				Topology: infrav1.Topology{
					Datacenter: fmt.Sprintf("dc-%s", suffix),
					Datastore:  fmt.Sprintf("ds-%s", suffix),
					Networks:   []string{fmt.Sprintf("nw-%s", suffix), "another-nw"},
				},
			},
		}
	}
	var (
		controllerCtx     *context.ControllerContext
		machineCtx        *context.VIMMachineContext
		vimMachineService *VimMachineService
	)

	BeforeEach(func() {
		controllerCtx = fake.NewControllerContext(fake.NewControllerManagerContext(deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two")))
		machineCtx = fake.NewMachineContext(fake.NewClusterContext(controllerCtx))
		vimMachineService = &VimMachineService{}
	})

	Context("When Failure Domain is not present", func() {
		It("does not generate an override function", func() {
			_, ok := vimMachineService.generateOverrideFunc(machineCtx)
			Expect(ok).To(BeFalse())
		})
	})

	Context("When Failure Domain is present", func() {
		BeforeEach(func() {
			machineCtx.Machine.Spec.FailureDomain = pointer.String("zone-one")
		})

		It("generates an override function", func() {
			_, ok := vimMachineService.generateOverrideFunc(machineCtx)
			Expect(ok).To(BeTrue())
		})

		It("uses the deployment zone placement constraint & failure domains topology for VM values", func() {
			overrideFunc, ok := vimMachineService.generateOverrideFunc(machineCtx)
			Expect(ok).To(BeTrue())

			vm := &infrav1.VSphereVM{Spec: infrav1.VSphereVMSpec{}}
			overrideFunc(vm)

			Expect(vm.Spec.Server).To(Equal("server-one"))
			Expect(vm.Spec.Folder).To(Equal("folder-one"))
			Expect(vm.Spec.Datastore).To(Equal("ds-one"))
			Expect(vm.Spec.ResourcePool).To(Equal("rp-one"))
			Expect(vm.Spec.Datacenter).To(Equal("dc-one"))
		})

		Context("for non-existent failure domain value", func() {
			BeforeEach(func() {
				machineCtx.Machine.Spec.FailureDomain = pointer.String("non-existent-zone")
			})

			It("fails to generate an override function", func() {
				overrideFunc, ok := vimMachineService.generateOverrideFunc(machineCtx)
				Expect(ok).To(BeFalse())
				Expect(overrideFunc).To(BeNil())
			})
		})

		Context("with network specified in the topology", func() {
			It("overrides the n/w names from the networks list of the topology", func() {
				By("For equal number of networks")
				vm := &infrav1.VSphereVM{
					Spec: infrav1.VSphereVMSpec{
						VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
							Network: infrav1.NetworkSpec{Devices: []infrav1.NetworkDeviceSpec{{NetworkName: "foo", DHCP4: false}, {NetworkName: "bar", DHCP6: false}}},
						},
					},
				}

				overrideFunc, ok := vimMachineService.generateOverrideFunc(machineCtx)
				Expect(ok).To(BeTrue())

				overrideFunc(vm)

				devices := vm.Spec.Network.Devices
				Expect(devices).To(HaveLen(2))
				Expect(devices[0].NetworkName).To(Equal("nw-one"))

				Expect(devices[1].NetworkName).To(Equal("another-nw"))
			})

			It("appends the n/w names present in the networks list of the topology", func() {
				By("With number of devices in VMSpec < number of networks in the placement constraint")
				vm := &infrav1.VSphereVM{
					Spec: infrav1.VSphereVMSpec{
						VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
							Network: infrav1.NetworkSpec{Devices: []infrav1.NetworkDeviceSpec{{NetworkName: "foo", DHCP4: false}}},
						},
					},
				}

				overrideFunc, ok := vimMachineService.generateOverrideFunc(machineCtx)
				Expect(ok).To(BeTrue())

				overrideFunc(vm)

				devices := vm.Spec.Network.Devices
				Expect(devices).To(HaveLen(2))
				Expect(devices[0].NetworkName).To(Equal("nw-one"))

				Expect(devices[1].NetworkName).To(Equal("another-nw"))
			})

			It("only overrides the n/w names present in the networks list of the topology", func() {
				By("With number of devices in VMSpec > number of networks in the placement constraint")
				vm := &infrav1.VSphereVM{
					Spec: infrav1.VSphereVMSpec{
						VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
							Network: infrav1.NetworkSpec{Devices: []infrav1.NetworkDeviceSpec{{NetworkName: "foo", DHCP4: false}, {NetworkName: "bar", DHCP6: false}, {NetworkName: "baz", DHCP6: false}}},
						},
					},
				}

				overrideFunc, ok := vimMachineService.generateOverrideFunc(machineCtx)
				Expect(ok).To(BeTrue())

				overrideFunc(vm)

				devices := vm.Spec.Network.Devices
				Expect(devices).To(HaveLen(3))
				Expect(devices[0].NetworkName).To(Equal("nw-one"))

				Expect(devices[1].NetworkName).To(Equal("another-nw"))

				Expect(devices[2].NetworkName).To(Equal("baz"))
			})
		})
	})
})
