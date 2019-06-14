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

// Package v1alpha1 contains API Schema definitions for the vsphereproviderconfig v1alpha1 API group
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig
// +k8s:defaulter-gen=TypeMeta
// +groupName=vsphereproviderconfig.sigs.k8s.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
	"sigs.k8s.io/yaml"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: "vsphereproviderconfig.sigs.k8s.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

// ClusterConfigFromCluster unmarshals a provider config into an vSphere Cluster type
func ClusterConfigFromCluster(in *clusterv1.Cluster) (*VsphereClusterProviderConfig, error) {
	return ClusterConfigFromProviderSpec(&in.Spec.ProviderSpec)
}

// ClusterStatusFromCluster unmarshals a provider status into an vSphere Cluster type
func ClusterStatusFromCluster(in *clusterv1.Cluster) (*VsphereClusterProviderStatus, error) {
	return ClusterStatusFromProviderStatus(&in.Status)
}

// MachineConfigFromMachine unmarshals a provider config into an vSphere Machine type
func MachineConfigFromMachine(in *clusterv1.Machine) (*VsphereMachineProviderConfig, error) {
	return MachineConfigFromProviderSpec(&in.Spec.ProviderSpec)
}

// MachineStatusFromMachine unmarshals a provider status into an vSphere Machine type
func MachineStatusFromMachine(in *clusterv1.Machine) (*VsphereMachineProviderStatus, error) {
	return MachineStatusFromProviderStatus(&in.Status)
}

// ClusterConfigFromProviderSpec unmarshals a provider config into an vSphere Cluster type
func ClusterConfigFromProviderSpec(in *clusterv1.ProviderSpec) (*VsphereClusterProviderConfig, error) {
	if in.Value == nil {
		in.Value = &runtime.RawExtension{}
	}
	ext := in.Value

	if v, ok := ext.Object.(*VsphereClusterProviderConfig); ok {
		return v, nil
	}

	var obj VsphereClusterProviderConfig
	ext.Object = &obj

	if len(ext.Raw) > 0 {
		if err := yaml.Unmarshal(ext.Raw, &obj); err != nil {
			return nil, err
		}
	}

	return &obj, nil
}

// ClusterStatusFromProviderStatus unmarshals a raw extension into an vSphere Cluster type
func ClusterStatusFromProviderStatus(in *clusterv1.ClusterStatus) (*VsphereClusterProviderStatus, error) {
	if in.ProviderStatus == nil {
		in.ProviderStatus = &runtime.RawExtension{}
	}
	ext := in.ProviderStatus

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

// MachineConfigFromProviderSpec unmarshals a provider config into an vSphere Machine type
func MachineConfigFromProviderSpec(in *clusterv1.ProviderSpec) (*VsphereMachineProviderConfig, error) {
	if in.Value == nil {
		in.Value = &runtime.RawExtension{}
	}
	ext := in.Value

	if v, ok := ext.Object.(*VsphereMachineProviderConfig); ok {
		return v, nil
	}

	var obj VsphereMachineProviderConfig
	ext.Object = &obj

	if len(ext.Raw) > 0 {
		if err := yaml.Unmarshal(ext.Raw, &obj); err != nil {
			return nil, err
		}
	}

	return &obj, nil
}

// MachineStatusFromProviderStatus unmarshals a raw extension into an vSphere machine type
func MachineStatusFromProviderStatus(in *clusterv1.MachineStatus) (*VsphereMachineProviderStatus, error) {
	if in.ProviderStatus == nil {
		in.ProviderStatus = &runtime.RawExtension{}
	}
	ext := in.ProviderStatus

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

// EncodeMachineStatus marshals the machine status
func EncodeMachineStatus(status *VsphereMachineProviderStatus) (*runtime.RawExtension, error) {
	if status == nil {
		return &runtime.RawExtension{}, nil
	}

	var rawBytes []byte
	var err error

	//  TODO: use apimachinery conversion https://godoc.org/k8s.io/apimachinery/pkg/runtime#Convert_runtime_Object_To_runtime_RawExtension
	if rawBytes, err = json.Marshal(status); err != nil {
		return nil, err
	}

	return &runtime.RawExtension{
		Raw:    rawBytes,
		Object: status,
	}, nil
}

// EncodeMachineSpec marshals the machine provider spec.
func EncodeMachineSpec(spec *VsphereMachineProviderConfig) (*runtime.RawExtension, error) {
	if spec == nil {
		return &runtime.RawExtension{}, nil
	}

	var rawBytes []byte
	var err error

	//  TODO: use apimachinery conversion https://godoc.org/k8s.io/apimachinery/pkg/runtime#Convert_runtime_Object_To_runtime_RawExtension
	if rawBytes, err = json.Marshal(spec); err != nil {
		return nil, err
	}

	return &runtime.RawExtension{
		Raw:    rawBytes,
		Object: spec,
	}, nil
}

// EncodeClusterStatus marshals the cluster status.
func EncodeClusterStatus(status *VsphereClusterProviderStatus) (*runtime.RawExtension, error) {
	if status == nil {
		return &runtime.RawExtension{}, nil
	}

	var rawBytes []byte
	var err error

	//  TODO: use apimachinery conversion https://godoc.org/k8s.io/apimachinery/pkg/runtime#Convert_runtime_Object_To_runtime_RawExtension
	if rawBytes, err = json.Marshal(status); err != nil {
		return nil, err
	}

	return &runtime.RawExtension{
		Raw:    rawBytes,
		Object: status,
	}, nil
}

// EncodeClusterSpec marshals the cluster provider spec.
func EncodeClusterSpec(spec *VsphereClusterProviderConfig) (*runtime.RawExtension, error) {
	if spec == nil {
		return &runtime.RawExtension{}, nil
	}

	var rawBytes []byte
	var err error

	//  TODO: use apimachinery conversion https://godoc.org/k8s.io/apimachinery/pkg/runtime#Convert_runtime_Object_To_runtime_RawExtension
	if rawBytes, err = json.Marshal(spec); err != nil {
		return nil, err
	}

	return &runtime.RawExtension{
		Raw:    rawBytes,
		Object: spec,
	}, nil
}
