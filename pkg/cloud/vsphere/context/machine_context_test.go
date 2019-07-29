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

package context

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/klogr"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphere/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func Test_MachineContextIPAddr(t *testing.T) {
	tests := []struct {
		name        string
		ctx         *MachineContext
		ipAddr      string
		expectedErr error
	}{
		{
			name: "single IPv4 address, no preferred CIDR",
			ctx: &MachineContext{
				MachineConfig: &v1alpha1.VsphereMachineProviderSpec{
					Network: v1alpha1.NetworkSpec{},
				},
				Machine: &clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: "192.168.0.1",
							},
						},
					},
				},
			},
			ipAddr:      "192.168.0.1",
			expectedErr: nil,
		},
		{
			name: "single IPv6 address, no preferred CIDR",
			ctx: &MachineContext{
				MachineConfig: &v1alpha1.VsphereMachineProviderSpec{
					Network: v1alpha1.NetworkSpec{},
				},
				Machine: &clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: "fdf3:35b5:9dad:6e09::0001",
							},
						},
					},
				},
			},
			ipAddr:      "fdf3:35b5:9dad:6e09::0001",
			expectedErr: nil,
		},
		{
			name: "multiple IPv4 addresses, only 1 internal, no preferred CIDR",
			ctx: &MachineContext{
				MachineConfig: &v1alpha1.VsphereMachineProviderSpec{
					Network: v1alpha1.NetworkSpec{},
				},
				Machine: &clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: "192.168.0.1",
							},
							{
								Type:    corev1.NodeExternalIP,
								Address: "1.1.1.1",
							},
							{
								Type:    corev1.NodeExternalIP,
								Address: "2.2.2.2",
							},
						},
					},
				},
			},
			ipAddr:      "192.168.0.1",
			expectedErr: nil,
		},
		{
			name: "multiple IPv4 addresses, preferred CIDR set to v4",
			ctx: &MachineContext{
				ClusterContext: &ClusterContext{
					Logger: klogr.New(),
				},
				MachineConfig: &v1alpha1.VsphereMachineProviderSpec{
					Network: v1alpha1.NetworkSpec{
						PreferredAPIServerCIDR: "192.168.0.0/16",
					},
				},
				Machine: &clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: "192.168.0.1",
							},
							{
								Type:    corev1.NodeInternalIP,
								Address: "172.17.0.1",
							},
						},
					},
				},
			},
			ipAddr:      "192.168.0.1",
			expectedErr: nil,
		},
		{
			name: "multiple IPv4 and IPv6 addresses, preferred CIDR set to v4",
			ctx: &MachineContext{
				ClusterContext: &ClusterContext{
					Logger: klogr.New(),
				},
				MachineConfig: &v1alpha1.VsphereMachineProviderSpec{
					Network: v1alpha1.NetworkSpec{
						PreferredAPIServerCIDR: "192.168.0.0/16",
					},
				},
				Machine: &clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: "192.168.0.1",
							},
							{
								Type:    corev1.NodeInternalIP,
								Address: "fdf3:35b5:9dad:6e09::0001",
							},
						},
					},
				},
			},
			ipAddr:      "192.168.0.1",
			expectedErr: nil,
		},
		{
			name: "multiple IPv4 and IPv6 addresses, preferred CIDR set to v6",
			ctx: &MachineContext{
				ClusterContext: &ClusterContext{
					Logger: klogr.New(),
				},
				MachineConfig: &v1alpha1.VsphereMachineProviderSpec{
					Network: v1alpha1.NetworkSpec{
						PreferredAPIServerCIDR: "fdf3:35b5:9dad:6e09::/64",
					},
				},
				Machine: &clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: "192.168.0.1",
							},
							{
								Type:    corev1.NodeInternalIP,
								Address: "fdf3:35b5:9dad:6e09::0001",
							},
						},
					},
				},
			},
			ipAddr:      "fdf3:35b5:9dad:6e09::0001",
			expectedErr: nil,
		},
		{
			name: "no addresses found",
			ctx: &MachineContext{
				ClusterContext: &ClusterContext{
					Logger: klogr.New(),
				},
				MachineConfig: &v1alpha1.VsphereMachineProviderSpec{
					Network: v1alpha1.NetworkSpec{},
				},
				Machine: &clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []corev1.NodeAddress{},
					},
				},
			},
			ipAddr:      "",
			expectedErr: ErrNoMachineIPAddr,
		},
		{
			name: "no addresses found with preferred CIDR",
			ctx: &MachineContext{
				ClusterContext: &ClusterContext{
					Logger: klogr.New(),
				},
				MachineConfig: &v1alpha1.VsphereMachineProviderSpec{
					Network: v1alpha1.NetworkSpec{
						PreferredAPIServerCIDR: "192.168.0.0/16",
					},
				},
				Machine: &clusterv1.Machine{
					Status: clusterv1.MachineStatus{
						Addresses: []corev1.NodeAddress{
							{
								Type:    corev1.NodeInternalIP,
								Address: "10.0.0.1",
							},
						},
					},
				},
			},
			ipAddr:      "",
			expectedErr: ErrNoMachineIPAddr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ipAddr, err := test.ctx.IPAddr()
			if err != test.expectedErr {
				t.Logf("expected err: %q", test.expectedErr)
				t.Logf("actual err: %q", err)
				t.Errorf("unexpected error")
			}

			if ipAddr != test.ipAddr {
				t.Logf("expected IP addr: %q", test.ipAddr)
				t.Logf("actual IP addr: %q", ipAddr)
				t.Error("unexpected IP addr from machine context")
			}
		})
	}
}
