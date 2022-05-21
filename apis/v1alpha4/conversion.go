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

package v1alpha4

import (
	conversion "k8s.io/apimachinery/pkg/conversion"

	v1beta1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

//nolint:revive,stylecheck
func Convert_v1beta1_VirtualMachineCloneSpec_To_v1alpha4_VirtualMachineCloneSpec(in *v1beta1.VirtualMachineCloneSpec, out *VirtualMachineCloneSpec, s conversion.Scope) error {
	if err := autoConvert_v1beta1_VirtualMachineCloneSpec_To_v1alpha4_VirtualMachineCloneSpec(in, out, s); err != nil {
		return err
	}
	if len(in.Disks) > 0 {
		out.DiskGiB = int32(in.Disks[0].SizeGiB)
	}
	return nil
}
