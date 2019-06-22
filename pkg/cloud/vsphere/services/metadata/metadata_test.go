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

package metadata_test

import (
	"testing"

	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/metadata"
)

func TestNew(t *testing.T) {
	testCases := []struct {
		name string
		ctx  *context.MachineContext
	}{
		{
			name: "dhcp4",
			ctx: &context.MachineContext{
				Machine: &clusterv1.Machine{},
				MachineConfig: &v1alpha1.VsphereMachineProviderConfig{
					MachineSpec: v1alpha1.VsphereMachineSpec{
						Network: v1alpha1.NetworkSpec{
							Devices: []v1alpha1.NetworkDeviceSpec{
								{
									NetworkName: "network1",
									MACAddr:     "00:00:00:00:00",
									DHCP4:       true,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "dhcp6",
			ctx: &context.MachineContext{
				Machine: &clusterv1.Machine{},
				MachineConfig: &v1alpha1.VsphereMachineProviderConfig{
					MachineSpec: v1alpha1.VsphereMachineSpec{
						Network: v1alpha1.NetworkSpec{
							Devices: []v1alpha1.NetworkDeviceSpec{
								{
									NetworkName: "network1",
									MACAddr:     "00:00:00:00:00",
									DHCP6:       true,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "dhcp4+dhcp6",
			ctx: &context.MachineContext{
				Machine: &clusterv1.Machine{},
				MachineConfig: &v1alpha1.VsphereMachineProviderConfig{
					MachineSpec: v1alpha1.VsphereMachineSpec{
						Network: v1alpha1.NetworkSpec{
							Devices: []v1alpha1.NetworkDeviceSpec{
								{
									NetworkName: "network1",
									MACAddr:     "00:00:00:00:00",
									DHCP4:       true,
									DHCP6:       true,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "static4+dhcp6",
			ctx: &context.MachineContext{
				Machine: &clusterv1.Machine{},
				MachineConfig: &v1alpha1.VsphereMachineProviderConfig{
					MachineSpec: v1alpha1.VsphereMachineSpec{
						Network: v1alpha1.NetworkSpec{
							Devices: []v1alpha1.NetworkDeviceSpec{
								{
									NetworkName: "network1",
									MACAddr:     "00:00:00:00:00",
									DHCP6:       true,
									IPAddrs:     []string{"192.168.4.21"},
									Gateway4:    "192.168.4.1",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "static4+dhcp6+static-routes",
			ctx: &context.MachineContext{
				Machine: &clusterv1.Machine{},
				MachineConfig: &v1alpha1.VsphereMachineProviderConfig{
					MachineSpec: v1alpha1.VsphereMachineSpec{
						Network: v1alpha1.NetworkSpec{
							Devices: []v1alpha1.NetworkDeviceSpec{
								{
									NetworkName: "network1",
									MACAddr:     "00:00:00:00:00",
									DHCP6:       true,
									IPAddrs:     []string{"192.168.4.21"},
									Gateway4:    "192.168.4.1",
								},
							},
							Routes: []v1alpha1.NetworkRouteSpec{
								{
									To:     "192.168.5.1/24",
									Via:    "192.168.4.254",
									Metric: 3,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "2nets",
			ctx: &context.MachineContext{
				Machine: &clusterv1.Machine{},
				MachineConfig: &v1alpha1.VsphereMachineProviderConfig{
					MachineSpec: v1alpha1.VsphereMachineSpec{
						Network: v1alpha1.NetworkSpec{
							Devices: []v1alpha1.NetworkDeviceSpec{
								{
									NetworkName: "network1",
									MACAddr:     "00:00:00:00:00",
									DHCP4:       true,
									Routes: []v1alpha1.NetworkRouteSpec{
										{
											To:     "192.168.5.1/24",
											Via:    "192.168.4.254",
											Metric: 3,
										},
									},
								},
								{
									NetworkName: "network12",
									MACAddr:     "00:00:00:00:01",
									DHCP6:       true,
									MTU:         mtu(100),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "2nets-static+dhcp",
			ctx: &context.MachineContext{
				Machine: &clusterv1.Machine{},
				MachineConfig: &v1alpha1.VsphereMachineProviderConfig{
					MachineSpec: v1alpha1.VsphereMachineSpec{
						Network: v1alpha1.NetworkSpec{
							Devices: []v1alpha1.NetworkDeviceSpec{
								{
									NetworkName:   "network1",
									MACAddr:       "00:00:00:00:00",
									IPAddrs:       []string{"192.168.4.21"},
									Gateway4:      "192.168.4.1",
									MTU:           mtu(0),
									Nameservers:   []string{"1.1.1.1"},
									SearchDomains: []string{"vmware.ci"},
								},
								{
									NetworkName:   "network12",
									MACAddr:       "00:00:00:00:01",
									DHCP6:         true,
									SearchDomains: []string{"vmware6.ci"},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.ctx.Machine.Name = tc.name
			actVal, err := metadata.New(tc.ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Log(string(actVal))
		})
	}
}

func mtu(i int64) *int64 {
	if i == 0 {
		return nil
	}
	return &i
}
