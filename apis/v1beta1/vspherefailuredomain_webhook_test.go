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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

// nolint
func TestVsphereFailureDomain_Default(t *testing.T) {
	tests := []struct {
		name         string
		spec         VSphereFailureDomainSpec
		expectedSpec VSphereFailureDomainSpec
	}{
		{
			name: "when autoconfigure is not set",
			spec: VSphereFailureDomainSpec{
				Zone: FailureDomain{
					AutoConfigure: nil,
				},
				Region: FailureDomain{
					AutoConfigure: nil,
				},
			},
			expectedSpec: VSphereFailureDomainSpec{
				Zone: FailureDomain{
					AutoConfigure: pointer.Bool(false),
				},
				Region: FailureDomain{
					AutoConfigure: pointer.Bool(false),
				},
			},
		},
		{
			name: "when autoconfigure is set just on one field",
			spec: VSphereFailureDomainSpec{
				Region: FailureDomain{
					AutoConfigure: pointer.Bool(true),
				},
			},
			expectedSpec: VSphereFailureDomainSpec{
				Zone: FailureDomain{
					AutoConfigure: pointer.Bool(false),
				},
				Region: FailureDomain{
					AutoConfigure: pointer.Bool(true),
				},
			},
		},
		{
			name: "when networkconfigs is null and network field exists",
			spec: VSphereFailureDomainSpec{
				Topology: Topology{
					Networks: []string{"network-a", "network-b"},
				},
			},
			expectedSpec: VSphereFailureDomainSpec{
				Zone: FailureDomain{
					AutoConfigure: pointer.Bool(false),
				},
				Region: FailureDomain{
					AutoConfigure: pointer.Bool(false),
				},
				Topology: Topology{
					Networks: []string{"network-a", "network-b"},
					NetworkConfigs: []FailureDomainNetwork{
						{
							NetworkName: "network-a",
						},
						{
							NetworkName: "network-b",
						},
					},
				},
			},
		},
		{
			name: "when networkconfigs is not null and network field exists",
			spec: VSphereFailureDomainSpec{
				Topology: Topology{
					Networks: []string{"network-a", "network-b"},
					NetworkConfigs: []FailureDomainNetwork{
						{
							NetworkName: "network-c",
						},
						{
							NetworkName: "network-d",
						},
					},
				},
			},
			expectedSpec: VSphereFailureDomainSpec{
				Zone: FailureDomain{
					AutoConfigure: pointer.Bool(false),
				},
				Region: FailureDomain{
					AutoConfigure: pointer.Bool(false),
				},
				Topology: Topology{
					Networks: []string{"network-a", "network-b"},
					NetworkConfigs: []FailureDomainNetwork{
						{
							NetworkName: "network-c",
						},
						{
							NetworkName: "network-d",
						},
					},
				},
			},
		},
	}

	g := NewWithT(t)

	for _, test := range tests {
		m := &VSphereFailureDomain{
			Spec: test.spec,
		}
		m.Default()
		g.Expect(m.Spec).To(Equal(test.expectedSpec))
	}

	//g.Expect(*m.Spec.Zone.AutoConfigure).To(BeFalse())
	//g.Expect(*m.Spec.Region.AutoConfigure).To(BeFalse())
}

func TestVSphereFailureDomain_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name          string
		errExpected   *bool
		failureDomain VSphereFailureDomain
	}{
		{
			name: "region failureDomain type is hostGroup",
			failureDomain: VSphereFailureDomain{Spec: VSphereFailureDomainSpec{
				Region: FailureDomain{
					Name:          "foo",
					Type:          HostGroupFailureDomain,
					TagCategory:   "k8s-bar",
					AutoConfigure: pointer.Bool(true),
				},
			}},
		},
		{
			name: "hostGroup failureDomain set but compute Cluster is empty",
			failureDomain: VSphereFailureDomain{Spec: VSphereFailureDomainSpec{
				Topology: Topology{
					Datacenter:     "/blah",
					ComputeCluster: nil,
					Hosts: &FailureDomainHosts{
						VMGroupName:   "vm-foo",
						HostGroupName: "host-foo",
					},
				},
			}},
		},
		{
			name: "type of zone failure domain is Hostgroup but topology's hostgroup is not set",
			failureDomain: VSphereFailureDomain{Spec: VSphereFailureDomainSpec{
				Region: FailureDomain{
					Name:        "foo",
					Type:        ComputeClusterFailureDomain,
					TagCategory: "k8s-bar",
				},
				Zone: FailureDomain{
					Name:        "foo",
					Type:        HostGroupFailureDomain,
					TagCategory: "k8s-bar",
				},
				Topology: Topology{
					Datacenter:     "/blah",
					ComputeCluster: pointer.String("blah2"),
					Hosts:          nil,
				},
			}},
		},
		{
			name: "type of region failure domain is Compute Cluster but topology is not set",
			failureDomain: VSphereFailureDomain{Spec: VSphereFailureDomainSpec{
				Region: FailureDomain{
					Name:        "foo",
					Type:        ComputeClusterFailureDomain,
					TagCategory: "k8s-bar",
				},
				Zone: FailureDomain{
					Name:        "foo",
					Type:        HostGroupFailureDomain,
					TagCategory: "k8s-bar",
				},
				Topology: Topology{
					Datacenter:     "/blah",
					ComputeCluster: nil,
					Hosts: &FailureDomainHosts{
						VMGroupName:   "vm-foo",
						HostGroupName: "host-foo",
					},
				},
			}},
		},
		{
			name: "type of zone failure domain is Compute Cluster but topology is not set",
			failureDomain: VSphereFailureDomain{Spec: VSphereFailureDomainSpec{
				Region: FailureDomain{
					Name:        "foo",
					Type:        DatacenterFailureDomain,
					TagCategory: "k8s-bar",
				},
				Zone: FailureDomain{
					Name:        "foo",
					Type:        ComputeClusterFailureDomain,
					TagCategory: "k8s-bar",
				},
				Topology: Topology{
					Datacenter:     "/blah",
					ComputeCluster: nil,
				},
			}},
		},
	}

	for _, tt := range tests {
		// Need to reinit the test variable
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.failureDomain.ValidateCreate()
			if tt.errExpected == nil || !*tt.errExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
