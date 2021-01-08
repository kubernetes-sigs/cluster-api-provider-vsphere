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

// ConvertTo converts this VSphereMachineTemplate to the Hub version (v1alpha3).
func (src *VSphereMachineTemplate) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.VSphereMachineTemplate)
	if err := Convert_v1alpha2_VSphereMachineTemplate_To_v1alpha3_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1alpha3.VSphereMachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.Template.Spec.VirtualMachineCloneSpec = restored.Spec.Template.Spec.VirtualMachineCloneSpec

	dst.Spec.Template.Spec.Tags = restored.Spec.Template.Spec.Tags

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *VSphereMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.VSphereMachineTemplate)
	if err := Convert_v1alpha3_VSphereMachineTemplate_To_v1alpha2_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this VSphereMachineTemplateList to the Hub version (v1alpha3).
func (src *VSphereMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.VSphereMachineTemplateList)
	return Convert_v1alpha2_VSphereMachineTemplateList_To_v1alpha3_VSphereMachineTemplateList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *VSphereMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.VSphereMachineTemplateList)
	return Convert_v1alpha3_VSphereMachineTemplateList_To_v1alpha2_VSphereMachineTemplateList(src, dst, nil)
}

// Convert_v1alpha2_VSphereMachineTemplateResource_To_v1alpha3_VSphereMachineTemplateResource converts VSphereMachineTemplateResource from v1alpha2 to v1alpha3.
func Convert_v1alpha2_VSphereMachineTemplateResource_To_v1alpha3_VSphereMachineTemplateResource(in *VSphereMachineTemplateResource, out *infrav1alpha3.VSphereMachineTemplateResource, s apiconversion.Scope) error { // nolint
	return autoConvert_v1alpha2_VSphereMachineTemplateResource_To_v1alpha3_VSphereMachineTemplateResource(in, out, s)

}
