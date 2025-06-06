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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

const (
	// MachineFinalizer allows ReconcileVSphereMachine to clean up VSphere
	// resources associated with VSphereMachine before removing it from the
	// API Server.
	MachineFinalizer = "vspheremachine.infrastructure.cluster.x-k8s.io"
)

// VSphereMachine's Ready condition and corresponding reasons that will be used in v1Beta2 API version.
const (
	// VSphereMachineReadyV1Beta2Condition is true if the VSphereMachine's deletionTimestamp is not set, VSphereMachine's
	// VirtualMachineProvisioned is true.
	VSphereMachineReadyV1Beta2Condition = clusterv1beta1.ReadyV1Beta2Condition

	// VSphereMachineReadyV1Beta2Reason surfaces when the VSphereMachine readiness criteria is met.
	VSphereMachineReadyV1Beta2Reason = clusterv1beta1.ReadyV1Beta2Reason

	// VSphereMachineNotReadyV1Beta2Reason surfaces when the VSphereMachine readiness criteria is not met.
	VSphereMachineNotReadyV1Beta2Reason = clusterv1beta1.NotReadyV1Beta2Reason

	// VSphereMachineReadyUnknownV1Beta2Reason surfaces when at least one VSphereMachine readiness criteria is unknown
	// and no VSphereMachine readiness criteria is not met.
	VSphereMachineReadyUnknownV1Beta2Reason = clusterv1beta1.ReadyUnknownV1Beta2Reason
)

// VSphereMachine's VirtualMachineProvisioned condition and corresponding reasons that will be used in v1Beta2 API version.
//
// NOTE:
//   - In supervisor mode, before creating the VM the VirtualMachine goes trough a series of preflight checks; if one is failing, the
//     reason for this failure and the message are surfaced in the VSphereMachine's VirtualMachineProvisioned condition.
//   - In govmomi mode, in some cases, reason and message from the VSphereVM are surfaced in the VSphereMachine's
//     VirtualMachineProvisioned condition.
const (
	// VSphereMachineVirtualMachineProvisionedV1Beta2Condition documents the status of the VirtualMachine that is controlled
	// by the VSphereMachine.
	VSphereMachineVirtualMachineProvisionedV1Beta2Condition = "VirtualMachineProvisioned"

	// VSphereMachineVirtualMachineWaitingForClusterInfrastructureReadyV1Beta2Reason documents the VirtualMachine that is controlled
	// by the VSphereMachine waiting for the cluster infrastructure to be ready.
	// Note: This reason is used only in govmomi mode.
	VSphereMachineVirtualMachineWaitingForClusterInfrastructureReadyV1Beta2Reason = clusterv1beta1.WaitingForClusterInfrastructureReadyV1Beta2Reason

	// VSphereMachineVirtualMachineWaitingForControlPlaneInitializedV1Beta2Reason documents the VirtualMachine that is controlled
	// by the VSphereMachine waiting for the control plane to be initialized.
	VSphereMachineVirtualMachineWaitingForControlPlaneInitializedV1Beta2Reason = clusterv1beta1.WaitingForControlPlaneInitializedV1Beta2Reason

	// VSphereMachineVirtualMachineWaitingForBootstrapDataV1Beta2Reason documents the VirtualMachine that is controlled
	// by the VSphereMachine waiting for the bootstrap data to be ready.
	VSphereMachineVirtualMachineWaitingForBootstrapDataV1Beta2Reason = clusterv1beta1.WaitingForBootstrapDataV1Beta2Reason

	// VSphereMachineVirtualMachineProvisioningV1Beta2Reason surfaces when the VirtualMachine that is controlled
	// by the VSphereMachine is provisioning.
	// Note: This reason is used only in supervisor mode.
	VSphereMachineVirtualMachineProvisioningV1Beta2Reason = "Provisioning"

	// VSphereMachineVirtualMachinePoweringOnV1Beta2Reason surfaces when the VirtualMachine that is controlled
	// by the VSphereMachine is executing the power on sequence.
	// Note: This reason is used only in supervisor mode.
	VSphereMachineVirtualMachinePoweringOnV1Beta2Reason = "PoweringOn"

	// VSphereMachineVirtualMachineWaitingForNetworkAddressV1Beta2Reason surfaces when the VirtualMachine that is controlled
	// by the VSphereMachine waiting for the machine network settings to be reported after machine being powered on.
	VSphereMachineVirtualMachineWaitingForNetworkAddressV1Beta2Reason = "WaitingForNetworkAddress"

	// VSphereMachineVirtualMachineWaitingForBIOSUUIDV1Beta2Reason surfaces when the VirtualMachine that is controlled
	// by the VSphereMachine waiting for the machine to have a BIOS UUID.
	// Note: This reason is used only in supervisor mode.
	VSphereMachineVirtualMachineWaitingForBIOSUUIDV1Beta2Reason = "WaitingForBIOSUUID"

	// VSphereMachineVirtualMachineProvisionedV1Beta2Reason surfaces when the VirtualMachine that is controlled
	// by the VSphereMachine is provisioned.
	VSphereMachineVirtualMachineProvisionedV1Beta2Reason = clusterv1beta1.ProvisionedV1Beta2Reason

	// VSphereMachineVirtualMachineNotProvisionedV1Beta2Reason surfaces when the VirtualMachine that is controlled
	// by the VSphereMachine is not provisioned.
	VSphereMachineVirtualMachineNotProvisionedV1Beta2Reason = clusterv1beta1.NotProvisionedV1Beta2Reason

	// VSphereMachineVirtualMachineDeletingV1Beta2Reason surfaces when the VirtualMachine that is controlled
	// by the VSphereMachine is being deleted.
	VSphereMachineVirtualMachineDeletingV1Beta2Reason = clusterv1beta1.DeletingV1Beta2Reason
)

// VSphereMachineSpec defines the desired state of VSphereMachine.
type VSphereMachineSpec struct {
	VirtualMachineCloneSpec `json:",inline"`

	// ProviderID is the virtual machine's BIOS UUID formated as
	// vsphere://12345678-1234-1234-1234-123456789abc
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// FailureDomain is the failure domain unique identifier this Machine should be attached to, as defined in Cluster API.
	// For this infrastructure provider, the name is equivalent to the name of the VSphereDeploymentZone.
	FailureDomain *string `json:"failureDomain,omitempty"`

	// PowerOffMode describes the desired behavior when powering off a VM.
	//
	// There are three, supported power off modes: hard, soft, and
	// trySoft. The first mode, hard, is the equivalent of a physical
	// system's power cord being ripped from the wall. The soft mode
	// requires the VM's guest to have VM Tools installed and attempts to
	// gracefully shut down the VM. Its variant, trySoft, first attempts
	// a graceful shutdown, and if that fails or the VM is not in a powered off
	// state after reaching the GuestSoftPowerOffTimeout, the VM is halted.
	//
	// If omitted, the mode defaults to hard.
	//
	// +optional
	// +kubebuilder:default=hard
	PowerOffMode VirtualMachinePowerOpMode `json:"powerOffMode,omitempty"`

	// GuestSoftPowerOffTimeout sets the wait timeout for shutdown in the VM guest.
	// The VM will be powered off forcibly after the timeout if the VM is still
	// up and running when the PowerOffMode is set to trySoft.
	//
	// This parameter only applies when the PowerOffMode is set to trySoft.
	//
	// If omitted, the timeout defaults to 5 minutes.
	//
	// +optional
	GuestSoftPowerOffTimeout *metav1.Duration `json:"guestSoftPowerOffTimeout,omitempty"`

	// NamingStrategy allows configuring the naming strategy used when calculating the name of the VSphereVM.
	// +optional
	NamingStrategy *VSphereVMNamingStrategy `json:"namingStrategy,omitempty"`
}

// VSphereVMNamingStrategy defines the naming strategy for the VSphereVMs.
type VSphereVMNamingStrategy struct {
	// Template defines the template to use for generating the name of the VSphereVM object.
	// If not defined, it will fall back to `{{ .machine.name }}`.
	// The templating has the following data available:
	// * `.machine.name`: The name of the Machine object.
	// The templating also has the following funcs available:
	// * `trimSuffix`: same as strings.TrimSuffix
	// * `trunc`: truncates a string, e.g. `trunc 2 "hello"` or `trunc -2 "hello"`
	// Notes:
	// * While the template offers some flexibility, we would like the name to link to the Machine name
	//   to ensure better user experience when troubleshooting
	// * Generated names must be valid Kubernetes names as they are used to create a VSphereVM object
	//   and usually also as the name of the Node object.
	// * Names are automatically truncated at 63 characters. Please note that this can lead to name conflicts,
	//   so we highly recommend to use a template which leads to a name shorter than 63 characters.
	// +optional
	Template *string `json:"template,omitempty"`
}

// VSphereMachineStatus defines the observed state of VSphereMachine.
type VSphereMachineStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Addresses contains the VSphere instance associated addresses.
	Addresses []clusterv1beta1.MachineAddress `json:"addresses,omitempty"`

	// Network returns the network status for each of the machine's configured
	// network interfaces.
	// +optional
	Network []NetworkStatus `json:"network,omitempty"`

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

	// Conditions defines current service state of the VSphereMachine.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in VSphereMachine's status with the V1Beta2 version.
	// +optional
	V1Beta2 *VSphereMachineV1Beta2Status `json:"v1beta2,omitempty"`
}

// VSphereMachineV1Beta2Status groups all the fields that will be added or modified in VSphereMachineStatus with the V1Beta2 version.
// See https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more context.
type VSphereMachineV1Beta2Status struct {
	// conditions represents the observations of a VSphereMachine's current state.
	// Known condition types are Ready, VirtualMachineProvisioned and Paused.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vspheremachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this VSphereMachine belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="VSphereMachine instance ID"
// +kubebuilder:printcolumn:name="Machine",type="string",JSONPath=".metadata.ownerReferences[?(@.kind==\"Machine\")].name",description="Machine object which owns with this VSphereMachine",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Machine"

// VSphereMachine is the Schema for the vspheremachines API.
type VSphereMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VSphereMachineSpec   `json:"spec,omitempty"`
	Status VSphereMachineStatus `json:"status,omitempty"`
}

// GetConditions returns the conditions for a VSphereMachine.
func (m *VSphereMachine) GetConditions() clusterv1beta1.Conditions {
	return m.Status.Conditions
}

// SetConditions sets the conditions on a VSphereMachine.
func (m *VSphereMachine) SetConditions(conditions clusterv1beta1.Conditions) {
	m.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the set of conditions for this object.
func (m *VSphereMachine) GetV1Beta2Conditions() []metav1.Condition {
	if m.Status.V1Beta2 == nil {
		return nil
	}
	return m.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets conditions for an API object.
func (m *VSphereMachine) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if m.Status.V1Beta2 == nil {
		m.Status.V1Beta2 = &VSphereMachineV1Beta2Status{}
	}
	m.Status.V1Beta2.Conditions = conditions
}

// +kubebuilder:object:root=true

// VSphereMachineList contains a list of VSphereMachine.
type VSphereMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VSphereMachine `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &VSphereMachine{}, &VSphereMachineList{})
}
