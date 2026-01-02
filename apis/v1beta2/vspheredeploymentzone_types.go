/*
Copyright 2025 The Kubernetes Authors.

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

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	// DeploymentZoneFinalizer allows ReconcileVSphereDeploymentZone to
	// check for dependents associated with VSphereDeploymentZone
	// before removing it from the API Server.
	DeploymentZoneFinalizer = "vspheredeploymentzone.infrastructure.cluster.x-k8s.io"
)

// VSphereDeploymentZone's Ready condition and corresponding reasons that will be used in v1Beta2 API version.
const (
	// VSphereDeploymentZoneReadyV1Beta2Condition is true if the VSphereDeploymentZone's deletionTimestamp is not set, VSphereDeploymentZone's
	// VCenterAvailable, PlacementConstraintReady and FailureDomainValidated conditions are true.
	VSphereDeploymentZoneReadyV1Beta2Condition = clusterv1.ReadyCondition

	// VSphereDeploymentZoneReadyV1Beta2Reason surfaces when the VSphereDeploymentZone readiness criteria is met.
	VSphereDeploymentZoneReadyV1Beta2Reason = clusterv1.ReadyReason

	// VSphereDeploymentZoneNotReadyV1Beta2Reason surfaces when the VSphereDeploymentZone readiness criteria is not met.
	VSphereDeploymentZoneNotReadyV1Beta2Reason = clusterv1.NotReadyReason

	// VSphereDeploymentZoneReadyUnknownV1Beta2Reason surfaces when at least one VSphereDeploymentZone readiness criteria is unknown
	// and no VSphereDeploymentZone readiness criteria is not met.
	VSphereDeploymentZoneReadyUnknownV1Beta2Reason = clusterv1.ReadyUnknownReason
)

// VSphereDeploymentZone's VCenterAvailable condition and corresponding reasons that will be used in v1Beta2 API version.
const (
	// VSphereDeploymentZoneVCenterAvailableV1Beta2Condition documents the status of vCenter for a VSphereDeploymentZone.
	VSphereDeploymentZoneVCenterAvailableV1Beta2Condition = "VCenterAvailable"

	// VSphereDeploymentZoneVCenterAvailableV1Beta2Reason surfaces when the vCenter for a VSphereDeploymentZone is available.
	VSphereDeploymentZoneVCenterAvailableV1Beta2Reason = clusterv1.AvailableReason

	// VSphereDeploymentZoneVCenterUnreachableV1Beta2Reason surfaces when the vCenter for a VSphereDeploymentZone is unreachable.
	VSphereDeploymentZoneVCenterUnreachableV1Beta2Reason = "VCenterUnreachable"

	// VSphereDeploymentZoneVCenterAvailableDeletingV1Beta2Reason surfaces when the vCenter for a VSphereDeploymentZone is being deleted.
	VSphereDeploymentZoneVCenterAvailableDeletingV1Beta2Reason = clusterv1.DeletingReason
)

// VSphereDeploymentZone's PlacementConstraintReady condition and corresponding reasons that will be used in v1Beta2 API version.
const (
	// VSphereDeploymentZonePlacementConstraintReadyV1Beta2Condition documents the placement constraint status for a VSphereDeploymentZone.
	VSphereDeploymentZonePlacementConstraintReadyV1Beta2Condition = "PlacementConstraintReady"

	// VSphereDeploymentZonePlacementConstraintReadyV1Beta2Reason surfaces when the placement status for a VSphereDeploymentZone is ready.
	VSphereDeploymentZonePlacementConstraintReadyV1Beta2Reason = clusterv1.ReadyReason

	// VSphereDeploymentZonePlacementConstraintResourcePoolNotFoundV1Beta2Reason surfaces when the resource pool for a VSphereDeploymentZone is not found.
	VSphereDeploymentZonePlacementConstraintResourcePoolNotFoundV1Beta2Reason = "ResourcePoolNotFound"

	// VSphereDeploymentZonePlacementConstraintFolderNotFoundV1Beta2Reason surfaces when the folder for a VSphereDeploymentZone is not found.
	VSphereDeploymentZonePlacementConstraintFolderNotFoundV1Beta2Reason = "FolderNotFound"

	// VSphereDeploymentZonePlacementConstraintDeletingV1Beta2Reason surfaces when the VSphereDeploymentZone is being deleted.
	VSphereDeploymentZonePlacementConstraintDeletingV1Beta2Reason = clusterv1.DeletingReason
)

// VSphereDeploymentZone's FailureDomainValidated condition and corresponding reasons that will be used in v1Beta2 API version.
const (
	// VSphereDeploymentZoneFailureDomainValidatedV1Beta2Condition documents failure domain validation status for a VSphereDeploymentZone.
	VSphereDeploymentZoneFailureDomainValidatedV1Beta2Condition = "FailureDomainValidated"

	// VSphereDeploymentZoneFailureDomainValidatedV1Beta2Reason surfaces when the failure domain for a VSphereDeploymentZone is validated.
	VSphereDeploymentZoneFailureDomainValidatedV1Beta2Reason = "Validated"

	// VSphereDeploymentZoneFailureDomainValidationFailedV1Beta2Reason surfaces when the failure domain's validation for a VSphereDeploymentZone failed.
	VSphereDeploymentZoneFailureDomainValidationFailedV1Beta2Reason = "ValidationFailed"

	// VSphereDeploymentZoneFailureDomainRegionMisconfiguredV1Beta2Reason surfaces when the failure domain's region for a VSphereDeploymentZone is misconfigured.
	VSphereDeploymentZoneFailureDomainRegionMisconfiguredV1Beta2Reason = "RegionMisconfigured"

	// VSphereDeploymentZoneFailureDomainZoneMisconfiguredV1Beta2Reason surfaces when the failure domain's zone for a VSphereDeploymentZone is misconfigured.
	VSphereDeploymentZoneFailureDomainZoneMisconfiguredV1Beta2Reason = "ZoneMisconfigured"

	// VSphereDeploymentZoneFailureDomainHostsMisconfiguredV1Beta2Reason surfaces when the failure domain's hosts for a VSphereDeploymentZone are misconfigured.
	VSphereDeploymentZoneFailureDomainHostsMisconfiguredV1Beta2Reason = "HostsMisconfigured"

	// VSphereDeploymentZoneFailureDomainDatastoreNotFoundV1Beta2Reason surfaces when the failure domain's datastore for a VSphereDeploymentZone is not found.
	VSphereDeploymentZoneFailureDomainDatastoreNotFoundV1Beta2Reason = "DatastoreNotFound"

	// VSphereDeploymentZoneFailureDomainNetworkNotFoundV1Beta2Reason surfaces when the failure domain's network for a VSphereDeploymentZone is not found.
	VSphereDeploymentZoneFailureDomainNetworkNotFoundV1Beta2Reason = "NetworkNotFound"

	// VSphereDeploymentZoneFailureDomainComputeClusterNotFoundV1Beta2Reason surfaces when the failure domain's compute cluster for a VSphereDeploymentZone is not found.
	VSphereDeploymentZoneFailureDomainComputeClusterNotFoundV1Beta2Reason = "ComputeClusterNotFound"

	// VSphereDeploymentZoneFailureDomainResourcePoolNotFoundV1Beta2Reason surfaces when the failure domain's resource pool for a VSphereDeploymentZone is not found.
	VSphereDeploymentZoneFailureDomainResourcePoolNotFoundV1Beta2Reason = "ResourcePoolNotFound"

	// VSphereDeploymentZoneFailureDomainDeletingV1Beta2Reason surfaces when the VSphereDeploymentZone is being deleted.
	VSphereDeploymentZoneFailureDomainDeletingV1Beta2Reason = clusterv1.DeletingReason
)

// VSphereDeploymentZoneSpec defines the desired state of VSphereDeploymentZone.
type VSphereDeploymentZoneSpec struct {

	// Server is the address of the vSphere endpoint.
	Server string `json:"server,omitempty"`

	// FailureDomain is the name of the VSphereFailureDomain used for this VSphereDeploymentZone
	FailureDomain string `json:"failureDomain,omitempty"`

	// ControlPlane determines if this failure domain is suitable for use by control plane machines.
	// +optional
	ControlPlane *bool `json:"controlPlane,omitempty"`

	// PlacementConstraint encapsulates the placement constraints
	// used within this deployment zone.
	PlacementConstraint PlacementConstraint `json:"placementConstraint"`
}

// PlacementConstraint is the context information for VM placements within a failure domain.
type PlacementConstraint struct {
	// ResourcePool is the name or inventory path of the resource pool in which
	// the virtual machine is created/located.
	// +optional
	ResourcePool string `json:"resourcePool,omitempty"`

	// Folder is the name or inventory path of the folder in which the
	// virtual machine is created/located.
	// +optional
	Folder string `json:"folder,omitempty"`
}

// Network holds information about the network.
type Network struct {
	// Name is the network name for this machine's VM.
	Name string `json:"name,omitempty"`

	// DHCP4 is a flag that indicates whether or not to use DHCP for IPv4
	// +optional
	DHCP4 *bool `json:"dhcp4,omitempty"`

	// DHCP6 indicates whether or not to use DHCP for IPv6
	// +optional
	DHCP6 *bool `json:"dhcp6,omitempty"`
}

// VSphereDeploymentZoneStatus contains the status for a VSphereDeploymentZone.
type VSphereDeploymentZoneStatus struct {
	// conditions represents the observations of a VSphereDeploymentZone's current state.
	// Known condition types are Ready, VCenterAvailable, PlacementConstraintReady, FailureDomainValidated and Paused.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Ready is true when the VSphereDeploymentZone resource is ready.
	// If set to false, it will be ignored by VSphereClusters
	// +optional
	Ready *bool `json:"ready,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed when all the nested field are removed.
	// +optional
	Deprecated *VSphereDeploymentZoneDeprecatedStatus `json:"deprecated,omitempty"`
}

// VSphereDeploymentZoneDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type VSphereDeploymentZoneDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *VSphereDeploymentZoneV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// VSphereDeploymentZoneV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type VSphereDeploymentZoneV1Beta1DeprecatedStatus struct {
	// conditions defines current service state of the VSphereCluster.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.
	//
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:path=vspheredeploymentzones,scope=Cluster,categories=cluster-api
// +kubebuilder:subresource:status

// VSphereDeploymentZone is the Schema for the vspheredeploymentzones API.
type VSphereDeploymentZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VSphereDeploymentZoneSpec   `json:"spec,omitempty"`
	Status VSphereDeploymentZoneStatus `json:"status,omitempty"`
}

// GetV1Beta1Conditions returns the set of conditions for this object.
func (c *VSphereDeploymentZone) GetV1Beta1Conditions() clusterv1.Conditions {
	if c.Status.Deprecated == nil || c.Status.Deprecated.V1Beta1 == nil {
		return nil
	}
	return c.Status.Deprecated.V1Beta1.Conditions
}

// SetV1Beta1Conditions sets the conditions on this object.
func (c *VSphereDeploymentZone) SetV1Beta1Conditions(conditions clusterv1.Conditions) {
	if c.Status.Deprecated == nil {
		c.Status.Deprecated = &VSphereDeploymentZoneDeprecatedStatus{}
	}
	if c.Status.Deprecated.V1Beta1 == nil {
		c.Status.Deprecated.V1Beta1 = &VSphereDeploymentZoneV1Beta1DeprecatedStatus{}
	}
	c.Status.Deprecated.V1Beta1.Conditions = conditions
}

// GetConditions returns the set of conditions for this object.
func (c *VSphereDeploymentZone) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions sets conditions for an API object.
func (c *VSphereDeploymentZone) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// VSphereDeploymentZoneList contains a list of VSphereDeploymentZone.
type VSphereDeploymentZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VSphereDeploymentZone `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &VSphereDeploymentZone{}, &VSphereDeploymentZoneList{})
}
