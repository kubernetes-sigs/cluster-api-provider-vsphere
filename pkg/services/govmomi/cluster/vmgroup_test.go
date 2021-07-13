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

func TestAddVMToGroup(t *testing.T) {
	g := NewWithT(t)
	sim, err := helpers.VCSimBuilder().
		WithOperations("cluster.group.create -cluster DC0_C0 -name blah-vm-group -vm").
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

	vmGroupName := "blah-vm-group"
	g.Expect(AddVMToGroup(computeClusterCtx, "DC0_C0", vmGroupName, "DC0_H0_VM0")).To(Succeed())
	g.Expect(AddVMToGroup(computeClusterCtx, "DC0_C0", vmGroupName, "DC0_H0_VM1")).To(Succeed())

	ccr, err := finder.ClusterComputeResource(ctx, "DC0_C0")
	g.Expect(err).NotTo(HaveOccurred())

	refs, err := listVMs(ctx, ccr, "blah-vm-group")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(refs).To(HaveLen(2))
}
