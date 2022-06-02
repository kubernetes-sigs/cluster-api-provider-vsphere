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
)

const (
	// IPAddressClaimFinalizer allows ReconcileVSphereMachine to clean up IP Addresses
	// resources associated with VSphereMachine before removing it from the
	// API server.
	IPAddressFinalizer = "ipaddress.infrastructure.cluster.x-k8s.io"
)

// IPAddressClaimSpec describes the desired state of an IPAddressClaim
type IPAddressSpec struct {
	// Pool is a reference to the pool from which an IP address should be allocated.
	Pool LocalObjectReference `json:"pool,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=ipaddressclaims,scope=Namespaced,categories=cluster-api
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for VSphereMachine"
//+kubebuilder:printcolumn:name="Server",type="string",JSONPath=".spec.server",description="Server is the address of the vSphere endpoint."
//+kubebuilder:printcolumn:name="ControlPlaneEndpoint",type="string",JSONPath=".spec.controlPlaneEndpoint[0]",description="API Endpoint",priority=1

// IPAddress is a representation of an IP Address that was allocated from an IP Pool.
type IPAddress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec IPAddressSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
// IPAddressList contains a list of VSphereCluster
type IPAddressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPAddressClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPAddress{}, &IPAddressList{})
}
