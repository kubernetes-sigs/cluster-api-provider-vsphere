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
	"maps"
	"reflect"
	"slices"
	"sort"

	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta2"
)

func (src *VSphereCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*vmwarev1.VSphereCluster)
	if err := Convert_v1beta1_VSphereCluster_To_v1beta2_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	restored := &vmwarev1.VSphereCluster{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	initialization := vmwarev1.VSphereClusterInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &initialization.Provisioned)
	if !reflect.DeepEqual(initialization, vmwarev1.VSphereClusterInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}
	return nil
}

func (dst *VSphereCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*vmwarev1.VSphereCluster)
	if err := Convert_v1beta2_VSphereCluster_To_v1beta1_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalData(src, dst)
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

	restored := &vmwarev1.VSphereMachine{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	initialization := vmwarev1.VSphereMachineInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &initialization.Provisioned)
	if !reflect.DeepEqual(initialization, vmwarev1.VSphereMachineInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}
	return nil
}

func (dst *VSphereMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*vmwarev1.VSphereMachine)
	if err := Convert_v1beta2_VSphereMachine_To_v1beta1_VSphereMachine(src, dst, nil); err != nil {
		return err
	}

	if dst.Spec.ProviderID != nil && *dst.Spec.ProviderID == "" {
		dst.Spec.ProviderID = nil
	}

	return utilconversion.MarshalData(src, dst)
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

	if dst.Spec.Template.Spec.ProviderID != nil && *dst.Spec.Template.Spec.ProviderID == "" {
		dst.Spec.Template.Spec.ProviderID = nil
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

func Convert_v1beta2_VSphereClusterStatus_To_v1beta1_VSphereClusterStatus(in *vmwarev1.VSphereClusterStatus, out *VSphereClusterStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_VSphereClusterStatus_To_v1beta1_VSphereClusterStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions
	// NOTE: v1beta2 conditions should not automatically be converted into legacy conditions (v1beta1).
	out.Conditions = nil

	// Retrieve legacy conditions (v1beta1) from the deprecated field.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		if in.Deprecated.V1Beta1.Conditions != nil {
			clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
		}
	}

	// Move initialization to old field
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)

	// Move FailureDomains
	if in.FailureDomains != nil {
		out.FailureDomains = clusterv1beta1.FailureDomains{}
		for _, fd := range in.FailureDomains {
			out.FailureDomains[fd.Name] = clusterv1beta1.FailureDomainSpec{
				ControlPlane: ptr.Deref(fd.ControlPlane, false),
				Attributes:   fd.Attributes,
			}
		}
	}

	// Move new conditions (v1beta2) to the v1beta2 field.
	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &VSphereClusterV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
	return nil
}

func Convert_v1beta1_VSphereClusterStatus_To_v1beta2_VSphereClusterStatus(in *VSphereClusterStatus, out *vmwarev1.VSphereClusterStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_VSphereClusterStatus_To_v1beta2_VSphereClusterStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions
	// NOTE: v1beta1 conditions should not be automatically be converted into v1beta2 conditions.
	out.Conditions = nil

	// Retrieve new conditions (v1beta2) from the v1beta2 field.
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}

	// Move FailureDomains
	if in.FailureDomains != nil {
		out.FailureDomains = []clusterv1.FailureDomain{}
		domainNames := slices.Collect(maps.Keys(in.FailureDomains))
		sort.Strings(domainNames)
		for _, name := range domainNames {
			fd := in.FailureDomains[name]
			out.FailureDomains = append(out.FailureDomains, clusterv1.FailureDomain{
				Name:         name,
				ControlPlane: ptr.To(fd.ControlPlane),
				Attributes:   fd.Attributes,
			})
		}
	}

	// Move legacy conditions (v1beta1) to the deprecated field.
	if in.Conditions == nil {
		return nil
	}

	if out.Deprecated == nil {
		out.Deprecated = &vmwarev1.VSphereClusterDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &vmwarev1.VSphereClusterV1Beta1DeprecatedStatus{}
	}
	if in.Conditions != nil {
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	return nil
}

func Convert_v1beta2_VSphereMachineStatus_To_v1beta1_VSphereMachineStatus(in *vmwarev1.VSphereMachineStatus, out *VSphereMachineStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_VSphereMachineStatus_To_v1beta1_VSphereMachineStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions
	// NOTE: v1beta2 conditions should not automatically be converted into legacy conditions (v1beta1).
	out.Conditions = nil

	// Retrieve legacy conditions (v1beta1) from the deprecated field.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		if in.Deprecated.V1Beta1.Conditions != nil {
			clusterv1beta1.Convert_v1beta2_Deprecated_V1Beta1_Conditions_To_v1beta1_Conditions(&in.Deprecated.V1Beta1.Conditions, &out.Conditions)
		}
	}

	// Move initialization to old field
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)

	// Move new conditions (v1beta2) to the v1beta2 field.
	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &VSphereMachineV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
	return nil
}

func Convert_v1beta1_VSphereMachineStatus_To_v1beta2_VSphereMachineStatus(in *VSphereMachineStatus, out *vmwarev1.VSphereMachineStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_VSphereMachineStatus_To_v1beta2_VSphereMachineStatus(in, out, s); err != nil {
		return err
	}

	// Reset conditions from autogenerated conversions
	// NOTE: v1beta1 conditions should not be automatically be converted into v1beta2 conditions.
	out.Conditions = nil

	// Retrieve new conditions (v1beta2) from the v1beta2 field.
	if in.V1Beta2 != nil {
		out.Conditions = in.V1Beta2.Conditions
	}

	// Move legacy conditions (v1beta1) to the deprecated field.
	if in.Conditions == nil {
		return nil
	}

	if out.Deprecated == nil {
		out.Deprecated = &vmwarev1.VSphereMachineDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &vmwarev1.VSphereMachineV1Beta1DeprecatedStatus{}
	}
	if in.Conditions != nil {
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	return nil
}

func Convert_v1beta2_APIEndpoint_To_v1beta1_APIEndpoint(in *vmwarev1.APIEndpoint, out *clusterv1beta1.APIEndpoint, _ apimachineryconversion.Scope) error {
	out.Host = in.Host
	out.Port = in.Port
	return nil
}

func Convert_v1beta1_APIEndpoint_To_v1beta2_APIEndpoint(in *clusterv1beta1.APIEndpoint, out *vmwarev1.APIEndpoint, _ apimachineryconversion.Scope) error {
	out.Host = in.Host
	out.Port = in.Port
	return nil
}
