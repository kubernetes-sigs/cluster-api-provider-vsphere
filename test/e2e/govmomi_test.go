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
	"context"
	"flag"
	"net/url"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/soap"
)

var (
	vsphereUsername   = os.Getenv("VSPHERE_USERNAME")
	vspherePassword   = os.Getenv("VSPHERE_PASSWORD")
	vsphereServer     string
	vsphereDatacenter string

	ctx           = context.TODO()
	vsphereClient *govmomi.Client
	vsphereFinder *find.Finder
)

func init() {
	flag.StringVar(&vsphereServer, "e2e.vsphereServer", os.Getenv("VSPHERE_SERVER"), "the vSphere server used for e2e tests")
	flag.StringVar(&vsphereDatacenter, "e2e.vsphereDataceter", os.Getenv("VSPHERE_DATACENTER"), "the inventory path of the vSphere datacenter in which VMs are created")
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
