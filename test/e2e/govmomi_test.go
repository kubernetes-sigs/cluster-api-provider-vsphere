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

package e2e

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/soap"
)

func init() {
	flag.StringVar(&vsphereServer, "e2e.vsphereServer", os.Getenv("VSPHERE_SERVER"), "the vSphere server used for e2e tests")
	flag.StringVar(&vsphereDatacenter, "e2e.vsphereDataceter", os.Getenv("VSPHERE_DATACENTER"), "the inventory path of the vSphere datacenter in which VMs are created")
	flag.StringVar(&vsphereFolder, "e2e.vsphereFolder", os.Getenv("VSPHERE_FOLDER"), "the inventory path of the vSphere folder in which VMs are created")
	flag.StringVar(&vspherePool, "e2e.vspherePool", os.Getenv("VSPHERE_RESOURCE_POOL"), "the inventory path of the vSphere resource pool in which VMs are created")
	flag.StringVar(&vsphereDatastore, "e2e.vsphereDatastore", os.Getenv("VSPHERE_DATASTORE"), "the name of the vSphere datastore in which VMs are created")
	flag.StringVar(&vsphereNetwork, "e2e.vsphereNetwork", os.Getenv("VSPHERE_NETWORK"), "the name of the vSphere network to which VMs are connected")
	flag.StringVar(&vsphereMachineTemplate, "e2e.vsphereMachineTemplate", os.Getenv("VSPHERE_MACHINE_TEMPLATE"), "the template from which the Kubernetes VMs are cloned")
	flag.StringVar(&vsphereHAProxyTemplate, "e2e.vsphereHAProxyTemplate", os.Getenv("VSPHERE_HAPROXY_TEMPLATE"), "the template from which the HAProxy load balancer VM is cloned")
}

func initVSphereSession() {
	By("parsing vSphere server URL")
	serverURL, err := soap.ParseURL(vsphereServer)
	Expect(err).ShouldNot(HaveOccurred())

	By("creating vSphere client", func() {
		var err error
		serverURL.User = url.UserPassword(vsphereUsername, vspherePassword)
		vsphereClient, err = govmomi.NewClient(ctx, serverURL, true)
		Expect(err).ShouldNot(HaveOccurred())
	})

	By("creating vSphere finder")
	vsphereFinder = find.NewFinder(vsphereClient.Client)

	By("configuring vSphere datacenter")
	datacenter, err := vsphereFinder.DatacenterOrDefault(ctx, vsphereDatacenter)
	Expect(err).ShouldNot(HaveOccurred())
	vsphereFinder.SetDatacenter(datacenter)
}

func destroyVMsWithPrefix(prefix string) {
	vmList, _ := vsphereFinder.VirtualMachineList(ctx, vspherePool)
	for _, vm := range vmList {
		if strings.HasPrefix(vm.Name(), prefix) {
			destroyVM(vm)
		}
	}
}

func destroyVM(vm *object.VirtualMachine) {
	if task, _ := vm.PowerOff(ctx); task != nil {
		if err := task.Wait(ctx); err != nil {
			fmt.Printf("error powering off %s machine: %s\n", vm.Name(), err)
		}
	}
	if task, _ := vm.Destroy(ctx); task != nil {
		if err := task.Wait(ctx); err != nil {
			fmt.Printf("error destroying  %s machine: %s\n", vm.Name(), err)
		}
	}
}
