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

// Package vmware is the package for webhooks of vmware resources.
package vmware

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
)

// resolveNetworkProvider returns the network provider name to validate an object against.
//
// When the ClusterNetworkProvider feature gate is disabled, the static value derived from the
// --network-provider flag (staticProvider) is returned and no cluster is loaded.
//
// When the gate is enabled, the provider is resolved from the owning Cluster's VSphereCluster
// following the resolution order:
//  1. read the cluster.x-k8s.io/cluster-name label, load the Cluster, then load the VSphereCluster
//     via Cluster.spec.infrastructureRef.
//  2. if the VSphereCluster cannot be loaded -> reject (surface the error).
//  3. if VSphereCluster.spec.network.provider is empty -> reject.
//  4. otherwise return the provider value as-is.
func resolveNetworkProvider(ctx context.Context, c client.Client, staticProvider string, obj metav1.Object) (string, error) {
	if !feature.Gates.Enabled(feature.ClusterNetworkProvider) {
		return staticProvider, nil
	}

	clusterName, ok := obj.GetLabels()[clusterv1.ClusterNameLabel]
	if !ok || clusterName == "" {
		return "", fmt.Errorf("missing %q label, cannot resolve the owning Cluster", clusterv1.ClusterNameLabel)
	}

	cluster := &clusterv1.Cluster{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: clusterName}, cluster); err != nil {
		return "", fmt.Errorf("failed to get Cluster %s: %w", klog.KRef(obj.GetNamespace(), clusterName), err)
	}

	if !cluster.Spec.InfrastructureRef.IsDefined() {
		return "", fmt.Errorf("Cluster %s does not have a spec.infrastructureRef set", klog.KObj(cluster)) //nolint:staticcheck // Cluster is a Kubernetes resource kind.
	}

	vsphereCluster := &vmwarev1.VSphereCluster{}
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.InfrastructureRef.Name}
	if err := c.Get(ctx, key, vsphereCluster); err != nil {
		return "", fmt.Errorf("failed to get VSphereCluster %s: %w", klog.KRef(key.Namespace, key.Name), err)
	}

	if vsphereCluster.Spec.Network.Provider == "" {
		return "", fmt.Errorf("VSphereCluster %s spec.network.provider is empty, wait for a valid value", klog.KObj(vsphereCluster))
	}

	return vsphereCluster.Spec.Network.Provider, nil
}
