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
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vcsimhelpers "sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/controllers/images"
)

// VMOperatorDependenciesSpec defines the desired state of the VMOperatorDependencies in
// the namespace where this object is created.
type VMOperatorDependenciesSpec struct {
	// OperatorRef provides a reference to the running instance of vm-operator.
	OperatorRef *VMOperatorRef `json:"operatorRef,omitempty"`

	// VCenter defines info about the vCenter instance that the vm-operator interacts with.
	// Only one between this field and VCenterSimulatorRef must be set.
	VCenter *VCenterSpec `json:"vCenter,omitempty"`

	// VCenterSimulatorRef defines info about the vCenter simulator instance that the vm-operator interacts with.
	// Only one between this field and VCenter must be set.
	VCenterSimulatorRef *NamespacedRef `json:"vCenterSimulatorRef,omitempty"`

	// StorageClasses defines a list of StorageClasses to be bound to the namespace where this object is created.
	StorageClasses []StorageClass `json:"storageClasses,omitempty"`

	// VirtualMachineClasses defines a list of VirtualMachineClasses to be bound to the namespace where this object is created.
	VirtualMachineClasses []VirtualMachineClass `json:"virtualMachineClasses,omitempty"`
}

// VMOperatorRef provide a reference to the running instance of vm-operator.
type VMOperatorRef struct {
	// Namespace where the vm-operator is running.
	Namespace string `json:"namespace,omitempty"`
}

// VCenterSpec defines info about the vCenter instance that the vm-operator interacts with.
type VCenterSpec struct {
	ServerURL  string `json:"serverURL,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	Thumbprint string `json:"thumbprint,omitempty"`

	// supervisor is based on a single vCenter cluster
	Datacenter     string               `json:"datacenter,omitempty"`
	Cluster        string               `json:"cluster,omitempty"`
	Folder         string               `json:"folder,omitempty"`
	ResourcePool   string               `json:"resourcePool,omitempty"`
	ContentLibrary ContentLibraryConfig `json:"contentLibrary,omitempty"`
	NetworkName    string               `json:"networkName,omitempty"`
}

type StorageClass struct {
	Name          string `json:"name,omitempty"`
	StoragePolicy string `json:"storagePolicy,omitempty"`
}

type VirtualMachineClass struct {
	Name   string            `json:"name,omitempty"`
	Cpus   int64             `json:"cpus,omitempty"`
	Memory resource.Quantity `json:"memory,omitempty"`
}

type ContentLibraryItemFilesConfig struct {
	Name    string `json:"name,omitempty"`
	Content []byte `json:"content,omitempty"`
	// TODO: ContentFrom a config map
}

type ContentLibraryItemConfig struct {
	Name        string                          `json:"datacenter,omitempty"`
	Files       []ContentLibraryItemFilesConfig `json:"files,omitempty"`
	ItemType    string                          `json:"itemType,omitempty"`
	ProductInfo string                          `json:"productInfo,omitempty"`
	OSInfo      string                          `json:"osInfo,omitempty"`
}

type ContentLibraryConfig struct {
	Name      string                     `json:"name,omitempty"`
	Datastore string                     `json:"datastore,omitempty"`
	Items     []ContentLibraryItemConfig `json:"items,omitempty"`
}

// VMOperatorDependenciesStatus defines the observed state of the VMOperatorDependencies.
type VMOperatorDependenciesStatus struct {
	Ready bool `json:"ready,omitempty"`
}

// +kubebuilder:resource:path=vmoperatordependencies,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:object:root=true

// VMOperatorDependencies is the schema for a VM operator dependencies.
type VMOperatorDependencies struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMOperatorDependenciesSpec   `json:"spec,omitempty"`
	Status VMOperatorDependenciesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VMOperatorDependenciesList contains a list of VMOperatorDependencies.
type VMOperatorDependenciesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VCenterSimulator `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &VMOperatorDependencies{}, &VMOperatorDependenciesList{})
}

// SetVCenterFromVCenterSimulator sets config.Spec.VCenter for a given VCenterSimulator.
// NOTE: by default it uses cluster DC0/C0, datastore LocalDS_0 in vcsim; it also sets up a
// content library with the templates that are expected by test cluster classes.
func (d *VMOperatorDependencies) SetVCenterFromVCenterSimulator(vCenterSimulator *VCenterSimulator) {
	datacenter := 0
	cluster := 0
	datastore := 0

	d.Spec.VCenter = &VCenterSpec{
		ServerURL:    vCenterSimulator.Status.Host,
		Username:     vCenterSimulator.Status.Username,
		Password:     vCenterSimulator.Status.Password,
		Thumbprint:   vCenterSimulator.Status.Thumbprint,
		Datacenter:   vcsimhelpers.DatacenterName(datacenter),
		Cluster:      vcsimhelpers.ClusterPath(datacenter, cluster),
		Folder:       vcsimhelpers.VMFolderName(datacenter),
		ResourcePool: vcsimhelpers.ResourcePoolPath(datacenter, cluster),
		ContentLibrary: ContentLibraryConfig{
			Name:      "vcsim",
			Datastore: vcsimhelpers.DatastorePath(datacenter, datastore),
			Items:     []ContentLibraryItemConfig{
				// Items are added right below this declaration
			},
		},
	}

	// Note: For the sake of testing with vcsim the template doesn't really matter (nor the version of K8s hosted on it)
	// but we must provide at least the templates that are expected by test cluster classes.
	for _, t := range vcsimhelpers.DefaultVMTemplates {
		d.Spec.VCenter.ContentLibrary.Items = append(d.Spec.VCenter.ContentLibrary.Items,
			ContentLibraryItemConfig{
				Name: t,
				Files: []ContentLibraryItemFilesConfig{
					{
						Name:    fmt.Sprintf("%s.ovf", t),
						Content: images.SampleOVF,
					},
				},
				ItemType:    "ovf",
				ProductInfo: "dummy-productInfo",
				OSInfo:      "dummy-OSInfo",
			},
		)
	}

	//  Add default storage and vm class for vcsim in not otherwise specified.
	if len(d.Spec.StorageClasses) == 0 {
		d.Spec.StorageClasses = []StorageClass{
			{
				Name:          "vcsim-default-storage-class",
				StoragePolicy: vcsimhelpers.DefaultStoragePolicyName,
			},
		}
	}
	if len(d.Spec.VirtualMachineClasses) == 0 {
		d.Spec.VirtualMachineClasses = []VirtualMachineClass{
			{
				Name:   "vcsim-default-vm-class",
				Cpus:   2,
				Memory: resource.MustParse("4G"),
			},
		}
	}
}
