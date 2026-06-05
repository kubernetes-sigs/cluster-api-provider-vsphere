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

package util

import (
	"testing"

	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestDetermineClusterIPFamily(t *testing.T) {
	tests := []struct {
		name      string
		cluster   *clusterv1.Cluster
		expected  ClusterIPFamily
		expectErr bool
	}{
		{
			name:      "nil cluster",
			cluster:   nil,
			expectErr: true,
		},
		{
			name:      "empty cluster network",
			cluster:   &clusterv1.Cluster{},
			expectErr: true,
		},
		{
			name: "IPv4 single stack (Pods)",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.0.0.0/16"},
						},
					},
				},
			},
			expected: IPv4SingleStack,
		},
		{
			name: "IPv6 single stack (Pods)",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"fd00::/32"},
						},
					},
				},
			},
			expected: IPv6SingleStack,
		},
		{
			name: "Dual stack IPv4 primary (Pods)",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.0.0.0/16", "fd00::/32"},
						},
					},
				},
			},
			expected: DualStackIPv4Primary,
		},
		{
			name: "Dual stack IPv6 primary (Pods)",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"fd00::/32", "10.0.0.0/16"},
						},
					},
				},
			},
			expected: DualStackIPv6Primary,
		},
		{
			name: "Fallback to Services: IPv4 single stack",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Services: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.96.0.0/12"},
						},
					},
				},
			},
			expected: IPv4SingleStack,
		},
		{
			name: "Fallback to Services: Dual stack IPv6 primary",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Services: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"fd00:1::/108", "10.96.0.0/12"},
						},
					},
				},
			},
			expected: DualStackIPv6Primary,
		},
		{
			name: "Invalid CIDR",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"invalid-cidr"},
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			actual, err := DetermineClusterIPFamily(tt.cluster)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(actual).To(Equal(tt.expected))
			}
		})
	}
}
