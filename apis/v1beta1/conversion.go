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
