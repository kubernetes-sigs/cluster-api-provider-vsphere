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

package v1alpha2

import (
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this VSphereMachine to the Hub version (v1alpha3).
func (src *VSphereMachine) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.VSphereMachine)
	if err := Convert_v1alpha2_VSphereMachine_To_v1alpha3_VSphereMachine(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1alpha3.VSphereMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.VirtualMachineCloneSpec = restored.Spec.VirtualMachineCloneSpec
	dst.Status.Conditions = restored.Status.Conditions

	dst.Spec.Tags = restored.Spec.Tags

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *VSphereMachine) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.VSphereMachine)
	if err := Convert_v1alpha3_VSphereMachine_To_v1alpha2_VSphereMachine(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this VSphereMachineList to the Hub version (v1alpha3).
func (src *VSphereMachineList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.VSphereMachineList)
	return Convert_v1alpha2_VSphereMachineList_To_v1alpha3_VSphereMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *VSphereMachineList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.VSphereMachineList)
	return Convert_v1alpha3_VSphereMachineList_To_v1alpha2_VSphereMachineList(src, dst, nil)
}

// Convert_v1alpha2_VSphereMachineSpec_To_v1alpha3_VSphereMachineSpec converts this VSphereMachineSpec to the Hub version (v1alpha3).
func Convert_v1alpha2_VSphereMachineSpec_To_v1alpha3_VSphereMachineSpec(in *VSphereMachineSpec, out *infrav1alpha3.VSphereMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha2_VSphereMachineSpec_To_v1alpha3_VSphereMachineSpec(in, out, s); err != nil {
		return err
	}
	out.Template = in.Template
	out.Datacenter = in.Datacenter
	if err := autoConvert_v1alpha2_NetworkSpec_To_v1alpha3_NetworkSpec(&in.Network, &out.Network, s); err != nil {
		return err
	}
	out.NumCPUs = in.NumCPUs
	out.NumCoresPerSocket = in.NumCoresPerSocket
	out.MemoryMiB = in.MemoryMiB
	out.DiskGiB = in.DiskGiB
	return nil
}

// Convert_v1alpha3_VSphereMachineSpec_To_v1alpha2_VSphereMachineSpec converts from the Hub version (v1alpha3) of the VSphereMachineSpec to this version.
func Convert_v1alpha3_VSphereMachineSpec_To_v1alpha2_VSphereMachineSpec(in *infrav1alpha3.VSphereMachineSpec, out *VSphereMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_VSphereMachineSpec_To_v1alpha2_VSphereMachineSpec(in, out, s); err != nil {
		return err
	}
	out.Template = in.Template
	out.Datacenter = in.Datacenter
	if err := autoConvert_v1alpha3_NetworkSpec_To_v1alpha2_NetworkSpec(&in.Network, &out.Network, s); err != nil {
		return err
	}
	out.NumCPUs = in.NumCPUs
	out.NumCoresPerSocket = in.NumCoresPerSocket
	out.MemoryMiB = in.MemoryMiB
	out.DiskGiB = in.DiskGiB
	return nil
}

// Convert_v1alpha2_VSphereMachineStatus_To_v1alpha3_VSphereMachineStatus converts this VSphereMachineStatus to the Hub version (v1alpha3).
func Convert_v1alpha2_VSphereMachineStatus_To_v1alpha3_VSphereMachineStatus(in *VSphereMachineStatus, out *infrav1alpha3.VSphereMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha2_VSphereMachineStatus_To_v1alpha3_VSphereMachineStatus(in, out, s); err != nil {
		return err
	}

	// Manually convert the Error fields to the Failure fields
	out.FailureMessage = in.ErrorMessage
	out.FailureReason = in.ErrorReason

	return nil
}

// Convert_v1alpha3_VSphereMachineStatus_To_v1alpha2_VSphereMachineStatus converts from the Hub version (v1alpha3) of the VSphereMachineStatus to this version.
func Convert_v1alpha3_VSphereMachineStatus_To_v1alpha2_VSphereMachineStatus(in *infrav1alpha3.VSphereMachineStatus, out *VSphereMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_VSphereMachineStatus_To_v1alpha2_VSphereMachineStatus(in, out, s); err != nil {
		return err
	}

	// Manually convert the Failure fields to the Error fields
	out.ErrorMessage = in.FailureMessage
	out.ErrorReason = in.FailureReason

	return nil
}
