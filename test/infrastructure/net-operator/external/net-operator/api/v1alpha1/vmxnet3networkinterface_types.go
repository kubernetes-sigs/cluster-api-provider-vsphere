/*
Copyright 2024 The Kubernetes Authors.

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

// VMXNET3NetworkInterfaceSpec defines the desired state of VMXNET3NetworkInterface.
type VMXNET3NetworkInterfaceSpec struct {
	// UPTCompatibilityEnabled indicates whether UPT(Universal Pass-through) compatibility is enabled
	// on this network interface.
	UPTCompatibilityEnabled bool `json:"uptCompatibilityEnabled,omitempty"`
	// WakeOnLanEnabled indicates whether wake-on-LAN is enabled on this network interface. Clients
	// can set this property to selectively enable or disable wake-on-LAN.
	WakeOnLanEnabled bool `json:"wakeOnLanEnabled,omitempty"`
}

// VMXNET3NetworkInterfaceStatus is unused. VMXNET3NetworkInterface is a configuration only resource.
type VMXNET3NetworkInterfaceStatus struct {
}

// +genclient
// +kubebuilder:object:root=true

// VMXNET3NetworkInterface is the Schema for the vmxnet3networkinterfaces API.
// It represents configuration of a vSphere VMXNET3 type  network interface card.
type VMXNET3NetworkInterface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMXNET3NetworkInterfaceSpec   `json:"spec,omitempty"`
	Status VMXNET3NetworkInterfaceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VMXNET3NetworkInterfaceList contains a list of VMXNET3NetworkInterface.
type VMXNET3NetworkInterfaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMXNET3NetworkInterface `json:"items"`
}

func init() {
	RegisterTypeWithScheme(&VMXNET3NetworkInterface{}, &VMXNET3NetworkInterfaceList{})
}
