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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/api/deprecated/errors"
)

// VolumeApplicationType describes the type of application the volume is intended for
// +kubebuilder:validation:Enum=OracleRAC;MicrosoftWSFC
type VolumeApplicationType string

// Supported values for VolumeApplicationType.
const (
	VolumeApplicationTypeOracleRAC     VolumeApplicationType = "OracleRAC"
	VolumeApplicationTypeMicrosoftWSFC VolumeApplicationType = "MicrosoftWSFC"
)

// VirtualControllerType describe the type of controller the volume will be attached to
// +kubebuilder:validation:Enum=IDE;NVME;SCSI;SATA
type VirtualControllerType string

// Supported values for VirtualControllerType.
const (
	VirtualControllerTypeIDE  VirtualControllerType = "IDE"
	VirtualControllerTypeNVME VirtualControllerType = "NVME"
	VirtualControllerTypeSCSI VirtualControllerType = "SCSI"
	VirtualControllerTypeSATA VirtualControllerType = "SATA"
)

// VolumeDiskMode describes whether the disk is persistent/independent
// +kubebuilder:validation:Enum=IndependentNonPersistent;IndependentPersistent;NonPersistent;Persistent
type VolumeDiskMode string

// Supported values for VolumeDiskMode.
const (
	VolumeDiskModeIndependentNonPersistent VolumeDiskMode = "IndependentNonPersistent"
	VolumeDiskModeIndependentPersistent    VolumeDiskMode = "IndependentPersistent"
	VolumeDiskModeNonPersistent            VolumeDiskMode = "NonPersistent"
	VolumeDiskModePersistent               VolumeDiskMode = "Persistent"
)

// VolumeSharingMode describes whether the volume is shared with multiple writers or is not shared
// +kubebuilder:validation:Enum=MultiWriter;None
type VolumeSharingMode string

// Supported values for VolumeSharingMode.
const (
	VolumeSharingModeMultiWriter VolumeSharingMode = "MultiWriter"
	VolumeSharingModeNone        VolumeSharingMode = "None"
)

// VSphereMachineVolume defines a PVC attachment.
type VSphereMachineVolume struct {
	// name is suffix used to name this PVC as: VSphereMachine.Name + "-" + Name
	Name string `json:"name"`
	// capacity is the PVC capacity
	Capacity corev1.ResourceList `json:"capacity"`
	// storageClass defaults to VSphereMachineSpec.StorageClass
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
	// applicationType describes the type of application for which this volume
	// is intended to be used.
	//
	//   - OracleRAC      -- The volume is configured with
	//                       diskMode=IndependentPersistent and
	//                       sharingMode=MultiWriter and attached to the first
	//                       SCSI controller with an available slot and
	//                       sharingMode=None. If no such controller exists,
	//                       a new ParaVirtual SCSI controller will be created
	//                       with sharingMode=None long as there are currently
	//                       three or fewer SCSI controllers.
	//   - MicrosoftWSFC  -- The volume is configured with
	//                       diskMode=IndependentPersistent and attached to a
	//                       SCSI controller with sharingMode=Physical.
	//                       If no such controller exists, a new ParaVirtual
	//                       SCSI controller will be created with
	//                       sharingMode=Physical as long as there are currently
	//                       three or fewer SCSI controllers.
	// +optional
	ApplicationType VolumeApplicationType `json:"applicationType,omitempty"`
	// controllerType is the type of the controller to which this volume
	// should be attached.
	//
	// Please keep in mind the number of volumes supported by the different
	// types of controllers:
	//
	//   - IDE                -- 4 total (2 per controller)
	//   - NVME               -- 256 total (64 per controller)
	//   - SATA               -- 120 total (30 per controller)
	//   - SCSI (ParaVirtual) -- 256 total (64 per controller)
	//   - SCSI (BusLogic)    -- 60 total (15 per controller)
	//   - SCSI (LsiLogic)    -- 60 total (15 per controller)
	//   - SCSI (LsiLogicSAS) -- 60 total (15 per controller)
	//
	// Please note, the number of supported volumes per SCSI controller may seem
	// off, but remember that a SCSI controller occupies a slot on its own bus.
	// Thus even though a ParaVirtual SCSI controller supports 65 targets and
	// the other types of SCSI controllers support 16 targets, one of the
	// targets is occupied by the controller itself.
	//
	// Defaults to SCSI when controllerBusNumber is also omitted; otherwise the
	// default value is determined by the logic outlined in the description of
	// the controllerBusNumber field.
	// +optional
	ControllerType VirtualControllerType `json:"controllerType,omitempty"`
	// controllerBusNumber is the bus number of the controller to which this
	// volume should be attached. The bus number identifies a controller of the
	// type given by controllerType (min and max vary depending on bus type).
	//
	// If this and controllerType are both omitted, the volume will be attached
	// to the first available SCSI controller. If there is no SCSI controller
	// with an available slot, a new ParaVirtual SCSI controller will be added
	// as long as there are currently three or fewer SCSI controllers.
	//
	// If the specified controller has no available slots, the request will be
	// denied.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ControllerBusNumber *int32 `json:"controllerBusNumber,omitempty"`
	// diskMode is the desired mode to use when attaching the volume.
	//
	// Please note, volumes attached as IndependentNonPersistent or
	// IndependentPersistent are not included in a VM's snapshots or backups.
	//
	// Also, any data written to volumes attached as IndependentNonPersistent or
	// NonPersistent will be discarded when the VM is powered off.
	//
	// When applicationType=OracleRAC or applicationType=MicrosoftWSFC, this
	// field defaults to IndependentPersistent.
	// Otherwise, defaults to Persistent.
	// +optional
	DiskMode VolumeDiskMode `json:"diskMode,omitempty"`
	// sharingMode is the volume's desired sharing mode.
	//
	// When applicationType=OracleRAC, this field defaults to MultiWriter.
	// Otherwise, defaults to None.
	// +optional
	SharingMode VolumeSharingMode `json:"sharingMode,omitempty"`
	// unitNumber is the desired unit number for attaching the volume to a
	// storage controller.
	//
	// When omitted, the next available unit number of the selected controller
	// is used.
	//
	// This value must be unique for the controller referenced by the
	// controllerBusNumber and controllerType properties. If the value is
	// already used by another device, this volume will not be attached.
	//
	// Please note the value 7 is invalid if controllerType=SCSI as 7 is the
	// unit number of the SCSI controller on its own bus.
	// +optional
	// +kubebuilder:validation:Minimum=0
	UnitNumber *int32 `json:"unitNumber,omitempty"`
	// removable describes whether or not this volume may be removed from
	// spec.volumes. This is a safety measure that allows users to prevent the
	// accidental removal of disks from a VM that should not be removed.
	//
	// Defaults to true.
	// +optional
	Removable *bool `json:"removable,omitempty"`
	// readOnly attaches the volume read-only
	// Defaults to false
	// +optional
	ReadOnly *bool `json:"readOnly,omitempty"`
}

// VSphereMachineSpec defines the desired state of VSphereMachine.
type VSphereMachineSpec struct {
	// providerID is the virtual machine's BIOS UUID formatted as
	// vsphere://12345678-1234-1234-1234-123456789abc.
	// This is required at runtime by CAPI. Do not remove this field.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// failureDomain is the failure domain the machine will be created in.
	// Must match a key in the FailureDomains map stored on the cluster object.
	// +optional
	FailureDomain *string `json:"failureDomain,omitempty"`

	// imageName is the name of the base image used when specifying the
	// underlying virtual machine
	ImageName string `json:"imageName"`

	// className is the name of the class used when specifying the underlying
	// virtual machine
	ClassName string `json:"className"`

	// storageClass is the name of the storage class used when specifying the
	// underlying virtual machine.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// volumes is the set of PVCs to be created and attached to the VSphereMachine
	// +optional
	Volumes []VSphereMachineVolume `json:"volumes,omitempty"`

	// policies specifies a list of optional infrastructure policies to be applied to the virtual machine.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	Policies []PolicyRef `json:"policies,omitempty"`

	// network is the network configuration for the VSphereMachine
	// +optional
	Network VSphereMachineNetworkSpec `json:"network,omitempty,omitzero"`

	// powerOffMode describes the desired behavior when powering off a VM.
	//
	// There are three, supported power off modes: hard, soft, and
	// trySoft. The first mode, hard, is the equivalent of a physical
	// system's power cord being ripped from the wall. The soft mode
	// requires the VM's guest to have VM Tools installed and attempts to
	// gracefully shut down the VM. Its variant, trySoft, first attempts
	// a graceful shutdown, and if that fails or the VM is not in a powered off
	// state after reaching 5 minutes timeout, the VM is halted.
	//
	// If omitted, the mode defaults to hard.
	//
	// +optional
	// +kubebuilder:default=hard
	PowerOffMode VirtualMachinePowerOpMode `json:"powerOffMode,omitempty"`

	// minHardwareVersion specifies the desired minimum hardware version
	// for this VM. Setting this field will ensure that the hardware version
	// of the VM is at least set to the specified value.
	// The expected format of the field is vmx-15.
	//
	// +optional
	MinHardwareVersion string `json:"minHardwareVersion,omitempty"`

	// namingStrategy allows configuring the naming strategy used when calculating the name of the VirtualMachine.
	// +optional
	NamingStrategy *VirtualMachineNamingStrategy `json:"namingStrategy,omitempty"`

	// affinity describes the VSphereMachine's scheduling constraints and placement rules (e.g. anti-affinity
	// with other VSphereMachines).
	// Set on the VSphereMachineTemplate machine spec for the MachineDeployment, propagated
	// to each VSphereMachine.
	// +optional
	Affinity AffinitySpec `json:"affinity,omitempty,omitzero"`
}

// AffinitySpec defines the group of affinity scheduling rules.
type AffinitySpec struct {
	// vmAntiAffinity describes anti-affinity scheduling rules related to other
	// VSphereMachines.
	// +optional
	VMAntiAffinity VMAntiAffinitySpec `json:"vmAntiAffinity,omitempty,omitzero"`
}

// IsDefined returns true if the AffinitySpec is defined.
func (r *AffinitySpec) IsDefined() bool {
	return !reflect.DeepEqual(r, &AffinitySpec{})
}

// VMAntiAffinitySpec defines the anti-affinity requirements for scheduling
// rules related to other VSphereMachines.
type VMAntiAffinitySpec struct {
	// preferredDuringSchedulingPreferredDuringExecution describes anti-affinity
	// requirements that should be met, but the VSphereMachine can still be scheduled if
	// the requirement cannot be satisfied. The scheduler will prefer to
	// schedule VSphereMachines that satisfy the anti-affinity expressions specified by
	// this field, but it may choose to violate one or more of the expressions.
	// Additionally, it also describes the anti-affinity requirements that
	// should be met during run-time, but the VSphereMachine can still be run if the
	// requirements cannot be satisfied.
	//
	// When there are multiple elements, the lists of nodes corresponding to
	// each term are intersected, i.e. all terms must be satisfied.
	//
	// Note: Any update to this field will replace the entire list rather than
	// merging with the existing elements.
	// +optional
	// +listType=atomic
	PreferredDuringSchedulingPreferredDuringExecution *[]VMAntiAffinityTerm `json:"preferredDuringSchedulingPreferredDuringExecution,omitempty"`

	// requiredDuringSchedulingPreferredDuringExecution describes anti-affinity
	// requirements that must be met or the VSphereMachine will not be scheduled.
	//
	// When there are multiple elements, the lists of nodes corresponding to
	// each term are intersected, i.e. all terms must be satisfied.
	//
	// Note: Any update to this field will replace the entire list rather than
	// merging with the existing elements.
	// +optional
	// +listType=atomic
	RequiredDuringSchedulingPreferredDuringExecution *[]VMAntiAffinityTerm `json:"requiredDuringSchedulingPreferredDuringExecution,omitempty"`

	// requiredDuringSchedulingRequiredDuringExecution describes anti-affinity
	// requirements that must be met or the VSphereMachine will not be scheduled and
	// that must continue to be met.
	//
	// When there are multiple elements, the lists of nodes corresponding to
	// each term are intersected, i.e. all terms must be satisfied.
	//
	// Note: Any update to this field will replace the entire list rather than
	// merging with the existing elements.
	// +optional
	// +listType=atomic
	RequiredDuringSchedulingRequiredDuringExecution *[]VMAntiAffinityTerm `json:"requiredDuringSchedulingRequiredDuringExecution,omitempty"`
}

// VMAntiAffinityTerm defines the VSphereMachine anti-affinity term.
type VMAntiAffinityTerm struct {
	// labelSelector is a label query over a set of VSphereMachines.
	// An empty selector matches all VSphereMachines in the applicable scope.
	// +required
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// topologyKey describes where this VSphereMachine should not be co-located
	// with the selected VSphereMachines.
	// Only `kubernetes.io/hostname` is currently supported.
	// The rule is executed in the context of hosts.
	// +required
	// +kubebuilder:validation:Enum=kubernetes.io/hostname
	TopologyKey string `json:"topologyKey,omitempty"`
}

// PolicyRef identifies an optional infrastructure policy to specify for the virtual machine by name and kind.
type PolicyRef struct {
	// name of the infrastructure policy.
	// name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name,omitempty"`

	// kind of the infrastructure policy object being referenced.
	// kind must be a CamelCase Kubernetes Kind starting with an uppercase letter (e.g. ComputePolicy).
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z]([-a-zA-Z0-9]*[a-zA-Z0-9])?$`
	Kind string `json:"kind,omitempty"`
}

// VSphereMachineNetworkSpec defines the network configuration of a VSphereMachine.
// +kubebuilder:validation:MinProperties=1
type VSphereMachineNetworkSpec struct {
	// interfaces is the list of network interfaces attached to this VSphereMachine.
	//
	// +optional
	Interfaces InterfacesSpec `json:"interfaces,omitempty,omitzero"`

	// vlans is a list of VLAN sub-interfaces to be configured on the secondary
	// network interfaces. Each VLAN is linked to a specific secondary interface
	// via the link field.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +listType=atomic
	// +optional
	VLANs []VLANSpec `json:"vlans,omitempty"`
}

// IsDefined returns true if the VSphereMachineNetworkSpec is defined.
func (r *VSphereMachineNetworkSpec) IsDefined() bool {
	return !reflect.DeepEqual(r, &VSphereMachineNetworkSpec{})
}

// InterfacesSpec defines all the network interfaces of a VSphereMachine from Kubernetes perspective.
// +kubebuilder:validation:MinProperties=1
type InterfacesSpec struct {
	// primary is the primary network interface.
	//
	// It is used to connect the Kubernetes primary network for Load balancer,
	// Service discovery, Pod traffic and management traffic etc.
	// Leave it unset if you don't want to customize the primary network and interface.
	// Customization is only supported with network provider NSX-VPC.
	// It should be set only when VSphereCluster spec.network.nsxVPC.createSubnetSet is set to false.
	//
	// +optional
	Primary InterfaceSpec `json:"primary,omitempty,omitzero"`

	// secondary are the secondary network interfaces.
	//
	// It is used for any purpose like deploying Antrea secondary network,
	// Multus, mounting NFS etc.
	// Secondary network is supported with network provider NSX-VPC and vsphere-network.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=9
	// +listType=atomic
	// +optional
	Secondary []SecondaryInterfaceSpec `json:"secondary,omitempty"`
}

// IsDefined returns true if the InterfacesSpec is defined.
func (r *InterfacesSpec) IsDefined() bool {
	return !reflect.DeepEqual(r, &InterfacesSpec{})
}

// SecondaryInterfaceSpec defines a secondary network interface for a VSphereMachine.
type SecondaryInterfaceSpec struct {
	// name describes the unique name of this network interface, used to
	// distinguish it from other network interfaces attached to this VSphereMachine.
	//
	// +kubebuilder:validation:Pattern="^[a-z0-9]{2,}$"
	// +kubebuilder:validation:MinLength=2
	// +kubebuilder:validation:MaxLength=15
	// +required
	Name string `json:"name,omitempty"`

	InterfaceSpec `json:",inline"`
}

// InterfaceSpec defines properties of a network interface.
type InterfaceSpec struct {
	// network is the name of the network resource to which this interface is
	// connected.
	// +required
	Network InterfaceNetworkReference `json:"network,omitempty,omitzero"`

	// mtu is the Maximum Transmission Unit size in bytes.
	//
	// +kubebuilder:validation:Minimum=576
	// +kubebuilder:validation:Maximum=9000
	// +optional
	MTU int32 `json:"mtu,omitempty"`

	// routes is a list of optional, static routes.
	//
	// Please note this feature is available only with the following bootstrap
	// providers: CloudInit.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=100
	// +listType=atomic
	// +optional
	Routes []RouteSpec `json:"routes,omitempty"`
}

// IsDefined returns true if the InterfaceSpec is defined.
func (r *InterfaceSpec) IsDefined() bool {
	return !reflect.DeepEqual(r, &InterfaceSpec{})
}

// InterfaceNetworkReference describes a reference to another object in the same
// namespace as the referrer.
type InterfaceNetworkReference struct {
	// kind of the remediation template.
	// kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z]([-a-zA-Z0-9]*[a-zA-Z0-9])?$`
	Kind string `json:"kind,omitempty"`

	// name of the remediation template.
	// name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name,omitempty"`

	// apiVersion of the remediation template.
	// apiVersion must be fully qualified domain name followed by / and a version.
	// NOTE: This field must be kept in sync with the APIVersion of the remediation template.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=317
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/[a-z]([-a-z0-9]*[a-z0-9])?$`
	APIVersion string `json:"apiVersion,omitempty"`
}

// GroupVersionKind gets the GroupVersionKind for an InterfaceNetworkReference.
func (r *InterfaceNetworkReference) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(r.APIVersion, r.Kind)
}

// RouteSpec defines a static route for a guest.
type RouteSpec struct {
	// to is an IP4 CIDR. IP6 is not supported yet.
	// Examples: 192.168.1.0/24, 192.168.100.100/32, 0.0.0.0/0
	//
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}\/[0-9]{1,2}$`
	// +kubebuilder:validation:MinLength=9
	// +kubebuilder:validation:MaxLength=18
	// +required
	To string `json:"to,omitempty"`

	// via is an IP4 address. IP6 is not supported yet.
	//
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	// +kubebuilder:validation:MinLength=7
	// +kubebuilder:validation:MaxLength=15
	// +required
	Via string `json:"via,omitempty"`
}

// VirtualMachineNamingStrategy defines the naming strategy for the VirtualMachines.
type VirtualMachineNamingStrategy struct {
	// template defines the template to use for generating the name of the VirtualMachine object.
	// If not defined, it will fall back to `{{ .machine.name }}`.
	// The templating has the following data available:
	// * `.machine.name`: The name of the Machine object.
	// The templating also has the following funcs available:
	// * `trimSuffix`: same as strings.TrimSuffix
	// * `trunc`: truncates a string, e.g. `trunc 2 "hello"` or `trunc -2 "hello"`
	// Notes:
	// * While the template offers some flexibility, we would like the name to link to the Machine name
	//   to ensure better user experience when troubleshooting
	// * Generated names must be valid Kubernetes names as they are used to create a VirtualMachine object
	//   and usually also as the name of the Node object.
	// * Names are automatically truncated at 63 characters. Please note that this can lead to name conflicts,
	//   so we highly recommend to use a template which leads to a name shorter than 63 characters.
	// +optional
	Template *string `json:"template,omitempty"`
}

// VSphereMachineStatus defines the observed state of VSphereMachine.
type VSphereMachineStatus struct {
	// ready is true when the provider resource is ready.
	// This is required at runtime by CAPI. Do not remove this field.
	// +optional
	Ready bool `json:"ready"`

	// addresses contains the instance associated addresses.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +listType=atomic
	// +optional
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

	// vmID is used to identify the virtual machine.
	// +optional
	ID *string `json:"vmID,omitempty"`

	// vmIp is the IP address used to access the virtual machine.
	// +optional
	IPAddr string `json:"vmIp,omitempty"`

	// failureReason will be set in the event that there is a terminal problem
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

	// failureMessage will be set in the event that there is a terminal problem
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

	// vmstatus is used to identify the virtual machine status.
	// +optional
	VMStatus VirtualMachineState `json:"vmstatus,omitempty"`

	// conditions defines current service state of the VSphereMachine.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in VSphereMachine's status with the V1Beta2 version.
	// +optional
	V1Beta2 *VSphereMachineV1Beta2Status `json:"v1beta2,omitempty"`

	// network describes the observed state of the VM's network configuration.
	// Please note much of the network status information is only available if
	// the guest has VM Tools installed.
	// +optional
	Network VSphereMachineNetworkStatus `json:"network,omitempty,omitzero"`
}

// VLANSpec defines a VLAN sub-interface configuration linked to a secondary interface.
type VLANSpec struct {
	// name is the name of the VLAN sub-interface as it appears inside
	// the guest operating system.
	//
	// The name must conform to Linux network interface naming rules:
	// it must be between 1 and 15 characters long, start with an
	// alphanumeric character, and contain only alphanumeric characters,
	// hyphens, underscores, or dots.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern="^[a-zA-Z0-9][a-zA-Z0-9._-]*$"
	Name string `json:"name,omitempty"`

	// id is the VLAN ID used to tag traffic on this sub-interface,
	// a number between 0 and 4094.
	//
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=4094
	ID *int32 `json:"id,omitempty"`

	// link is the name of the secondary interface this VLAN is associated with.
	//
	// +required
	// +kubebuilder:validation:Pattern="^[a-z0-9]{2,}$"
	// +kubebuilder:validation:MinLength=2
	// +kubebuilder:validation:MaxLength=15
	Link string `json:"link,omitempty"`
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

// VSphereMachine is the Schema for the vspheremachines API
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vspheremachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:deprecatedversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Zone",type="string",JSONPath=".spec.failureDomain",description="Zone"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="IPAddr",type="string",JSONPath=".status.vmIp",description="IP address"
// +kubebuilder:validation:XValidation:rule="has(self.spec.policies) == has(oldSelf.spec.policies) && (!has(self.spec.policies) || self.spec.policies == oldSelf.spec.policies)",message="policies are immutable after creation"
type VSphereMachine struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of VSphereMachine.
	Spec VSphereMachineSpec `json:"spec,omitempty"`

	// status is the observed state of VSphereMachine.
	Status VSphereMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VSphereMachineList contains a list of VSphereMachine.
type VSphereMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VSphereMachine `json:"items"`
}

// GetConditions returns the conditions for the VSphereMachine.
func (r *VSphereMachine) GetConditions() clusterv1beta1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets conditions on the VSphereMachine.
func (r *VSphereMachine) SetConditions(conditions clusterv1beta1.Conditions) {
	r.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the set of conditions for this object.
func (r *VSphereMachine) GetV1Beta2Conditions() []metav1.Condition {
	if r.Status.V1Beta2 == nil {
		return nil
	}
	return r.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets conditions for an API object.
func (r *VSphereMachine) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if r.Status.V1Beta2 == nil {
		r.Status.V1Beta2 = &VSphereMachineV1Beta2Status{}
	}
	r.Status.V1Beta2.Conditions = conditions
}

func init() {
	objectTypes = append(objectTypes, &VSphereMachine{}, &VSphereMachineList{})
}
