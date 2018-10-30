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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type APIStatus string

const (
	ApiNotReady APIStatus = "NotReady"
	ApiReady    APIStatus = "Ready"
)

// VsphereClusterProviderConfigSpec defines the desired state of VsphereClusterProviderConfig
type VsphereClusterProviderConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// VsphereClusterProviderConfigStatus defines the observed state of VsphereClusterProviderConfig
type VsphereClusterProviderConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// VsphereClusterProviderStatus defines the observed state of VsphereClusterProviderConfig
type VsphereClusterProviderStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	LastUpdated string    `json:"lastUpdated"`
	APIStatus   APIStatus `json:"clusterApiStatus"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VsphereClusterProviderConfig is the Schema for the vsphereclusterproviderconfigs API
// +k8s:openapi-gen=true
type VsphereClusterProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	//Spec   VsphereClusterProviderConfigSpec   `json:"spec,omitempty"`
	//Status VsphereClusterProviderConfigStatus `json:"status,omitempty"`
	VsphereUser     string `json:"vsphereUser"`
	VspherePassword string `json:"vspherePassword"`
	VsphereServer   string `json:"vsphereServer"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VsphereClusterProviderConfigList contains a list of VsphereClusterProviderConfig
type VsphereClusterProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VsphereClusterProviderConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VsphereClusterProviderConfig{}, &VsphereClusterProviderConfigList{})
}
