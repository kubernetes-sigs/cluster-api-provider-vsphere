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

package validate

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

const (
	// SuccessMessage to deliver back to caller when object is found in vSphere
	SuccessMessage = "Success"
	// FailureMessage to deliver back to caller when object is NOT found in vSphere
	FailureMessage = "Fail"
)

// CheckVSphereMachineSpec Runs govmomi checks against vsphere objects that are fields of VsphereMachineSpec
func CheckVSphereMachineSpec(acluster *v1alpha2.VSphereClusterSpec, amachine *v1alpha2.VSphereMachineSpec) map[string]string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create MAP to store Connection results for all objects
	var VSphereMachineStatus = make(map[string]string)

	// Call Vcvalidate func and return client object.
	client, vcstatus, _ := VcValidate(ctx, acluster.CloudProviderConfiguration.Global.Username, acluster.CloudProviderConfiguration.Global.Password, acluster.Server)
	VSphereMachineStatus["VCenter"] = vcstatus

	// Instantiate a finder object for the client
	f := find.NewFinder(client.Client, true)

	dcObject, dcvalidate, _ := DatacenterValidate(ctx, f, amachine.Datacenter)
	VSphereMachineStatus["Datacenter"] = dcvalidate

	// Now that DC is verified set the DC for finder object for all further object searches
	f.SetDatacenter(dcObject)

	netstatus, _ := NetworkValidate(ctx, f, amachine.Network.Devices[0].NetworkName)
	VSphereMachineStatus["Network"] = netstatus

	templatestatus, _ := TemplateValidate(ctx, f, amachine.Template)
	VSphereMachineStatus["Template"] = templatestatus

	return VSphereMachineStatus
}

// CheckVSphereClusterSpec Runs govmomi checks against vsphere objects that are fields of VSphereClusterSpec
func CheckVSphereClusterSpec(acluster v1alpha2.VSphereClusterSpec) map[string]string {
	// Creating the connection context for all API calls to VC
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create MAP to store Connection results for all objects
	var VSphereClusterStatus = make(map[string]string)

	// Call Vcvalidate func and return client object.
	client, vcstatus, _ := VcValidate(ctx, acluster.CloudProviderConfiguration.Global.Username, acluster.CloudProviderConfiguration.Global.Password, acluster.Server)
	VSphereClusterStatus["VCenter"] = vcstatus

	// Instantiate a finder object for the client
	f := find.NewFinder(client.Client, true)

	dcObject, dcvalidate, _ := DatacenterValidate(ctx, f, acluster.CloudProviderConfiguration.Workspace.Datacenter)
	VSphereClusterStatus["Datacenter"] = dcvalidate

	// Now that DC is verified set the DC for finder object for all further object searches
	f.SetDatacenter(dcObject)

	datastorestatus, _ := DatastoreValidate(ctx, f, acluster.CloudProviderConfiguration.Workspace.Datastore)
	VSphereClusterStatus["Datastore"] = datastorestatus

	rpstatus, _ := ResourcePoolValidate(ctx, f, acluster.CloudProviderConfiguration.Workspace.ResourcePool)
	VSphereClusterStatus["ResourcePool"] = rpstatus

	folderstatus, _ := FolderValidate(ctx, f, acluster.CloudProviderConfiguration.Workspace.Folder)
	VSphereClusterStatus["VMFolder"] = folderstatus

	return VSphereClusterStatus
}

// VcValidate instantiates connection to vCenter object and validats health
func VcValidate(ctx context.Context, user string, pass string, vcenter string) (*govmomi.Client, string, error) {
	// Validate VC Connection and return client.Client object for additional validation.
	fmt.Printf("\nWorking on connecting to vCenter  ")
	// Parsing URL
	urlString := fmt.Sprintf("https://%s:%s@%s/sdk", user, pass, vcenter)
	url, err := url.Parse(urlString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "URL Parsing Error: %s\n", err)
		os.Exit(1)
	}
	// Client Connection to vCenter
	client, err := govmomi.NewClient(ctx, url, true)
	if err != nil {
		return client, FailureMessage, err
	}
	info := client.ServiceContent.About
	fmt.Printf("Connected to vCenter version %s\n", info.Version)
	return client, SuccessMessage, nil
}

// DatacenterValidate - Use govmomi to check for existence of DC object
func DatacenterValidate(ctx context.Context, f *find.Finder, vsphereDatacenter string) (*object.Datacenter, string, error) {

	fmt.Printf("\nLooking for vSphere Datacenter %s ", vsphereDatacenter)
	dc, err := f.Datacenter(ctx, vsphereDatacenter)
	if err != nil {
		return nil, FailureMessage, err
	}
	fmt.Printf("\n -Datacenter found %s ", dc.Name())
	return dc, SuccessMessage, nil
}

// DatastoreValidate - Use govmomi to check for existence of Datastore object
func DatastoreValidate(ctx context.Context, f *find.Finder, vsphereDatastore string) (string, error) {

	fmt.Printf("\nLooking for vSphere datastore %s ", vsphereDatastore)
	ds, err := f.Datastore(ctx, vsphereDatastore)
	if err != nil {
		return FailureMessage, err
	}
	fmt.Printf("\n -Datastore found %s ", ds.Name())
	return SuccessMessage, nil
}

// NetworkValidate - Use govmomi to check for existence of vSphere Object by its Name Property
func NetworkValidate(ctx context.Context, f *find.Finder, vsphereNetwork string) (string, error) {

	fmt.Printf("\nLooking for vSphere network %s ", vsphereNetwork)
	net, err := f.Network(ctx, vsphereNetwork)
	if err != nil {
		return FailureMessage, err
	}
	fmt.Printf("\n -Found Network with net.Reference() %s", net.Reference())
	return SuccessMessage, nil
}

// ResourcePoolValidate - Use govmomi to check for existence of RP object in vSphere
func ResourcePoolValidate(ctx context.Context, f *find.Finder, vsphereResourcePool string) (string, error) {

	fmt.Printf("\nLooking for vSphere ResourcePool %s", vsphereResourcePool)
	rp, err := f.ResourcePool(ctx, vsphereResourcePool)
	if err != nil {
		return FailureMessage, err
	}
	fmt.Printf("\n -Resource Pool found %s", rp.Name())
	return SuccessMessage, nil
}

// TemplateValidate - Use govmomi to check for existence of vSphere Template object.
func TemplateValidate(ctx context.Context, f *find.Finder, vsphereTemplate string) (string, error) {

	fmt.Printf("\nLooking for vSphere Template %s", vsphereTemplate)
	vm, err := f.VirtualMachine(ctx, vsphereTemplate)
	if err != nil {
		fmt.Printf("Error is  %s\n", err)
		return FailureMessage, err
	}
	fmt.Printf("\n -VM Template found %s", vm.Name())
	return SuccessMessage, nil
}

// FolderValidate - Use govmomi to check for existence of vSphere Folder object.
func FolderValidate(ctx context.Context, f *find.Finder, vsphereFolder string) (string, error) {

	fmt.Printf("\nLooking for vSphere VM Folder %s", vsphereFolder)
	folder, err := f.Folder(ctx, vsphereFolder)
	if err != nil {
		fmt.Printf("Error is  %s\n", err)
		return FailureMessage, err
	}
	fmt.Printf("\n -VM Folder found %s", folder.Name())
	return SuccessMessage, nil
}
