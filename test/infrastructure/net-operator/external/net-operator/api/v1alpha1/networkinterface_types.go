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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// NetworkInterfaceFinalizer allows the Controller to clean up resources associated
	// with a NetworkInterface before removing it from the API Server.
	NetworkInterfaceFinalizer = "networkinterface.netoperator.vmware.com"
)

// IPFamily represents the IP Family (IPv4 or IPv6). This type is used
// to express the family of an IP expressed by a type (i.e. service.Spec.IPFamily).
// NOTE: Copied from k8s.io/api/core/v1" because VM Operator is using old version.
type IPFamily string

const (
	// IPv4Protocol indicates that this IP is IPv4 protocol.
	IPv4Protocol IPFamily = "IPv4"
	// IPv6Protocol indicates that this IP is IPv6 protocol.
	IPv6Protocol IPFamily = "IPv6"
)

// IPConfig represents an IP configuration.
type IPConfig struct {
	// IP setting.
	IP string `json:"ip"`
	// IPFamily specifies the IP family (IPv4 vs IPv6) the IP belongs to.
	IPFamily IPFamily `json:"ipFamily"`
	// Gateway setting.
	Gateway string `json:"gateway"`
	// SubnetMask setting.
	SubnetMask string `json:"subnetMask"`
}

// NetworkInterfaceProviderReference contains info to locate a network interface provider object.
type NetworkInterfaceProviderReference struct {
	// APIGroup is the group for the resource being referenced.
	APIGroup string `json:"apiGroup"`
	// Kind is the type of resource being referenced.
	Kind string `json:"kind"`
	// Name is the name of resource being referenced.
	Name string `json:"name"`
	// API version of the referent.
	APIVersion string `json:"apiVersion,omitempty"`
}

type NetworkInterfaceConditionType string

const (
	// NetworkInterfaceReady is added when all network settings have been updated and the network
	// interface is ready to be used.
	NetworkInterfaceReady NetworkInterfaceConditionType = "Ready"
	// NetworkInterfaceFailure is added when network provider plugin returns an error.
	NetworkInterfaceFailure NetworkInterfaceConditionType = "Failure"
)

type NetworkInterfaceConditionReason string

const (
	// NetworkInterfaceFailureReasonCannotAllocIP is in failed state because an IPConfig cannot be allocated.
	NetworkInterfaceFailureReasonCannotAllocIP NetworkInterfaceConditionReason = "CannotAllocIP"
)

// NetworkInterfaceCondition describes the state of a NetworkInterface at a certain point.
type NetworkInterfaceCondition struct {
	// Type is the type of network interface condition.
	Type NetworkInterfaceConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	// Machine understandable string that gives the reason for condition's last transition.
	Reason NetworkInterfaceConditionReason `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

// NetworkInterfaceStatus defines the observed state of NetworkInterface.
// Once NetworkInterfaceReady condition is True, it should contain configuration to use to place
// a VM/Pod/Container's nic on the specified network.
type NetworkInterfaceStatus struct {
	// Conditions is an array of current observed network interface conditions.
	Conditions []NetworkInterfaceCondition `json:"conditions,omitempty"`
	// IPConfigs is an array of IP configurations for the network interface.
	IPConfigs []IPConfig `json:"ipConfigs,omitempty"`
	// MacAddress setting for the network interface.
	MacAddress string `json:"macAddress,omitempty"`
	// ExternalID is a network provider specific identifier assigned to the network interface.
	ExternalID string `json:"externalID,omitempty"`
	// NetworkID is an network provider specific identifier for the network backing the network
	// interface.
	NetworkID string `json:"networkID,omitempty"`
}

type NetworkInterfaceType string

const (
	// NetworkInterfaceTypeVMXNet3 is for a VMXNET3 device.
	NetworkInterfaceTypeVMXNet3 = NetworkInterfaceType("vmxnet3")
)

// NetworkInterfaceSpec defines the desired state of NetworkInterface.
type NetworkInterfaceSpec struct {
	// NetworkName refers to a NetworkObject in the same namespace.
	NetworkName string `json:"networkName,omitempty"`
	// Type is the type of NetworkInterface. Supported values are vmxnet3.
	Type NetworkInterfaceType `json:"type,omitempty"`
	// ProviderRef is a reference to a provider specific network interface object
	// that specifies the network interface configuration.
	// If unset, default configuration is assumed.
	ProviderRef *NetworkInterfaceProviderReference `json:"providerRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NetworkInterface is the Schema for the networkinterfaces API.
// A NetworkInterface represents a user's request for network configuration to use to place a
// VM/Pod/Container's nic on a specified network.
type NetworkInterface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkInterfaceSpec   `json:"spec,omitempty"`
	Status NetworkInterfaceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkInterfaceList contains a list of NetworkInterface.
type NetworkInterfaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkInterface `json:"items"`
}

func init() {
	RegisterTypeWithScheme(&NetworkInterface{}, &NetworkInterfaceList{})
}
