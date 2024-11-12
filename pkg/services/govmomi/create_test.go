/*
Copyright 2018 The Kubernetes Authors.

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

package govmomi

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
	vmcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

func TestCreate(t *testing.T) {
	ctx := context.Background()

	model := simulator.VPX()
	model.Host = 0 // ClusterHost only

	simr, err := vcsim.NewBuilder().WithModel(model).Build()
	if err != nil {
		t.Fatalf("unable to create simulator: %s", err)
	}
	defer simr.Destroy()
	vmRef := simulator.Map.Any("VirtualMachine")
	vm, ok := vmRef.(*simulator.VirtualMachine)
	if !ok {
		t.Fatal("failed to get reference to an existing VM on the vcsim instance")
	}

	executeTest := func(vmname string, replaceFunc func(vmContext *vmcontext.VMContext, vimClient *vim25.Client)) {
		vmContext := fake.NewVMContext(ctx, fake.NewControllerManagerContext())
		vmContext.VSphereVM.Spec.Server = simr.ServerURL().Host
		vmContext.VSphereVM.SetName(vmname)

		authSession, err := session.GetOrCreate(
			ctx,
			session.NewParams().
				WithServer(vmContext.VSphereVM.Spec.Server).
				WithUserInfo(simr.Username(), simr.Password()).
				WithDatacenter("*"))
		if err != nil {
			t.Fatal(err)
		}
		vmContext.Session = authSession

		vmContext.VSphereVM.Spec.Template = vm.Name

		disk := object.VirtualDeviceList(vm.Config.Hardware.Device).SelectByType((*types.VirtualDisk)(nil))[0].(*types.VirtualDisk)
		disk.CapacityInKB = int64(vmContext.VSphereVM.Spec.DiskGiB) * 1024 * 1024

		vimClient, err := vim25.NewClient(ctx, vmContext.Session.RoundTripper)
		if err != nil {
			t.Fatalf("could not create vim25 client: %v", err)
		}

		if replaceFunc != nil {
			replaceFunc(vmContext, vimClient)
		}

		if err := createVM(ctx, vmContext, []byte(""), ""); err != nil {
			t.Fatal(err)
		}

		taskRef := types.ManagedObjectReference{
			Type:  morefTypeTask,
			Value: vmContext.VSphereVM.Status.TaskRef,
		}

		task := object.NewTask(vimClient, taskRef)
		if err := task.Wait(ctx); err != nil {
			t.Fatalf("error waiting for task: %v", err)
		}
	}

	t.Log("executing with canonical name")
	executeTest("canonical", nil)
	if model.Machine+1 != model.Count().Machine {
		t.Error("failed to clone vm")
	}

	t.Log("executing with MOID")
	replaceFuncMOID := func(vmContext *vmcontext.VMContext, vimClient *vim25.Client) {
		finderClient := find.NewFinder(vimClient)
		dc, err := finderClient.DatacenterOrDefault(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		folder, err := finderClient.FolderOrDefault(ctx, "")
		if err != nil {
			t.Fatal(err)
		}

		rp, err := finderClient.ResourcePoolOrDefault(ctx, "")
		if err != nil {
			t.Fatal(err)
		}

		ds, err := finderClient.DatastoreOrDefault(ctx, "")
		if err != nil {
			t.Fatal(err)
		}

		netw, err := finderClient.NetworkOrDefault(ctx, "VM Network")
		if err != nil {
			t.Fatal(err)
		}

		vmContext.VSphereVM.Spec.Datacenter = dc.Reference().String()
		vmContext.VSphereVM.Spec.Folder = folder.Reference().String()
		vmContext.VSphereVM.Spec.ResourcePool = rp.Reference().String()
		vmContext.VSphereVM.Spec.Datastore = ds.Reference().String()
		vmContext.VSphereVM.Spec.Network.Devices[0].NetworkName = netw.Reference().String()
		vmContext.VSphereVM.Spec.Template = vmRef.Reference().String()

		t.Logf("creating using MOID. Datacenter: %s, Datastore: %s, Folder: %s, ResourcePool: %s, Network %s, Template %s",
			vmContext.VSphereVM.Spec.Datacenter,
			vmContext.VSphereVM.Spec.Datastore,
			vmContext.VSphereVM.Spec.Folder,
			vmContext.VSphereVM.Spec.ResourcePool,
			vmContext.VSphereVM.Spec.Network.Devices[0].NetworkName,
			vmContext.VSphereVM.Spec.Template)
	}

	executeTest("moid", replaceFuncMOID)
	if model.Machine+2 != model.Count().Machine {
		t.Error("failed to clone vm")
	}

	t.Log("executing with mixed, moref, etc")
	replaceFuncMixed := func(vmContext *vmcontext.VMContext, vimClient *vim25.Client) {
		finderClient := find.NewFinder(vimClient)
		dc, err := finderClient.DatacenterOrDefault(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		folder, err := finderClient.FolderOrDefault(ctx, "")
		if err != nil {
			t.Fatal(err)
		}

		rp, err := finderClient.ResourcePoolOrDefault(ctx, "")
		if err != nil {
			t.Fatal(err)
		}

		ds, err := finderClient.DatastoreOrDefault(ctx, "")
		if err != nil {
			t.Fatal(err)
		}

		netw, err := finderClient.NetworkOrDefault(ctx, "VM Network")
		if err != nil {
			t.Fatal(err)
		}

		vmContext.VSphereVM.Spec.Datacenter = dc.Reference().Value   // moref
		vmContext.VSphereVM.Spec.Datastore = ds.Reference().String() // moid
		vmContext.VSphereVM.Spec.Folder = folder.InventoryPath       // inventory path
		vmContext.VSphereVM.Spec.ResourcePool = rp.Name()
		vmContext.VSphereVM.Spec.Network.Devices[0].NetworkName = netw.Reference().String()

		t.Logf("creating using mixed entries. Datacenter: %s, Datastore: %s, Folder: %s, ResourcePool: %s, Network %s, Template %s",
			vmContext.VSphereVM.Spec.Datacenter,
			vmContext.VSphereVM.Spec.Datastore,
			vmContext.VSphereVM.Spec.Folder,
			vmContext.VSphereVM.Spec.ResourcePool,
			vmContext.VSphereVM.Spec.Network.Devices[0].NetworkName,
			vmContext.VSphereVM.Spec.Template)
	}
	executeTest("mixed", replaceFuncMixed)
	if model.Machine+3 != model.Count().Machine {
		t.Error("failed to clone vm")
	}
}
