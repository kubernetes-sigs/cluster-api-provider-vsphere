/*
Copyright 2025 The Kubernetes Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta2"
)

func (src *VSphereCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereCluster)
	if err := Convert_v1beta1_VSphereCluster_To_v1beta2_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereCluster)
	if err := Convert_v1beta2_VSphereCluster_To_v1beta1_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereClusterTemplate)
	if err := Convert_v1beta1_VSphereClusterTemplate_To_v1beta2_VSphereClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereClusterTemplate)
	if err := Convert_v1beta2_VSphereClusterTemplate_To_v1beta1_VSphereClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereClusterIdentity) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereClusterIdentity)
	if err := Convert_v1beta1_VSphereClusterIdentity_To_v1beta2_VSphereClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereClusterIdentity) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereClusterIdentity)
	if err := Convert_v1beta2_VSphereClusterIdentity_To_v1beta1_VSphereClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereDeploymentZone) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereDeploymentZone)
	if err := Convert_v1beta1_VSphereDeploymentZone_To_v1beta2_VSphereDeploymentZone(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereDeploymentZone) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereDeploymentZone)
	if err := Convert_v1beta2_VSphereDeploymentZone_To_v1beta1_VSphereDeploymentZone(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereFailureDomain) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereFailureDomain)
	if err := Convert_v1beta1_VSphereFailureDomain_To_v1beta2_VSphereFailureDomain(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereFailureDomain) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereFailureDomain)
	if err := Convert_v1beta2_VSphereFailureDomain_To_v1beta1_VSphereFailureDomain(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereMachine)
	if err := Convert_v1beta1_VSphereMachine_To_v1beta2_VSphereMachine(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereMachine)
	if err := Convert_v1beta2_VSphereMachine_To_v1beta1_VSphereMachine(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereMachineTemplate)
	if err := Convert_v1beta1_VSphereMachineTemplate_To_v1beta2_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereMachineTemplate)
	if err := Convert_v1beta2_VSphereMachineTemplate_To_v1beta1_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (src *VSphereVM) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereVM)
	if err := Convert_v1beta1_VSphereVM_To_v1beta2_VSphereVM(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func (dst *VSphereVM) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereVM)
	if err := Convert_v1beta2_VSphereVM_To_v1beta1_VSphereVM(src, dst, nil); err != nil {
		return err
	}

	return nil
}

func Convert_v1beta2_VSphereClusterStatus_To_v1beta1_VSphereClusterStatus(in *infrav1.VSphereClusterStatus, out *VSphereClusterStatus, s apimachineryconversion.Scope) error {
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

	// Move new conditions (v1beta2) to the v1beta2 field.
	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &VSphereClusterV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
	return nil
}

func Convert_v1beta1_VSphereClusterStatus_To_v1beta2_VSphereClusterStatus(in *VSphereClusterStatus, out *infrav1.VSphereClusterStatus, s apimachineryconversion.Scope) error {
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

	// Move legacy conditions (v1beta1) to the deprecated field.
	if in.Conditions == nil {
		return nil
	}

	if out.Deprecated == nil {
		out.Deprecated = &infrav1.VSphereClusterDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1.VSphereClusterV1Beta1DeprecatedStatus{}
	}
	if in.Conditions != nil {
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	return nil
}

func Convert_v1beta2_VSphereClusterIdentityStatus_To_v1beta1_VSphereClusterIdentityStatus(in *infrav1.VSphereClusterIdentityStatus, out *VSphereClusterIdentityStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_VSphereClusterIdentityStatus_To_v1beta1_VSphereClusterIdentityStatus(in, out, s); err != nil {
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

	// Move new conditions (v1beta2) to the v1beta2 field.
	if in.Conditions == nil {
		return nil
	}
	out.V1Beta2 = &VSphereClusterIdentityV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
	return nil
}

func Convert_v1beta1_VSphereClusterIdentityStatus_To_v1beta2_VSphereClusterIdentityStatus(in *VSphereClusterIdentityStatus, out *infrav1.VSphereClusterIdentityStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_VSphereClusterIdentityStatus_To_v1beta2_VSphereClusterIdentityStatus(in, out, s); err != nil {
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
		out.Deprecated = &infrav1.VSphereClusterIdentityDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1.VSphereClusterIdentityV1Beta1DeprecatedStatus{}
	}
	if in.Conditions != nil {
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	return nil
}

func Convert_v1_Condition_To_v1beta1_Condition(_ *metav1.Condition, _ *clusterv1beta1.Condition, _ apimachineryconversion.Scope) error {
	// NOTE: v1beta2 conditions should not be automatically converted into legacy (v1beta1) conditions.
	return nil
}

func Convert_v1beta1_Condition_To_v1_Condition(_ *clusterv1beta1.Condition, _ *metav1.Condition, _ apimachineryconversion.Scope) error {
	// NOTE: legacy (v1beta1) conditions should not be automatically converted into v1beta2 conditions.
	return nil
}
