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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

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

	failureDomainWithNetConfig := func(suffix string, addOldNetwork bool) *infrav1.VSphereFailureDomain {
		var networks []string
		if addOldNetwork {
			networks = append(networks, fmt.Sprintf("nw-%s", suffix), "another-nw")
		}
		return &infrav1.VSphereFailureDomain{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("fd-%s", suffix)},
			Spec: infrav1.VSphereFailureDomainSpec{
				Topology: infrav1.Topology{
					Datacenter: fmt.Sprintf("dc-%s", suffix),
					Datastore:  fmt.Sprintf("ds-%s", suffix),
					Networks:   networks,
					NetworkConfigs: []infrav1.FailureDomainNetwork{
						{
							NetworkName: fmt.Sprintf("newnw-%s", suffix),
							DHCP4:       pointer.Bool(false),
						},
						{
							NetworkName: "another-new-nw",
							Nameservers: []string{"10.10.10.10", "10.10.20.20"},
						},
					},
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
		controllerCtx = fake.NewControllerContext(fake.NewControllerManagerContext(deplZone("one"), deplZone("two"), deplZone("three"), deplZone("four"),
			failureDomain("one"), failureDomain("two"), failureDomainWithNetConfig("three", false), failureDomainWithNetConfig("four", true)))
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

		Context("with only network config specified in the topology", func() {
			BeforeEach(func() {
				machineCtx.Machine.Spec.FailureDomain = pointer.String("zone-three")
			})
			It("overrides the n/w configs from the networks list of the topology", func() {
				By("For equal number of networks")
				vm := &infrav1.VSphereVM{
					Spec: infrav1.VSphereVMSpec{
						VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
							Network: infrav1.NetworkSpec{Devices: []infrav1.NetworkDeviceSpec{{NetworkName: "foo", DHCP4: true}, {NetworkName: "bar", DHCP6: true, Nameservers: []string{"10.50.50.10"}}}},
						},
					},
				}

				overrideFunc, ok := vimMachineService.generateOverrideFunc(machineCtx)
				Expect(ok).To(BeTrue())

				overrideFunc(vm)

				devices := vm.Spec.Network.Devices
				Expect(devices).To(HaveLen(2))
				Expect(devices[0].NetworkName).To(Equal("newnw-three"))
				Expect(devices[0].DHCP4).To(BeFalse())
				Expect(devices[0].DHCP6).To(BeFalse())

				Expect(devices[1].NetworkName).To(Equal("another-new-nw"))
				Expect(devices[1].DHCP4).To(BeFalse())
				Expect(devices[1].DHCP6).To(BeTrue())
				Expect(devices[1].Nameservers).To(HaveLen(2))
				Expect(devices[1].Nameservers).To(Equal([]string{"10.10.10.10", "10.10.20.20"}))

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
				Expect(devices[0].NetworkName).To(Equal("newnw-three"))
				Expect(devices[0].DHCP4).To(BeFalse())
				Expect(devices[0].DHCP6).To(BeFalse())
				Expect(devices[1].NetworkName).To(Equal("another-new-nw"))
			})

			It("only overrides the n/w names present in the networks list of the topology", func() {
				By("With number of devices in VMSpec > number of networks in the placement constraint")
				vm := &infrav1.VSphereVM{
					Spec: infrav1.VSphereVMSpec{
						VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
							Network: infrav1.NetworkSpec{Devices: []infrav1.NetworkDeviceSpec{{NetworkName: "foo", DHCP4: true}, {NetworkName: "bar", DHCP6: true}, {NetworkName: "baz", DHCP6: false}}},
						},
					},
				}

				overrideFunc, ok := vimMachineService.generateOverrideFunc(machineCtx)
				Expect(ok).To(BeTrue())

				overrideFunc(vm)

				devices := vm.Spec.Network.Devices
				Expect(devices).To(HaveLen(3))
				Expect(devices[0].NetworkName).To(Equal("newnw-three"))
				Expect(devices[0].DHCP4).To(BeFalse())

				Expect(devices[1].NetworkName).To(Equal("another-new-nw"))
				Expect(devices[1].DHCP6).To(BeTrue())

				Expect(devices[2].NetworkName).To(Equal("baz"))
			})
		})

		Context("with network config and networks specified in the topology", func() {
			BeforeEach(func() {
				machineCtx.Machine.Spec.FailureDomain = pointer.String("zone-four")
			})
			It("overrides the n/w configs using the networkconfig and discarding networks", func() {
				By("For equal number of networks")
				vm := &infrav1.VSphereVM{
					Spec: infrav1.VSphereVMSpec{
						VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
							Network: infrav1.NetworkSpec{Devices: []infrav1.NetworkDeviceSpec{{NetworkName: "foo", DHCP4: true}, {NetworkName: "bar", DHCP6: true, Nameservers: []string{"10.50.50.10"}}}},
						},
					},
				}

				overrideFunc, ok := vimMachineService.generateOverrideFunc(machineCtx)
				Expect(ok).To(BeTrue())

				overrideFunc(vm)

				devices := vm.Spec.Network.Devices
				Expect(devices).To(HaveLen(2))
				Expect(devices[0].NetworkName).To(Equal("newnw-four"))
				Expect(devices[0].DHCP4).To(BeFalse())
				Expect(devices[0].DHCP6).To(BeFalse())

				Expect(devices[1].NetworkName).To(Equal("another-new-nw"))
				Expect(devices[1].DHCP4).To(BeFalse())
				Expect(devices[1].DHCP6).To(BeTrue())
				Expect(devices[1].Nameservers).To(HaveLen(2))
				Expect(devices[1].Nameservers).To(Equal([]string{"10.10.10.10", "10.10.20.20"}))

			})
		})
	})

})

var _ = Describe("VimMachineService_GetHostInfo", func() {
	var (
		controllerCtx     *context.ControllerContext
		machineCtx        *context.VIMMachineContext
		vimMachineService = &VimMachineService{}
		hostAddr          = "1.2.3.4"
	)

	getVSphereVM := func(hostAddr string, conditionStatus corev1.ConditionStatus) *infrav1.VSphereVM {
		return &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: fake.Namespace,
				Name:      fake.Clusterv1a2Name,
			},
			Status: infrav1.VSphereVMStatus{
				Host: hostAddr,
				Conditions: []clusterv1.Condition{
					{
						Type:   infrav1.VMProvisionedCondition,
						Status: conditionStatus,
					},
				},
			},
		}
	}

	Context("When VMProvisioned Condition is set", func() {
		BeforeEach(func() {
			controllerCtx = fake.NewControllerContext(fake.NewControllerManagerContext(getVSphereVM(hostAddr, corev1.ConditionTrue)))
			machineCtx = fake.NewMachineContext(fake.NewClusterContext(controllerCtx))
		})
		It("Fetches host address from the VSphereVM object", func() {
			host, err := vimMachineService.GetHostInfo(machineCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(host).To(Equal(hostAddr))
		})
	})

	Context("When VMProvisioned Condition is unset", func() {
		BeforeEach(func() {
			controllerCtx = fake.NewControllerContext(fake.NewControllerManagerContext(getVSphereVM(hostAddr, corev1.ConditionFalse)))
			machineCtx = fake.NewMachineContext(fake.NewClusterContext(controllerCtx))
		})
		It("returns empty string", func() {
			host, err := vimMachineService.GetHostInfo(machineCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(host).To(BeEmpty())
		})
	})

})

var _ = Describe("VimMachineService_createOrPatchVSphereVM", func() {
	var (
		controllerCtx       *context.ControllerContext
		machineCtx          *context.VIMMachineContext
		vimMachineService   = &VimMachineService{}
		hostAddr            = "1.2.3.4"
		fakeLongClusterName = "fake-long-clustername"
	)

	getVSphereVM := func(hostAddr string, conditionStatus corev1.ConditionStatus) *infrav1.VSphereVM {
		return &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: fake.Namespace,
				Name:      fakeLongClusterName,
			},
			Status: infrav1.VSphereVMStatus{
				Host: hostAddr,
				Conditions: []clusterv1.Condition{
					{
						Type:   infrav1.VMProvisionedCondition,
						Status: conditionStatus,
					},
				},
			},
		}
	}

	controllerCtx = fake.NewControllerContext(fake.NewControllerManagerContext(getVSphereVM(hostAddr, corev1.ConditionTrue)))
	machineCtx = fake.NewMachineContext(fake.NewClusterContext(controllerCtx))
	machineCtx.Machine.SetName(fakeLongClusterName)

	Context("When VSphereMachine OS is Windows", func() {
		BeforeEach(func() {
			machineCtx.VSphereMachine.Spec.OS = infrav1.Windows
		})
		It("returns a renamed vspherevm object", func() {
			vm, err := vimMachineService.createOrPatchVSphereVM(machineCtx, getVSphereVM(hostAddr, corev1.ConditionTrue))
			vmName := vm.(*infrav1.VSphereVM).GetName()
			Expect(err).NotTo(HaveOccurred())
			Expect(vmName).To(Equal("fake-long-rname"))
		})
	})

	Context("When VSphereMachine OS is Linux", func() {
		BeforeEach(func() {
			machineCtx.VSphereMachine.Spec.OS = infrav1.Linux
		})
		It("returns the same vspherevm name", func() {
			vm, err := vimMachineService.createOrPatchVSphereVM(machineCtx, getVSphereVM(hostAddr, corev1.ConditionTrue))
			vmName := vm.(*infrav1.VSphereVM).GetName()
			Expect(err).NotTo(HaveOccurred())
			Expect(vmName).To(Equal(fakeLongClusterName))
		})
	})
})
