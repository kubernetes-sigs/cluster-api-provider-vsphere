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
	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this VSphereCluster to the Hub version (v1alpha3).
func (src *VSphereCluster) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.VSphereCluster)

	if err := Convert_v1alpha2_VSphereCluster_To_v1alpha3_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	// Manually convert Status.APIEndpoints to Spec.ControlPlaneEndpoint.
	if len(src.Status.APIEndpoints) > 0 {
		endpoint := src.Status.APIEndpoints[0]
		dst.Spec.ControlPlaneEndpoint.Host = endpoint.Host
		dst.Spec.ControlPlaneEndpoint.Port = int32(endpoint.Port)
	}
	// Manually restore data.
	restored := &infrav1alpha3.VSphereCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	// prefer a current CPI configuration, but otherwise restore extraArgs
	if dst.Spec.CloudProviderConfiguration.ProviderConfig.Cloud != nil {
		dst.Spec.CloudProviderConfiguration.ProviderConfig.Cloud.ExtraArgs = restored.Spec.CloudProviderConfiguration.ProviderConfig.Cloud.ExtraArgs
	}

	if restored.Spec.LoadBalancerRef != nil {
		dst.Spec.LoadBalancerRef = restored.Spec.LoadBalancerRef
	}
	if restored.Spec.ControlPlaneEndpoint.Host != "" {
		dst.Spec.ControlPlaneEndpoint.Host = restored.Spec.ControlPlaneEndpoint.Host
	}
	if restored.Spec.ControlPlaneEndpoint.Port != 0 {
		dst.Spec.ControlPlaneEndpoint.Port = restored.Spec.ControlPlaneEndpoint.Port
	}
	if restored.Spec.Thumbprint != "" {
		dst.Spec.Thumbprint = restored.Spec.Thumbprint
	}

	dst.Status.Conditions = restored.Status.Conditions

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *VSphereCluster) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.VSphereCluster)

	if err := Convert_v1alpha3_VSphereCluster_To_v1alpha2_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	// Manually convert Spec.ControlPlaneEndpoint to Status.APIEndpoints.
	if !src.Spec.ControlPlaneEndpoint.IsZero() {
		dst.Status.APIEndpoints = []APIEndpoint{
			{
				Host: src.Spec.ControlPlaneEndpoint.Host,
				Port: int(src.Spec.ControlPlaneEndpoint.Port),
			},
		}
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this VSphereClusterList to the Hub version (v1alpha3).
func (src *VSphereClusterList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.VSphereClusterList)
	return Convert_v1alpha2_VSphereClusterList_To_v1alpha3_VSphereClusterList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *VSphereClusterList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.VSphereClusterList)
	return Convert_v1alpha3_VSphereClusterList_To_v1alpha2_VSphereClusterList(src, dst, nil)
}

// Convert_v1alpha3_VSphereClusterSpec_To_v1alpha2_VSphereClusterSpec converts from the Hub version (v1alpha3) of the VSphereClusterSpec to this version.
// Requires manual conversion as infrav1alpha3.VSphereClusterSpec.LoadBalancerRef does not exist in VSphereClusterSpec.
func Convert_v1alpha3_VSphereClusterSpec_To_v1alpha2_VSphereClusterSpec(in *infrav1alpha3.VSphereClusterSpec, out *VSphereClusterSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_VSphereClusterSpec_To_v1alpha2_VSphereClusterSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha2_VSphereClusterStatus_To_v1alpha3_VSphereClusterStatus converts VSphereCluster.Status from v1alpha2 to v1alpha3.
func Convert_v1alpha2_VSphereClusterStatus_To_v1alpha3_VSphereClusterStatus(in *VSphereClusterStatus, out *infrav1alpha3.VSphereClusterStatus, s apiconversion.Scope) error { // nolint
	return autoConvert_v1alpha2_VSphereClusterStatus_To_v1alpha3_VSphereClusterStatus(in, out, s)
}

// Convert_v1alpha2_CPICloudConfig_To_v1alpha3_CPICloudConfig converts VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Cloud from v1alpha2 to v1alpha3.
func Convert_v1alpha2_CPICloudConfig_To_v1alpha3_CPICloudConfig(in *CPICloudConfig, out *v1alpha3.CPICloudConfig, s apiconversion.Scope) error { // nolint
	// extraArgs is handled through the annotation marshalling
	out.ControllerImage = in.ControllerImage
	return nil
}

// Convert_v1alpha3_CPICloudConfig_To_v1alpha2_CPICloudConfig converts VSphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Cloud from v1alpha3 to v1alpha2.
func Convert_v1alpha3_CPICloudConfig_To_v1alpha2_CPICloudConfig(in *v1alpha3.CPICloudConfig, out *CPICloudConfig, s apiconversion.Scope) error { // nolint
	// extraArgs is handled through the annotation marshalling
	out.ControllerImage = in.ControllerImage
	return nil
}

// Convert_v1alpha3_VSphereClusterStatus_To_v1alpha2_VSphereClusterStatus converts VSphereCluster.Status from v1alpha3 to v1alpha2.
// Requires manual conversion as infrav1alpha3.VSphereClusterStatus.Conditions does not exist in VSphereClusterSpec.
func Convert_v1alpha3_VSphereClusterStatus_To_v1alpha2_VSphereClusterStatus(in *v1alpha3.VSphereClusterStatus, out *VSphereClusterStatus, s apiconversion.Scope) error { // nolint
	// Conditions is handled through the annotation marshalling
	out.Ready = in.Ready
	return nil
}
