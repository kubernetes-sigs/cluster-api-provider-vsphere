/*
Copyright 2018 The Kubernetes Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeadmv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VsphereMachineProviderConfig is the Schema for the vspheremachineproviderconfigs API
// +k8s:openapi-gen=true
type VsphereMachineProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	MachineRef        string             `json:"machineRef,omitempty"`
	MachineSpec       VsphereMachineSpec `json:"machineSpec,omitempty"`

	// KubeadmConfiguration holds the kubeadm configuration options
	// +optional
	KubeadmConfiguration KubeadmConfiguration `json:"kubeadmConfiguration,omitempty"`
}

// KubeadmConfiguration holds the various configurations that kubeadm uses
type KubeadmConfiguration struct {
	// JoinConfiguration is used to customize any kubeadm join configuration
	// parameters.
	Join kubeadmv1beta1.JoinConfiguration `json:"join,omitempty"`

	// InitConfiguration is used to customize any kubeadm init configuration
	// parameters.
	Init kubeadmv1beta1.InitConfiguration `json:"init,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VsphereMachineProviderConfigList contains a list of VsphereMachineProviderConfig
type VsphereMachineProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VsphereMachineProviderConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VsphereMachineProviderConfig{}, &VsphereMachineProviderConfigList{})
}

//**** New extensions

type VsphereMachineSpec struct {
	Datacenter       string      `json:"datacenter"`
	Datastore        string      `json:"datastore"`
	ResourcePool     string      `json:"resourcePool,omitempty"`
	VMFolder         string      `json:"vmFolder,omitempty"`
	Network          NetworkSpec `json:"network"`
	NumCPUs          int32       `json:"numCPUs,omitempty"`
	MemoryMB         int64       `json:"memoryMB,omitempty"`
	VMTemplate       string      `json:"template" yaml:"template"`
	Disks            []DiskSpec  `json:"disks"`
	DiskGiB          int32       `json:"diskGiB,omitempty"`
	Preloaded        bool        `json:"preloaded,omitempty"`
	VsphereCloudInit bool        `json:"vsphereCloudInit,omitempty"`
	TrustedCerts     []string    `json:"trustedCerts,omitempty"`
	NTPServers       []string    `json:"ntpServers,omitempty"`
}

// NetworkSpec defines the virtual machine's network configuration.
type NetworkSpec struct {
	// Devices is the list of network devices used by the virtual machine.
	Devices []NetworkDeviceSpec `json:"devices"`

	// Routes is a list of optional, static routes applied to the virtual
	// machine.
	// +optional
	Routes []NetworkRouteSpec `json:"routes,omitempty"`
}

// NetworkDeviceSpec defines the network configuration for a virtual machine's
// network device.
type NetworkDeviceSpec struct {
	// NetworkName is the name of the vSphere network to which the device
	// will be connected.
	NetworkName string `json:"networkName"`

	// DHCP4 is a flag that indicates whether or not to use DHCP for IPv4
	// on this device.
	// If true then IPAddrs should not contain any IPv4 addresses.
	// +optional
	DHCP4 bool `json:"dhcp4,omitempty"`

	// DHCP6 is a flag that indicates whether or not to use DHCP for IPv6
	// on this device.
	// If true then IPAddrs should not contain any IPv6 addresses.
	// +optional
	DHCP6 bool `json:"dhcp6,omitempty"`

	// Gateway4 is the IPv4 gateway used by this device.
	// Required when DHCP4 is false.
	// +optional
	Gateway4 string `json:"gateway4,omitempty"`

	// Gateway4 is the IPv4 gateway used by this device.
	// Required when DHCP6 is false.
	// +optional
	Gateway6 string `json:"gateway6,omitempty"`

	// IPAddrs is a list of one or more IPv4 and/or IPv6 addresses to assign
	// to this device.
	// Required when DHCP4 and DHCP6 are both false.
	// +optional
	IPAddrs []string `json:"ipAddrs,omitempty"`

	// MTU is the device’s Maximum Transmission Unit size in bytes.
	// +optional
	MTU *int64 `json:"mtu,omitempty"`

	// MACAddr is the MAC address used by this device.
	// It is generally a good idea to omit this field and allow a MAC address
	// to be generated.
	// Please note that this value must use the VMware OUI to work with the
	// in-tree vSphere cloud provider.
	// +optional
	MACAddr string `json:"macAddr,omitempty"`

	// Nameservers is a list of IPv4 and/or IPv6 addresses used as DNS
	// nameservers.
	// Please note that Linux allows only three nameservers (https://linux.die.net/man/5/resolv.conf).
	// +optional
	Nameservers []string `json:"nameservers,omitempty"`

	// Routes is a list of optional, static routes applied to the device.
	// +optional
	Routes []NetworkRouteSpec `json:"routes,omitempty"`

	// SearchDomains is a list of search domains used when resolving IP
	// addresses with DNS.
	// +optional
	SearchDomains []string `json:"searchDomains,omitempty"`
}

// NetworkRouteSpec defines a static network route.
type NetworkRouteSpec struct {
	// To is an IPv4 or IPv6 address.
	To string `json:"to"`
	// Via is an IPv4 or IPv6 address.
	Via string `json:"via"`
	// Metric is the weight/priority of the route.
	Metric int32 `json:"metric"`
}

type DiskSpec struct {
	DiskSizeGB int64  `json:"diskSizeGB,omitempty"`
	DiskLabel  string `json:"diskLabel,omitempty"`
}
