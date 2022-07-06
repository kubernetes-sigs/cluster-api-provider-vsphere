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
		expectedSizes            []int32
		cloneDiskSize            int32
		additionalCloneDiskSizes []int32
		name                     string
		devices                  object.VirtualDeviceList
		err                      string
		cloneDisks               []v1beta1.DiskSpec
	}{
		{
			name:          "Successfully clone template with correct deprecated disk requirements",
			devices:       defaultDisks,
			cloneDiskSize: defaultSizeGiB,
			expectedSizes: []int32{defaultSizeGiB},
		},
		{
			name:          "Successfully clone template and increase deprecated disk requirements",
			devices:       defaultDisks,
			cloneDiskSize: defaultSizeGiB + 1,
			expectedSizes: []int32{defaultSizeGiB + 1},
		},
		{
			name:          "Fail to clone template with lower deprecated disk requirements then on template",
			devices:       defaultDisks,
			cloneDiskSize: defaultSizeGiB - 1,
			err:           "Error getting disk config spec for disk 0: can't resize template disk down, initial capacity is larger: 6291456KiB > 4194304KiB",
		},
		{
			name:    "Fail to clone template without disk devices",
			devices: object.VirtualDeviceList{},
			err:     "Invalid disk count: 0",
		},
		{
			name:    "Successfully clone template with 2 correct deprecated disk requirements",
			devices: append(defaultDisks, defaultDisks...),
			// Disk sizes were bumped up by 1 in the previous test case, defaultSize + 1 is the defaultSize now.
			cloneDiskSize:            defaultSizeGiB + 1,
			additionalCloneDiskSizes: []int32{defaultSizeGiB + 1},
			expectedSizes:            []int32{defaultSizeGiB + 1, defaultSizeGiB + 1},
		},
		{
			name:                     "Fails to clone template and decrease second deprecated disk size",
			devices:                  append(defaultDisks, defaultDisks...),
			cloneDiskSize:            defaultSizeGiB + 2,
			additionalCloneDiskSizes: []int32{defaultSizeGiB},
			err:                      "Error getting disk config spec for disk 1: can't resize template disk down, initial capacity is larger: 7340032KiB > 5242880KiB",
		},

		{
			name:          "Successfully clone template with correct disks requirements",
			devices:       defaultDisks,
			cloneDisks:    []v1beta1.DiskSpec{{SizeGiB: int64(defaultSizeGiB + 2)}},
			expectedSizes: []int32{defaultSizeGiB + 2},
		},
		{
			name:          "Successfully clone template and increase disk requirements",
			devices:       defaultDisks,
			cloneDisks:    []v1beta1.DiskSpec{{SizeGiB: int64(defaultSizeGiB + 3)}},
			expectedSizes: []int32{defaultSizeGiB + 3},
		},
		{
			name:       "Fail to clone template with lower disk requirements then on template",
			devices:    defaultDisks,
			cloneDisks: []v1beta1.DiskSpec{{SizeGiB: int64(defaultSizeGiB - 1)}},
			err:        "Error getting disk config spec for disk 0: can't resize template disk down, initial capacity is larger: 8388608KiB > 4194304KiB",
		},
		{
			name:    "Successfully clone template with 2 correct disk requirements",
			devices: append(defaultDisks, defaultDisks...),
			// Disk sizes were bumped up by 1 in the previous test case, defaultSize + 1 is the defaultSize now.
			cloneDisks:    []v1beta1.DiskSpec{{SizeGiB: int64(defaultSizeGiB + 4)}, {SizeGiB: int64(defaultSizeGiB + 4)}},
			expectedSizes: []int32{defaultSizeGiB + 4, defaultSizeGiB + 4},
		},
		{
			name:       "Fails to clone template and decrease second disk size",
			devices:    append(defaultDisks, defaultDisks...),
			cloneDisks: []v1beta1.DiskSpec{{SizeGiB: int64(defaultSizeGiB + 5)}, {SizeGiB: int64(defaultSizeGiB)}},
			err:        "Error getting disk config spec for disk 1: can't resize template disk down, initial capacity is larger: 10485760KiB > 5242880KiB",
		},

		{
			name:          "Fails to clone template when deprecated and new fields are set",
			devices:       append(defaultDisks, defaultDisks...),
			cloneDiskSize: defaultSizeGiB + 6,
			cloneDisks:    []v1beta1.DiskSpec{{SizeGiB: int64(defaultSizeGiB + 6)}, {SizeGiB: int64(defaultSizeGiB + 6)}},
			err:           "can't set deprecated fields (diskGiB and additionalDisksGiB) and disks",
		},
		{
			name:                     "Fails to clone template when deprecated and new fields are set",
			devices:                  append(defaultDisks, defaultDisks...),
			additionalCloneDiskSizes: []int32{defaultSizeGiB + 6},
			cloneDisks:               []v1beta1.DiskSpec{{SizeGiB: int64(defaultSizeGiB + 6)}, {SizeGiB: int64(defaultSizeGiB + 6)}},
			err:                      "can't set deprecated fields (diskGiB and additionalDisksGiB) and disks",
		},
	}

	for _, test := range testCases {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			cloneSpec := v1beta1.VirtualMachineCloneSpec{
				DiskGiB:            tc.cloneDiskSize,
				AdditionalDisksGiB: tc.additionalCloneDiskSizes,
				Disks:              tc.cloneDisks,
			}
			vsphereVM := &v1beta1.VSphereVM{
				Spec: v1beta1.VSphereVMSpec{
					VirtualMachineCloneSpec: cloneSpec,
				},
			}
			vmContext := &context.VMContext{VSphereVM: vsphereVM}
			devices, err := getDiskSpecs(vmContext, tc.devices)
			switch {
			case tc.err != "" && err == nil:
				fallthrough
			case tc.err == "" && err != nil:
				fallthrough
			case err != nil && tc.err != err.Error():
				t.Fatalf("Expected to get '%v' error from getDiskSpec, got: '%v'", tc.err, err)
			}
			if len(devices) != len(tc.expectedSizes) {
				t.Fatalf("Expected to get %d devices, but got %d: %q", len(tc.expectedSizes), len(devices), devices)
			}
			if len(tc.expectedSizes) > 0 {
				for i, expectedSize := range tc.expectedSizes {
					validateDiskSpec(t, devices[i], expectedSize)
				}
			}
		})
	}
}

func TestPCISpec(t *testing.T) {
	defaultVendorID := int32(7864)
	defaultDeviceID := int32(4318)

	anotherDeviceID := int32(1234)
	anotherVendorID := int32(5678)

	testCases := []struct {
		deviceSpecs []v1beta1.PCIDeviceSpec
		name        string
		err         string
	}{
		{
			name: "single device",
			deviceSpecs: []v1beta1.PCIDeviceSpec{
				{
					DeviceID: &defaultDeviceID,
					VendorID: &defaultVendorID,
				},
			},
		},
		{
			name: "multiple devices",
			deviceSpecs: []v1beta1.PCIDeviceSpec{
				{
					DeviceID: &defaultDeviceID,
					VendorID: &defaultVendorID,
				},
				{
					DeviceID: &anotherDeviceID,
					VendorID: &anotherVendorID,
				},
			},
		},
	}

	for _, test := range testCases {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			vsphereVM := &v1beta1.VSphereVM{
				Spec: v1beta1.VSphereVMSpec{
					VirtualMachineCloneSpec: v1beta1.VirtualMachineCloneSpec{
						PciDevices: tc.deviceSpecs,
					},
				},
			}
			vmContext := &context.VMContext{VSphereVM: vsphereVM}
			deviceSpecs, err := getGpuSpecs(vmContext)
			if err != nil {
				t.Fatal(err)
			}
			if len(deviceSpecs) != len(tc.deviceSpecs) {
				t.Fatalf("Expected number of deviceSpecs: %d, but got: '%d'", len(deviceSpecs), len(tc.deviceSpecs))
			}
			for _, deviceSpec := range deviceSpecs {
				if deviceSpec.GetVirtualDeviceConfigSpec().Operation != types.VirtualDeviceConfigSpecOperationAdd {
					t.Fatalf("incorrect operation: %s", deviceSpec.GetVirtualDeviceConfigSpec().Operation)
				}
			}
			validatePCISpec(t, vmContext.VSphereVM.Spec.PciDevices, tc.deviceSpecs)
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

func validatePCISpec(t *testing.T, devices []v1beta1.PCIDeviceSpec, expectedDevices []v1beta1.PCIDeviceSpec) {
	t.Helper()
	expectedDeviceMap := make(map[int32]int32, len(expectedDevices))
	for _, expected := range expectedDevices {
		expectedDeviceMap[*expected.DeviceID] = *expected.VendorID
	}

	for _, device := range devices {
		val, ok := expectedDeviceMap[*device.DeviceID]
		if !ok {
			t.Errorf("expected to found device with deviceID %d", *device.DeviceID)
		}
		if val != *device.VendorID {
			t.Errorf("expected to find matching vendor id, found %d expected %d", val, *device.DeviceID)
		}
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
