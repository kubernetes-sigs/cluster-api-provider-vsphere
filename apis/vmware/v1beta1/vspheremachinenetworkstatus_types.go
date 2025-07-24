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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// VSphereMachineNetworkDNSStatus describes the observed state of the guest's
// RFC 1034 client-side DNS settings.
type VSphereMachineNetworkDNSStatus struct {
	// DHCP indicates whether or not dynamic host control protocol (DHCP) was
	// used to configure DNS configuration.
	//
	// +optional
	DHCP bool `json:"dhcp,omitempty"`

	// DomainName is the domain name portion of the DNS name. For example,
	// the "domain.local" part of "my-vm.domain.local".
	//
	// +optional
	DomainName string `json:"domainName,omitempty"`

	// HostName is the host name portion of the DNS name. For example,
	// the "my-vm" part of "my-vm.domain.local".
	//
	// +optional
	HostName string `json:"hostName,omitempty"`

	// Nameservers is a list of the IP addresses for the DNS servers to use.
	//
	// IP4 addresses are specified using dotted decimal notation. For example,
	// "192.0.2.1".
	//
	// IP6 addresses are 128-bit addresses represented as eight fields of up to
	// four hexadecimal digits. A colon separates each field (:). For example,
	// 2001:DB8:101::230:6eff:fe04:d9ff. The address can also consist of the
	// symbol '::' to represent multiple 16-bit groups of contiguous 0's only
	// once in an address as described in RFC 2373.
	//
	// +optional
	Nameservers []string `json:"nameservers,omitempty"`

	// SearchDomains is a list of domains in which to search for hosts, in the
	// order of preference.
	//
	// +optional
	SearchDomains []string `json:"searchDomains,omitempty"`
}

// KeyValuePair is useful when wanting to realize a map as a list of key/value
// pairs.
type KeyValuePair struct {
	// Key is the key part of the key/value pair.
	Key string `json:"key"`
	// Value is the optional value part of the key/value pair.
	// +optional
	Value string `json:"value,omitempty"`
}

// VSphereMachineNetworkDHCPOptionsStatus describes the observed state of
// DHCP options.
type VSphereMachineNetworkDHCPOptionsStatus struct {
	// Config describes platform-dependent settings for the DHCP client.
	//
	// The key part is a unique number while the value part is the platform
	// specific configuration command. For example on Linux and BSD systems
	// using the file dhclient.conf output would be reported at system scope:
	// key='1', value='timeout 60;' key='2', value='reboot 10;'. The output
	// reported per interface would be:
	// key='1', value='prepend domain-name-servers 192.0.2.1;'
	// key='2', value='require subnet-mask, domain-name-servers;'.
	//
	// +optional
	// +listType=map
	// +listMapKey=key
	Config []KeyValuePair `json:"config,omitempty"`

	// Enabled reports the status of the DHCP client services.
	// +omitempty
	Enabled bool `json:"enabled,omitempty"`
}

// VSphereMachineNetworkDHCPStatus describes the observed state of the
// client-side, system-wide DHCP settings for IP4 and IP6.
type VSphereMachineNetworkDHCPStatus struct {

	// IP4 describes the observed state of the IP4 DHCP client settings.
	//
	// +optional
	IP4 VSphereMachineNetworkDHCPOptionsStatus `json:"ip4,omitempty"`

	// IP6 describes the observed state of the IP6 DHCP client settings.
	//
	// +optional
	IP6 VSphereMachineNetworkDHCPOptionsStatus `json:"ip6,omitempty"`
}

// VSphereMachineNetworkInterfaceIPAddrStatus describes information about a
// specific IP address.
type VSphereMachineNetworkInterfaceIPAddrStatus struct {
	// Address is an IP4 or IP6 address and their network prefix length.
	//
	// An IP4 address is specified using dotted decimal notation. For example,
	// "192.0.2.1".
	//
	// IP6 addresses are 128-bit addresses represented as eight fields of up to
	// four hexadecimal digits. A colon separates each field (:). For example,
	// 2001:DB8:101::230:6eff:fe04:d9ff. The address can also consist of the
	// symbol '::' to represent multiple 16-bit groups of contiguous 0's only
	// once in an address as described in RFC 2373.
	Address string `json:"address"`

	// Lifetime describes when this address will expire.
	//
	// +optional
	Lifetime metav1.Time `json:"lifetime,omitempty"`

	// Origin describes how this address was configured.
	//
	// +optional
	// +kubebuilder:validation:Enum=dhcp;linklayer;manual;other;random
	Origin string `json:"origin,omitempty"`

	// State describes the state of this IP address.
	//
	// +optional
	// +kubebuilder:validation:Enum=deprecated;duplicate;inaccessible;invalid;preferred;tentative;unknown
	State string `json:"state,omitempty"`
}

// VSphereMachineNetworkInterfaceIPStatus describes the observed state of a
// VM's network interface's IP configuration.
type VSphereMachineNetworkInterfaceIPStatus struct {
	// AutoConfigurationEnabled describes whether or not ICMPv6 router
	// solicitation requests are enabled or disabled from a given interface.
	//
	// These requests acquire an IP6 address and default gateway route from
	// zero-to-many routers on the connected network.
	//
	// If not set then ICMPv6 is not available on this VM.
	//
	// +optional
	AutoConfigurationEnabled *bool `json:"autoConfigurationEnabled,omitempty"`

	// DHCP describes the VM's observed, client-side, interface-specific DHCP
	// options.
	//
	// +optional
	DHCP *VSphereMachineNetworkDHCPStatus `json:"dhcp,omitempty"`

	// Addresses describes observed IP addresses for this interface.
	//
	// +optional
	Addresses []VSphereMachineNetworkInterfaceIPAddrStatus `json:"addresses,omitempty"`

	// MACAddr describes the observed MAC address for this interface.
	//
	// +optional
	MACAddr string `json:"macAddr,omitempty"`
}

// VSphereMachineNetworkInterfaceStatus describes the observed state of a
// VM's network interface.
type VSphereMachineNetworkInterfaceStatus struct {
	// Name describes the corresponding network interface with the same name
	// in the VM's desired network interface list. If unset, then there is no
	// corresponding entry for this interface.
	//
	// Please note this name is not necessarily related to the name of the
	// device as it is surfaced inside of the guest.
	//
	// +optional
	Name string `json:"name,omitempty"`

	// DeviceKey describes the unique hardware device key of this network
	// interface.
	//
	// +optional
	DeviceKey int32 `json:"deviceKey,omitempty"`

	// IP describes the observed state of the interface's IP configuration.
	//
	// +optional
	IP *VSphereMachineNetworkInterfaceIPStatus `json:"ip,omitempty"`

	// DNS describes the observed state of the interface's DNS configuration.
	//
	// +optional
	DNS *VSphereMachineNetworkDNSStatus `json:"dns,omitempty"`
}

// VSphereMachineNetworkStatus defines the observed state of a VM's
// network configuration.
//
// This a mirror of the v1alpha2 VirtualMachineNetworkStatus. See
// https://github.com/vmware-tanzu/vm-operator/blob/main/api/v1alpha2/virtualmachine_network_types.go
// for more information. When vm-operator v1alpha2 is updated, this type need to be synchronized.
type VSphereMachineNetworkStatus struct {
	// Interfaces describes the status of the VM's network interfaces.
	//
	// +optional
	Interfaces []VSphereMachineNetworkInterfaceStatus `json:"interfaces,omitempty"`
}
