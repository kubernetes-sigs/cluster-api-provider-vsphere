/*
Copyright 2026 The Kubernetes Authors.

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

package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta2"
)

func (src *VSphereCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*vmwarev1.VSphereCluster)
	if err := Convert_v1beta1_VSphereCluster_To_v1beta2_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*vmwarev1.VSphereCluster)
	if err := Convert_v1beta2_VSphereCluster_To_v1beta1_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*vmwarev1.VSphereClusterTemplate)
	if err := Convert_v1beta1_VSphereClusterTemplate_To_v1beta2_VSphereClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*vmwarev1.VSphereClusterTemplate)
	if err := Convert_v1beta2_VSphereClusterTemplate_To_v1beta1_VSphereClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*vmwarev1.VSphereMachine)
	if err := Convert_v1beta1_VSphereMachine_To_v1beta2_VSphereMachine(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*vmwarev1.VSphereMachine)
	if err := Convert_v1beta2_VSphereMachine_To_v1beta1_VSphereMachine(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*vmwarev1.VSphereMachineTemplate)
	if err := Convert_v1beta1_VSphereMachineTemplate_To_v1beta2_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*vmwarev1.VSphereMachineTemplate)
	if err := Convert_v1beta2_VSphereMachineTemplate_To_v1beta1_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *ProviderServiceAccount) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*vmwarev1.ProviderServiceAccount)
	if err := Convert_v1beta1_ProviderServiceAccount_To_v1beta2_ProviderServiceAccount(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *ProviderServiceAccount) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*vmwarev1.ProviderServiceAccount)
	if err := Convert_v1beta2_ProviderServiceAccount_To_v1beta1_ProviderServiceAccount(src, dst, nil); err != nil {
		return err
	}

	return nil
}
