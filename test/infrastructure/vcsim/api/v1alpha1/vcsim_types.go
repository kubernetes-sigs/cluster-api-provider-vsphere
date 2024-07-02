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
	// VCenterFinalizer allows VCenterReconciler to clean up resources associated with VCenter before
	// removing it from the API server.
	VCenterFinalizer = "vcenter.vcsim.infrastructure.cluster.x-k8s.io"

	// VMFinalizer allows this reconciler to cleanup resources before removing the
	// VSphereVM from the API Server.
	VMFinalizer = "vcsim.fake.infrastructure.cluster.x-k8s.io"
)

// VCenterSimulatorSpec defines the desired state of the VCenterSimulator.
type VCenterSimulatorSpec struct {
	Model *VCenterSimulatorModel `json:"model,omitempty"`
}

// VCenterSimulatorModel defines the model to be used by the VCenterSimulator.
type VCenterSimulatorModel struct {
	// VSphereVersion specifies the VSphere version to use
	// Default: 7.0.0 (the minimal vCenter version required by CAPV, vcsim default is 6.5)
	VSphereVersion *string `json:"vsphereVersion,omitempty"`

	// Datacenter specifies the number of Datacenter entities to create
	// Name prefix: DC, vcsim flag: -dc
	// Default: 1
	Datacenter *int32 `json:"datacenter,omitempty"`

	// Cluster specifies the number of ClusterComputeResource entities to create per Datacenter
	// Name prefix: C, vcsim flag: -cluster
	// Default: 1
	Cluster *int32 `json:"cluster,omitempty"`

	// ClusterHost specifies the number of HostSystems entities to create within a Cluster
	// Name prefix: H, vcsim flag: -host
	// Default: 3
	ClusterHost *int32 `json:"clusterHost,omitempty"`

	// Pool specifies the number of ResourcePool entities to create per Cluster
	// Note that every cluster has a root ResourcePool named "Resources", as real vCenter does.
	// For example: /DC0/host/DC0_C0/Resources
	// The root ResourcePool is named "RP0" within other object names.
	// When Model.Pool is set to 1 or higher, this creates child ResourcePools under the root pool.
	// Note that this flag is not effective on standalone hosts (ESXi without vCenter).
	// For example: /DC0/host/DC0_C0/Resources/DC0_C0_RP1
	// Name prefix: RP, vcsim flag: -pool
	// Default: 0
	// TODO: model pool selection for each cluster; for now ResourcePool named "Resources" will be always used
	//   but ideally we should use RPx as per documentation above.
	Pool *int32 `json:"pool,omitempty"`

	// Datastore specifies the number of Datastore entities to create
	// Each Datastore will have temporary local file storage and will be mounted
	// on every HostSystem created by the ModelConfig
	// Name prefix: LocalDS, vcsim flag: -ds
	// Default: 1
	Datastore *int32 `json:"datastore,omitempty"`

	// TODO: consider if to add options for creating more folders, networks, custom storage policies
}

// VCenterSimulatorStatus defines the observed state of the VCenterSimulator.
type VCenterSimulatorStatus struct {
	// The vcsim server  url's host.
	Host string `json:"host,omitempty"`

	// The vcsim server username.
	Username string `json:"username,omitempty"`

	// The vcsim server password.
	Password string `json:"password,omitempty"`

	// The vcsim server thumbprint.
	Thumbprint string `json:"thumbprint,omitempty"`
}

// +kubebuilder:resource:path=vcentersimulators,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:object:root=true

// VCenterSimulator is the schema for a VCenter simulator server.
type VCenterSimulator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VCenterSimulatorSpec   `json:"spec,omitempty"`
	Status VCenterSimulatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VCenterSimulatorList contains a list of VCenterSimulator.
type VCenterSimulatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VCenterSimulator `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &VCenterSimulator{}, &VCenterSimulatorList{})
}
