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

	// run init func to register the tagging API endpoints.
	_ "github.com/vmware/govmomi/vapi/simulator"
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

func TestGetDiskSpec(t *testing.T) {
	defaultSizeGiB := int32(5)

	model, session, server := initSimulator(t)
	t.Cleanup(model.Remove)
	t.Cleanup(server.Close)
	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine) //nolint:forcetypeassert
	machine := object.NewVirtualMachine(session.Client.Client, vm.Reference())

	devices, err := machine.Device(ctx.TODO())
	if err != nil {
		t.Fatalf("Failed to obtain vm devices: %v", err)
	}
	defaultDisks := devices.SelectByType((*types.VirtualDisk)(nil))
	if len(defaultDisks) < 1 {
		t.Fatal("Unable to find attached disk for resize")
	}
	disk := defaultDisks[0].(*types.VirtualDisk)            //nolint:forcetypeassert
	disk.CapacityInKB = int64(defaultSizeGiB) * 1024 * 1024 // GiB
	if err := machine.EditDevice(ctx.TODO(), disk); err != nil {
		t.Fatalf("Can't resize disk for specified size")
	}

	testCases := []struct {
		expectDevice             bool
		cloneDiskSize            int32
		additionalCloneDiskSizes []int32
		name                     string
		disks                    object.VirtualDeviceList
		err                      string
	}{
		{
			name:          "Successfully clone template with correct disk requirements",
			disks:         defaultDisks,
			cloneDiskSize: defaultSizeGiB,
			expectDevice:  true,
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
			err:           "Error getting disk config spec for primary disk: can't resize template disk down, initial capacity is larger: 6291456KiB > 4194304KiB",
		},
		{
			name:  "Fail to clone template without disk devices",
			disks: object.VirtualDeviceList{},
			err:   "Invalid disk count: 0",
		},
		{
			name:  "Successfully clone template with 2 correct disk requirements",
			disks: append(defaultDisks, defaultDisks...),
			// Disk sizes were bumped up by 1 in the previous test case, defaultSize + 1 is the defaultSize now.
			cloneDiskSize:            defaultSizeGiB + 1,
			additionalCloneDiskSizes: []int32{defaultSizeGiB + 1},
			expectDevice:             true,
		},
		{
			name:                     "Fails to clone template and decrease second disk size",
			disks:                    append(defaultDisks, defaultDisks...),
			cloneDiskSize:            defaultSizeGiB + 2,
			additionalCloneDiskSizes: []int32{defaultSizeGiB},
			err:                      "Error getting disk config spec for additional disk: can't resize template disk down, initial capacity is larger: 7340032KiB > 5242880KiB",
		},
	}

	for _, test := range testCases {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			cloneSpec := v1beta1.VirtualMachineCloneSpec{
				DiskGiB:            tc.cloneDiskSize,
				AdditionalDisksGiB: tc.additionalCloneDiskSizes,
			}
			vsphereVM := &v1beta1.VSphereVM{
				Spec: v1beta1.VSphereVMSpec{
					VirtualMachineCloneSpec: cloneSpec,
				},
			}
			vmContext := &context.VMContext{VSphereVM: vsphereVM}
			devices, err := getDiskSpec(vmContext, tc.disks)
			switch {
			case tc.err != "" && err == nil:
				fallthrough
			case tc.err == "" && err != nil:
				fallthrough
			case err != nil && tc.err != err.Error():
				t.Fatalf("Expected to get '%v' error from getDiskSpec, got: '%v'", tc.err, err)
			}
			if deviceFound := len(devices) != 0; tc.expectDevice != deviceFound {
				t.Fatalf("Expected to get a device: %v, but got: '%#v'", tc.expectDevice, devices)
			}
			if tc.expectDevice {
				primaryDevice := devices[0]
				validateDiskSpec(t, primaryDevice, tc.cloneDiskSize)
				if len(tc.additionalCloneDiskSizes) != 0 {
					secondaryDevice := devices[1]
					validateDiskSpec(t, secondaryDevice, tc.additionalCloneDiskSizes[0])
				}
			}
		})
	}
}

func validateDiskSpec(t *testing.T, device types.BaseVirtualDeviceConfigSpec, cloneDiskSize int32) {
	t.Helper()
	disk := device.GetVirtualDeviceConfigSpec().Device.(*types.VirtualDisk)
	expectedSizeKB := int64(cloneDiskSize) * 1024 * 1024
	if device.GetVirtualDeviceConfigSpec().Operation != types.VirtualDeviceConfigSpecOperationEdit {
		t.Errorf("Disk operation does not match '%s', got: %s",
			types.VirtualDeviceConfigSpecOperationEdit, device.GetVirtualDeviceConfigSpec().Operation)
	}
	if disk.CapacityInKB != expectedSizeKB {
		t.Errorf("Disk size does not match: expected %d, got %d", expectedSizeKB, disk.CapacityInKB)
	}
}

func initSimulator(t *testing.T) (*simulator.Model, *session.Session, *simulator.Server) {
	t.Helper()

	model := simulator.VPX()
	model.Host = 0
	if err := model.Create(); err != nil {
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
			WithUserInfo(server.URL.User.Username(), pass).
			WithDatacenter("*"))
	if err != nil {
		t.Fatal(err)
	}

	return model, authSession, server
}
