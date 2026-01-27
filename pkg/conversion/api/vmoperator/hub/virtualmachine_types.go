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
	// VirtualMachineConditionClassReady indicates that a referenced
	// VirtualMachineClass is ready.
	//
	// For more information please see VirtualMachineClass.Status.Ready.
	VirtualMachineConditionClassReady = "VirtualMachineClassReady"

	// VirtualMachineConditionImageReady indicates that a referenced
	// VirtualMachineImage is ready.
	//
	// For more information please see VirtualMachineImage.Status.Ready.
	VirtualMachineConditionImageReady = "VirtualMachineImageReady"

	// VirtualMachineConditionVMSetResourcePolicyReady indicates that a referenced
	// VirtualMachineSetResourcePolicy is Ready.
	VirtualMachineConditionVMSetResourcePolicyReady = "VirtualMachineConditionVMSetResourcePolicyReady"

	// VirtualMachineConditionStorageReady indicates that the storage prerequisites for the VM are ready.
	VirtualMachineConditionStorageReady = "VirtualMachineStorageReady"

	// VirtualMachineConditionBootstrapReady indicates that the bootstrap prerequisites for the VM are ready.
	VirtualMachineConditionBootstrapReady = "VirtualMachineBootstrapReady"

	// VirtualMachineConditionNetworkReady indicates that the network prerequisites for the VM are ready.
	VirtualMachineConditionNetworkReady = "VirtualMachineNetworkReady"

	// VirtualMachineConditionPlacementReady indicates that the placement decision for the VM is ready.
	VirtualMachineConditionPlacementReady = "VirtualMachineConditionPlacementReady"

	// VirtualMachineEncryptionSynced indicates that the VirtualMachine's
	// encryption state is synced to the desired encryption state.
	VirtualMachineEncryptionSynced = "VirtualMachineEncryptionSynced"

	// VirtualMachineDiskPromotionSynced indicates that the VirtualMachine's
	// disk promotion state is synced to the desired promotion state.
	VirtualMachineDiskPromotionSynced = "VirtualMachineDiskPromotionSynced"

	// VirtualMachineConditionCreated indicates that the VM has been created.
	VirtualMachineConditionCreated = "VirtualMachineCreated"

	// VirtualMachineClassConfigurationSynced indicates that the VM's current configuration is synced to the
	// current version of its VirtualMachineClass.
	VirtualMachineClassConfigurationSynced = "VirtualMachineClassConfigurationSynced"
)

// +kubebuilder:validation:Enum=PoweredOff;PoweredOn;Suspended

// VirtualMachinePowerState defines a VM's desired and observed power states.
type VirtualMachinePowerState string

const (
	// VirtualMachinePowerStateOff indicates to shut down a VM and/or it is
	// shut down.
	VirtualMachinePowerStateOff VirtualMachinePowerState = "PoweredOff"

	// VirtualMachinePowerStateOn indicates to power on a VM and/or it is
	// powered on.
	VirtualMachinePowerStateOn VirtualMachinePowerState = "PoweredOn"

	// VirtualMachinePowerStateSuspended indicates to suspend a VM and/or it is
	// suspended.
	VirtualMachinePowerStateSuspended VirtualMachinePowerState = "Suspended"
)

// +kubebuilder:validation:Enum=Hard;Soft;TrySoft

// VirtualMachinePowerOpMode represents the various power operation modes when
// powering off or suspending a VM.
type VirtualMachinePowerOpMode string

const (
	// VirtualMachinePowerOpModeHard indicates to halt a VM when powering it
	// off or when suspending a VM to not involve the guest.
	VirtualMachinePowerOpModeHard VirtualMachinePowerOpMode = "Hard"

	// VirtualMachinePowerOpModeSoft indicates to ask VM Tools running
	// inside of a VM's guest to shutdown the guest gracefully when powering
	// off a VM or when suspending a VM to allow the guest to participate.
	//
	// If this mode is set on a VM whose guest does not have VM Tools or if
	// VM Tools is present but the operation fails, the VM may never realize
	// the desired power state. This can prevent a VM from being deleted as well
	// as many other unexpected issues. It is recommended to use trySoft
	// instead.
	VirtualMachinePowerOpModeSoft VirtualMachinePowerOpMode = "Soft"

	// VirtualMachinePowerOpModeTrySoft indicates to first attempt a Soft
	// operation and fall back to Hard if VM Tools is not present in the guest,
	// if the Soft operation fails, or if the VM is not in the desired power
	// state within five minutes.
	VirtualMachinePowerOpModeTrySoft VirtualMachinePowerOpMode = "TrySoft"
)

// VirtualMachineSpec defines the desired state of a VirtualMachine.
type VirtualMachineSpec struct {
	// +optional

	// ImageName describes the name of the image resource used to deploy this
	// VM.
	//
	// This field may be used to specify the name of a VirtualMachineImage
	// or ClusterVirtualMachineImage resource. The resolver first checks to see
	// if there is a VirtualMachineImage with the specified name in the
	// same namespace as the VM being deployed. If no such resource exists, the
	// resolver then checks to see if there is a ClusterVirtualMachineImage
	// resource with the specified name.
	//
	// This field may also be used to specify the display name (vSphere name) of
	// a VirtualMachineImage or ClusterVirtualMachineImage resource. If the
	// display name unambiguously resolves to a distinct VM image (among all
	// existing VirtualMachineImages in the VM's namespace and all existing
	// ClusterVirtualMachineImages), then a mutation webhook updates the
	// spec.image field with the reference to the resolved VM image. If the
	// display name resolves to multiple or no VM images, then the mutation
	// webhook denies the request and returns an error.
	//
	// Please also note, when creating a new VirtualMachine, if this field and
	// spec.image are both non-empty, then they must refer to the same
	// resource or an error is returned.
	//
	// Please note, this field *may* be empty if the VM was imported instead of
	// deployed by VM Operator. An imported VirtualMachine resource references
	// an existing VM on the underlying platform that was not deployed from a
	// VM image.
	ImageName string `json:"imageName,omitempty"`

	// +optional

	// ClassName describes the name of the VirtualMachineClass resource used to
	// deploy this VM.
	//
	// When creating a virtual machine, if this field is empty and a
	// VirtualMachineClassInstance is specified in spec.class, then
	// this field is populated with the VirtualMachineClass object's
	// name.
	//
	// Please also note, when creating a new VirtualMachine, if this field and
	// spec.class are both non-empty, then they must refer to the same
	// VirtualMachineClass or an error is returned.
	//
	// Please note, this field *may* be empty if the VM was imported instead of
	// deployed by VM Operator. An imported VirtualMachine resource references
	// an existing VM on the underlying platform that was not deployed from a
	// VM class.
	//
	// If a VM is using a class, a different value in spec.className
	// leads to the VM being resized.
	ClassName string `json:"className,omitempty"`

	// +optional

	// Affinity describes the VM's scheduling constraints.
	Affinity *AffinitySpec `json:"affinity,omitempty"`

	// +optional

	// StorageClass describes the name of a Kubernetes StorageClass resource
	// used to configure this VM's storage-related attributes.
	//
	// Please see https://kubernetes.io/docs/concepts/storage/storage-classes/
	// for more information on Kubernetes storage classes.
	StorageClass string `json:"storageClass,omitempty"`

	// +optional

	// Bootstrap describes the desired state of the guest's bootstrap
	// configuration.
	//
	// If omitted, a default bootstrap method may be selected based on the
	// guest OS identifier. If Linux, then the LinuxPrep method is used.
	Bootstrap *VirtualMachineBootstrapSpec `json:"bootstrap,omitempty"`

	// +optional

	// Network describes the desired network configuration for the VM.
	//
	// Please note this value may be omitted entirely and the VM will be
	// assigned a single, virtual network interface that is connected to the
	// Namespace's default network.
	Network *VirtualMachineNetworkSpec `json:"network,omitempty"`

	// +optional

	// PowerState describes the desired power state of a VirtualMachine.
	//
	// Please note this field may be omitted when creating a new VM and will
	// default to "PoweredOn." However, once the field is set to a non-empty
	// value, it may no longer be set to an empty value.
	//
	// Additionally, setting this value to "Suspended" is not supported when
	// creating a new VM. The valid values when creating a new VM are
	// "PoweredOn" and "PoweredOff." An empty value is also allowed on create
	// since this value defaults to "PoweredOn" for new VMs.
	PowerState VirtualMachinePowerState `json:"powerState,omitempty"`

	// +optional
	// +kubebuilder:default=TrySoft

	// PowerOffMode describes the desired behavior when powering off a VM.
	//
	// There are three, supported power off modes: Hard, Soft, and
	// TrySoft. The first mode, Hard, is the equivalent of a physical
	// system's power cord being ripped from the wall. The Soft mode
	// requires the VM's guest to have VM Tools installed and attempts to
	// gracefully shutdown the VM. Its variant, TrySoft, first attempts
	// a graceful shutdown, and if that fails or the VM is not in a powered off
	// state after five minutes, the VM is halted.
	//
	// If omitted, the mode defaults to TrySoft.
	PowerOffMode VirtualMachinePowerOpMode `json:"powerOffMode,omitempty"`

	// +optional
	// +listType=map
	// +listMapKey=name

	// Volumes describes a list of volumes that can be mounted to the VM.
	Volumes []VirtualMachineVolume `json:"volumes,omitempty"`

	// +optional

	// ReadinessProbe describes a probe used to determine the VM's ready state.
	ReadinessProbe *VirtualMachineReadinessProbeSpec `json:"readinessProbe,omitempty"`

	// Reserved describes a set of VM configuration options reserved for system
	// use.
	//
	// Please note attempts to modify the value of this field by a DevOps user
	// will result in a validation error.
	Reserved *VirtualMachineReservedSpec `json:"reserved,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum=13

	// MinHardwareVersion describes the desired, minimum hardware version.
	//
	// The logic that determines the hardware version is as follows:
	//
	// 1. If this field is set, then its value is used.
	// 2. Otherwise, if the VirtualMachineClass used to deploy the VM contains a
	//    non-empty hardware version, then it is used.
	// 3. Finally, if the hardware version is still undetermined, the value is
	//    set to the default hardware version for the Datacenter/Cluster/Host
	//    where the VM is provisioned.
	//
	// This field is never updated to reflect the derived hardware version.
	// Instead, VirtualMachineStatus.HardwareVersion surfaces
	// the observed hardware version.
	//
	// Please note, setting this field's value to N ensures a VM's hardware
	// version is equal to or greater than N. For example, if a VM's observed
	// hardware version is 10 and this field's value is 13, then the VM will be
	// upgraded to hardware version 13. However, if the observed hardware
	// version is 17 and this field's value is 13, no change will occur.
	//
	// Several features are hardware version dependent, for example:
	//
	// * NVMe Controllers                >= 14
	// * Dynamic Direct Path I/O devices >= 17
	//
	// Please refer to https://kb.vmware.com/s/article/1003746 for a list of VM
	// hardware versions.
	//
	// It is important to remember that a VM's hardware version may not be
	// downgraded and upgrading a VM deployed from an image based on an older
	// hardware version to a more recent one may result in unpredictable
	// behavior. In other words, please be careful when choosing to upgrade a
	// VM to a newer hardware version.
	MinHardwareVersion int32 `json:"minHardwareVersion,omitempty"`

	// +optional

	// GroupName indicates the name of the VirtualMachineGroup to which this
	// VM belongs.
	//
	// VMs that belong to a group do not drive their own placement, rather that
	// is handled by the group.
	//
	// When this field is set to a valid group that contains this VM as a
	// member, an owner reference to that group is added to this VM.
	//
	// When this field is deleted or changed, any existing owner reference to
	// the previous group will be removed from this VM.
	GroupName string `json:"groupName,omitempty"`
}

// VirtualMachineReservedSpec describes a set of VM configuration options
// reserved for system use. Modification attempts by DevOps users will result
// in a validation error.
type VirtualMachineReservedSpec struct {
	// +optional

	// ResourcePolicyName describes the name of a
	// VirtualMachineSetResourcePolicy resource used to configure the VM's

	ResourcePolicyName string `json:"resourcePolicyName,omitempty"`
}

// VirtualMachineStatus defines the observed state of a VirtualMachine instance.
type VirtualMachineStatus struct {
	// +optional

	// NodeName describes the observed name of the node where the VirtualMachine
	// is scheduled.
	NodeName string `json:"nodeName,omitempty"`

	// +optional

	// PowerState describes the observed power state of the VirtualMachine.
	PowerState VirtualMachinePowerState `json:"powerState,omitempty"`

	// +optional

	// Conditions describes the observed conditions of the VirtualMachine.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional

	// Network describes the observed state of the VM's network configuration.
	// Please note much of the network status information is only available if
	// the guest has VM Tools installed.
	Network *VirtualMachineNetworkStatus `json:"network,omitempty"`

	// +optional

	// BiosUUID describes a unique identifier provided by the underlying
	// infrastructure provider that is exposed to the Guest OS BIOS as a unique
	// hardware identifier.
	BiosUUID string `json:"biosUUID,omitempty"`

	// +optional

	// Zone describes the availability zone where the VirtualMachine has been
	// scheduled.
	//
	// Please note this field may be empty when the cluster is not zone-aware.
	Zone string `json:"zone,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=vm
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Power-State",type="string",JSONPath=".status.powerState"
// +kubebuilder:printcolumn:name="Class",type="string",priority=1,JSONPath=".spec.className"
// +kubebuilder:printcolumn:name="Image",type="string",priority=1,JSONPath=".spec.image.name"
// +kubebuilder:printcolumn:name="Primary-IP4",type="string",priority=1,JSONPath=".status.network.primaryIP4"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VirtualMachine is the schema for the virtualmachines API and represents the
// desired state and observed status of a virtualmachines resource.
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineSpec   `json:"spec,omitempty"`
	Status VirtualMachineStatus `json:"status,omitempty"`

	Source conversionmeta.SourceTypeMeta `json:"source,omitempty,omitzero"`
}

// GetConditions returns the set of conditions for this object.
func (vm VirtualMachine) GetConditions() []metav1.Condition {
	return vm.Status.Conditions
}

// SetConditions sets conditions for an API object.
func (vm *VirtualMachine) SetConditions(conditions []metav1.Condition) {
	vm.Status.Conditions = conditions
}

// GetSource returns the Source for this object.
func (vm *VirtualMachine) GetSource() conversionmeta.SourceTypeMeta {
	return vm.Source
}

// SetSource sets Source for an API object.
func (vm *VirtualMachine) SetSource(source conversionmeta.SourceTypeMeta) {
	vm.Source = source
}

// +kubebuilder:object:root=true

// VirtualMachineList contains a list of VirtualMachine.
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachine `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &VirtualMachine{}, &VirtualMachineList{})
}
