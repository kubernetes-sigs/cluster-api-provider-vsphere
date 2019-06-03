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
	"crypto/tls"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

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
	clusterConfig := vsphereconfigv1.VsphereClusterProviderConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "vsphereproviderconfig/v1alpha1",
		},
		VsphereUser:             s.URL.User.Username(),
		VspherePassword:         pass,
		VsphereServer:           s.URL.Host,
		VsphereCredentialSecret: "",
	}
	clusterConfig.TypeMeta.Kind = reflect.TypeOf(clusterConfig).Name()

	raw, err := yaml.Marshal(clusterConfig)
	if err != nil {
		log.Fatal(err)
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.KubeadmToken:           "__TODO__", // see govmomi.Provisioner.GetKubeadmToken
				constants.KubeadmTokenExpiryTime: time.Now().Add(time.Hour).Format(time.RFC3339),
			},
		},
		Spec: v1alpha1.ClusterSpec{
			ProviderSpec: v1alpha1.ProviderSpec{
				Value: &runtime.RawExtension{
					Raw: raw,
				},
			},
		},
		Status: v1alpha1.ClusterStatus{
			ProviderStatus: &runtime.RawExtension{
				Raw: []byte(`{"clusterApiStatus": "Ready"}`),
			},
			APIEndpoints: []v1alpha1.APIEndpoint{
				{
					Host: "127.0.0.1",
					Port: 0, // TODO
				},
			},
		},
	}

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	disk := object.VirtualDeviceList(vm.Config.Hardware.Device).SelectByType((*types.VirtualDisk)(nil))[0].(*types.VirtualDisk)
	disk.CapacityInKB = 20 * 1024 * 1024 // bump since default disk size is < 1GB

	machineConfig := vsphereconfigv1.VsphereMachineProviderConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "vsphereproviderconfig/v1alpha1",
		},
		MachineSpec: vsphereconfigv1.VsphereMachineSpec{
			Datacenter:   "",
			Datastore:    "",
			ResourcePool: "",
			VMFolder:     "",
			Networks: []vsphereconfigv1.NetworkSpec{
				{
					NetworkName: "VM Network",
					IPConfig: vsphereconfigv1.IPConfig{
						NetworkType: vsphereconfigv1.DHCP,
					},
				},
			},
			NumCPUs:    2,
			MemoryMB:   2048,
			VMTemplate: vm.Name,
			Disks: []vsphereconfigv1.DiskSpec{
				{
					DiskSizeGB: disk.CapacityInKB / 1024 / 1024,
					DiskLabel:  disk.DeviceInfo.GetDescription().Label,
				},
			},
			Preloaded:        false,
			VsphereCloudInit: true,
		},
	}
	machineConfig.TypeMeta.Kind = reflect.TypeOf(machineConfig).Name()

	raw, err = yaml.Marshal(machineConfig)
	if err != nil {
		log.Fatal(err)
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "machine1",
		},
		Spec: v1alpha1.MachineSpec{
			ProviderSpec: v1alpha1.ProviderSpec{
				Value: &runtime.RawExtension{
					Raw: raw,
				},
			},
			Versions: v1alpha1.MachineVersionInfo{
				ControlPlane: "1.12.3",
				Kubelet:      "1.12.3",
			},
		},
	}

	p := &Provisioner{
		clusterV1alpha1: nil,
		lister:          nil,
		eventRecorder:   nil,
		sessioncache:    make(map[string]interface{}),
		k8sClient:       nil,
	}

	DefaultSSHPublicKeyFile = "create_test.go" // any file will avoid the k8s client path in GetSSHPublicKey()

	err = p.Create(context.Background(), cluster, machine)
	if err != nil {
		log.Fatal(err)
	}

	if model.Machine+1 != model.Count().Machine {
		t.Error("failed to clone vm")
	}
}
