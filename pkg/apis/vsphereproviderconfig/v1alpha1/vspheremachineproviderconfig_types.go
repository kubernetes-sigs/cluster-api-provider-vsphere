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

// VsphereMachineProviderConfigStatus defines the observed state of VsphereMachineProviderConfig
type VsphereMachineProviderStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	LastUpdated string `json:"lastUpdated"`
	TaskRef     string `json:"taskRef"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VsphereMachineProviderConfig is the Schema for the vspheremachineproviderconfigs API
// +k8s:openapi-gen=true
type VsphereMachineProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Name of the machine that's registered in the NamedMachines ConfigMap.
	VsphereMachine string `json:"vsphereMachine"`
	MachineRef     string `json:"machineRef,omitempty"`
	// List of variables for the chosen machine.
	MachineSpec VsphereMachineSpec `json:"machineSpec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VsphereMachineProviderConfigList contains a list of VsphereMachineProviderConfig
type VsphereMachineProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VsphereMachineProviderConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VsphereMachineProviderConfig{}, &VsphereMachineProviderConfigList{})
}

//**** New extensions

type VsphereMachineSpec struct {
	Datacenter       string        `json:"datacenter"`
	Datastore        string        `json:"datastore"`
	ResourcePool     string        `json:"resourcePool,omitempty"`
	VMFolder         string        `json:"vmFolder,omitempty"`
	Networks         []NetworkSpec `json:"networks"`
	NumCPUs          int32         `json:"numCPUs,omitempty"`
	MemoryMB         int64         `json:"memoryMB,omitempty"`
	VMTemplate       string        `json:"template"`
	Disks            []DiskSpec    `json:"disks"`
	Preloaded        bool          `json:"preloaded,omitempty"`
	VsphereCloudInit bool          `json:"vsphereCloudInit,omitempty"`
}

type NetworkSpec struct {
	NetworkName string   `json:"networkName"`
	IPConfig    IPConfig `json:"ipConfig,omitempty"`
}

type IPConfig struct {
	NetworkType NetworkType `json:"networkType"`
	IP          string      `json:"ip,omitempty"`
	Netmask     string      `json:"netmask,omitempty"`
	Gateway     string      `json:"gateway,omitempty"`
	Dns         []string    `json:"dns,omitempty"`
}

type NetworkType string

const (
	Static NetworkType = "static"
	DHCP   NetworkType = "dhcp"
)

type DiskSpec struct {
	DiskSizeGB int64  `json:"diskSizeGB,omitempty"`
	DiskLabel  string `json:"diskLabel,omitempty"`
}
