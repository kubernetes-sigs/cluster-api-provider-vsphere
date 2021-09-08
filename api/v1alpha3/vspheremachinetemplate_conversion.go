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
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	clusterv1a3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1a4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo
func (src *VSphereMachineTemplate) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha4.VSphereMachineTemplate)
	return Convert_v1alpha3_VSphereMachineTemplate_To_v1alpha4_VSphereMachineTemplate(src, dst, nil)
}

func (dst *VSphereMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha4.VSphereMachineTemplate)
	return Convert_v1alpha4_VSphereMachineTemplate_To_v1alpha3_VSphereMachineTemplate(src, dst, nil)
}

func (src *VSphereMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha4.VSphereMachineTemplateList)
	return Convert_v1alpha3_VSphereMachineTemplateList_To_v1alpha4_VSphereMachineTemplateList(src, dst, nil)
}

func (dst *VSphereMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha4.VSphereMachineTemplateList)
	return Convert_v1alpha4_VSphereMachineTemplateList_To_v1alpha3_VSphereMachineTemplateList(src, dst, nil)
}

//nolint
func Convert_v1alpha3_VSphereMachineTemplateResource_To_v1alpha4_VSphereMachineTemplateResource(in *VSphereMachineTemplateResource, out *infrav1alpha4.VSphereMachineTemplateResource, s apiconversion.Scope) error {
	if err := clusterv1a3.Convert_v1alpha3_ObjectMeta_To_v1alpha4_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1alpha3_VSphereMachineSpec_To_v1alpha4_VSphereMachineSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

//nolint
func Convert_v1alpha4_VSphereMachineTemplateResource_To_v1alpha3_VSphereMachineTemplateResource(in *infrav1alpha4.VSphereMachineTemplateResource, out *VSphereMachineTemplateResource, s apiconversion.Scope) error {
	if err := clusterv1a3.Convert_v1alpha4_ObjectMeta_To_v1alpha3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1alpha4_VSphereMachineSpec_To_v1alpha3_VSphereMachineSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

//nolint
func Convert_v1alpha3_ObjectMeta_To_v1alpha4_ObjectMeta(in *clusterv1a3.ObjectMeta, out *clusterv1a4.ObjectMeta, s apiconversion.Scope) error {
	// wrapping the conversion func to avoid having compile errors due to compileErrorOnMissingConversion()
	// more details at https://github.com/kubernetes/kubernetes/issues/98380
	return clusterv1a3.Convert_v1alpha3_ObjectMeta_To_v1alpha4_ObjectMeta(in, out, s)
}

//nolint
func Convert_v1alpha4_ObjectMeta_To_v1alpha3_ObjectMeta(in *clusterv1a4.ObjectMeta, out *clusterv1a3.ObjectMeta, s apiconversion.Scope) error {
	// wrapping the conversion func to avoid having compile errors due to compileErrorOnMissingConversion()
	// more details at https://github.com/kubernetes/kubernetes/issues/98380
	return clusterv1a3.Convert_v1alpha4_ObjectMeta_To_v1alpha3_ObjectMeta(in, out, s)
}
