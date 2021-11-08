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

package cluster

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"

	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers"
)

func TestVerifyAffinityRule(t *testing.T) {
	g := NewWithT(t)
	sim, err := helpers.VCSimBuilder().
		WithOperations("cluster.group.create -cluster DC0_C0 -name blah-vm-group -vm",
			"cluster.group.create -cluster DC0_C0 -name blah-host-group -host DC0_C0_H0 DC0_C0_H1",
			"cluster.rule.create -name blah-rule -enable -mandatory -vm-host -vm-group blah-vm-group -host-affine-group blah-host-group").
		Build()
	if err != nil {
		t.Fatalf("failed to create a VC simulator object %s", err)
	}
	defer sim.Destroy()

	ctx := context.Background()
	client, _ := govmomi.NewClient(ctx, sim.ServerURL(), true)
	finder := find.NewFinder(client.Client, false)

	dc, _ := finder.DatacenterOrDefault(ctx, "DC0")
	finder.SetDatacenter(dc)

	computeClusterCtx := testComputeClusterCtx{
		Context: context.Background(),
		finder:  finder,
	}

	rule, err := VerifyAffinityRule(computeClusterCtx, "DC0_C0", "blah-host-group", "blah-vm-group")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rule.IsMandatory()).To(BeTrue())
	g.Expect(rule.Disabled()).To(BeFalse())
}
