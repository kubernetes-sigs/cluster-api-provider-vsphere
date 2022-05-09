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

package fake

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

func NewVSphereCluster(namespace string) vmwarev1.VSphereCluster {
	return vmwarev1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      VSphereClusterName,
			UID:       VSphereClusterUUID,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: vmwarev1.GroupVersion.String(),
			Kind:       "VSphereCluster",
		},
		Spec: vmwarev1.VSphereClusterSpec{},
	}
}

func newCluster(vSphereCluster *vmwarev1.VSphereCluster) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vSphereCluster.Name,
			Namespace: vSphereCluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         vSphereCluster.APIVersion,
					Kind:               vSphereCluster.Kind,
					Name:               vSphereCluster.Name,
					UID:                vSphereCluster.UID,
					BlockOwnerDeletion: &boolTrue,
					Controller:         &boolTrue,
				},
			},
			UID: ClusterUUID,
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
		},
	}
}
