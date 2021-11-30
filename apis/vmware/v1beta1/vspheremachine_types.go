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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

// VSphereMachineVolume defines a PVC attachment
type VSphereMachineVolume struct {
	// Name is suffix used to name this PVC as: VSphereMachine.Name + "-" + Name
	Name string `json:"name"`
	// Capacity is the PVC capacity
	Capacity v1.ResourceList `json:"capacity"`
	// StorageClass defaults to VSphereMachineSpec.StorageClass
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// VSphereMachineSpec defines the desired state of VSphereMachine
type VSphereMachineSpec struct {
	// ProviderID is the virtual machine's BIOS UUID formated as
	// vsphere://12345678-1234-1234-1234-123456789abc.
	// This is required at runtime by CAPI. Do not remove this field.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// FailureDomain is the failure domain the machine will be created in.
	// Must match a key in the FailureDomains map stored on the cluster object.
	// +optional
	FailureDomain *string `json:"failureDomain,omitempty"`

	// ImageName is the name of the base image used when specifying the
	// underlying virtual machine
	ImageName string `json:"imageName"`

	// ClassName is the name of the class used when specifying the underlying
	// virtual machine
	ClassName string `json:"className"`

	// StorageClass is the name of the storage class used when specifying the
	// underlying virtual machine.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// Volumes is the set of PVCs to be created and attached to the VSphereMachine
	// +optional
	Volumes []VSphereMachineVolume `json:"volumes,omitempty"`
}

// VSphereMachineStatus defines the observed state of VSphereMachine
type VSphereMachineStatus struct {
	// Ready is true when the provider resource is ready.
	// This is required at runtime by CAPI. Do not remove this field.
	// +optional
	Ready bool `json:"ready"`

	// Addresses contains the instance associated addresses.
	Addresses []v1.NodeAddress `json:"addresses,omitempty"`

	// ID is used to identify the virtual machine.
	// +optional
	ID *string `json:"vmID,omitempty"`

	// IPAddr is the IP address used to access the virtual machine.
	// +optional
	IPAddr string `json:"vmIp,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// VMStatus is used to identify the virtual machine status.
	// +optional
	VMStatus VirtualMachineState `json:"vmstatus,omitempty"`

	// Conditions defines current service state of the VSphereMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// VSphereMachine is the Schema for the vspheremachines API
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vspheremachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Zone",type="string",JSONPath=".spec.failureDomain",description="Zone"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="IPAddr",type="string",JSONPath=".status.vmIp",description="IP address"
type VSphereMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VSphereMachineSpec   `json:"spec,omitempty"`
	Status VSphereMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VSphereMachineList contains a list of VSphereMachine
type VSphereMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VSphereMachine `json:"items"`
}

func (r *VSphereMachine) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

func (r *VSphereMachine) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&VSphereMachine{}, &VSphereMachineList{})
}
