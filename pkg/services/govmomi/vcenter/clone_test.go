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
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	_ "github.com/vmware/govmomi/vapi/simulator" // run init func to register the tagging API endpoints.
	"github.com/vmware/govmomi/vim25/types"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

func TestGetDiskSpec(t *testing.T) {
	defaultSizeGiB := int32(20)

	model, session, server := initSimulator(t)
	t.Cleanup(model.Remove)
	t.Cleanup(server.Close)
	vm := model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
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
		expectDevice             bool
		cloneDiskSize            int32
		additionalCloneDiskSizes []int32
		name                     string
		disks                    object.VirtualDeviceList
		dataDisks                []infrav1.VSphereDisk
		expectedDiskCount        int
		err                      string
	}{
		{
			name:              "Successfully clone template with correct disk requirements",
			disks:             defaultDisks,
			cloneDiskSize:     defaultSizeGiB,
			expectDevice:      true,
			expectedDiskCount: 1,
		},
		{
			name:              "Successfully clone template and increase disk requirements",
			disks:             defaultDisks,
			cloneDiskSize:     defaultSizeGiB + 1,
			expectDevice:      true,
			expectedDiskCount: 1,
		},
		{
			name:              "Successfully clone template with no explicit disk requirements",
			disks:             defaultDisks,
			cloneDiskSize:     0,
			expectDevice:      true,
			expectedDiskCount: 1,
		},
		{
			name:          "Fail to clone template with lower disk requirements then on template",
			disks:         defaultDisks,
			cloneDiskSize: defaultSizeGiB - 1,
			err:           "Error getting disk config spec for primary disk: can't resize template disk down, initial capacity is larger: 22020096KiB > 19922944KiB",
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
			expectedDiskCount:        2,
		},
		{
			name:                     "Fails to clone template and decrease second disk size",
			disks:                    append(defaultDisks, defaultDisks...),
			cloneDiskSize:            defaultSizeGiB + 2,
			additionalCloneDiskSizes: []int32{defaultSizeGiB},
			err:                      "Error getting disk config spec for additional disk: can't resize template disk down, initial capacity is larger: 23068672KiB > 20971520KiB",
		},
	}

	for _, test := range testCases {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			cloneSpec := infrav1.VirtualMachineCloneSpec{
				DiskGiB:            tc.cloneDiskSize,
				AdditionalDisksGiB: tc.additionalCloneDiskSizes,
			}
			vsphereVM := &infrav1.VSphereVM{
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec: cloneSpec,
				},
			}
			vmContext := &capvcontext.VMContext{VSphereVM: vsphereVM}
			deviceResults, err := getDiskSpec(vmContext, tc.disks)
			if (tc.err != "" && err == nil) || (tc.err == "" && err != nil) || (err != nil && tc.err != err.Error()) {
				t.Fatalf("Expected to get '%v' error from getDiskSpec, got: '%v'", tc.err, err)
			}
			if deviceFound := len(deviceResults) != 0; tc.expectDevice != deviceFound {
				t.Fatalf("Expected to get a device: %v, but got: '%#v'", tc.expectDevice, deviceResults)
			}
			if tc.expectDevice {
				primaryDevice := deviceResults[0]
				validateDiskSpec(t, primaryDevice, tc.cloneDiskSize)
				if len(tc.additionalCloneDiskSizes) != 0 {
					secondaryDevice := deviceResults[1]
					validateDiskSpec(t, secondaryDevice, tc.additionalCloneDiskSizes[0])
				}

				// Check number of disks present
				if len(deviceResults) != tc.expectedDiskCount {
					t.Fatalf("Expected device count to be %v, but found %v", tc.expectedDiskCount, len(deviceResults))
				}
			}
		})
	}
}

func TestCreateDataDisks(t *testing.T) {
	model, session, server := initSimulator(t)
	t.Cleanup(model.Remove)
	t.Cleanup(server.Close)
	vm := model.Map().Any("VirtualMachine").(*simulator.VirtualMachine)
	machine := object.NewVirtualMachine(session.Client.Client, vm.Reference())

	deviceList, err := machine.Device(ctx.TODO())
	if err != nil {
		t.Fatalf("Failed to obtain vm devices: %v", err)
	}

	// Find primary disk and get controller
	disks := deviceList.SelectByType((*types.VirtualDisk)(nil))
	primaryDisk := disks[0].(*types.VirtualDisk)
	controller, ok := deviceList.FindByKey(primaryDisk.ControllerKey).(types.BaseVirtualController)
	if !ok {
		t.Fatalf("unable to get controller for test")
	}

	testCases := []struct {
		name               string
		devices            object.VirtualDeviceList
		controller         types.BaseVirtualController
		dataDisks          []infrav1.VSphereDisk
		expectedUnitNumber []int
		err                string
	}{
		{
			name:               "Add data disk with 1 ova disk",
			devices:            deviceList,
			controller:         controller,
			dataDisks:          createDataDiskDefinitions(1, nil),
			expectedUnitNumber: []int{1},
		},
		{
			name:               "Add data disk with 2 ova disk",
			devices:            createAdditionalDisks(deviceList, controller, 1),
			controller:         controller,
			dataDisks:          createDataDiskDefinitions(1, nil),
			expectedUnitNumber: []int{2},
		},
		{
			name:               "Add multiple data disk with 1 ova disk",
			devices:            deviceList,
			controller:         controller,
			dataDisks:          createDataDiskDefinitions(2, nil),
			expectedUnitNumber: []int{1, 2},
		},
		{
			name:       "Add too many data disks with 1 ova disk",
			devices:    deviceList,
			controller: controller,
			dataDisks:  createDataDiskDefinitions(30, nil),
			err:        "all unit numbers are already in-use",
		},
		{
			name:       "Add data disk with no ova disk",
			devices:    nil,
			controller: nil,
			dataDisks:  createDataDiskDefinitions(1, nil),
			err:        "Invalid disk count: 0",
		},
		{
			name:       "Add too many data disks with 1 ova disk",
			devices:    deviceList,
			controller: controller,
			dataDisks:  createDataDiskDefinitions(40, nil),
			err:        "all unit numbers are already in-use",
		},
		{
			name:               "Create data disk with Thin provisioning",
			devices:            deviceList,
			controller:         controller,
			dataDisks:          createDataDiskDefinitions(1, &infrav1.ThinProvisioningMode),
			expectedUnitNumber: []int{1},
		},
		{
			name:               "Create data disk with Thick provisioning",
			devices:            deviceList,
			controller:         controller,
			dataDisks:          createDataDiskDefinitions(1, &infrav1.ThickProvisioningMode),
			expectedUnitNumber: []int{1},
		},
		{
			name:               "Create data disk with EagerZeroed provisioning",
			devices:            deviceList,
			controller:         controller,
			dataDisks:          createDataDiskDefinitions(1, &infrav1.EagerlyZeroedProvisioningMode),
			expectedUnitNumber: []int{1},
		},
		{
			name:               "Create data disk without provisioning type set",
			devices:            deviceList,
			controller:         controller,
			dataDisks:          createDataDiskDefinitions(1, nil),
			expectedUnitNumber: []int{1},
		},
	}

	for _, test := range testCases {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			var funcError error

			g := gomega.NewWithT(t)

			// Create the data disks
			newDisks, funcError := createDataDisks(ctx.TODO(), tc.dataDisks, tc.devices)
			if (tc.err != "" && funcError == nil) || (tc.err == "" && funcError != nil) || (funcError != nil && tc.err != funcError.Error()) {
				t.Fatalf("Expected to get '%v' error from assignUnitNumber, got: '%v'", tc.err, funcError)
			}

			if tc.err == "" && funcError == nil {
				// Check number of disks present
				if len(newDisks) != len(tc.dataDisks) {
					t.Fatalf("Expected device count to be %v, but found %v", len(tc.dataDisks), len(newDisks))
				}

				// Validate the configs of new data disks
				for index, disk := range newDisks {
					// Check disk size matches original request
					vd := disk.GetVirtualDeviceConfigSpec().Device.(*types.VirtualDisk)
					expectedSize := int64(tc.dataDisks[index].SizeGiB * 1024 * 1024)
					if vd.CapacityInKB != expectedSize {
						t.Fatalf("Expected disk size (KB) %d to match %d", vd.CapacityInKB, expectedSize)
					}

					// Check unit number
					unitNumber := *disk.GetVirtualDeviceConfigSpec().Device.GetVirtualDevice().UnitNumber
					if tc.err == "" && unitNumber != int32(tc.expectedUnitNumber[index]) {
						t.Fatalf("Expected to get unitNumber '%d' error from assignUnitNumber, got: '%d'", tc.expectedUnitNumber[index], unitNumber)
					}

					// Check to see if the provision type matches.
					backingInfo := disk.GetVirtualDeviceConfigSpec().Device.GetVirtualDevice().Backing.(*types.VirtualDiskFlatVer2BackingInfo)
					switch tc.dataDisks[index].ProvisioningMode {
					case infrav1.ThinProvisioningMode:
						g.Expect(backingInfo.ThinProvisioned).To(gomega.Equal(types.NewBool(true)))
						g.Expect(backingInfo.EagerlyScrub).To(gomega.BeNil())
					case infrav1.ThickProvisioningMode:
						g.Expect(backingInfo.ThinProvisioned).To(gomega.Equal(types.NewBool(false)))
						g.Expect(backingInfo.EagerlyScrub).To(gomega.BeNil())
					case infrav1.EagerlyZeroedProvisioningMode:
						g.Expect(backingInfo.ThinProvisioned).To(gomega.Equal(types.NewBool(false)))
						g.Expect(backingInfo.EagerlyScrub).To(gomega.Equal(types.NewBool(true)))
					default:
						// If not set, the behaviour may depend on the configuration of the backing datastore.
					}
				}
			}
		})
	}
}

func createAdditionalDisks(devices object.VirtualDeviceList, controller types.BaseVirtualController, numOfDisks int) object.VirtualDeviceList {
	deviceList := devices
	disks := devices.SelectByType((*types.VirtualDisk)(nil))
	primaryDisk := disks[0].(*types.VirtualDisk)

	for i := 0; i < numOfDisks; i++ {
		newDevice := createVirtualDisk(primaryDisk.ControllerKey+1, controller, 10)
		newUnitNumber := *primaryDisk.UnitNumber + int32(i+1)
		newDevice.UnitNumber = &newUnitNumber
		deviceList = append(deviceList, newDevice)
	}
	return deviceList
}

func createVirtualDisk(key int32, controller types.BaseVirtualController, diskSize int32) *types.VirtualDisk {
	dev := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key: key,
			Backing: &types.VirtualDiskFlatVer2BackingInfo{
				DiskMode:        string(types.VirtualDiskModePersistent),
				ThinProvisioned: types.NewBool(true),
				VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
					FileName: "",
				},
			},
		},
		CapacityInKB: int64(diskSize) * 1024 * 1024,
	}

	if controller != nil {
		dev.VirtualDevice.ControllerKey = controller.GetVirtualController().Key
	}
	return dev
}

func createDataDiskDefinitions(numOfDataDisks int, provisionType *infrav1.ProvisioningMode) []infrav1.VSphereDisk {
	disks := []infrav1.VSphereDisk{}

	for i := 0; i < numOfDataDisks; i++ {
		disk := infrav1.VSphereDisk{
			Name:    fmt.Sprintf("disk_%d", i),
			SizeGiB: 10 * int32(i),
		}
		if provisionType != nil {
			disk.ProvisioningMode = *provisionType
		}
		disks = append(disks, disk)
	}
	return disks
}

func validateDiskSpec(t *testing.T, device types.BaseVirtualDeviceConfigSpec, cloneDiskSize int32) {
	t.Helper()
	disk := device.GetVirtualDeviceConfigSpec().Device.(*types.VirtualDisk)
	expectedSizeKB := disk.CapacityInKB
	if cloneDiskSize > 0 {
		expectedSizeKB = int64(cloneDiskSize) * 1024 * 1024
	}
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
