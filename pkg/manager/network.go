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

package manager

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
)

const (
	// NSXVPCNetworkProvider identifies the NSX VPC network provider.
	NSXVPCNetworkProvider = "VPC"
	// NSXNetworkProvider identifies the NSX Tier-1 network provider.
	NSXNetworkProvider = "NSXTier1"
	// VDSNetworkProvider identifies the vSphere Distributed Switch network provider.
	VDSNetworkProvider = "VSphereDistributed"
	// ExternallyManagedNetworkProvider identifies the ExternallyManaged network provider.
	// Used when network objects are provisioned and managed externally and CAPV only attaches VMs.
	ExternallyManagedNetworkProvider = "ExternallyManaged"
	// DummyLBNetworkProvider identifies the Dummy network provider.
	DummyLBNetworkProvider = "DummyLBNetworkProvider"

	// Legacy global network provider names (pre-rename values of --network-provider).
	// The current flag values reuse NSXNetworkProvider / NSXVPCNetworkProvider / VDSNetworkProvider
	// (NSXTier1 / VPC / VSphereDistributed), matching VSphereCluster.spec.network.provider.
	legacyNSXNetworkProvider    = "NSX"
	legacyNSXVPCNetworkProvider = "NSX-VPC"
	legacyVDSNetworkProvider    = "vsphere-network"
)

var legacyToNetworkProviderName = map[string]string{
	legacyNSXNetworkProvider:    NSXNetworkProvider,
	legacyNSXVPCNetworkProvider: NSXVPCNetworkProvider,
	legacyVDSNetworkProvider:    VDSNetworkProvider,
}

// ConvertNetworkProviderName converts a legacy network provider name to the
// PascalCase name used by CAPV. Unknown names are returned unchanged.
func ConvertNetworkProviderName(name string) string {
	if converted, ok := legacyToNetworkProviderName[name]; ok {
		return converted
	}
	return name
}

// GetNetworkProvider will return a network provider instance based on the environment
// the cfg is used to initialize a client that talks directly to api-server without using the cache.
func GetNetworkProvider(ctx context.Context, client client.Client, networkProvider string) (services.NetworkProvider, error) {
	log := ctrl.LoggerFrom(ctx)
	networkProvider = ConvertNetworkProviderName(networkProvider)

	switch networkProvider {
	case NSXVPCNetworkProvider:
		log.Info("Pick NSX-VPC network provider")
		return network.NSXTVpcNetworkProvider(client), nil
	case NSXNetworkProvider:
		// TODO: disableFirewall not configurable
		log.Info("Pick NSX-T network provider")
		return network.NsxtNetworkProvider(client, "false"), nil
	case VDSNetworkProvider:
		log.Info("Pick NetOp (VDS) network provider")
		return network.NetOpNetworkProvider(client), nil
	case ExternallyManagedNetworkProvider:
		log.Info("Pick ExternallyManaged network provider")
		return network.ExternallyManagedNetworkProvider(client), nil
	case DummyLBNetworkProvider:
		log.Info("Pick Dummy network provider")
		return network.DummyLBNetworkProvider(), nil
	default:
		log.Info("NetworkProvider not set. Pick Dummy network provider")
		return network.DummyNetworkProvider(), nil
	}
}
