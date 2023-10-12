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

package govmomi

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

func TestIsSoftPowerOffTimeoutExceeded(t *testing.T) {
	var g *WithT
	t.Run("does not time out with no condition", func(t *testing.T) {
		g = NewWithT(t)
		vm := &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1",
				Namespace: "my-namespace",
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
				PowerOffMode:             infrav1.VirtualMachinePowerOpModeTrySoft,
				GuestSoftPowerOffTimeout: nil,
			},
		}
		vms := &VMService{}

		g.Expect(vms.isSoftPowerOffTimeoutExceeded(vm)).To(BeFalse())
	})
	t.Run("does not time out when powerOffMode set to soft", func(t *testing.T) {
		g = NewWithT(t)
		vm := &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1",
				Namespace: "my-namespace",
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
				PowerOffMode:             infrav1.VirtualMachinePowerOpModeSoft,
				GuestSoftPowerOffTimeout: nil,
			},
			Status: infrav1.VSphereVMStatus{
				Conditions: []clusterv1.Condition{
					{
						Type:               infrav1.GuestSoftPowerOffSucceededCondition,
						Status:             infrav1.GuestSoftPowerOffInProgressReason,
						LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
					},
				},
			},
		}
		vms := &VMService{}

		g.Expect(vms.isSoftPowerOffTimeoutExceeded(vm)).To(BeFalse())
	})
	t.Run("does not time out when guestSoftPowerOffTimeout set to 0", func(t *testing.T) {
		g = NewWithT(t)
		vm := &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1",
				Namespace: "my-namespace",
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
				PowerOffMode:             infrav1.VirtualMachinePowerOpModeTrySoft,
				GuestSoftPowerOffTimeout: &metav1.Duration{Duration: 0},
			},
			Status: infrav1.VSphereVMStatus{
				Conditions: []clusterv1.Condition{
					{
						Type:               infrav1.GuestSoftPowerOffSucceededCondition,
						Status:             infrav1.GuestSoftPowerOffInProgressReason,
						LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
					},
				},
			},
		}
		vms := &VMService{}

		g.Expect(vms.isSoftPowerOffTimeoutExceeded(vm)).To(BeFalse())
	})
	t.Run("time out", func(t *testing.T) {
		g = NewWithT(t)
		vm := &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1",
				Namespace: "my-namespace",
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
				PowerOffMode:             infrav1.VirtualMachinePowerOpModeTrySoft,
				GuestSoftPowerOffTimeout: &metav1.Duration{Duration: time.Minute},
			},
			Status: infrav1.VSphereVMStatus{
				Conditions: []clusterv1.Condition{
					{
						Type:               infrav1.GuestSoftPowerOffSucceededCondition,
						Status:             infrav1.GuestSoftPowerOffInProgressReason,
						LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
					},
				},
			},
		}
		vms := &VMService{}

		g.Expect(vms.isSoftPowerOffTimeoutExceeded(vm)).To(BeTrue())
	})
}

func TestTriggerSoftPowerOff(t *testing.T) {
	var vmCtx *virtualMachineContext
	var g *WithT
	var vms *VMService

	before := func() {
		vmCtx = emptyVirtualMachineContext()
		vmCtx.Client = fake.NewClientBuilder().Build()

		vms = &VMService{}
	}

	t.Run("should report no pending when powerOffMode set to hard", func(t *testing.T) {
		g = NewWithT(t)
		before()

		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
			finder := find.NewFinder(c)
			vm, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
			g.Expect(err).NotTo(HaveOccurred())

			vmCtx.Obj = vm
			vmCtx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
					PowerOffMode:             infrav1.VirtualMachinePowerOpModeHard,
					GuestSoftPowerOffTimeout: nil,
				},
			}

			pending, err := vms.triggerSoftPowerOff(ctx, vmCtx)
			g.Expect(pending).To(BeFalse())
			g.Expect(err).ToNot(HaveOccurred())
			return nil
		})
	})

	t.Run("should report pending after the triggering when the powerOffMode set to soft", func(t *testing.T) {
		g = NewWithT(t)
		before()

		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
			finder := find.NewFinder(c)
			vm, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
			g.Expect(err).NotTo(HaveOccurred())

			vmCtx.Obj = vm
			vmCtx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
					PowerOffMode:             infrav1.VirtualMachinePowerOpModeSoft,
					GuestSoftPowerOffTimeout: nil,
				},
				Status: infrav1.VSphereVMStatus{
					Conditions: []clusterv1.Condition{
						{
							Type:               infrav1.GuestSoftPowerOffSucceededCondition,
							Status:             infrav1.GuestSoftPowerOffInProgressReason,
							LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						},
					},
				},
			}

			pending, err := vms.triggerSoftPowerOff(ctx, vmCtx)
			g.Expect(pending).To(BeTrue())
			g.Expect(err).ToNot(HaveOccurred())
			return nil
		})
	})
	t.Run("should return pending before the last operation times out when powerOffMode set to trySoft", func(t *testing.T) {
		g = NewWithT(t)
		before()

		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
			finder := find.NewFinder(c)
			vm, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
			g.Expect(err).NotTo(HaveOccurred())

			vmCtx.Obj = vm
			vmCtx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
					PowerOffMode:             infrav1.VirtualMachinePowerOpModeTrySoft,
					GuestSoftPowerOffTimeout: &metav1.Duration{Duration: 3 * time.Minute},
				},
				Status: infrav1.VSphereVMStatus{
					Conditions: []clusterv1.Condition{
						{
							Type:               infrav1.GuestSoftPowerOffSucceededCondition,
							Status:             infrav1.GuestSoftPowerOffInProgressReason,
							LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						},
					},
				},
			}

			pending, err := vms.triggerSoftPowerOff(ctx, vmCtx)
			g.Expect(pending).To(BeTrue())
			g.Expect(err).ToNot(HaveOccurred())
			return nil
		})
	})
	t.Run("should return no pending if the last operation times out when powerOffMode set to trySoft", func(t *testing.T) {
		g = NewWithT(t)
		before()

		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
			finder := find.NewFinder(c)
			vm, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
			g.Expect(err).NotTo(HaveOccurred())

			vmCtx.Obj = vm
			vmCtx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
					PowerOffMode:             infrav1.VirtualMachinePowerOpModeTrySoft,
					GuestSoftPowerOffTimeout: &metav1.Duration{Duration: 1 * time.Minute},
				},
				Status: infrav1.VSphereVMStatus{
					Conditions: []clusterv1.Condition{
						{
							Type:               infrav1.GuestSoftPowerOffSucceededCondition,
							Status:             infrav1.GuestSoftPowerOffInProgressReason,
							LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						},
					},
				},
			}

			pending, err := vms.triggerSoftPowerOff(ctx, vmCtx)
			g.Expect(pending).To(BeFalse())
			g.Expect(err).ToNot(HaveOccurred())
			return nil
		})
	})
	t.Run("should return pending when VMware Tools is not running and powerOffMode set to soft", func(t *testing.T) {
		g = NewWithT(t)
		before()

		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
			finder := find.NewFinder(c)
			vm, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
			g.Expect(err).NotTo(HaveOccurred())

			vmCtx.Obj = vm
			vmCtx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
					PowerOffMode:             infrav1.VirtualMachinePowerOpModeSoft,
					GuestSoftPowerOffTimeout: nil,
				},
			}

			pending, err := vms.triggerSoftPowerOff(ctx, vmCtx)
			g.Expect(pending).To(BeTrue())
			g.Expect(err).ToNot(HaveOccurred())
			return nil
		})
	})
	t.Run("should return no pending when VMware Tools is not running and powerOffMode set to trySoft", func(t *testing.T) {
		g = NewWithT(t)
		before()

		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
			finder := find.NewFinder(c)
			vm, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
			g.Expect(err).NotTo(HaveOccurred())

			vmCtx.Obj = vm
			vmCtx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec:  infrav1.VirtualMachineCloneSpec{},
					PowerOffMode:             infrav1.VirtualMachinePowerOpModeTrySoft,
					GuestSoftPowerOffTimeout: &metav1.Duration{Duration: 1 * time.Minute},
				},
				Status: infrav1.VSphereVMStatus{
					Conditions: []clusterv1.Condition{
						{
							Type:               infrav1.GuestSoftPowerOffSucceededCondition,
							Status:             infrav1.GuestSoftPowerOffInProgressReason,
							LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						},
					},
				},
			}

			pending, err := vms.triggerSoftPowerOff(ctx, vmCtx)
			g.Expect(pending).To(BeFalse())
			g.Expect(err).ToNot(HaveOccurred())
			return nil
		})
	})
	// TODO: add more tests on VMware Tools reports running
}
