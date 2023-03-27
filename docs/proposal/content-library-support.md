# Content Library Support

```text
---
title: Support for Content Library
authors:
  - "@adam-jian-zhang"
reviewers:  
  - "@srm09"
  - "@yastij"
creation-date: 2023-03-27
last-updated: 2023-03-27
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

**Content Library**: The content library of vSphere, which stores artifacts such as OVA, vm template, etc. which can serve as the source the VM clones from.

## Summary

In current CAPV implementation, we only support cloning VM from vSphere template. This works fine for normal single vCenter
scenario as everything is under a single vCenter context. If we consider multi-vCenter scenario, the template management becomes
a problem, because we can not reference template cross vCenter boundary. We can manually copy the template to different vCenters as a workaround, but there is no mechanism to ensure the validity/consistency of the templates, it is error-prone and not maintainable.

Content library can help template management in this case, it supports subscriptions among different content libraries and provides sync capability to help distribute OVA/templates to downstream content libraries. For example, we can set up a content library on the primary vCenter that hosts the management cluster, put the OVA/templates in the content library which serves as the version of truth for the OVA/templates we intended to use. And then for other vCenters that
hosts workload clusters, we set up a content library and subscribe to the primary content library, and sync the content library items(templates/OVAs) to it. In this way it is easy to manage templates/OVAs and have confidence in the content of the templates/OVAs.

Using content library as the source of templates has another little benefit that enables using local datastore as destination for the VMs, currently we are limited to shared datastore.

## Motivation

Enhance CAPV to clone VM from content library, to make multi-vCenter template management easier.

### Goals

* Use ContentLibrary items as the VM image template.

### Non-Goals

* Turn content library the only supported template source.

## Proposal

### User Stories

#### Story 1

As an admin, I'd like to manage templates/OVAs across the vCenters that can be consumed by CAPV.

#### Story 2

As an cluster user, I'd like to use content library items(templates/OVAs) as the source of cluster nodes.

### Overall Design

The design focuses on how to clone VMs from content library items. For the content library management itself users can reference vSphere documentation.

### Implementation Details/Notes/Constraints

#### Naming scheme

We use `Template` field of `VirtualMachineCloneSpec` to specify the source we want to clone,
since we want to augment it to also from the content library, we need to have a way to differentiate the source.
I propose we use URI scheme for that, it has the format: `scheme://location` which

* scheme, can be vsphere|library
  * vsphere for conventional vm templates
  * library for OVA/templates from content library
* to make it backward compatible, the scheme part can be omitted for conventional vm templates, so if we encounter
  template without scheme part, we assume it is vsphere template, existing CAPV does not need to do anything.

#### API changes

##### VirtualMachineCloneSpec CR

The `VirtualMachineCloneSpec` has following changes, the spec is used to specify clone spec for `VsphereVM`, it needs the following change to deal with newly added content library capabilities.

```golang
type VirtualMachineCloneSpec struct {
  // Template is the name or inventory path of the template used to clone
  // the virtual machine. It can be a vSphere template, or a content library item. For example:
  // vsphere template: [vsphere://location of the vsphere template], vsphere:// can be omitted for backward compatibility
  // content library: [library://name of the content library item]
  // note that this has impact on cloneMode field, since it is only possible to do full clone from content library items,
  // and we should have validation rule for this.
  // +kubebuilder:validation:MinLength=1
  Template string `json:"template"`

  // The secret that stores the credentials for accessing the content library.
  // It should reside on the same namespace of the Cluster that reference it.
  // It is optional since enabling access control for the content library is optional. It's up to content library admin to decide if
  // it is required to enable access control.
  // +optional
  LibraryCredentials string `json:"library_credentials"`
}
```

#### Implementation Details

##### VSphereVM clone changes

We need to enhance the [clone](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/19c5deb79e97a8cd9d546ec39db3f29231bbcb91/pkg/services/govmomi/vcenter/clone.go#L75) function to recogize the scheme we defined, and choose appropriate API to clone the VM.

There are no changes to reconciling logic of various controllers.

## Resources

* [Using Content Libraries](https://docs.vmware.com/en/VMware-vSphere/7.0/com.vmware.vsphere.vm_admin.doc/GUID-254B2CE8-20A8-43F0-90E8-3F6776C2C896.html)
* [Content Library API](https://developer.vmware.com/apis/vsphere-automation/latest/content/content/)
* [govc library clone](https://github.com/vmware/govmomi/blob/main/govc/library/clone.go)
