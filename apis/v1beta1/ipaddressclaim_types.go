/*
Copyright 2022 The Kubernetes Authors.

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

//nolint:godot
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// IPAddressClaimFinalizer allows ReconcileVSphereMachine to clean up IP Addresses
	// resources associated with VSphereMachine before removing it from the
	// API server.
	IPAddressClaimFinalizer = "ipaddressclaim.infrastructure.cluster.x-k8s.io"
)

// IPAddressClaimSpec describes the desired state of an IPAddressClaim
type IPAddressClaimSpec struct {
	// Pool is a reference to the pool from which an IP address should be allocated.
	Pool LocalObjectReference `json:"pool,omitempty"`
}

// IPAddressClaimStatus contains the status of an IPAddressClaim
type IPAddressClaimStatus struct {
	// Address is a reference to the address that was allocated for this claim.
	Address LocalObjectReference `json:"address,omitempty"`

	// Conditions provide details about the status of the claim.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=ipaddressclaims,scope=Namespaced,categories=cluster-api
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for VSphereMachine"
//+kubebuilder:printcolumn:name="Server",type="string",JSONPath=".spec.server",description="Server is the address of the vSphere endpoint."
//+kubebuilder:printcolumn:name="ControlPlaneEndpoint",type="string",JSONPath=".spec.controlPlaneEndpoint[0]",description="API Endpoint",priority=1

// IPAddressClaim can be used to allocate IPAddresses from an IP Pool.
type IPAddressClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPAddressClaimSpec   `json:"spec,omitempty"`
	Status IPAddressClaimStatus `json:"status,omitempty"`
}

type LocalObjectReference struct {
	Group string `json:"group,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Name  string `json:"name,omitempty"`
}

func (c *IPAddressClaim) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

func (c *IPAddressClaim) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true
// IPAddressClaimList contains a list of VSphereCluster
type IPAddressClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPAddressClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPAddressClaim{}, &IPAddressClaimList{})
}
