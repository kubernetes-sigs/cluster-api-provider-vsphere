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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1a2 "sigs.k8s.io/cluster-api/api/v1alpha2"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// NewClusterContext returns a fake ClusterContext for unit testing
// reconcilers with a fake client.
func NewClusterContext(ctx *context.ControllerContext) *context.ClusterContext {

	// Create the cluster resources.
	cluster := newClusterV1a2()
	vsphereCluster := newVSphereCluster(cluster)

	// Add the cluster resources to the fake cluster client.
	if err := ctx.Client.Create(ctx, &cluster); err != nil {
		panic(err)
	}
	if err := ctx.Client.Create(ctx, &vsphereCluster); err != nil {
		panic(err)
	}

	return &context.ClusterContext{
		ControllerContext: ctx,
		Cluster:           &cluster,
		VSphereCluster:    &vsphereCluster,
		Logger:            ctx.Logger.WithName(vsphereCluster.Name),
	}
}

func newClusterV1a2() clusterv1a2.Cluster {
	return clusterv1a2.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      Clusterv1a2Name,
			UID:       types.UID(Clusterv1a2UUID),
		},
		Spec: clusterv1a2.ClusterSpec{
			ClusterNetwork: &clusterv1a2.ClusterNetwork{
				Pods: &clusterv1a2.NetworkRanges{
					CIDRBlocks: []string{PodCIDR},
				},
				Services: &clusterv1a2.NetworkRanges{
					CIDRBlocks: []string{ServiceCIDR},
				},
			},
		},
	}
}

func newVSphereCluster(owner clusterv1a2.Cluster) infrav1.VSphereCluster {
	return infrav1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: owner.Namespace,
			Name:      owner.Name,
			UID:       types.UID(VSphereClusterUUID),
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
		Spec: infrav1.VSphereClusterSpec{},
	}
}
