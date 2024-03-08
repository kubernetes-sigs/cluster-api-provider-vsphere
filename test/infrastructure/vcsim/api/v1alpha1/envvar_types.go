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

// EnvVarSpec defines the desired state of the EnvVar.
type EnvVarSpec struct {
	// Name of the VCenterSimulator instance to use as source for EnvVar values.
	VCenterSimulator *NamespacedRef `json:"vCenterSimulator,omitempty"`

	// Name of the ControlPlaneEndpoint instance to use as source for EnvVar values.
	ControlPlaneEndpoint NamespacedRef `json:"controlPlaneEndpoint,omitempty"`

	// Cluster specific values to use as source for EnvVar values.
	Cluster ClusterEnvVarSpec `json:"cluster,omitempty"`

	// Name of the VMOperatorDependencies instance to use as source for EnvVar values.
	// If not specified, a default dependenciesConfig that works for vcsim is used.
	// NOTE: this is required only for supervisor mode; also:
	// - the system automatically picks the first StorageClass defined in the VMOperatorDependencies
	// - the system automatically picks the first VirtualMachine class defined in the VMOperatorDependencies
	// - the system automatically picks the first Image from the content library defined in the VMOperatorDependencies
	VMOperatorDependencies *NamespacedRef `json:"vmOperatorDependencies,omitempty"`
}

// NamespacedRef defines a reference to an object of a well known API Group and kind.
type NamespacedRef struct {
	// Namespace of the referenced object.
	// If empty, it defaults to the namespace of the parent object.
	Namespace string `json:"namespace,omitempty"`

	// Name of the referenced object.
	Name string `json:"name,omitempty"`
}

// ClusterEnvVarSpec defines the spec for the EnvVar generator targeting a specific Cluster API cluster.
type ClusterEnvVarSpec struct {
	// The name of the Cluster API cluster.
	Name string `json:"name"`

	// The namespace of the Cluster API cluster.
	Namespace string `json:"namespace"`

	// The Kubernetes version of the Cluster API cluster.
	// NOTE: This variable isn't related to the vcsim controller, but we are handling it here
	// in order to have a single point of control for all the variables related to a Cluster API template.
	// Default: v1.28.0
	KubernetesVersion *string `json:"kubernetesVersion,omitempty"`

	// The number of the control plane machines in the Cluster API cluster.
	// NOTE: This variable isn't related to the vcsim controller, but we are handling it here
	// in order to have a single point of control for all the variables related to a Cluster API template.
	// Default: 1
	ControlPlaneMachines *int32 `json:"controlPlaneMachines,omitempty"`

	// The number of the worker machines in the Cluster API cluster.
	// NOTE: This variable isn't related to the vcsim controller, but we are handling it here
	// in order to have a single point of control for all the variables related to a Cluster API template.
	// Default: 1
	WorkerMachines *int32 `json:"workerMachines,omitempty"`

	// Datacenter specifies the Datacenter for the Cluster API cluster.
	// Default: 0 (DC0)
	Datacenter *int32 `json:"datacenter,omitempty"`

	// Cluster specifies the VCenter Cluster for the Cluster API cluster.
	// Default: 0 (C0)
	Cluster *int32 `json:"cluster,omitempty"`

	// Datastore specifies the Datastore for the Cluster API cluster.
	// Default: 0 (LocalDS_0)
	Datastore *int32 `json:"datastore,omitempty"`

	// The PowerOffMode for the machines in the cluster.
	// Default: trySoft
	PowerOffMode *string `json:"powerOffMode,omitempty"`
}

// EnvVarStatus defines the observed state of the EnvVar.
type EnvVarStatus struct {
	// variables to use when creating the Cluster API cluster.
	Variables map[string]string `json:"variables,omitempty"`
}

// +kubebuilder:resource:path=envvars,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:object:root=true

// EnvVar is the schema for a EnvVar generator.
type EnvVar struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvVarSpec   `json:"spec,omitempty"`
	Status EnvVarStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvVarList contains a list of EnvVar.
type EnvVarList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnvVar `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &EnvVar{}, &EnvVarList{})
}
