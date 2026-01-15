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
	"maps"
	"reflect"
	"slices"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta2"
)

func (src *VSphereCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereCluster)
	if err := Convert_v1beta1_VSphereCluster_To_v1beta2_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	restored := &infrav1.VSphereCluster{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	clusterv1.Convert_bool_To_Pointer_bool(src.Spec.DisableClusterModule, ok, restored.Spec.DisableClusterModule, &dst.Spec.DisableClusterModule)

	if len(src.Spec.ClusterModules) == len(dst.Spec.ClusterModules) {
		for i, dstClusterModule := range dst.Spec.ClusterModules {
			srcClusterModule := src.Spec.ClusterModules[i]
			var restoredControlPlane *bool
			if len(dst.Spec.ClusterModules) == len(restored.Spec.ClusterModules) {
				restoredClusterModules := restored.Spec.ClusterModules[i]
				restoredControlPlane = restoredClusterModules.ControlPlane
			}

			clusterv1.Convert_bool_To_Pointer_bool(srcClusterModule.ControlPlane, ok, restoredControlPlane, &dstClusterModule.ControlPlane)

			dst.Spec.ClusterModules[i] = dstClusterModule
		}
	}

	initialization := infrav1.VSphereClusterInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &initialization.Provisioned)
	if !reflect.DeepEqual(initialization, infrav1.VSphereClusterInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}
	return nil
}

func (dst *VSphereCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereCluster)
	if err := Convert_v1beta2_VSphereCluster_To_v1beta1_VSphereCluster(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalData(src, dst)
}

func (src *VSphereClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereClusterTemplate)
	if err := Convert_v1beta1_VSphereClusterTemplate_To_v1beta2_VSphereClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	restored := &infrav1.VSphereClusterTemplate{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	clusterv1.Convert_bool_To_Pointer_bool(src.Spec.Template.Spec.DisableClusterModule, ok, restored.Spec.Template.Spec.DisableClusterModule, &dst.Spec.Template.Spec.DisableClusterModule)

	if len(src.Spec.Template.Spec.ClusterModules) == len(dst.Spec.Template.Spec.ClusterModules) {
		for i, dstClusterModule := range dst.Spec.Template.Spec.ClusterModules {
			srcClusterModule := src.Spec.Template.Spec.ClusterModules[i]
			var restoredControlPlane *bool
			if len(dst.Spec.Template.Spec.ClusterModules) == len(restored.Spec.Template.Spec.ClusterModules) {
				restoredClusterModules := restored.Spec.Template.Spec.ClusterModules[i]
				restoredControlPlane = restoredClusterModules.ControlPlane
			}

			clusterv1.Convert_bool_To_Pointer_bool(srcClusterModule.ControlPlane, ok, restoredControlPlane, &dstClusterModule.ControlPlane)

			dst.Spec.Template.Spec.ClusterModules[i] = dstClusterModule
		}
	}

	return nil
}

func (dst *VSphereClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereClusterTemplate)
	if err := Convert_v1beta2_VSphereClusterTemplate_To_v1beta1_VSphereClusterTemplate(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalData(src, dst)
}

func (src *VSphereClusterIdentity) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereClusterIdentity)
	if err := Convert_v1beta1_VSphereClusterIdentity_To_v1beta2_VSphereClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	restored := &infrav1.VSphereClusterIdentity{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Ready, &dst.Status.Ready)
	return nil
}

func (dst *VSphereClusterIdentity) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereClusterIdentity)
	if err := Convert_v1beta2_VSphereClusterIdentity_To_v1beta1_VSphereClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalData(src, dst)
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

	if dst.Spec.Topology.ComputeCluster != nil && *dst.Spec.Topology.ComputeCluster == "" {
		dst.Spec.Topology.ComputeCluster = nil
	}
	return nil
}

func (src *VSphereMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereMachine)
	if err := Convert_v1beta1_VSphereMachine_To_v1beta2_VSphereMachine(src, dst, nil); err != nil {
		return err
	}

	restored := &infrav1.VSphereMachine{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	if len(src.Spec.Network.Routes) == len(dst.Spec.Network.Routes) {
		for i, dstRoute := range dst.Spec.Network.Routes {
			srcRoute := src.Spec.Network.Routes[i]
			var restoredMetric *int32
			if len(dst.Spec.Network.Routes) == len(restored.Spec.Network.Routes) {
				restoredMetric = restored.Spec.Network.Routes[i].Metric
			}
			clusterv1.Convert_int32_To_Pointer_int32(srcRoute.Metric, ok, restoredMetric, &dstRoute.Metric)
			dst.Spec.Network.Routes[i] = dstRoute
		}
	}

	if len(src.Spec.Network.Devices) == len(dst.Spec.Network.Devices) {
		for i, dstDevice := range dst.Spec.Network.Devices {
			srcDevice := src.Spec.Network.Devices[i]
			var restoredDeviceDHCP4, restoredDeviceDHCP6, restoredSkipIPAllocation *bool
			if len(dst.Spec.Network.Devices) == len(restored.Spec.Network.Devices) {
				restoredDevice := restored.Spec.Network.Devices[i]
				restoredDeviceDHCP4 = restoredDevice.DHCP4
				restoredDeviceDHCP6 = restoredDevice.DHCP6
				restoredSkipIPAllocation = restoredDevice.SkipIPAllocation

				if len(srcDevice.Routes) == len(dstDevice.Routes) {
					for i, dstRoute := range dstDevice.Routes {
						srcRoute := srcDevice.Routes[i]
						var restoredMetric *int32
						if len(dstDevice.Routes) == len(restoredDevice.Routes) {
							restoredMetric = restoredDevice.Routes[i].Metric
						}
						clusterv1.Convert_int32_To_Pointer_int32(srcRoute.Metric, ok, restoredMetric, &dstRoute.Metric)
						dstDevice.Routes[i] = dstRoute
					}
				}
			}

			clusterv1.Convert_bool_To_Pointer_bool(srcDevice.DHCP4, ok, restoredDeviceDHCP4, &dstDevice.DHCP4)
			clusterv1.Convert_bool_To_Pointer_bool(srcDevice.DHCP6, ok, restoredDeviceDHCP6, &dstDevice.DHCP6)
			clusterv1.Convert_bool_To_Pointer_bool(srcDevice.SkipIPAllocation, ok, restoredSkipIPAllocation, &dstDevice.SkipIPAllocation)

			dst.Spec.Network.Devices[i] = dstDevice
		}
	}

	initialization := infrav1.VSphereMachineInitializationStatus{}
	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Initialization.Provisioned, &initialization.Provisioned)
	if !reflect.DeepEqual(initialization, infrav1.VSphereMachineInitializationStatus{}) {
		dst.Status.Initialization = initialization
	}

	if len(src.Status.Network) == len(dst.Status.Network) {
		for i, dstNetwork := range dst.Status.Network {
			srcNetwork := src.Status.Network[i]
			var restoredConnected *bool
			if len(dst.Status.Network) == len(restored.Status.Network) {
				restoredNetwork := restored.Status.Network[i]
				restoredConnected = restoredNetwork.Connected
			}

			clusterv1.Convert_bool_To_Pointer_bool(srcNetwork.Connected, ok, restoredConnected, &dstNetwork.Connected)

			dst.Status.Network[i] = dstNetwork
		}
	}
	return nil
}

func (dst *VSphereMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereMachine)
	if err := Convert_v1beta2_VSphereMachine_To_v1beta1_VSphereMachine(src, dst, nil); err != nil {
		return err
	}

	if dst.Spec.ProviderID != nil && *dst.Spec.ProviderID == "" {
		dst.Spec.ProviderID = nil
	}

	if dst.Spec.FailureDomain != nil && *dst.Spec.FailureDomain == "" {
		dst.Spec.FailureDomain = nil
	}

	if dst.Spec.NamingStrategy != nil && dst.Spec.NamingStrategy.Template != nil && *dst.Spec.NamingStrategy.Template == "" {
		dst.Spec.NamingStrategy.Template = nil
	}

	return utilconversion.MarshalData(src, dst)
}

func (src *VSphereMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereMachineTemplate)
	if err := Convert_v1beta1_VSphereMachineTemplate_To_v1beta2_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	restored := &infrav1.VSphereMachineTemplate{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	if len(src.Spec.Template.Spec.Network.Routes) == len(dst.Spec.Template.Spec.Network.Routes) {
		for i, dstRoute := range dst.Spec.Template.Spec.Network.Routes {
			srcRoute := src.Spec.Template.Spec.Network.Routes[i]
			var restoredMetric *int32
			if len(dst.Spec.Template.Spec.Network.Routes) == len(restored.Spec.Template.Spec.Network.Routes) {
				restoredMetric = restored.Spec.Template.Spec.Network.Routes[i].Metric
			}
			clusterv1.Convert_int32_To_Pointer_int32(srcRoute.Metric, ok, restoredMetric, &dstRoute.Metric)
			dst.Spec.Template.Spec.Network.Routes[i] = dstRoute
		}
	}

	if len(src.Spec.Template.Spec.Network.Devices) == len(dst.Spec.Template.Spec.Network.Devices) {
		for i, dstDevice := range dst.Spec.Template.Spec.Network.Devices {
			srcDevice := src.Spec.Template.Spec.Network.Devices[i]
			var restoredDeviceDHCP4, restoredDeviceDHCP6, restoredSkipIPAllocation *bool
			if len(dst.Spec.Template.Spec.Network.Devices) == len(restored.Spec.Template.Spec.Network.Devices) {
				restoredDevice := restored.Spec.Template.Spec.Network.Devices[i]
				restoredDeviceDHCP4 = restoredDevice.DHCP4
				restoredDeviceDHCP6 = restoredDevice.DHCP6
				restoredSkipIPAllocation = restoredDevice.SkipIPAllocation

				if len(srcDevice.Routes) == len(dstDevice.Routes) {
					for i, dstRoute := range dstDevice.Routes {
						srcRoute := srcDevice.Routes[i]
						var restoredMetric *int32
						if len(dstDevice.Routes) == len(restoredDevice.Routes) {
							restoredMetric = restoredDevice.Routes[i].Metric
						}
						clusterv1.Convert_int32_To_Pointer_int32(srcRoute.Metric, ok, restoredMetric, &dstRoute.Metric)
						dstDevice.Routes[i] = dstRoute
					}
				}
			}

			clusterv1.Convert_bool_To_Pointer_bool(srcDevice.DHCP4, ok, restoredDeviceDHCP4, &dstDevice.DHCP4)
			clusterv1.Convert_bool_To_Pointer_bool(srcDevice.DHCP6, ok, restoredDeviceDHCP6, &dstDevice.DHCP6)
			clusterv1.Convert_bool_To_Pointer_bool(srcDevice.SkipIPAllocation, ok, restoredSkipIPAllocation, &dstDevice.SkipIPAllocation)

			dst.Spec.Template.Spec.Network.Devices[i] = dstDevice
		}
	}

	return nil
}

func (dst *VSphereMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereMachineTemplate)
	if err := Convert_v1beta2_VSphereMachineTemplate_To_v1beta1_VSphereMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	if dst.Spec.Template.Spec.ProviderID != nil && *dst.Spec.Template.Spec.ProviderID == "" {
		dst.Spec.Template.Spec.ProviderID = nil
	}

	if dst.Spec.Template.Spec.FailureDomain != nil && *dst.Spec.Template.Spec.FailureDomain == "" {
		dst.Spec.Template.Spec.FailureDomain = nil
	}

	if dst.Spec.Template.Spec.NamingStrategy != nil && dst.Spec.Template.Spec.NamingStrategy.Template != nil && *dst.Spec.Template.Spec.NamingStrategy.Template == "" {
		dst.Spec.Template.Spec.NamingStrategy.Template = nil
	}

	return utilconversion.MarshalData(src, dst)
}

func (src *VSphereVM) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.VSphereVM)
	if err := Convert_v1beta1_VSphereVM_To_v1beta2_VSphereVM(src, dst, nil); err != nil {
		return err
	}

	restored := &infrav1.VSphereVM{}
	ok, err := utilconversion.UnmarshalData(src, restored)
	if err != nil {
		return err
	}

	if len(src.Spec.Network.Routes) == len(dst.Spec.Network.Routes) {
		for i, dstRoute := range dst.Spec.Network.Routes {
			srcRoute := src.Spec.Network.Routes[i]
			var restoredMetric *int32
			if len(dst.Spec.Network.Routes) == len(restored.Spec.Network.Routes) {
				restoredMetric = restored.Spec.Network.Routes[i].Metric
			}
			clusterv1.Convert_int32_To_Pointer_int32(srcRoute.Metric, ok, restoredMetric, &dstRoute.Metric)
			dst.Spec.Network.Routes[i] = dstRoute
		}
	}

	if len(src.Spec.Network.Devices) == len(dst.Spec.Network.Devices) {
		for i, dstDevice := range dst.Spec.Network.Devices {
			srcDevice := src.Spec.Network.Devices[i]
			var restoredDeviceDHCP4, restoredDeviceDHCP6, restoredSkipIPAllocation *bool
			if len(dst.Spec.Network.Devices) == len(restored.Spec.Network.Devices) {
				restoredDevice := restored.Spec.Network.Devices[i]
				restoredDeviceDHCP4 = restoredDevice.DHCP4
				restoredDeviceDHCP6 = restoredDevice.DHCP6
				restoredSkipIPAllocation = restoredDevice.SkipIPAllocation

				if len(srcDevice.Routes) == len(dstDevice.Routes) {
					for i, dstRoute := range dstDevice.Routes {
						srcRoute := srcDevice.Routes[i]
						var restoredMetric *int32
						if len(dstDevice.Routes) == len(restoredDevice.Routes) {
							restoredMetric = restoredDevice.Routes[i].Metric
						}
						clusterv1.Convert_int32_To_Pointer_int32(srcRoute.Metric, ok, restoredMetric, &dstRoute.Metric)
						dstDevice.Routes[i] = dstRoute
					}
				}
			}

			clusterv1.Convert_bool_To_Pointer_bool(srcDevice.DHCP4, ok, restoredDeviceDHCP4, &dstDevice.DHCP4)
			clusterv1.Convert_bool_To_Pointer_bool(srcDevice.DHCP6, ok, restoredDeviceDHCP6, &dstDevice.DHCP6)
			clusterv1.Convert_bool_To_Pointer_bool(srcDevice.SkipIPAllocation, ok, restoredSkipIPAllocation, &dstDevice.SkipIPAllocation)

			dst.Spec.Network.Devices[i] = dstDevice
		}
	}

	clusterv1.Convert_bool_To_Pointer_bool(src.Status.Ready, ok, restored.Status.Ready, &dst.Status.Ready)
	if len(src.Status.Network) == len(dst.Status.Network) {
		for i, dstNetwork := range dst.Status.Network {
			srcNetwork := src.Status.Network[i]
			var restoredConnected *bool
			if len(dst.Status.Network) == len(restored.Status.Network) {
				restoredNetwork := restored.Status.Network[i]
				restoredConnected = restoredNetwork.Connected
			}

			clusterv1.Convert_bool_To_Pointer_bool(srcNetwork.Connected, ok, restoredConnected, &dstNetwork.Connected)

			dst.Status.Network[i] = dstNetwork
		}
	}
	return nil
}

func (dst *VSphereVM) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.VSphereVM)
	if err := Convert_v1beta2_VSphereVM_To_v1beta1_VSphereVM(src, dst, nil); err != nil {
		return err
	}

	return utilconversion.MarshalData(src, dst)
}

func Convert_v1beta2_VSphereClusterSpec_To_v1beta1_VSphereClusterSpec(in *infrav1.VSphereClusterSpec, out *VSphereClusterSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_VSphereClusterSpec_To_v1beta1_VSphereClusterSpec(in, out, s); err != nil {
		return err
	}
	if !reflect.DeepEqual(in.IdentityRef, infrav1.VSphereIdentityReference{}) {
		out.IdentityRef = &VSphereIdentityReference{}
		if err := autoConvert_v1beta2_VSphereIdentityReference_To_v1beta1_VSphereIdentityReference(&in.IdentityRef, out.IdentityRef, s); err != nil {
			return err
		}
	}
	return nil
}

func Convert_v1beta1_VSphereClusterSpec_To_v1beta2_VSphereClusterSpec(in *VSphereClusterSpec, out *infrav1.VSphereClusterSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_VSphereClusterSpec_To_v1beta2_VSphereClusterSpec(in, out, s); err != nil {
		return err
	}
	if in.IdentityRef != nil {
		if err := autoConvert_v1beta1_VSphereIdentityReference_To_v1beta2_VSphereIdentityReference(in.IdentityRef, &out.IdentityRef, s); err != nil {
			return err
		}
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

func Convert_v1beta2_VSphereClusterTemplateResource_To_v1beta1_VSphereClusterTemplateResource(in *infrav1.VSphereClusterTemplateResource, out *VSphereClusterTemplateResource, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_VSphereClusterTemplateResource_To_v1beta1_VSphereClusterTemplateResource(in, out, s); err != nil {
		return err
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

func Convert_v1beta2_VSphereDeploymentZoneStatus_To_v1beta1_VSphereDeploymentZoneStatus(in *infrav1.VSphereDeploymentZoneStatus, out *VSphereDeploymentZoneStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_VSphereDeploymentZoneStatus_To_v1beta1_VSphereDeploymentZoneStatus(in, out, s); err != nil {
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
	out.V1Beta2 = &VSphereDeploymentZoneV1Beta2Status{}
	out.V1Beta2.Conditions = in.Conditions
	return nil
}

func Convert_v1beta1_VSphereDeploymentZoneStatus_To_v1beta2_VSphereDeploymentZoneStatus(in *VSphereDeploymentZoneStatus, out *infrav1.VSphereDeploymentZoneStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_VSphereDeploymentZoneStatus_To_v1beta2_VSphereDeploymentZoneStatus(in, out, s); err != nil {
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
		out.Deprecated = &infrav1.VSphereDeploymentZoneDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1.VSphereDeploymentZoneV1Beta1DeprecatedStatus{}
	}
	if in.Conditions != nil {
		clusterv1beta1.Convert_v1beta1_Conditions_To_v1beta2_Deprecated_V1Beta1_Conditions(&in.Conditions, &out.Deprecated.V1Beta1.Conditions)
	}
	return nil
}

func Convert_v1beta2_Topology_To_v1beta1_Topology(in *infrav1.Topology, out *Topology, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_Topology_To_v1beta1_Topology(in, out, s); err != nil {
		return err
	}
	if !reflect.DeepEqual(in.Hosts, infrav1.FailureDomainHosts{}) {
		out.Hosts = &FailureDomainHosts{}
		if err := autoConvert_v1beta2_FailureDomainHosts_To_v1beta1_FailureDomainHosts(&in.Hosts, out.Hosts, s); err != nil {
			return err
		}
	}
	return nil
}

func Convert_v1beta1_Topology_To_v1beta2_Topology(in *Topology, out *infrav1.Topology, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_Topology_To_v1beta2_Topology(in, out, s); err != nil {
		return err
	}
	if in.Hosts != nil {
		if err := autoConvert_v1beta1_FailureDomainHosts_To_v1beta2_FailureDomainHosts(in.Hosts, &out.Hosts, s); err != nil {
			return err
		}
	}
	return nil
}

func Convert_v1beta2_VSphereMachineSpec_To_v1beta1_VSphereMachineSpec(in *infrav1.VSphereMachineSpec, out *VSphereMachineSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_VSphereMachineSpec_To_v1beta1_VSphereMachineSpec(in, out, s); err != nil {
		return err
	}

	if in.GuestSoftPowerOffTimeoutSeconds != 0 {
		out.GuestSoftPowerOffTimeout = clusterv1.ConvertFromSeconds(&in.GuestSoftPowerOffTimeoutSeconds)
	}
	return nil
}

func Convert_v1beta1_VSphereMachineSpec_To_v1beta2_VSphereMachineSpec(in *VSphereMachineSpec, out *infrav1.VSphereMachineSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_VSphereMachineSpec_To_v1beta2_VSphereMachineSpec(in, out, s); err != nil {
		return err
	}

	out.GuestSoftPowerOffTimeoutSeconds = ptr.Deref(clusterv1.ConvertToSeconds(in.GuestSoftPowerOffTimeout), 0)
	return nil
}

func Convert_v1beta2_VSphereMachineStatus_To_v1beta1_VSphereMachineStatus(in *infrav1.VSphereMachineStatus, out *VSphereMachineStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_VSphereMachineStatus_To_v1beta1_VSphereMachineStatus(in, out, s); err != nil {
		return err
	}

	// Retrieve legacy failureReason and failureMessage from the deprecated field.
	if in.Deprecated != nil && in.Deprecated.V1Beta1 != nil {
		out.FailureReason = in.Deprecated.V1Beta1.FailureReason
		out.FailureMessage = in.Deprecated.V1Beta1.FailureMessage
	}

	// Move initialization to old field
	out.Ready = ptr.Deref(in.Initialization.Provisioned, false)

	return nil
}

func Convert_v1beta1_VSphereMachineStatus_To_v1beta2_VSphereMachineStatus(in *VSphereMachineStatus, out *infrav1.VSphereMachineStatus, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_VSphereMachineStatus_To_v1beta2_VSphereMachineStatus(in, out, s); err != nil {
		return err
	}

	// Move failureReason and failureMessage to the deprecated field.
	if in.FailureReason == nil && in.FailureMessage == nil {
		return nil
	}

	if out.Deprecated == nil {
		out.Deprecated = &infrav1.VSphereMachineDeprecatedStatus{}
	}
	if out.Deprecated.V1Beta1 == nil {
		out.Deprecated.V1Beta1 = &infrav1.VSphereMachineV1Beta1DeprecatedStatus{}
	}
	out.Deprecated.V1Beta1.FailureReason = in.FailureReason
	out.Deprecated.V1Beta1.FailureMessage = in.FailureMessage
	return nil
}

func Convert_v1beta2_VSphereVMSpec_To_v1beta1_VSphereVMSpec(in *infrav1.VSphereVMSpec, out *VSphereVMSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta2_VSphereVMSpec_To_v1beta1_VSphereVMSpec(in, out, s); err != nil {
		return err
	}

	if in.GuestSoftPowerOffTimeoutSeconds != 0 {
		out.GuestSoftPowerOffTimeout = clusterv1.ConvertFromSeconds(&in.GuestSoftPowerOffTimeoutSeconds)
	}
	return nil
}

func Convert_v1beta1_VSphereVMSpec_To_v1beta2_VSphereVMSpec(in *VSphereVMSpec, out *infrav1.VSphereVMSpec, s apimachineryconversion.Scope) error {
	if err := autoConvert_v1beta1_VSphereVMSpec_To_v1beta2_VSphereVMSpec(in, out, s); err != nil {
		return err
	}

	out.GuestSoftPowerOffTimeoutSeconds = ptr.Deref(clusterv1.ConvertToSeconds(in.GuestSoftPowerOffTimeout), 0)
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

func Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta(in *clusterv1beta1.ObjectMeta, out *clusterv1.ObjectMeta, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta1_ObjectMeta_To_v1beta2_ObjectMeta(in, out, s)
}

func Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta(in *clusterv1.ObjectMeta, out *clusterv1beta1.ObjectMeta, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta2_ObjectMeta_To_v1beta1_ObjectMeta(in, out, s)
}

func Convert_v1beta1_MachineAddress_To_v1beta2_MachineAddress(in *clusterv1beta1.MachineAddress, out *clusterv1.MachineAddress, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta1_MachineAddress_To_v1beta2_MachineAddress(in, out, s)
}

func Convert_v1beta2_MachineAddress_To_v1beta1_MachineAddress(in *clusterv1.MachineAddress, out *clusterv1beta1.MachineAddress, s apimachineryconversion.Scope) error {
	return clusterv1beta1.Convert_v1beta2_MachineAddress_To_v1beta1_MachineAddress(in, out, s)
}
