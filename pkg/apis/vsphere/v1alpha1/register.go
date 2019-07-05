/*
Copyright 2018 The Kubernetes Authors.

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

// Package v1alpha1 contains API Schema definitions for the vsphere v1alpha1 API group
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphere
// +k8s:defaulter-gen=TypeMeta
// +groupName=vsphere.cluster.sigs.k8s.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphere"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: vsphere.GroupName, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

// GetClusterProviderSpec unmarshals a vSphere ClusterProviderSpec from a Cluster object.
func GetClusterProviderSpec(cluster *clusterv1.Cluster) (*VsphereClusterProviderSpec, error) {
	if cluster.Spec.ProviderSpec.Value == nil {
		cluster.Spec.ProviderSpec.Value = &runtime.RawExtension{}
	}
	ext := cluster.Spec.ProviderSpec.Value

	if v, ok := ext.Object.(*VsphereClusterProviderSpec); ok {
		return v, nil
	}

	var obj VsphereClusterProviderSpec
	ext.Object = &obj

	if len(ext.Raw) > 0 {
		if err := yaml.Unmarshal(ext.Raw, &obj); err != nil {
			return nil, err
		}
	}

	return &obj, nil
}

// GetClusterProviderStatus unmarshals a vSphere ClusterProviderStatus from a Cluster object.
func GetClusterProviderStatus(cluster *clusterv1.Cluster) (*VsphereClusterProviderStatus, error) {
	if cluster.Status.ProviderStatus == nil {
		cluster.Status.ProviderStatus = &runtime.RawExtension{}
	}
	ext := cluster.Status.ProviderStatus

	if v, ok := ext.Object.(*VsphereClusterProviderStatus); ok {
		return v, nil
	}

	var obj VsphereClusterProviderStatus
	ext.Object = &obj

	if len(ext.Raw) > 0 {
		if err := yaml.Unmarshal(ext.Raw, &obj); err != nil {
			return nil, err
		}
	}

	return &obj, nil
}

// GetMachineProviderSpec unmarshals a vSphere MachineProviderSpec from a Machine object.
func GetMachineProviderSpec(machine *clusterv1.Machine) (*VsphereMachineProviderSpec, error) {
	if machine.Spec.ProviderSpec.Value == nil {
		machine.Spec.ProviderSpec.Value = &runtime.RawExtension{}
	}
	ext := machine.Spec.ProviderSpec.Value

	if v, ok := ext.Object.(*VsphereMachineProviderSpec); ok {
		return v, nil
	}

	var obj VsphereMachineProviderSpec
	ext.Object = &obj

	if len(ext.Raw) > 0 {
		if err := yaml.Unmarshal(ext.Raw, &obj); err != nil {
			return nil, err
		}
	}

	return &obj, nil
}

// GetMachineProviderStatus unmarshals a provider status into an vSphere Machine type
func GetMachineProviderStatus(machine *clusterv1.Machine) (*VsphereMachineProviderStatus, error) {
	if machine.Status.ProviderStatus == nil {
		machine.Status.ProviderStatus = &runtime.RawExtension{}
	}
	ext := machine.Status.ProviderStatus

	if v, ok := ext.Object.(*VsphereMachineProviderStatus); ok {
		return v, nil
	}

	var obj VsphereMachineProviderStatus
	ext.Object = &obj

	if len(ext.Raw) > 0 {
		if err := yaml.Unmarshal(ext.Raw, &obj); err != nil {
			return nil, err
		}
	}

	return &obj, nil
}
