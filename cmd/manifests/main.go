/*
Copyright 2019 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/cmd/manifests/pkg/app"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphere/v1alpha1"
)

func main() {
	flag.Parse()
	if err := app.Run(provider{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var (
	sshAuthKeys app.StringSliceFlag
	server      = flag.String(
		"vsphere-server",
		"",
		"The remote vSphere server. Defaults to value of env var VSPHERE_SERVER.")
	username = flag.String(
		"vsphere-username",
		"",
		"The username used to access the vSphere server. Defaults to value of env var VSPHERE_USERNAME.")
	password = flag.String(
		"vsphere-password",
		"",
		"The password used to access the vSphere server Defaults to value of env var VSPHERE_PASSWORD.")
	datacenter = flag.String(
		"datacenter",
		"",
		"The datacenter in which VMs will be created")
	datastore = flag.String(
		"datastore",
		"",
		"The datastore in which VMs will be created")
	folder = flag.String(
		"folder",
		"vm",
		"The folder in which VMs will be created")
	pool = flag.String(
		"pool",
		`*/Resources`,
		"The resource pool to which VMs will belong")
	network = flag.String(
		"network",
		"",
		"The network to which VMs will be connected")
	template = flag.String(
		"template",
		"ubuntu-1804-kube-v1.13.6",
		"The template from which the VMs will be cloned")
	dhcp4 = flag.Bool(
		"dhcp4",
		true,
		"A flag that indicates whether or not to enable DHCP4 on a VM's initial network device")
	dhcp6 = flag.Bool(
		"dhcp6",
		false,
		"A flag that indicates whether or not to enable DHCP6 on a VM's initial network device")
	cpus = flag.Int(
		"cpus",
		2,
		"The number of CPUs to assign to a VM.")
	cores = flag.Int(
		"cores",
		2,
		`The number of cores per socket. For example, "-cpus 4 -cores 2" results in a VM with two CPU sockets with two cores per socket.`)
	memMiB = flag.Int(
		"mem",
		2048,
		"The amount of memory in MiB for a VM")
	diskGiB = flag.Int(
		"disk",
		2,
		"The disk size in GiB for a VM")
	managerImage = flag.String(
		"manager-image",
		"gcr.io/cluster-api-provider-vsphere/release/manager:latest",
		"The manager image")
	managerLogLevel = flag.Int(
		"manager-log-level",
		2,
		"The manager log level")
)

func init() {
	flag.Var(&sshAuthKeys,
		"ssh-public-key",
		"An SSH public key to grant access to the provisioned machines. May be specified more than once.")

	// The flag default values are not used to prevent the environment variable
	// values from being printed to stdout with the program's usage.
	if *server == "" {
		*server = os.Getenv("VSPHERE_SERVER")
	}
	if *username == "" {
		*username = os.Getenv("VSPHERE_USERNAME")
	}
	if *password == "" {
		*password = os.Getenv("VSPHERE_PASSWORD")
	}
}

type provider struct{}

func (p provider) GetClusterProviderSpec() (runtime.Object, error) {
	return &v1alpha1.VsphereClusterProviderSpec{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "VsphereClusterProviderSpec",
		},
		Server:            *server,
		Username:          *username,
		Password:          *password,
		SSHAuthorizedKeys: sshAuthKeys,
	}, nil
}

func (p provider) GetMachineProviderSpec() (runtime.Object, error) {
	return &v1alpha1.VsphereMachineProviderSpec{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "VsphereMachineProviderSpec",
		},
		Datacenter:   *datacenter,
		Datastore:    *datastore,
		Folder:       *folder,
		ResourcePool: *pool,
		Network: v1alpha1.NetworkSpec{
			Devices: []v1alpha1.NetworkDeviceSpec{
				{
					NetworkName: *network,
					DHCP4:       *dhcp4,
					DHCP6:       *dhcp6,
				},
			},
		},
		NumCPUs:           int32(*cpus),
		NumCoresPerSocket: int32(*cores),
		MemoryMiB:         int64(*memMiB),
		DiskGiB:           int32(*diskGiB),
	}, nil
}
