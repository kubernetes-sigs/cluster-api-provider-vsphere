# Content Library Support

```text
---
title: Support for Content Library
authors:
  - "@adam-jian-zhang"
  - "@rikatz"
reviewers:  
  - "@srm09"
  - "@yastij"
  - "@chrischdi"
  - "@randomvariable"
creation-date: 2023-03-27
last-updated: 2023-09-21
status: proposed
---
```

## Table of Contents

* [Content Library Support](#Content-Library-Support)
  * [Table of Contents](#table-of-contents)
  * [Glossary](#glossary)
  * [Summary](#summary)
  * [Motivation](#motivation)
    * [Goals](#goals)
    * [Non-Goals](#non-goals)
  * [Proposal](#proposal)
    * [User Stories](#user-stories)
      * [Story 1](#story-1)
      * [Story 2](#story-2)
    * [Overall Design](#overall-design)
    * [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
      * [Naming Scheme](#naming-scheme)
      * [API changes](#api-changes)
        * [VirtualMachineCloneSpec CR](#virtualmachineclonespec-cr)
      * [Implementation Details](#implementation-details)
        * [VSphereVM clone changes](#vspherevm-clone-changes)
    * [Resources](#resources)

## Glossary

**Content Library**: The content library of vSphere, that stores artifacts such as OVA, vm template, etc. and can serve as the source for VM clones.

## Summary

In current CAPV implementation, we only support cloning VM from vSphere template. This works fine for normal single vCenter
scenario as everything is under a single vCenter context. If we consider multi-vCenter scenario, the template management becomes
a problem, because we can not reference template cross vCenter boundary. We can manually copy the template to different vCenters as a workaround, but there is no mechanism to ensure the validity/consistency of the templates, it is error-prone and not maintainable.

Content library can help template management in this case, it supports subscriptions among different content libraries and provides sync capability to help distribute OVA/templates to downstream content libraries. 
For example, we can set up a content library on the primary vCenter that hosts the management cluster, put the OVA/templates in this content library OVA/templates and then for other vCenters that
hosts workload clusters, we set up a content library and subscribe to the primary content library, syncing the content library items(templates/OVAs) to it. 
This way becomes easier to manage templates/OVAs and have confidence in the content of the templates/OVAs.

As another example, pushing a new template to a Content Library makes the maintenance of the images easier, even for a single vCenter, so 
there is no need for the user to know the full path of the template when creating a new cluster and instead, 
just reference a content library and the item that should be consumed.

## Motivation

Enhance CAPV to be able to provision VMs based on templates from a Content Library

### Goals

* Use ContentLibrary as a template source for CAPV
* Support the cloning based on the OVA format provided by CAPV project

### Non-Goals

* Turn content library the only supported template source.
* Provide a public subscribable content library provided by CAPV project
* Support VM template as a source of cloning, when using Content Library

## Proposal

### User Stories

#### Story 1

As an admin, I'd like to manage templates/OVAs across the vCenters that can be consumed by CAPV.

#### Story 2

As a cluster user, I'd like to use content library items(templates/OVAs) as the source of cluster nodes.

### Overall Design

The design focuses on how to clone VMs from content library items. For the content library management itself users can reference vSphere documentation.

### Implementation Details/Notes/Constraints

#### Naming scheme

We use `Template` field of `VirtualMachineCloneSpec` to specify the source we want to clone,
since we want to augment it to also from the content library, we need to have a way to differentiate the source.
The proposal is to mutate the template field to accept an URI scheme, as: `scheme://location` which:

* scheme, can be vsphere|library
  * vsphere for conventional vm templates
  * library for OVA/templates from content library
* to make it backward compatible, the scheme part can be omitted for conventional vm templates, so if we encounter
  template without scheme part, we assume it is vsphere template, existing CAPV does not need to do anything.
* When a content library scheme is being used, the scheme should look like `library://[name_or_id_of_library]/item`.
  * In case of two libraries existing with the same name, cloning a new VM should error and user should be 
  requested to use the library UUID instead
  * Library UUID can be queried with GoVC: `govc library.info`

#### API changes

##### VirtualMachineCloneSpec CR

The `VirtualMachineCloneSpec` has following changes, the spec is used to specify clone spec for `VsphereVM`, it needs the following change to deal with newly added content library capabilities.

```golang
type VirtualMachineCloneSpec struct {
  // Template is the name or inventory path of the template used to clone
  // the virtual machine. It can be a vSphere template, or a content library item. For example:
  // vsphere template: [vsphere://location of the vsphere template], vsphere:// can be omitted for backward compatibility
  // content library: [library://library_name_or_id/content_library_item_name]
  // note that this has impact on cloneMode field, since it is only possible to do full clone from content library items,
  // and we should have validation rule for this.
  // +kubebuilder:validation:MinLength=1
  Template string `json:"template"`
}
```

#### Implementation Details


##### Getting the right Content Library item
Because content libraries can have the same name, and items on different content libraries
can have the same name, we should have a way to disambiguate what exact template users wants to deploy.

This way, given the schema of a content library item is `library://[library_name_or_id]/itemname` we should:
* Parse the field as a URI. If just a "Host" is present, it means user is requesting an item name, so it should be queried by item name
* If both host and item are present, then host is the content library name (or uuid) and path is the item name
* If more than one item is returned, this should be an error
* If more than one content library with the same name is returned, it should be an error and users should use the Content Library ID

```go
import (
    ...
   	"github.com/vmware/govmomi/vapi/library"
    ...
)

func getContentLibraryItem(someargs) (itemID string, err error) {
    var clibrary, citem string
    u, err := url.Parse(itemToSearch)
    if err != nil {
    	log.Fatal(err)
    }
    
    citem = u.Host
    if u.Path != "" {
    	clibrary = u.Host
    	citem = strings.TrimPrefix(u.Path, "/") // Remove leading prefix /
    }
    
    m := library.NewManager(restclient)
    
    if clibrary != "" {
        // Try to get Content Library by ID
    	_, err = m.GetLibraryByID(ctx, clibrary)
    	if err != nil {
    		// In this case, we should get all libraries with a specific name,
    		libs, err := m.FindLibrary(ctx, library.Find{
    			Name: clibrary,
    		})
    		if err != nil {
                // ERROR on getting library			
    		}
    		if len(libs) != 1 {
    			// ERROR as no library or too many libraries has been found
    		}
    		clibrary = libs[0]
    	}
    }
    
    items, err := m.FindLibraryItems(ctx, library.FindItem{
    	Name:      citem,
    	Type:      "ovf",
    	LibraryID: clibrary,
    })
    if err != nil {
        // ERROR getting the Library items	
    }
    
    if len(items) != 1 {
        // ERROR no items or too many items with same name. In this case, should be more specific and pass
        // a content library name
    }
    
    return items[0], nil
}
```

##### VSphereVM clone changes

We need to enhance the [clone](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/19c5deb79e97a8cd9d546ec39db3f29231bbcb91/pkg/services/govmomi/vcenter/clone.go#L75) function to recogize the scheme we defined, and choose appropriate API to clone the VM.

Once defined if Content Library should be used, a new specific clone function should be used,
to make the proper cloning, but using vCenter Manager client instead of template/VM manager.

The "customization" function can and should be reused, and decoupled from the main clone process. Instead 
after "customizing" it should be passed to the proper cloning function (library item or template)

```go
// Based on https://github.com/vmware-tanzu/vm-operator/blob/main/pkg/vmprovider/providers/vsphere/session/session_vm_create.go and
// https://github.com/vmware/govmomi/blob/main/govc/library/deploy.go
import (
    ...
   	"github.com/vmware/govmomi/vapi/vcenter"
    ...
)

func deployContentLibraryAsVM(itemID string, additionalargs...) (*object.VirtualMachine, error) {
	m := vcenter.NewManager(restclient)
	ref, err := m.DeployLibraryItem(ctx, itemID string, vcenter.Deploy{
		DeploymentSpec: vcenter.DeploymentSpec{
			// Add Spec here. Contains network, storage, datastore, storagepolicy
		},
        Target: vcenter.Target {
            // Add target spec here. Contains selected resource Pool, hostID, VMFolder
        },
	})
    if err != nil {
        // ERROR deploying the content library item
    }

	finder := find.NewFinder(vim25Client)
	obj, err := finder.ObjectReference(ctx, *ref)
	if err != nil {
		// ERROR failed to find the VM provisioned
	}

	vm, ok := obj.(*object.VirtualMachine)
    if !ok {
        // ERROR on object type assertion
    }
    return vm, nil
}
```

##### Constraints
* Content Libraries can have same name. CAPV should have a deduplication process, where when 
multiple content libraries with the same name are found, the process fails with a proper condition
* In case of a name conflict, users can still use the content library ID instead and CAPV should be 
able to clone the OVA properly

## Resources

* [Using Content Libraries](https://docs.vmware.com/en/VMware-vSphere/7.0/com.vmware.vsphere.vm_admin.doc/GUID-254B2CE8-20A8-43F0-90E8-3F6776C2C896.html)
* [Content Library API](https://developer.vmware.com/apis/vsphere-automation/latest/content/content/)
* [govc library clone](https://github.com/vmware/govmomi/blob/main/govc/library/clone.go)
