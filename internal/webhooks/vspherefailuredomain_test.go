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

package webhooks

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

func TestVsphereFailureDomain_Default(t *testing.T) {
	g := NewWithT(t)
	m := &infrav1.VSphereFailureDomain{
		Spec: infrav1.VSphereFailureDomainSpec{},
	}
	webhook := &VSphereFailureDomainWebhook{}
	g.Expect(webhook.Default(context.Background(), m)).ToNot(HaveOccurred())

	g.Expect(*m.Spec.Zone.AutoConfigure).To(BeFalse())
	g.Expect(*m.Spec.Region.AutoConfigure).To(BeFalse())
}

func TestVSphereFailureDomain_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name          string
		errExpected   *bool
		failureDomain infrav1.VSphereFailureDomain
	}{
		{
			name: "region failureDomain type is hostGroup",
			failureDomain: infrav1.VSphereFailureDomain{Spec: infrav1.VSphereFailureDomainSpec{
				Region: infrav1.FailureDomain{
					Name:          "foo",
					Type:          infrav1.HostGroupFailureDomain,
					TagCategory:   "k8s-bar",
					AutoConfigure: pointer.Bool(true),
				},
			}},
		},
		{
			name: "hostGroup failureDomain set but compute Cluster is empty",
			failureDomain: infrav1.VSphereFailureDomain{Spec: infrav1.VSphereFailureDomainSpec{
				Topology: infrav1.Topology{
					Datacenter:     "/blah",
					ComputeCluster: nil,
					Hosts: &infrav1.FailureDomainHosts{
						VMGroupName:   "vm-foo",
						HostGroupName: "host-foo",
					},
				},
			}},
		},
		{
			name: "type of zone failure domain is Hostgroup but topology's hostgroup is not set",
			failureDomain: infrav1.VSphereFailureDomain{Spec: infrav1.VSphereFailureDomainSpec{
				Region: infrav1.FailureDomain{
					Name:        "foo",
					Type:        infrav1.ComputeClusterFailureDomain,
					TagCategory: "k8s-bar",
				},
				Zone: infrav1.FailureDomain{
					Name:        "foo",
					Type:        infrav1.HostGroupFailureDomain,
					TagCategory: "k8s-bar",
				},
				Topology: infrav1.Topology{
					Datacenter:     "/blah",
					ComputeCluster: pointer.String("blah2"),
					Hosts:          nil,
				},
			}},
		},
		{
			name: "type of region failure domain is Compute Cluster but topology is not set",
			failureDomain: infrav1.VSphereFailureDomain{Spec: infrav1.VSphereFailureDomainSpec{
				Region: infrav1.FailureDomain{
					Name:        "foo",
					Type:        infrav1.ComputeClusterFailureDomain,
					TagCategory: "k8s-bar",
				},
				Zone: infrav1.FailureDomain{
					Name:        "foo",
					Type:        infrav1.HostGroupFailureDomain,
					TagCategory: "k8s-bar",
				},
				Topology: infrav1.Topology{
					Datacenter:     "/blah",
					ComputeCluster: nil,
					Hosts: &infrav1.FailureDomainHosts{
						VMGroupName:   "vm-foo",
						HostGroupName: "host-foo",
					},
				},
			}},
		},
		{
			name: "type of zone failure domain is Compute Cluster but topology is not set",
			failureDomain: infrav1.VSphereFailureDomain{Spec: infrav1.VSphereFailureDomainSpec{
				Region: infrav1.FailureDomain{
					Name:        "foo",
					Type:        infrav1.DatacenterFailureDomain,
					TagCategory: "k8s-bar",
				},
				Zone: infrav1.FailureDomain{
					Name:        "foo",
					Type:        infrav1.ComputeClusterFailureDomain,
					TagCategory: "k8s-bar",
				},
				Topology: infrav1.Topology{
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
			webhook := &VSphereFailureDomainWebhook{}
			_, err := webhook.ValidateCreate(context.Background(), &tt.failureDomain)
			if tt.errExpected == nil || !*tt.errExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
