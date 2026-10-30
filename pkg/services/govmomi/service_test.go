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
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	pbmsimulator "github.com/vmware/govmomi/pbm/simulator"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/govmomi/v1beta2"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

const (
	defaultStoragePolicy = "vSAN Default Storage Policy"
)

func emptyVirtualMachineContext() *virtualMachineContext {
	return &virtualMachineContext{
		VMContext: capvcontext.VMContext{
			ControllerManagerContext: &capvcontext.ControllerManagerContext{},
		},
	}
}

func Test_reconcileClearBootstrapUserData(t *testing.T) {
	guestNetwork := []infrav1.NetworkStatus{{IPAddrs: []string{"192.0.2.1"}}}
	tests := []struct {
		name        string
		network     []infrav1.NetworkStatus
		extraConfig map[string]string
		wantClear   bool
	}{
		{
			name: "no network status preserves bootstrap data",
			extraConfig: map[string]string{
				guestInfoCloudInitData: "cloud-init-data",
				guestInfoIgnitionData:  "ignition-data",
			},
		},
		{
			name:    "network without guest IP preserves bootstrap data",
			network: []infrav1.NetworkStatus{{}},
			extraConfig: map[string]string{
				guestInfoCloudInitData: "cloud-init-data",
				guestInfoIgnitionData:  "ignition-data",
			},
		},
		{
			name:        "guest IP clears cloud-init data",
			network:     guestNetwork,
			extraConfig: map[string]string{guestInfoCloudInitData: "cloud-init-data"},
			wantClear:   true,
		},
		{
			name:        "guest IP clears Ignition data",
			network:     guestNetwork,
			extraConfig: map[string]string{guestInfoIgnitionData: "ignition-data"},
			wantClear:   true,
		},
		{
			name:    "guest IP on a later network device clears both payloads",
			network: []infrav1.NetworkStatus{{}, {IPAddrs: []string{"2001:db8::1"}}},
			extraConfig: map[string]string{
				guestInfoCloudInitData: "cloud-init-data",
				guestInfoIgnitionData:  "ignition-data",
			},
			wantClear: true,
		},
		{
			name:    "absent bootstrap data does not create a task",
			network: guestNetwork,
		},
		{
			name:    "empty bootstrap data does not create a task",
			network: guestNetwork,
			extraConfig: map[string]string{
				guestInfoCloudInitData: "",
				guestInfoIgnitionData:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			simulator.Run(func(ctx context.Context, c *vim25.Client) error {
				vm, err := find.NewFinder(c).VirtualMachine(ctx, "DC0_H0_VM0")
				g.Expect(err).ToNot(HaveOccurred())

				extraConfig := []types.BaseOptionValue{
					&types.OptionValue{Key: guestInfoKeyMetadata, Value: "metadata"},
					&types.OptionValue{Key: "guestinfo.userdata.encoding", Value: "base64"},
					&types.OptionValue{Key: "guestinfo.ignition.config.data.encoding", Value: "base64"},
					&types.OptionValue{Key: "unrelated", Value: "preserve-me"},
				}
				for key, value := range tt.extraConfig {
					extraConfig = append(extraConfig, &types.OptionValue{Key: key, Value: value})
				}
				task, err := vm.Reconfigure(ctx, types.VirtualMachineConfigSpec{ExtraConfig: extraConfig})
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(task.Wait(ctx)).To(Succeed())

				getExtraConfig := func() map[string]interface{} {
					var obj mo.VirtualMachine
					g.Expect(vm.Properties(ctx, vm.Reference(), []string{"config.extraConfig"}, &obj)).To(Succeed())
					config := map[string]interface{}{}
					for _, ec := range obj.Config.ExtraConfig {
						opt := ec.GetOptionValue()
						config[opt.Key] = opt.Value
					}
					return config
				}
				wantExtraConfig := getExtraConfig()
				if tt.wantClear {
					// Setting an existing extraConfig key to an empty string removes it.
					delete(wantExtraConfig, guestInfoCloudInitData)
					delete(wantExtraConfig, guestInfoIgnitionData)
				}

				vmCtx := emptyVirtualMachineContext()
				vmCtx.Session = &session.Session{Client: &govmomi.Client{Client: c}}
				vmCtx.VSphereVM = &infrav1.VSphereVM{}
				vmCtx.Obj = vm
				vmCtx.Ref = vm.Reference()
				vmCtx.State = &services.VirtualMachine{Network: tt.network}
				vms := &VMService{}

				g.Expect(vms.reconcileClearBootstrapUserData(ctx, vmCtx)).To(Succeed())
				if tt.wantClear {
					g.Expect(vmCtx.VSphereVM.Status.TaskRef).ToNot(BeEmpty())
					task := object.NewTask(c, types.ManagedObjectReference{
						Type:  morefTypeTask,
						Value: vmCtx.VSphereVM.Status.TaskRef,
					})
					g.Expect(task.Wait(ctx)).To(Succeed())
				} else {
					g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
				}
				g.Expect(getExtraConfig()).To(Equal(wantExtraConfig))

				// Once the clearing task completes, another reconcile should be a no-op.
				vmCtx.VSphereVM.Status.TaskRef = ""
				g.Expect(vms.reconcileClearBootstrapUserData(ctx, vmCtx)).To(Succeed())
				g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
				g.Expect(getExtraConfig()).To(Equal(wantExtraConfig))
				return nil
			})
		})
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

		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
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
							{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
							{DeviceID: ptr.To[int32](1234), VendorID: ptr.To[int32](5678)},
						},
					},
				},
			}

			g.Expect(vms.reconcilePCIDevices(ctx, vmCtx)).ToNot(HaveOccurred())

			// get the VM's virtual device list
			devices, err := vm.Device(ctx)
			g.Expect(err).ToNot(HaveOccurred())
			// filter the device with the given backing info
			pciDevices := devices.SelectByBackingInfo(&types.VirtualPCIPassthroughDynamicBackingInfo{
				AllowedDevice: []types.VirtualPCIPassthroughAllowedDevice{
					{DeviceId: 1234, VendorId: 5678},
				},
			})
			g.Expect(pciDevices).To(HaveLen(2))
			return nil
		})
	})
}

func Test_ReconcileStoragePolicy(t *testing.T) {
	var vmCtx *virtualMachineContext
	var g *WithT
	var vms *VMService

	before := func() {
		vmCtx = emptyVirtualMachineContext()
		vmCtx.Client = fake.NewClientBuilder().Build()

		vms = &VMService{}
	}
	t.Run("when VM has no storage policy spec", func(t *testing.T) {
		g = NewWithT(t)
		before()
		vmCtx.VSphereVM = &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1",
				Namespace: "my-namespace",
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{},
			},
		}
		g.Expect(vms.reconcileStoragePolicy(context.Background(), vmCtx)).ToNot(HaveOccurred())
		g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
	})

	t.Run("when the requested storage policy does not exists should fail", func(t *testing.T) {
		g = NewWithT(t)
		before()
		model, err := storagePolicyModel()
		g.Expect(err).ToNot(HaveOccurred())

		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
			authSession, err := getAuthSession(ctx, model.Service.Listen.Host)
			g.Expect(err).ToNot(HaveOccurred())
			vmCtx.Session = authSession
			vm, err := getPoweredoffVM(ctx, c)
			g.Expect(err).ToNot(HaveOccurred())

			vmCtx.Obj = vm
			vmCtx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
						StoragePolicyName: "non-existing-storagepolicy",
					},
				},
			}
			err = vms.reconcileStoragePolicy(context.Background(), vmCtx)
			g.Expect(err.Error()).To(ContainSubstring("no pbm profile found with name"))
			return nil
		}, model)
	})

	t.Run("when the requested storage policy exists should pass", func(t *testing.T) {
		// This Method should be implemented on Govmomi vcsim and then we can unskip this test
		t.Skip("PbmQueryAssociatedProfiles is not yet implemented on PBM simulator")
		g = NewWithT(t)
		before()
		model, err := storagePolicyModel()
		g.Expect(err).ToNot(HaveOccurred())

		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
			authSession, err := getAuthSession(ctx, model.Service.Listen.Host)
			g.Expect(err).ToNot(HaveOccurred())
			vmCtx.Session = authSession
			vm, err := getPoweredoffVM(ctx, c)
			g.Expect(err).ToNot(HaveOccurred())

			vmCtx.Obj = vm
			vmCtx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
						StoragePolicyName: defaultStoragePolicy,
					},
				},
			}
			err = vms.reconcileStoragePolicy(context.Background(), vmCtx)
			g.Expect(err).ToNot(HaveOccurred())
			return nil
		}, model)
	})
}

func Test_DestroyVM(t *testing.T) {
	// A VM whose datastore is inaccessible is not returned by the BIOS UUID lookup,
	// this is simulated by using a BIOS UUID that does not match any VM.
	const unknownBiosUUID = "00000000-0000-0000-0000-000000000000"

	// vmRefFunc returns the VSphereVM's status.vmRef for the given simulator.
	destroyVM := func(t *testing.T, vmRefFunc func(ctx context.Context, model *simulator.Model, c *vim25.Client) string) (services.VirtualMachine, error) {
		t.Helper()
		g := NewWithT(t)
		model := simulator.VPX()
		g.Expect(model.Create()).To(Succeed())
		defer model.Remove()

		var vm services.VirtualMachine
		var destroyErr error
		simulator.Run(func(ctx context.Context, c *vim25.Client) error {
			authSession, err := getAuthSession(ctx, model.Service.Listen.Host)
			g.Expect(err).ToNot(HaveOccurred())
			vmCtx := &capvcontext.VMContext{
				Session: authSession,
				VSphereVM: &infrav1.VSphereVM{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vsphereVM1",
						Namespace: "my-namespace",
					},
					Spec: infrav1.VSphereVMSpec{
						BiosUUID: unknownBiosUUID,
					},
					Status: infrav1.VSphereVMStatus{
						VMRef: vmRefFunc(ctx, model, c),
					},
				},
			}
			_, vm, destroyErr = (&VMService{}).DestroyVM(ctx, vmCtx)
			return nil
		}, model)
		return vm, destroyErr
	}

	t.Run("when VM is not found by bios uuid but still exists by vmRef should not report it as not found", func(t *testing.T) {
		g := NewWithT(t)
		vm, err := destroyVM(t, func(ctx context.Context, model *simulator.Model, c *vim25.Client) string {
			vm, err := find.NewFinder(c).VirtualMachine(ctx, "DC0_H0_VM0")
			g.Expect(err).ToNot(HaveOccurred())
			simVM := model.Map().Get(vm.Reference()).(*simulator.VirtualMachine)
			simVM.Summary.Runtime.ConnectionState = types.VirtualMachineConnectionStateInaccessible
			return vm.Reference().String()
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("inaccessible"))
		g.Expect(vm.State).ToNot(Equal(services.VirtualMachineStateNotFound))
	})

	t.Run("when VM is not found by bios uuid nor by vmRef should report it as not found", func(t *testing.T) {
		g := NewWithT(t)
		vm, err := destroyVM(t, func(context.Context, *simulator.Model, *vim25.Client) string {
			return "VirtualMachine:vm-does-not-exist"
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(vm.State).To(Equal(services.VirtualMachineStateNotFound))
	})

	t.Run("when VM is not found by bios uuid and has no vmRef should report it as not found", func(t *testing.T) {
		g := NewWithT(t)
		vm, err := destroyVM(t, func(context.Context, *simulator.Model, *vim25.Client) string {
			return ""
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(vm.State).To(Equal(services.VirtualMachineStateNotFound))
	})
}

func getAuthSession(ctx context.Context, server string) (*session.Session, error) {
	password, _ := simulator.DefaultLogin.Password()
	return session.GetOrCreate(
		ctx,
		session.NewParams().
			WithUserInfo(simulator.DefaultLogin.Username(), password).
			WithServer(fmt.Sprintf("http://%s", server)).
			WithDatacenter("*"))
}

func getPoweredoffVM(ctx context.Context, c *vim25.Client) (*object.VirtualMachine, error) {
	finder := find.NewFinder(c)
	vm, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
	if err != nil {
		return nil, err
	}

	_, err = vm.PowerOff(ctx)
	return vm, err
}

func storagePolicyModel() (*simulator.Model, error) {
	model := simulator.VPX()
	err := model.Create()
	if err != nil {
		return nil, err
	}
	model.Service.RegisterSDK(pbmsimulator.New())
	model.Machine = 1
	model.Datacenter = 1
	model.Host = 1
	return model, nil
}
