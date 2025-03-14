/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterFinalizer allows ReconcileVSphereCluster to clean up vSphere
	// resources associated with VSphereCluster before removing it from the
	// API server.
	ClusterFinalizer = "vspherecluster.vmware.infrastructure.cluster.x-k8s.io"

	// ProviderServiceAccountFinalizer allows ServiceAccountReconciler to clean up service accounts
	// resources associated with VSphereCluster from the SERVICE_ACCOUNTS_CM (service accounts ConfigMap).
	//
	// Deprecated: ProviderServiceAccountFinalizer will be removed in a future release.
	ProviderServiceAccountFinalizer = "providerserviceaccount.vmware.infrastructure.cluster.x-k8s.io"
)

// VSphereClusterSpec defines the desired state of VSphereCluster.
type VSphereClusterSpec struct {
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
	// placement allows to configure the placement of machines of a VSphereCluster.
	// +optional
	Placement *VSphereClusterPlacement `json:"placement,omitempty"`
}

// VSphereClusterPlacement defines a placement strategy for machines of a VSphereCluster.
// +kubebuilder:validation:MinProperties=1
type VSphereClusterPlacement struct {
	// workerAntiAffinity configures soft anti-affinity for workers.
	// +optional
	WorkerAntiAffinity *VSphereClusterWorkerAntiAffinity `json:"workerAntiAffinity,omitempty"`
}

// VSphereClusterWorkerAntiAffinity defines the anti-affinity configuration for workers.
// +kubebuilder:validation:MinProperties=1
type VSphereClusterWorkerAntiAffinity struct {
	// mode allows to set the grouping of (soft) anti-affinity for worker nodes.
	// Defaults to `Cluster`.
	// +kubebuilder:validation:Enum=Cluster;None;MachineDeployment
	// +optional
	Mode VSphereClusterWorkerAntiAffinityMode `json:"mode,omitempty"`
}

// VSphereClusterWorkerAntiAffinityMode describes the soft anti-affinity mode used across a for distributing virtual machines.
type VSphereClusterWorkerAntiAffinityMode string

const (
	// VSphereClusterWorkerAntiAffinityModeCluster means to use all workers as a single group for soft anti-affinity.
	VSphereClusterWorkerAntiAffinityModeCluster VSphereClusterWorkerAntiAffinityMode = "Cluster"

	// VSphereClusterWorkerAntiAffinityModeNone means to not configure any soft anti-affinity for workers.
	VSphereClusterWorkerAntiAffinityModeNone VSphereClusterWorkerAntiAffinityMode = "None"

	// VSphereClusterWorkerAntiAffinityModeMachineDeployment means to configure soft anti-affinity for all workers per MachineDeployment.
	VSphereClusterWorkerAntiAffinityModeMachineDeployment VSphereClusterWorkerAntiAffinityMode = "MachineDeployment"
)

// VSphereClusterStatus defines the observed state of VSphereClusterSpec.
type VSphereClusterStatus struct {
	// Ready indicates the infrastructure required to deploy this cluster is
	// ready.
	// +optional
	Ready bool `json:"ready"`

	// ResourcePolicyName is the name of the VirtualMachineSetResourcePolicy for
	// the cluster, if one exists
	// +optional
	ResourcePolicyName string `json:"resourcePolicyName,omitempty"`

	// Conditions defines current service state of the VSphereCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureDomains is a list of failure domain objects synced from the
	// infrastructure provider.
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vsphereclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// VSphereCluster is the Schema for the VSphereClusters API.
type VSphereCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VSphereClusterSpec   `json:"spec,omitempty"`
	Status VSphereClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VSphereClusterList contains a list of VSphereCluster.
type VSphereClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VSphereCluster `json:"items"`
}

// GetConditions returns conditions for VSphereCluster.
func (r *VSphereCluster) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets conditions on the VSphereCluster.
func (r *VSphereCluster) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

func init() {
	objectTypes = append(objectTypes, &VSphereCluster{}, &VSphereClusterList{})
}
