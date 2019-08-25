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

package govmomi_test

import (
	"crypto/tls"
	"log"
	"os"
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi"
)

func init() {
	os.Unsetenv("VSPHERE_USERNAME")
	os.Unsetenv("VSPHERE_PASSWORD")
}

func TestCreate(t *testing.T) {
	model := simulator.VPX()
	model.Host = 0 // ClusterHost only

	defer model.Remove()
	err := model.Create()
	if err != nil {
		log.Fatal(err)
	}
	model.Service.TLS = new(tls.Config)

	s := model.Service.NewServer()
	defer s.Close()
	pass, _ := s.URL.User.Password()
	os.Setenv("VSPHERE_USERNAME", s.URL.User.Username())
	os.Setenv("VSPHERE_PASSWORD", pass)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
	}
	vsphereCluster := &infrav1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       "VSphereCluster",
		},
		Spec: infrav1.VSphereClusterSpec{
			Server: s.URL.Host,
		},
	}

	clusterContext, err := context.NewClusterContext(&context.ClusterContextParams{
		Cluster:        cluster,
		VSphereCluster: vsphereCluster,
	})
	if err != nil {
		t.Fatal(err)
	}

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	disk := object.VirtualDeviceList(vm.Config.Hardware.Device).SelectByType((*types.VirtualDisk)(nil))[0].(*types.VirtualDisk)
	disk.CapacityInKB = 20 * 1024 * 1024 // bump since default disk size is < 1GB

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine",
			Namespace: "test-namespace",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
		},
	}
	vsphereMachine := &infrav1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine",
			Namespace: "test-namespace",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       "VSphereMachine",
		},
		Spec: infrav1.VSphereMachineSpec{
			Datacenter: "",
			Network: infrav1.NetworkSpec{
				Devices: []infrav1.NetworkDeviceSpec{
					{
						NetworkName: "VM Network",
						DHCP4:       true,
						DHCP6:       true,
					},
				},
			},
			NumCPUs:   2,
			MemoryMiB: 2048,
			Template:  vm.Name,
		},
	}

	machineContext, err := context.NewMachineContextFromClusterContext(
		clusterContext, machine, vsphereMachine)
	if err != nil {
		t.Fatal(err)
	}

	if err := govmomi.Create(machineContext, []byte("")); err != nil {
		log.Fatal(err)
	}

	if model.Machine+1 != model.Count().Machine {
		t.Error("failed to clone vm")
	}
}
