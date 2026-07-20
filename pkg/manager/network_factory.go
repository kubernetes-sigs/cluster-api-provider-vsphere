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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
)

// ErrNetworkProviderEmpty is returned when a VSphereCluster does not yet have a
// network provider set on spec.network.provider. Callers should wait and retry
// until the value is populated.
var ErrNetworkProviderEmpty = errors.New("network provider is empty, wait for a valid value")

// ErrNoOwningCluster is returned when an object has no cluster.x-k8s.io/cluster-name
// label, so the owning Cluster cannot be resolved. This is expected for ClusterClass
// templates that are not yet bound to a Cluster. Callers that only need the provider
// for optional validation (e.g. VSphereMachineTemplate webhooks) may skip that
// validation; callers that require a provider (e.g. VSphereMachine) should treat
// this as an error.
var ErrNoOwningCluster = fmt.Errorf("missing %q label, cannot resolve the owning Cluster", clusterv1.ClusterNameLabel)

// NetworkProviderFactory resolves the NetworkProvider to use for a given VSphereCluster.
type NetworkProviderFactory interface {
	// ForCluster returns the NetworkProvider that should be used for the given VSphereCluster.
	ForCluster(ctx context.Context, cluster *vmwarev1.VSphereCluster) (services.NetworkProvider, error)

	// ForObject resolves the NetworkProvider for a cluster-scoped object
	// (e.g. VSphereMachine) by reading the cluster.x-k8s.io/cluster-name label,
	// loading Cluster → VSphereCluster, then calling ForCluster.
	// The static factory may ignore the object and return the flag-based provider.
	// The per-cluster factory returns ErrNoOwningCluster when the cluster-name
	// label is missing (e.g. ClusterClass templates).
	ForObject(ctx context.Context, c client.Client, obj metav1.Object) (services.NetworkProvider, error)
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
	names := []string{VDSNetworkProvider, NSXNetworkProvider, NSXVPCNetworkProvider}
	if feature.Gates.Enabled(feature.ExternallyManagedProvider) {
		names = append(names, ExternallyManagedNetworkProvider)
	}
	for _, name := range names {
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

// ForObject resolves the owning Cluster via the cluster.x-k8s.io/cluster-name label,
// loads the VSphereCluster from Cluster.spec.infrastructureRef, then calls ForCluster.
func (f *perClusterNetworkProviderFactory) ForObject(ctx context.Context, c client.Client, obj metav1.Object) (services.NetworkProvider, error) {
	vsphereCluster, err := getVSphereClusterForObject(ctx, c, obj)
	if err != nil {
		return nil, err
	}
	return f.ForCluster(ctx, vsphereCluster)
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

// ForObject always returns the statically configured NetworkProvider without
// loading the owning Cluster or VSphereCluster.
func (f *staticNetworkProviderFactory) ForObject(_ context.Context, _ client.Client, _ metav1.Object) (services.NetworkProvider, error) {
	return f.networkProvider, nil
}

// getVSphereClusterForObject loads the owning Cluster via the
// cluster.x-k8s.io/cluster-name label and then the VSphereCluster referenced by
// Cluster.spec.infrastructureRef.
func getVSphereClusterForObject(ctx context.Context, c client.Client, obj metav1.Object) (*vmwarev1.VSphereCluster, error) {
	clusterName, ok := obj.GetLabels()[clusterv1.ClusterNameLabel]
	if !ok || clusterName == "" {
		return nil, ErrNoOwningCluster
	}

	cluster := &clusterv1.Cluster{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: clusterName}, cluster); err != nil {
		return nil, fmt.Errorf("failed to get Cluster %s: %w", klog.KRef(obj.GetNamespace(), clusterName), err)
	}

	if !cluster.Spec.InfrastructureRef.IsDefined() {
		return nil, fmt.Errorf("Cluster %s does not have a spec.infrastructureRef set", klog.KObj(cluster)) //nolint:staticcheck // Cluster is a Kubernetes resource kind.
	}

	vsphereCluster := &vmwarev1.VSphereCluster{}
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.InfrastructureRef.Name}
	if err := c.Get(ctx, key, vsphereCluster); err != nil {
		return nil, fmt.Errorf("failed to get VSphereCluster %s: %w", klog.KRef(key.Namespace, key.Name), err)
	}

	return vsphereCluster, nil
}
