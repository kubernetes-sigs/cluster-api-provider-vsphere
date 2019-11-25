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
	"crypto/tls"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

func TestCreate(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0 // ClusterHost only

	defer model.Remove()
	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}
	model.Service.TLS = new(tls.Config)

	s := model.Service.NewServer()
	defer s.Close()
	pass, _ := s.URL.User.Password()

	clusterContext := fake.NewClusterContext(fake.NewControllerContext(fake.NewControllerManagerContext()))
	clusterContext.VSphereCluster.Spec.Server = s.URL.Host
	machineContext := fake.NewMachineContext(clusterContext)

	authSession, err := session.GetOrCreate(
		machineContext,
		clusterContext.VSphereCluster.Spec.Server, "",
		s.URL.User.Username(), pass)
	if err != nil {
		t.Fatal(err)
	}
	machineContext.Session = authSession

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	machineContext.VSphereMachine.Spec.Template = vm.Name

	disk := object.VirtualDeviceList(vm.Config.Hardware.Device).SelectByType((*types.VirtualDisk)(nil))[0].(*types.VirtualDisk)
	disk.CapacityInKB = int64(machineContext.VSphereMachine.Spec.DiskGiB) * 1024 * 1024

	if err := createVM(machineContext, []byte("")); err != nil {
		t.Fatal(err)
	}

	if model.Machine+1 != model.Count().Machine {
		t.Error("failed to clone vm")
	}
}
