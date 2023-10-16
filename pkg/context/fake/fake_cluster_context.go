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

package fake

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// NewClusterContext returns a fake ClusterContext for unit testing
// reconcilers with a fake client.
func NewClusterContext(ctx context.Context, controllerManagerCtx *capvcontext.ControllerManagerContext) *capvcontext.ClusterContext {
	// Create the cluster resources.
	cluster := newClusterV1()
	vsphereCluster := newVSphereCluster(cluster)

	// Add the cluster resources to the fake cluster client.
	if err := controllerManagerCtx.Client.Create(ctx, &cluster); err != nil {
		panic(err)
	}
	if err := controllerManagerCtx.Client.Create(ctx, &vsphereCluster); err != nil {
		panic(err)
	}

	return &capvcontext.ClusterContext{
		Cluster:        &cluster,
		VSphereCluster: &vsphereCluster,
	}
}

func newClusterV1() clusterv1.Cluster {
	return clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      Clusterv1a2Name,
			UID:       Clusterv1a2UUID,
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{PodCIDR},
				},
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{ServiceCIDR},
				},
			},
			InfrastructureRef: &corev1.ObjectReference{
				Namespace: Namespace,
				Name:      InfrastructureRefName,
			},
		},
	}
}

func newVSphereCluster(owner clusterv1.Cluster) infrav1.VSphereCluster {
	return infrav1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: owner.Namespace,
			Name:      owner.Name,
			UID:       VSphereClusterUUID,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: owner.Name},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         owner.APIVersion,
					Kind:               owner.Kind,
					Name:               owner.Name,
					UID:                owner.UID,
					BlockOwnerDeletion: &boolTrue,
					Controller:         &boolTrue,
				},
			},
		},
		Spec: infrav1.VSphereClusterSpec{
			Server: VCenterURL,
		},
	}
}
