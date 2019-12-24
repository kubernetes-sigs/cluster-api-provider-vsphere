/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha3

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// VMFinalizer allows the reconciler to clean up resources associated
	// with a VSphereVM before removing it from the API Server.
	VMFinalizer = "vspherevm.infrastructure.cluster.x-k8s.io"
)

// VSphereVMSpec defines the desired state of VSphereVM.
type VSphereVMSpec struct {
	// BootstrapRef is a reference to a bootstrap provider-specific resource
	// that holds configuration details.
	// This field is optional in case no bootstrap data is required to create
	// a VM.
	// +optional
	BootstrapRef *corev1.ObjectReference `json:"bootstrapRef"`

	// BiosUUID is the the VM's BIOS UUID that is assigned at runtime after
	// the VM has been created.
	// +optional
	BiosUUID string `json:"biosUUID,omitempty"`

	// Template is the name, inventory path, or instance UUID of the template
	// used to clone new VMs.
	Template string `json:"template"`

	// Server is the name of the vSphere server on which this VM is
	// created/located.
	Server string `json:"server"`

	// Datacenter is the name or inventory path of the datacenter where this
	// VM is created/located.
	Datacenter string `json:"datacenter"`

	// Folder is the name of inventory path of the folder where this VM is
	// located/created.
	Folder string `json:"folder"`

	// Datastore is the name of inventory path of the datastore where this VM is
	// located/created.
	Datastore string `json:"datastore"`

	// ResourcePool is the name of inventory path of the resource pool where
	// this VM is located/created.
	ResourcePool string `json:"resourcePool"`

	// Network is the network configuration for this VM.
	Network NetworkSpec `json:"network"`

	// NumCPUs is the number of virtual processors in a virtual machine.
	// Defaults to the analogue property value in the template from which this
	// VM is cloned.
	// +optional
	NumCPUs int32 `json:"numCPUs,omitempty"`
	// NumCoresPerSocket is the number of cores among which to distribute CPUs
	// in this virtual machine.
	// Defaults to the analogue property value in the template from which this
	// VM is cloned.
	// +optional
	NumCoresPerSocket int32 `json:"numCoresPerSocket,omitempty"`
	// MemoryMiB is the size of a virtual machine's memory, in MiB.
	// Defaults to the analogue property value in the template from which this
	// VM is cloned.
	// +optional
	MemoryMiB int64 `json:"memoryMiB,omitempty"`
	// DiskGiB is the size of a virtual machine's disk, in GiB.
	// Defaults to the analogue property value in the template from which this
	// VM is cloned.
	// +optional
	DiskGiB int32 `json:"diskGiB,omitempty"`
}

// VSphereVMStatus defines the observed state of VSphereVM
type VSphereVMStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// TaskRef is a managed object reference to a Task related to the machine.
	// This value is set automatically at runtime and should not be set or
	// modified by users.
	// +optional
	TaskRef string `json:"taskRef,omitempty"`

	// Network returns the network status for each of the machine's configured
	// network interfaces.
	// +optional
	Network []NetworkStatus `json:"networkStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vspherevms,scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// VSphereVM is the Schema for the vspherevms API
type VSphereVM struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VSphereVMSpec   `json:"spec,omitempty"`
	Status VSphereVMStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VSphereVMList contains a list of VSphereVM
type VSphereVMList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VSphereVM `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VSphereVM{}, &VSphereVMList{})
}
