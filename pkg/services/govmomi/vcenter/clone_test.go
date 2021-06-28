/*
Copyright 2020 The Kubernetes Authors.

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

package vcenter

import (
	ctx "context"
	"crypto/tls"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

func TestGetDiskSpec(t *testing.T) {
	defaultSizeGiB := int32(5)

	model, session, server := initSimulator(t)
	defer model.Remove()
	defer server.Close()
	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	machine := object.NewVirtualMachine(session.Client.Client, vm.Reference())

	devices, err := machine.Device(ctx.TODO())
	if err != nil {
		t.Fatalf("Failed to obtain vm devices: %v", err)
	}
	defaultDisks := devices.SelectByType((*types.VirtualDisk)(nil))
	if len(defaultDisks) < 1 {
		t.Fatal("Unable to find attached disk for resize")
	}
	disk := defaultDisks[0].(*types.VirtualDisk)
	disk.CapacityInKB = int64(defaultSizeGiB) * 1024 * 1024 // GiB
	if err := machine.EditDevice(ctx.TODO(), disk); err != nil {
		t.Fatalf("Can't resize disk for specified size")
	}

	testCases := []struct {
		expectDevice  bool
		cloneDiskSize int32
		name          string
		disks         object.VirtualDeviceList
		err           string
	}{
		{
			name:          "Successfully clone template with correct disk requirements",
			disks:         defaultDisks,
			cloneDiskSize: defaultSizeGiB,
			expectDevice:  true,
		},
		{
			name:  "Fail to clone template without disk devices",
			disks: object.VirtualDeviceList{},
			err:   "invalid disk count: 0",
		},
		{
			name:  "Fail to clone template with multiple disk devices",
			disks: append(defaultDisks, defaultDisks...),
			err:   "invalid disk count: 2",
		},
		{
			name:          "Successfully clone template and increase disk requirements",
			disks:         defaultDisks,
			cloneDiskSize: defaultSizeGiB + 1,
			expectDevice:  true,
		},
		{
			name:          "Fail to clone template with lower disk requirements then on template",
			disks:         defaultDisks,
			cloneDiskSize: defaultSizeGiB - 1,
			err:           "can't resize template disk down, initial capacity is larger: 6291456KiB > 4194304KiB",
		},
	}

	for _, test := range testCases {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			cloneSpec := v1alpha4.VirtualMachineCloneSpec{
				DiskGiB: tc.cloneDiskSize,
			}
			vsphereVM := &v1alpha4.VSphereVM{
				Spec: v1alpha4.VSphereVMSpec{
					VirtualMachineCloneSpec: cloneSpec,
				},
			}
			vmContext := &context.VMContext{VSphereVM: vsphereVM}
			device, err := getDiskSpec(vmContext, tc.disks)
			switch {
			case tc.err != "" && err == nil:
				fallthrough
			case tc.err == "" && err != nil:
				fallthrough
			case err != nil && tc.err != err.Error():
				t.Fatalf("Expected to get '%v' error from getDiskSpec, got: '%v'", tc.err, err)
			}
			if deviceFound := device != nil; tc.expectDevice != deviceFound {
				t.Fatalf("Expected to get a device: %v, but got: '%#v'", tc.expectDevice, device)
			}
			if tc.expectDevice {
				disk := device.GetVirtualDeviceConfigSpec().Device.(*types.VirtualDisk)
				expectedSizeKB := int64(tc.cloneDiskSize) * 1024 * 1024
				if device.GetVirtualDeviceConfigSpec().Operation != types.VirtualDeviceConfigSpecOperationEdit {
					t.Errorf("Disk operation does not match '%s', got: %s",
						types.VirtualDeviceConfigSpecOperationEdit, device.GetVirtualDeviceConfigSpec().Operation)
				}
				if disk.CapacityInKB != expectedSizeKB {
					t.Errorf("Disk size does not match: expected %d, got %d", expectedSizeKB, disk.CapacityInKB)
				}
			}
		})
	}
}

func initSimulator(t *testing.T) (*simulator.Model, *session.Session, *simulator.Server) {
	model := simulator.VPX()
	model.Host = 0
	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}
	model.Service.TLS = new(tls.Config)
	model.Service.RegisterEndpoints = true

	server := model.Service.NewServer()
	pass, _ := server.URL.User.Password()

	authSession, err := session.GetOrCreate(
		ctx.TODO(),
		session.NewParams().
			WithServer(server.URL.Host).
			WithUserInfo(server.URL.User.Username(), pass))
	if err != nil {
		t.Fatal(err)
	}

	return model, authSession, server
}
