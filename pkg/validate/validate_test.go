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

package validate

import (
	"fmt"
	"testing"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2/cloudprovider"
)

// Temporary Test Settings for vSphere object Validation
const (
	vsphereURL          = "192.168.56.7"
	vsphereUsername     = "administrator@vsphere.local"
	vspherePassword     = "PASSWORD"
	vsphereDatacenter   = "Datacenter"
	vsphereDatastore    = "ISCSI-DATASTORE"
	vsphereNetwork      = "VLAN 182"
	vsphereResourcePool = "High-CAPV" // Can use Full Path or just object name and will search recursively for object
	vsphereFolder       = "/Datacenter/vm/capv_vms"
	vsphereTemplate     = "ubuntu-1804-capv"
)

// Build & Validate Test VsphereMachineSpec object
func TestVSphereMachineSpec(t *testing.T) {
	createdmachinespec := &v1alpha2.VSphereMachineSpec{
		Datacenter: vsphereDatacenter,
		Template:   vsphereTemplate,
		Network: v1alpha2.NetworkSpec{
			Devices: []v1alpha2.NetworkDeviceSpec{
				{
					NetworkName: vsphereNetwork,
					IPAddrs:     []string{"1.2.3.4"},
					Gateway4:    "1.2.3.1",
					Nameservers: []string{"1.2.3.10"},
				},
			},
		},
	}
	createdclusterSpec := &v1alpha2.VSphereClusterSpec{
		Server: vsphereURL,
		//Insecure: *true,
		CloudProviderConfiguration: cloudprovider.Config{
			Global: cloudprovider.GlobalConfig{
				Username:    vsphereUsername,
				Password:    vspherePassword,
				Insecure:    true,
				Datacenters: "dc1,dc2",
			},
			VCenter: map[string]cloudprovider.VCenterConfig{
				"0.0.0.0": {
					Username: "user",
					Password: "password",
				},
			},
		},
	}
	VSphereMachineStatus := CheckVSphereMachineSpec(createdclusterSpec, createdmachinespec)

	fmt.Printf("\n Test Response MAP VSphereMachineSpecStatus returned from validate library is %s\n", VSphereMachineStatus)

	// Test Create
	for k, v := range VSphereMachineStatus {
		fmt.Println(k, "\t", v)
		if v != "Success" {
			t.Error("Expected Success, got ", v)
		}
	}
}

// Build & Validate VSphereClusterSpec object
func TestVSphereClusterSpec(t *testing.T) {
	createdclusterSpec := &v1alpha2.VSphereClusterSpec{
		Server: vsphereURL,
		//Insecure: *true,
		CloudProviderConfiguration: cloudprovider.Config{
			Global: cloudprovider.GlobalConfig{
				Username:    vsphereUsername,
				Password:    vspherePassword,
				Insecure:    true,
				Datacenters: "dc1,dc2",
			},
			VCenter: map[string]cloudprovider.VCenterConfig{
				"0.0.0.0": {
					Username: "user",
					Password: "password",
				},
			},
			Network: cloudprovider.NetworkConfig{
				Name: "testNet",
			},
			Workspace: cloudprovider.WorkspaceConfig{
				Server:       "myserver",
				Datacenter:   vsphereDatacenter,
				Folder:       vsphereFolder,
				Datastore:    vsphereDatastore,
				ResourcePool: vsphereResourcePool,
			},
		},
	}

	VSphereClusterStatus := CheckVSphereClusterSpec(*createdclusterSpec)

	fmt.Printf("\n Test Response MAP VSphereClusterSpecStatus returned from validate library is %s\n", VSphereClusterStatus)

	// Test Create
	for k, v := range VSphereClusterStatus {
		fmt.Println(k, "\t", v)
		if v != "Success" {
			t.Error("Expected Success, got ", v)
		}
	}
}
