/*
Copyright 2026 The Kubernetes Authors.

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

package hub

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conversionmeta "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta"
)

const (
	// VirtualMachineGroupMemberConditionPlacementReady indicates that the
	// member has a placement decision ready.
	VirtualMachineGroupMemberConditionPlacementReady = "PlacementReady"
)

// GroupMember describes a member of a VirtualMachineGroup.
type GroupMember struct {
	// Name is the name of member of this group.
	Name string `json:"name"`

	// +optional
	// +kubebuilder:default=VirtualMachine
	// +kubebuilder:validation:Enum=VirtualMachine;VirtualMachineGroup

	// Kind is the kind of member of this group, which can be either
	// VirtualMachine or VirtualMachineGroup.
	//
	// If omitted, it defaults to VirtualMachine.
	Kind string `json:"kind,omitempty"`
}

// VirtualMachineGroupBootOrderGroup describes a boot order group within a
// VirtualMachineGroup.
type VirtualMachineGroupBootOrderGroup struct {

	// +optional
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name

	// Members describes the names of VirtualMachine or VirtualMachineGroup
	// objects that are members of this boot order group. The VM or VM Group
	// objects must be in the same namespace as this group.
	Members []GroupMember `json:"members,omitempty"`
}

// VirtualMachineGroupSpec defines the desired state of VirtualMachineGroup.
type VirtualMachineGroupSpec struct {
	// +optional

	// BootOrder describes the boot sequence for this group members. Each boot
	// order contains a set of members that will be powered on simultaneously,
	// with an optional delay before powering on. The orders are processed
	// sequentially in the order they appear in this list, with delays being
	// cumulative across orders.
	//
	// When powering off, all members are stopped immediately without delays.
	BootOrder []VirtualMachineGroupBootOrderGroup `json:"bootOrder,omitempty"`
}

// VirtualMachinePlacementStatus describes the placement results for this member.
type VirtualMachinePlacementStatus struct {
	// +optional

	// Zone describes the recommended zone for this VM.
	Zone string `json:"zoneID,omitempty"`
}

// VirtualMachineGroupMemberStatus describes the observed status of a group
// member.
type VirtualMachineGroupMemberStatus struct {
	// Name is the name of this member.
	Name string `json:"name"`

	// +optional

	// Placement describes the placement results for this member.
	//
	// Please note this field is only set for VirtualMachine members.
	Placement *VirtualMachinePlacementStatus `json:"placement,omitempty"`

	// +optional

	// Conditions describes any conditions associated with this member.
	//
	// - The GroupLinked condition is True when the member exists and has its
	//   "Spec.GroupName" field set to the group's name.
	// - The PowerStateSynced condition is True for the VirtualMachine member
	//   when the member's power state matches the group's power state.
	// - The PlacementReady condition is True for the VirtualMachine member
	//   when the member has a placement decision ready.
	// - The ReadyType condition is True for the VirtualMachineGroup member
	//   when all of its members' conditions are True.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// VirtualMachineGroupStatus defines the observed state of VirtualMachineGroup.
type VirtualMachineGroupStatus struct {
	// +optional
	// +listType=map
	// +listMapKey=name
	// +listMapKey=kind

	// Members describes the observed status of group members.
	Members []VirtualMachineGroupMemberStatus `json:"members,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=vmg
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// VirtualMachineGroup is the schema for the VirtualMachineGroup API and
// represents the desired state and observed status of a VirtualMachineGroup
// resource.
type VirtualMachineGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineGroupSpec   `json:"spec,omitempty"`
	Status VirtualMachineGroupStatus `json:"status,omitempty"`

	Source conversionmeta.SourceTypeMeta `json:"source,omitempty,omitzero"`
}

// GetConditions returns the set of conditions for this object.
func (m VirtualMachineGroupMemberStatus) GetConditions() []metav1.Condition {
	return m.Conditions
}

// SetConditions sets conditions for an API object.
func (m *VirtualMachineGroupMemberStatus) SetConditions(conditions []metav1.Condition) {
	m.Conditions = conditions
}

// +kubebuilder:object:root=true

// VirtualMachineGroupList contains a list of VirtualMachineGroup.
type VirtualMachineGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineGroup `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &VirtualMachineGroup{}, &VirtualMachineGroupList{})
}
