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

package v1alpha3

import (
	"k8s.io/apimachinery/pkg/conversion"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

// Convert_v1beta1_VirtualMachineCloneSpec_To_v1alpha3_VirtualMachineCloneSpec is an autogenerated conversion function.
func Convert_v1beta1_VirtualMachineCloneSpec_To_v1alpha3_VirtualMachineCloneSpec(in *infrav1.VirtualMachineCloneSpec, out *VirtualMachineCloneSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_VirtualMachineCloneSpec_To_v1alpha3_VirtualMachineCloneSpec(in, out, s)
}

// Convert_v1beta1_VSphereVMStatus_To_v1alpha3_VSphereVMStatus is an autogenerated conversion function.
func Convert_v1beta1_VSphereVMStatus_To_v1alpha3_VSphereVMStatus(in *infrav1.VSphereVMStatus, out *VSphereVMStatus, s conversion.Scope) error {
	// V1Beta2 was added in v1beta1
	return autoConvert_v1beta1_VSphereVMStatus_To_v1alpha3_VSphereVMStatus(in, out, s)
}

func Convert_v1beta1_VSphereClusterStatus_To_v1alpha3_VSphereClusterStatus(in *infrav1.VSphereClusterStatus, out *VSphereClusterStatus, s conversion.Scope) error {
	// V1Beta2 was added in v1beta1
	return autoConvert_v1beta1_VSphereClusterStatus_To_v1alpha3_VSphereClusterStatus(in, out, s)
}

func Convert_v1beta1_VSphereClusterSpec_To_v1alpha3_VSphereClusterSpec(in *infrav1.VSphereClusterSpec, out *VSphereClusterSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_VSphereClusterSpec_To_v1alpha3_VSphereClusterSpec(in, out, s)
}

func Convert_v1beta1_VSphereMachineSpec_To_v1alpha3_VSphereMachineSpec(in *infrav1.VSphereMachineSpec, out *VSphereMachineSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_VSphereMachineSpec_To_v1alpha3_VSphereMachineSpec(in, out, s)
}

func Convert_v1beta1_VSphereVMSpec_To_v1alpha3_VSphereVMSpec(in *infrav1.VSphereVMSpec, out *VSphereVMSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_VSphereVMSpec_To_v1alpha3_VSphereVMSpec(in, out, s)
}

func Convert_v1beta1_VSphereMachineStatus_To_v1alpha3_VSphereMachineStatus(in *infrav1.VSphereMachineStatus, out *VSphereMachineStatus, s conversion.Scope) error {
	// V1Beta2 was added in v1beta1
	return autoConvert_v1beta1_VSphereMachineStatus_To_v1alpha3_VSphereMachineStatus(in, out, s)
}
