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

	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
)

func Test_VMGroup(t *testing.T) {
	g := NewWithT(t)
	sim, err := vcsim.NewBuilder().
		WithOperations("cluster.group.create -cluster DC0_C0 -name blah-vm-group -vm DC0_C0_RP0_VM0 DC0_C0_RP0_VM1").
		Build()
	g.Expect(err).NotTo(HaveOccurred())
	defer sim.Destroy()

	ctx := context.Background()
	client, _ := govmomi.NewClient(ctx, sim.ServerURL(), true)
	finder := find.NewFinder(client.Client, false)

	dc, _ := finder.DatacenterOrDefault(ctx, "DC0")
	finder.SetDatacenter(dc)

	computeClusterCtx := testComputeClusterCtx{
		finder: finder,
	}

	computeClusterName := "DC0_C0"
	vmGroupName := "blah-vm-group"

	vmGrp, err := FindVMGroup(ctx, computeClusterCtx, computeClusterName, vmGroupName)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(vmGrp.listVMs()).To(HaveLen(2))

	vmObjOne, err := finder.VirtualMachine(ctx, "DC0_H0_VM0")
	g.Expect(err).NotTo(HaveOccurred())
	vmRef := vmObjOne.Reference()

	hasVM, err := vmGrp.HasVM(vmRef)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(hasVM).To(BeFalse())

	task, err := vmGrp.Add(ctx, vmRef)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(task.Wait(ctx)).To(Succeed())
	g.Expect(vmGrp.listVMs()).To(HaveLen(3))

	hasVM, err = vmGrp.HasVM(vmRef)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(hasVM).To(BeTrue())

	task, err = vmGrp.Remove(ctx, vmRef)
	g.Expect(task.Wait(ctx)).To(Succeed())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(vmGrp.listVMs()).To(HaveLen(2))

	hasVM, err = vmGrp.HasVM(vmRef)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(hasVM).To(BeFalse())

	vmGroupName = "incorrect-vm-group"
	_, err = FindVMGroup(ctx, computeClusterCtx, computeClusterName, vmGroupName)
	g.Expect(err).To(HaveOccurred())
}
