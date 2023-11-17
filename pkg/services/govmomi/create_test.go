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

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vcsim"
)

func TestCreate(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0 // ClusterHost only

	simr, err := vcsim.NewBuilder().WithModel(model).Build()
	if err != nil {
		t.Fatalf("unable to create simulator: %s", err)
	}
	defer simr.Destroy()

	ctx := context.Background()
	vmContext := fake.NewVMContext(ctx, fake.NewControllerManagerContext())
	vmContext.VSphereVM.Spec.Server = simr.ServerURL().Host

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

	vmRef := simulator.Map.Any("VirtualMachine")
	vm, ok := vmRef.(*simulator.VirtualMachine)
	if !ok {
		t.Fatal("failed to get reference to an existing VM on the vcsim instance")
	}
	vmContext.VSphereVM.Spec.Template = vm.Name

	disk := object.VirtualDeviceList(vm.Config.Hardware.Device).SelectByType((*types.VirtualDisk)(nil))[0].(*types.VirtualDisk)
	disk.CapacityInKB = int64(vmContext.VSphereVM.Spec.DiskGiB) * 1024 * 1024

	if err := createVM(ctx, vmContext, []byte(""), ""); err != nil {
		t.Fatal(err)
	}

	taskRef := types.ManagedObjectReference{
		Type:  morefTypeTask,
		Value: vmContext.VSphereVM.Status.TaskRef,
	}
	vimClient, err := vim25.NewClient(ctx, vmContext.Session.RoundTripper)
	if err != nil {
		t.Fatal("could not make vim25 client.")
	}
	task := object.NewTask(vimClient, taskRef)
	err = task.Wait(ctx)
	if err != nil {
		t.Fatal("error waiting for task:", err)
	}

	if model.Machine+1 != model.Count().Machine {
		t.Error("failed to clone vm")
	}
}
