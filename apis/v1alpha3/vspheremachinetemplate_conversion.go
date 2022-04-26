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

//nolint:forcetypeassert,golint,revive,stylecheck
package v1alpha3

import (
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	clusterv1a3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1b1 "sigs.k8s.io/cluster-api/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

// ConvertTo.
func (src *VSphereMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta1.VSphereMachineTemplate)
	if err := Convert_v1alpha3_VSphereMachineTemplate_To_v1beta1_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1beta1.VSphereMachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	dst.Spec.Template.Spec.TagIDs = restored.Spec.Template.Spec.TagIDs
	dst.Spec.Template.Spec.AdditionalDisksGiB = restored.Spec.Template.Spec.AdditionalDisksGiB

	return nil
}

func (dst *VSphereMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta1.VSphereMachineTemplate)
	if err := Convert_v1beta1_VSphereMachineTemplate_To_v1alpha3_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

func (src *VSphereMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta1.VSphereMachineTemplateList)
	return Convert_v1alpha3_VSphereMachineTemplateList_To_v1beta1_VSphereMachineTemplateList(src, dst, nil)
}

func (dst *VSphereMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta1.VSphereMachineTemplateList)
	return Convert_v1beta1_VSphereMachineTemplateList_To_v1alpha3_VSphereMachineTemplateList(src, dst, nil)
}

//nolint
func Convert_v1alpha3_ObjectMeta_To_v1beta1_ObjectMeta(in *clusterv1a3.ObjectMeta, out *clusterv1b1.ObjectMeta, s apiconversion.Scope) error {
	// wrapping the conversion func to avoid having compile errors due to compileErrorOnMissingConversion()
	// more details at https://github.com/kubernetes/kubernetes/issues/98380
	return clusterv1a3.Convert_v1alpha3_ObjectMeta_To_v1beta1_ObjectMeta(in, out, s)
}

//nolint
func Convert_v1beta1_ObjectMeta_To_v1alpha3_ObjectMeta(in *clusterv1b1.ObjectMeta, out *clusterv1a3.ObjectMeta, s apiconversion.Scope) error {
	// wrapping the conversion func to avoid having compile errors due to compileErrorOnMissingConversion()
	// more details at https://github.com/kubernetes/kubernetes/issues/98380
	return clusterv1a3.Convert_v1beta1_ObjectMeta_To_v1alpha3_ObjectMeta(in, out, s)
}
