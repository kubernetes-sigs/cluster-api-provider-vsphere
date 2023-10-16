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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

var ctx = ctrl.SetupSignalHandler()

func Test_VimMachineService_GenerateOverrideFunc(t *testing.T) {
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

	t.Run("does not generate an override function when Failure Domain is not present", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two"))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		_, ok := vimMachineService.generateOverrideFunc(ctx, machineCtx)
		g.Expect(ok).To(BeFalse())
	})

	t.Run("generates an override function when Failure Domain is present", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two"))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.Spec.FailureDomain = pointer.String("zone-one")
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		_, ok := vimMachineService.generateOverrideFunc(ctx, machineCtx)
		g.Expect(ok).To(BeTrue())
	})

	t.Run("uses the deployment zone placement constraint & failure domains topology for VM values", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two"))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.Spec.FailureDomain = pointer.String("zone-one")
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		overrideFunc, ok := vimMachineService.generateOverrideFunc(ctx, machineCtx)
		g.Expect(ok).To(BeTrue())

		vm := &infrav1.VSphereVM{Spec: infrav1.VSphereVMSpec{}}
		overrideFunc(vm)

		g.Expect(vm.Spec.Server).To(Equal("server-one"))
		g.Expect(vm.Spec.Folder).To(Equal("folder-one"))
		g.Expect(vm.Spec.Datastore).To(Equal("ds-one"))
		g.Expect(vm.Spec.ResourcePool).To(Equal("rp-one"))
		g.Expect(vm.Spec.Datacenter).To(Equal("dc-one"))
	})

	t.Run("fails to generate an override function for non-existent failure domain value", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two"))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.Spec.FailureDomain = pointer.String("non-existent-zone")
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		overrideFunc, ok := vimMachineService.generateOverrideFunc(ctx, machineCtx)
		g.Expect(ok).To(BeFalse())
		g.Expect(overrideFunc).To(BeNil())
	})

	t.Run("overrides the n/w names from the networks list of the topology for equal number of networks", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two"))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.Spec.FailureDomain = pointer.String("zone-one")
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		vm := &infrav1.VSphereVM{
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Network: infrav1.NetworkSpec{Devices: []infrav1.NetworkDeviceSpec{{NetworkName: "foo", DHCP4: false}, {NetworkName: "bar", DHCP6: false}}},
				},
			},
		}

		overrideFunc, ok := vimMachineService.generateOverrideFunc(ctx, machineCtx)
		g.Expect(ok).To(BeTrue())

		overrideFunc(vm)

		devices := vm.Spec.Network.Devices
		g.Expect(devices).To(HaveLen(2))
		g.Expect(devices[0].NetworkName).To(Equal("nw-one"))

		g.Expect(devices[1].NetworkName).To(Equal("another-nw"))
	})

	t.Run("appends the n/w names present in the networks list of the topology with number of devices in VMSpec < number of networks in the placement constraint", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two"))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.Spec.FailureDomain = pointer.String("zone-one")
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		vm := &infrav1.VSphereVM{
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Network: infrav1.NetworkSpec{Devices: []infrav1.NetworkDeviceSpec{{NetworkName: "foo", DHCP4: false}}},
				},
			},
		}

		overrideFunc, ok := vimMachineService.generateOverrideFunc(ctx, machineCtx)
		g.Expect(ok).To(BeTrue())

		overrideFunc(vm)

		devices := vm.Spec.Network.Devices
		g.Expect(devices).To(HaveLen(2))
		g.Expect(devices[0].NetworkName).To(Equal("nw-one"))

		g.Expect(devices[1].NetworkName).To(Equal("another-nw"))
	})

	t.Run("only overrides the n/w names present in the networks list of the topology with number of devices in VMSpec > number of networks in the placement constraint", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two"))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.Spec.FailureDomain = pointer.String("zone-one")
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		vm := &infrav1.VSphereVM{
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Network: infrav1.NetworkSpec{Devices: []infrav1.NetworkDeviceSpec{{NetworkName: "foo", DHCP4: false}, {NetworkName: "bar", DHCP6: false}, {NetworkName: "baz", DHCP6: false}}},
				},
			},
		}

		overrideFunc, ok := vimMachineService.generateOverrideFunc(ctx, machineCtx)
		g.Expect(ok).To(BeTrue())

		overrideFunc(vm)

		devices := vm.Spec.Network.Devices
		g.Expect(devices).To(HaveLen(3))
		g.Expect(devices[0].NetworkName).To(Equal("nw-one"))

		g.Expect(devices[1].NetworkName).To(Equal("another-nw"))

		g.Expect(devices[2].NetworkName).To(Equal("baz"))
	})
}

func Test_VimMachineService_GetHostInfo(t *testing.T) {
	var (
		hostAddr = "1.2.3.4"
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

	t.Run("fetches host address from the VSphereVM object when VMProvisioned condition is set", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(getVSphereVM(hostAddr, corev1.ConditionTrue))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		vimMachineService := &VimMachineService{controllerManagerContext.Client}
		host, err := vimMachineService.GetHostInfo(ctx, machineCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(host).To(Equal(hostAddr))
	})

	t.Run("returns empty string when VMProvisioned condition is unset", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(getVSphereVM(hostAddr, corev1.ConditionFalse))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		vimMachineService := &VimMachineService{controllerManagerContext.Client}
		host, err := vimMachineService.GetHostInfo(ctx, machineCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(host).To(BeEmpty())
	})
}

func Test_VimMachineService_createOrPatchVSphereVM(t *testing.T) {
	var (
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

	t.Run("returns a renamed VSphereVM object when VSphereMachine OS is Windows", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(getVSphereVM(hostAddr, corev1.ConditionTrue), deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two"))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.VSphereMachine.Spec.OS = infrav1.Windows
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		failureDomain := "zone-one"
		machineCtx.Machine.Spec.FailureDomain = &failureDomain
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		vm, err := vimMachineService.createOrPatchVSphereVM(ctx, machineCtx, getVSphereVM(hostAddr, corev1.ConditionTrue))
		vmName := vm.Name
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(vmName).To(Equal("fake-long-rname"))
	})

	t.Run("returns the same VSphereVM name when VSphereMachine OS is Linux", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext(getVSphereVM(hostAddr, corev1.ConditionTrue), deplZone("one"), deplZone("two"), failureDomain("one"), failureDomain("two"))
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.VSphereMachine.Spec.OS = infrav1.Linux
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		vm, err := vimMachineService.createOrPatchVSphereVM(ctx, machineCtx, getVSphereVM(hostAddr, corev1.ConditionTrue))
		vmName := vm.Name
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(vmName).To(Equal(fakeLongClusterName))
	})
}

func Test_VimMachineService_reconcileProviderID(t *testing.T) {
	var (
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

	vsphereVM := getVSphereVM(hostAddr, corev1.ConditionTrue)
	biosUUID := "42055285-ff20-2c28-965c-05558ea1b4c7"

	t.Run("returns false when VSphereVM biosUUID is not set", func(t *testing.T) {
		g := NewWithT(t)
		vsphereVM.Spec.BiosUUID = ""
		controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		ok, err := vimMachineService.reconcileProviderID(ctx, machineCtx, vsphereVM)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ok).To(BeFalse())
	})

	t.Run("returns true when VSphereVM biosUUID is valid", func(t *testing.T) {
		g := NewWithT(t)
		vsphereVM.Spec.BiosUUID = biosUUID
		controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		ok, err := vimMachineService.reconcileProviderID(ctx, machineCtx, vsphereVM)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ok).To(BeTrue())
		g.Expect(*machineCtx.VSphereMachine.Spec.ProviderID).To(Equal(util.ProviderIDPrefix + biosUUID))
	})

	t.Run("returns error when VSphereVM biosUUID is not valid", func(t *testing.T) {
		g := NewWithT(t)
		vsphereVM.Spec.BiosUUID = "abcde"
		controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		_, err := vimMachineService.reconcileProviderID(ctx, machineCtx, vsphereVM)
		g.Expect(err).To(HaveOccurred())
	})
}

func Test_VimMachineService_reconcileNetwork(t *testing.T) {
	var (
		hostAddr            = "1.2.3.4"
		fakeLongClusterName = "fake-long-clustername"
	)

	getVSphereVM := func(hostAddr string, conditionStatus corev1.ConditionStatus, addresses []string, networkStatus []infrav1.NetworkStatus) *infrav1.VSphereVM {
		return &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: fake.Namespace,
				Name:      fakeLongClusterName,
			},
			Status: infrav1.VSphereVMStatus{
				Host:      hostAddr,
				Ready:     conditionStatus == corev1.ConditionTrue,
				Addresses: addresses,
				Network:   networkStatus,
				Conditions: []clusterv1.Condition{
					{
						Type:   infrav1.VMProvisionedCondition,
						Status: conditionStatus,
					},
				},
			},
		}
	}

	networkStatus := []infrav1.NetworkStatus{
		{Connected: true, IPAddrs: []string{hostAddr}, MACAddr: "aa:bb:cc:dd:ee:ff", NetworkName: "fake"},
	}
	networkStatusWithoutMACAddr := []infrav1.NetworkStatus{
		{Connected: true, IPAddrs: []string{hostAddr}, MACAddr: "", NetworkName: "fake"},
	}
	addresses := []string{"1.2.3.4"}

	t.Run("returns false when VSphereVM addresses and networkStatus are both valid", func(t *testing.T) {
		g := NewWithT(t)
		vsphereVM := getVSphereVM(hostAddr, corev1.ConditionTrue, addresses, networkStatus)
		controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		ok, err := vimMachineService.reconcileNetwork(ctx, machineCtx, vsphereVM)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ok).To(BeTrue())
		g.Expect(machineCtx.VSphereMachine.Status.Addresses).To(ContainElement(clusterv1.MachineAddress{
			Type:    clusterv1.MachineInternalDNS,
			Address: vsphereVM.Name,
		}))
	})
	t.Run("returns true when VSphereVM address is set and network status has no MAC address", func(t *testing.T) {
		g := NewWithT(t)
		vsphereVM := getVSphereVM(hostAddr, corev1.ConditionTrue, addresses, networkStatusWithoutMACAddr)
		controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		ok, err := vimMachineService.reconcileNetwork(ctx, machineCtx, vsphereVM)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ok).To(BeTrue())
	})
}

func Test_VimMachineService_ReconcileNormal(t *testing.T) {
	var (
		hostAddr            = "1.2.3.4"
		fakeLongClusterName = "fake-long-clustername"
	)

	getVSphereVM := func(hostAddr string, conditionStatus corev1.ConditionStatus, addresses []string, networkStatus []infrav1.NetworkStatus) *infrav1.VSphereVM {
		return &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: fake.Namespace,
				Name:      fakeLongClusterName,
			},
			Status: infrav1.VSphereVMStatus{
				Host:      hostAddr,
				Ready:     conditionStatus == corev1.ConditionTrue,
				Addresses: addresses,
				Network:   networkStatus,
				Conditions: []clusterv1.Condition{
					{
						Type:   infrav1.VMProvisionedCondition,
						Status: conditionStatus,
					},
					{
						Type:   clusterv1.ReadyCondition,
						Status: conditionStatus,
					},
				},
			},
		}
	}

	networkStatus := []infrav1.NetworkStatus{
		{Connected: true, IPAddrs: []string{hostAddr}, MACAddr: "aa:bb:cc:dd:ee:ff", NetworkName: "fake"},
	}
	addresses := []string{"1.2.3.4"}
	biosUUID := "42055285-ff20-2c28-965c-05558ea1b4c7"
	t.Run("completes the reconciliation with an existing resource", func(t *testing.T) {
		g := NewWithT(t)
		vsphereVM := getVSphereVM(hostAddr, corev1.ConditionTrue, addresses, networkStatus)
		vsphereVM.Spec.BiosUUID = biosUUID
		controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		requeue, err := vimMachineService.ReconcileNormal(ctx, machineCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(requeue).To(BeFalse())
		g.Expect(machineCtx.VSphereMachine.Status.Ready).To(BeTrue())
	})
	t.Run("creates the VSphereVM when no resource found", func(t *testing.T) {
		g := NewWithT(t)
		controllerManagerContext := fake.NewControllerManagerContext()
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		requeue, err := vimMachineService.ReconcileNormal(ctx, machineCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(requeue).To(BeTrue())
		g.Expect(machineCtx.VSphereMachine.Status.Ready).To(BeFalse())
	})
	t.Run("returns error when the BIOS UUID is invalid", func(t *testing.T) {
		g := NewWithT(t)
		vsphereVM := getVSphereVM(hostAddr, corev1.ConditionTrue, addresses, networkStatus)
		vsphereVM.Spec.BiosUUID = "abcde"
		controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		_, err := vimMachineService.ReconcileNormal(ctx, machineCtx)
		g.Expect(err).To(HaveOccurred())
	})
	t.Run("requeues when the BIOS UUID is not set", func(t *testing.T) {
		g := NewWithT(t)
		vsphereVM := getVSphereVM(hostAddr, corev1.ConditionTrue, addresses, networkStatus)
		vsphereVM.Spec.BiosUUID = ""
		controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		requeue, err := vimMachineService.ReconcileNormal(ctx, machineCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(requeue).To(BeTrue())
	})
	t.Run("requeues when VSphereVM is not ready", func(t *testing.T) {
		g := NewWithT(t)
		vsphereVM := getVSphereVM(hostAddr, corev1.ConditionFalse, nil, nil)
		controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
		machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
		machineCtx.Machine.SetName(fakeLongClusterName)
		machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
		vimMachineService := &VimMachineService{controllerManagerContext.Client}

		requeue, err := vimMachineService.ReconcileNormal(ctx, machineCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(requeue).To(BeTrue())
	})
}

func Test_VimMachineService_ReconcileDelete(t *testing.T) {
	var (
		hostAddr            = "1.2.3.4"
		fakeLongClusterName = "fake-long-clustername"
	)

	getVSphereVM := func(hostAddr string, conditionStatus corev1.ConditionStatus) *infrav1.VSphereVM {
		return &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fakeLongClusterName,
				Namespace: fake.Namespace,
			},
			Status: infrav1.VSphereVMStatus{
				Host: hostAddr,
				Conditions: []clusterv1.Condition{
					{
						Type:   infrav1.VMProvisionedCondition,
						Status: conditionStatus,
					},
					{
						Type:   clusterv1.ReadyCondition,
						Status: conditionStatus,
					},
				},
			},
		}
	}

	vsphereVM := getVSphereVM(hostAddr, corev1.ConditionTrue)
	controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
	machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
	machineCtx.Machine.SetName(fakeLongClusterName)
	machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
	vimMachineService := &VimMachineService{controllerManagerContext.Client}

	t.Run("deletes VSphereVM", func(t *testing.T) {
		g := NewWithT(t)
		err := vimMachineService.ReconcileDelete(ctx, machineCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.Get(machineCtx.VSphereMachine, infrav1.VMProvisionedCondition).Status).To(Equal(conditions.Get(vsphereVM, clusterv1.ReadyCondition).Status))
	})
}

func Test_VimMachineService_FetchVSphereMachine(t *testing.T) {
	var (
		fakeLongClusterName = "fake-long-clustername"
	)

	vsphereMachine := &infrav1.VSphereMachine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VSphereMachine",
			APIVersion: infrav1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: fake.Namespace,
			Name:      fakeLongClusterName,
		},
		Spec: infrav1.VSphereMachineSpec{
			VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
				Datacenter: "dc0",
				Network: infrav1.NetworkSpec{
					Devices: []infrav1.NetworkDeviceSpec{
						{
							NetworkName: "VM Network",
							DHCP4:       true,
							DHCP6:       true,
						},
					},
				},
				NumCPUs:   2,
				MemoryMiB: 2048,
				DiskGiB:   20,
			},
		},
	}

	controllerManagerContext := fake.NewControllerManagerContext(vsphereMachine)
	machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
	machineCtx.Machine.SetName(fakeLongClusterName)
	machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
	vimMachineService := &VimMachineService{controllerManagerContext.Client}

	t.Run("fetches VSphereMachine successfully", func(t *testing.T) {
		g := NewWithT(t)
		_, err := vimMachineService.FetchVSphereMachine(ctx, ctrlclient.ObjectKeyFromObject(vsphereMachine))
		g.Expect(err).NotTo(HaveOccurred())
	})
}

func Test_VimMachineService_FetchVSphereCluster(t *testing.T) {
	var (
		fakeLongClusterName = "fake-long-clustername"
	)

	vsphereCluster := &infrav1.VSphereCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VSphereCluster",
			APIVersion: infrav1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: fake.Namespace,
			Name:      fake.InfrastructureRefName,
		},
		Spec: infrav1.VSphereClusterSpec{
			Server:     "test-server",
			Thumbprint: "test-thumbprint",
			ControlPlaneEndpoint: infrav1.APIEndpoint{
				Host: "1.2.3.4",
				Port: 443,
			},
		},
	}

	controllerManagerContext := fake.NewControllerManagerContext(vsphereCluster)
	machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
	machineCtx.Machine.SetName(fakeLongClusterName)
	machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
	vimMachineService := &VimMachineService{controllerManagerContext.Client}

	t.Run("fetches VSphereCluster successfully", func(t *testing.T) {
		g := NewWithT(t)
		_, err := vimMachineService.FetchVSphereCluster(ctx, machineCtx.Cluster, machineCtx)
		g.Expect(err).NotTo(HaveOccurred())
	})
}

func Test_VimMachineService_SyncFailureReason(t *testing.T) {
	var (
		hostAddr            = "1.2.3.4"
		fakeLongClusterName = "fake-long-clustername"
	)

	getVSphereVM := func(hostAddr string, conditionStatus corev1.ConditionStatus) *infrav1.VSphereVM {
		return &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fakeLongClusterName,
				Namespace: fake.Namespace,
			},
			Status: infrav1.VSphereVMStatus{
				Host: hostAddr,
				Conditions: []clusterv1.Condition{
					{
						Type:   infrav1.VMProvisionedCondition,
						Status: conditionStatus,
					},
				},
				Ready: conditionStatus == corev1.ConditionTrue,
			},
		}
	}

	vsphereVM := getVSphereVM(hostAddr, corev1.ConditionTrue)
	controllerManagerContext := fake.NewControllerManagerContext(vsphereVM)
	machineCtx := fake.NewMachineContext(ctx, fake.NewClusterContext(ctx, controllerManagerContext), controllerManagerContext)
	machineCtx.Machine.SetName(fakeLongClusterName)
	machineCtx.Machine.SetLabels(map[string]string{clusterv1.MachineControlPlaneLabel: "fake-control-plane"})
	vimMachineService := &VimMachineService{controllerManagerContext.Client}

	t.Run("syncs failure reason successfully", func(t *testing.T) {
		g := NewWithT(t)
		_, err := vimMachineService.SyncFailureReason(ctx, machineCtx)
		g.Expect(err).NotTo(HaveOccurred())
	})
}
