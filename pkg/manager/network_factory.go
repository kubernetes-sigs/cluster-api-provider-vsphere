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

package manager

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
)

// ErrNetworkProviderEmpty is returned when a VSphereCluster does not yet have a
// network provider set on spec.network.provider. Callers should wait and retry
// until the value is populated.
var ErrNetworkProviderEmpty = errors.New("network provider is empty, wait for a valid value")

// NetworkProviderFactory resolves the NetworkProvider to use for a given VSphereCluster.
type NetworkProviderFactory interface {
	// ForCluster returns the NetworkProvider that should be used for the given VSphereCluster.
	ForCluster(ctx context.Context, cluster *vmwarev1.VSphereCluster) (services.NetworkProvider, error)
}

// perClusterNetworkProviderFactory resolves the NetworkProvider from the
// VSphereCluster's spec.network.provider field. It is used when the
// ClusterNetworkProvider feature gate is enabled.
type perClusterNetworkProviderFactory struct {
	registry map[string]services.NetworkProvider
}

// NewPerClusterNetworkProviderFactory returns a NetworkProviderFactory that
// resolves the provider per-cluster from spec.network.provider. The registry is
// pre-built for the in-scope provider names.
func NewPerClusterNetworkProviderFactory(ctx context.Context, client client.Client) (NetworkProviderFactory, error) {
	registry := map[string]services.NetworkProvider{}
	for _, name := range []string{VDSNetworkProvider, NSXNetworkProvider, NSXVPCNetworkProvider} {
		np, err := GetNetworkProvider(ctx, client, name)
		if err != nil {
			return nil, err
		}
		registry[name] = np
	}
	return &perClusterNetworkProviderFactory{registry: registry}, nil
}

// ForCluster returns the NetworkProvider matching the cluster's spec.network.provider.
func (f *perClusterNetworkProviderFactory) ForCluster(_ context.Context, cluster *vmwarev1.VSphereCluster) (services.NetworkProvider, error) {
	provider := cluster.Spec.Network.Provider
	if provider == "" {
		return nil, ErrNetworkProviderEmpty
	}
	np, ok := f.registry[provider]
	if !ok {
		return nil, fmt.Errorf("unknown network provider %q", provider)
	}
	return np, nil
}

// staticNetworkProviderFactory always returns the provider built from the
// --network-provider flag. It is used when the ClusterNetworkProvider feature
// gate is disabled, preserving the previous behavior.
type staticNetworkProviderFactory struct {
	networkProvider services.NetworkProvider
}

// NewStaticNetworkProviderFactory returns a NetworkProviderFactory that always
// returns the provider built from the given flag value.
func NewStaticNetworkProviderFactory(ctx context.Context, client client.Client, networkProvider string) (NetworkProviderFactory, error) {
	np, err := GetNetworkProvider(ctx, client, networkProvider)
	if err != nil {
		return nil, err
	}
	return &staticNetworkProviderFactory{networkProvider: np}, nil
}

// ForCluster always returns the statically configured NetworkProvider.
func (f *staticNetworkProviderFactory) ForCluster(_ context.Context, _ *vmwarev1.VSphereCluster) (services.NetworkProvider, error) {
	return f.networkProvider, nil
}
