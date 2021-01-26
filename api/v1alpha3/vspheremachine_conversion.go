/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha3

import (
	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this VSphereMachine to the Hub version (v1alpha4).
func (src *VSphereMachine) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha4.VSphereMachine)
	return Convert_v1alpha3_VSphereMachine_To_v1alpha4_VSphereMachine(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha4) to this VSphereMachine.
func (dst *VSphereMachine) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha4.VSphereMachine)
	return Convert_v1alpha4_VSphereMachine_To_v1alpha3_VSphereMachine(src, dst, nil)
}

// ConvertTo converts this VSphereMachineList to the Hub version (v1alpha4).
func (src *VSphereMachineList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha4.VSphereMachineList)
	return Convert_v1alpha3_VSphereMachineList_To_v1alpha4_VSphereMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha4) to this VSphereMachineList.
func (dst *VSphereMachineList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha4.VSphereMachineList)
	return Convert_v1alpha4_VSphereMachineList_To_v1alpha3_VSphereMachineList(src, dst, nil)
}
