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

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

// CalculateVGPUsToBeAdded calculates the vGPU devices which should be added to the VM.
func CalculateVGPUsToBeAdded(ctx context.Context, vm *object.VirtualMachine, deviceSpecs []infrav1.VGPUSpec) ([]infrav1.VGPUSpec, error) {
	// store the number of expected devices for each deviceID + vendorID combo
	deviceVendorIDComboMap := map[string]int{}
	for _, spec := range deviceSpecs {
		key := spec.ProfileName
		if _, ok := deviceVendorIDComboMap[key]; !ok {
			deviceVendorIDComboMap[key] = 1
		} else {
			deviceVendorIDComboMap[key]++
		}
	}

	devices, err := vm.Device(ctx)
	if err != nil {
		return nil, err
	}

	specsToBeAdded := []infrav1.VGPUSpec{}
	for _, spec := range deviceSpecs {
		key := spec.ProfileName
		pciDeviceList := devices.SelectByBackingInfo(createBackingInfoVGPU(spec))
		expectedDeviceLen := deviceVendorIDComboMap[key]
		if expectedDeviceLen-len(pciDeviceList) > 0 {
			specsToBeAdded = append(specsToBeAdded, spec)
			deviceVendorIDComboMap[key]--
		}
	}
	return specsToBeAdded, nil
}

// ConstructDeviceSpecsVGPU transforms a list of VGPUSpec into a list of BaseVirutalDevices used by govmomi.
func ConstructDeviceSpecsVGPU(vGPUDeviceSpecs []infrav1.VGPUSpec) []types.BaseVirtualDevice {
	vGPUDevices := []types.BaseVirtualDevice{}
	deviceKey := int32(-200)

	for _, pciDevice := range vGPUDeviceSpecs {
		backingInfo := createBackingInfoVGPU(pciDevice)
		vGPUDevices = append(vGPUDevices, &types.VirtualPCIPassthrough{
			VirtualDevice: types.VirtualDevice{
				Key:     deviceKey,
				Backing: backingInfo,
			},
		})
		deviceKey--
	}
	return vGPUDevices
}

func createBackingInfoVGPU(spec infrav1.VGPUSpec) *types.VirtualPCIPassthroughVmiopBackingInfo {
	return &types.VirtualPCIPassthroughVmiopBackingInfo{
		Vgpu: spec.ProfileName,
	}
}
