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

const (
	// ControlPlaneEndpointFinalizer allows ControlPlaneEndpointReconciler to clean up resources associated with ControlPlaneEndpoint before
	// removing it from the API server.
	ControlPlaneEndpointFinalizer = "control-plane-endpoint.vcsim.infrastructure.cluster.x-k8s.io"
)

// ControlPlaneEndpointSpec defines the desired state of the ControlPlaneEndpoint.
type ControlPlaneEndpointSpec struct {
}

// ControlPlaneEndpointStatus defines the observed state of the ControlPlaneEndpoint.
type ControlPlaneEndpointStatus struct {
	// The control plane host.
	Host string `json:"host,omitempty"`

	// The control plane port.
	Port int32 `json:"port,omitempty"`
}

// +kubebuilder:resource:path=controlplaneendpoints,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:object:root=true

// ControlPlaneEndpoint is the schema for a cluster virtual ip.
// IMPORTANT: The name of the ControlPlaneEndpoint should match the name of the cluster.
type ControlPlaneEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneEndpointSpec   `json:"spec,omitempty"`
	Status ControlPlaneEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ControlPlaneEndpointList contains a list of ControlPlaneEndpoint.
type ControlPlaneEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlaneEndpoint `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &ControlPlaneEndpoint{}, &ControlPlaneEndpointList{})
}
