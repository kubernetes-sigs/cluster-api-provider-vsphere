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

// VirtualMachineImageProductInfo describes product information for an image.
type VirtualMachineImageProductInfo struct {
	// +optional

	// FullVersion describes the long-form version of the image.
	FullVersion string `json:"fullVersion,omitempty"`
}

// VirtualMachineImageOSInfo describes the image's guest operating system.
type VirtualMachineImageOSInfo struct {
	// +optional

	// Type describes the operating system type.
	//
	// This value is also added to the image resource's labels as
	// VirtualMachineImageOSTypeLabel.
	Type string `json:"type,omitempty"`
}

// VirtualMachineImageSpec defines the desired state of VirtualMachineImage.
type VirtualMachineImageSpec struct {
	// +optional

	// ProviderRef is a reference to the resource that contains the source of
	// this image's information.
	ProviderRef *LocalObjectRef `json:"providerRef,omitempty"`
}

// VirtualMachineImageStatus defines the observed state of VirtualMachineImage.
type VirtualMachineImageStatus struct {
	// +optional

	// Name describes the display name of this image.
	Name string `json:"name,omitempty"`

	// +optional

	// OSInfo describes the observed operating system information for this
	// image.
	//
	// The OS information is also added to the image resource's labels. Please
	// refer to VirtualMachineImageOSInfo for more information.
	OSInfo VirtualMachineImageOSInfo `json:"osInfo,omitempty"`

	// +optional

	// ProductInfo describes the observed product information for this image.
	ProductInfo VirtualMachineImageProductInfo `json:"productInfo,omitempty"`

	// +optional

	// ProviderItemID describes the ID of the provider item that this image corresponds to.
	// If the provider of this image is a Content Library, this ID will be that of the
	// corresponding Content Library item.
	ProviderItemID string `json:"providerItemID,omitempty"`

	// +optional
	// +listType=map
	// +listMapKey=type

	// Conditions describes the observed conditions for this image.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=vmi;vmimage
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".status.name"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".status.type"
// +kubebuilder:printcolumn:name="Image Version",type="string",JSONPath=".status.productInfo.version"
// +kubebuilder:printcolumn:name="OS Name",type="string",JSONPath=".status.osInfo.type"
// +kubebuilder:printcolumn:name="OS Version",type="string",JSONPath=".status.osInfo.version"
// +kubebuilder:printcolumn:name="Hardware Version",type="string",JSONPath=".status.hardwareVersion"
// +kubebuilder:printcolumn:name="Capabilities",type="string",JSONPath=".status.capabilities"

// VirtualMachineImage is the schema for the virtualmachineimages API.
type VirtualMachineImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineImageSpec   `json:"spec,omitempty"`
	Status VirtualMachineImageStatus `json:"status,omitempty"`

	Source conversionmeta.SourceTypeMeta `json:"source,omitempty,omitzero"`
}

// GetConditions returns the set of conditions for this object.
func (in *VirtualMachineImage) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets conditions for an API object.
func (in *VirtualMachineImage) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetSource returns the Source for this object.
func (in *VirtualMachineImage) GetSource() conversionmeta.SourceTypeMeta {
	return in.Source
}

// SetSource sets Source for an API object.
func (in *VirtualMachineImage) SetSource(source conversionmeta.SourceTypeMeta) {
	in.Source = source
}

// +kubebuilder:object:root=true

// VirtualMachineImageList contains a list of VirtualMachineImage.
type VirtualMachineImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineImage `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &VirtualMachineImage{}, &VirtualMachineImageList{})
}
