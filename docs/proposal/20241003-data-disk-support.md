# CAPV Additional Data Disks When Creating New Machines

```text
---
title: CAPV Additional Data Disks When Creating New Machines
authors:
  - "@vr4manta"
reviewers:
  - "@chrischdi"
  - "@neolit123"
creation-date: 2024-10-03
last-updated: 2024-10-03
status: implementable
---
```

## Table of Contents

* [Glossary](#glossary)
* [Summary](#summary)
* [Motivation](#motivation)
    * [Goals](#goals)
    * [Non-Goals/Future Work](#non-goalsfuture-work)
* [Proposal](#proposal)
    * [User Stories](#user-stories)
        * [Story 1](#story-1)
        * [Story 2](#story-2)
    * [Requirements](#requirements)
        * [Functional Requirements](#functional-requirements)
    * [Overall Design](#overall-design)
        * [For User Story 1](#for-user-story-1)
        * [For User Story 2](#for-user-story-2)
    * [API Design](#api-design)
    * [Implementation Details](#implementation-details)
    * [Notes/Constraints](#notesconstraints)
* [Upgrade Strategy](#upgrade-strategy)

## Glossary


## Summary

As the use of Kubernetes clusters grows, admins are needing more and more improvements to the VMs themselves to make sure they run as smoothly as possible.  The number of cores and memory continue to increase for each machine and this is causing the amount of workloads to increase on each virtual machine.  This growth is now causing the base VM image to not provide enough storage for OS needs.  In some cases, users just increase the size of the primary disk using the existing configuration options for machines; however, this does not allows for all desired configuration choices.  Admins are now wanting the ability to add additional disks to these VMs for things such as etcd storage, image storage, container runtime and even swap.  

Before this feature, CAPV only allows for the configuration of additional disks that are found within the vSphere VM Template that is used for cloning.  As clusters stretch failure domains and as clusters contain multiple different machine sets, attempting to just create custom vSphere VM templates will cause the admin to have to manage a large number of vSphere OVA templates.  Instead, it would be ideal if admins can just configure a machine or machineset to add additional disks to a VM during the cloning process that are not part of the template.

This feature enhancement aims to allow admins the ability to configure additional disks that are not present in the VM templates by enhancing the vsphere machine API and adding the capability to the cloning process.

## Motivation

Cluster administrators are asking for the ability to add additional data disks to be used by the OS without having to create custom OVA images to be used by the VM cloning process.

### Goals

* Add new configuration for machines that are used to define new data disks to add to a VM.
* Align new property naming to be similar or even match other providers if possible.
* Do not boil the ocean with the initial implementation of this feature.

### Non-Goals/Future Work

* Add ability to advance configure additions disks, such as, define controller type (IDE, scsi, etc) or unit number in the controller.
* Any disk management features such as encryption.

## Proposal

### User Stories

#### Story 1

As an admin, I’d like my control plane machines to have an extra disk added so I can dedicate that disk for etcd database through my own means of mount management.

#### Story 2

As an admin, I’d like my compute machine set to have an extra disk added to each VM so that I can use it as a dedicated disk for container image storage.

### Requirements

#### Functional Requirements

### Overall Design

The CAPV vSphere machine clone spec (VirtualMachineCloneSpec) contains a new array field to specify all additional data disks (dataDisks).  These disks will be created and added to the VM during the clone process.

Once the machine has been created, any change to the DataDisks field will follow existing change behaviors for machine sets and single machine definitions.

CAPV will not be responsible for any custom mounting of the data disks.  CAPV will create the disk and attach to the VM.  Each OS may assign them differently so admin will need to understand the OS image they are using for their template and then configure each node.

#### For User Story 1

CAPV will create the new disks for control plane nodes during cluster creation.  

The new disks will be placed in the same location as the primary disk (datastore) and will use the controller as the primary disk.  

Creating new controllers is out of scope for this enhancement.  The new disk will use the same controller as the primary disks.  The unit number for the disks on that controller will be in the order in which the disks are defined in the machine spec configuration.

#### For User Story 2

CAPV will treat all compute machine creation the same as the control plane machines.

### API Design

The following new struct has been created to represent all configuration for a vSphere disk

```go
// VSphereDisk describes additional disks for vSphere to be added to VM that are not part of the VM OVA template.
type VSphereDisk struct {
    // Name is used to identify the disk definition. If Name is not specified, the disk will still be created.
    // The Name should be unique so that it can be used to clearly identify purpose of the disk, but is not
    // required to be unique.
    // +optional
    Name string `json:"name,omitempty"`
    // SizeGiB is the size of the disk (in GiB).
    // +kubebuilder:validation:Required
    SizeGiB int32 `json:"sizeGiB"`
    // ProvisioningMode specifies the provisioning type to be used by this vSphere data disk.
    // If not set, the setting will be provided by the default storage policy.
    // +optional
    ProvisioningMode ProvisioningMode `json:"provisioningMode,omitempty"`
}
```

Provisioning type currently will be represented by the following configuration options:

```go
type ProvisioningType string

var (
    // ThinProvisioningMode creates the disk using thin provisioning. This means a sparse (allocate on demand)
    // format with additional space optimizations.
    ThinProvisioningMode ProvisioningMode = "Thin"
	
    // ThickProvisioningMode creates the disk with all space allocated.
    ThickProvisioningMode ProvisioningMode = "Thick"

    // EagerlyZeroedProvisioningMode creates the disk using eager zero provisioning. An eager zeroed thick disk
    // has all space allocated and wiped clean of any previous contents on the physical media at
    // creation time. Such disks may take longer time during creation compared to other disk formats.
    EagerlyZeroedProvisioningMode ProvisioningMode = "EagerlyZeroed"
)
```

The above type has been added to the machine spec section

```go
// VirtualMachineCloneSpec is information used to clone a virtual machine.
type VirtualMachineCloneSpec struct {
    ...
    // DataDisks holds information for additional disks to add to the VM that are not part of the VM's OVA template.
    // +optional
    DataDisks []VSphereDisk `json:"dataDisks,omitempty"`
}
```

### Implementation Details

The cloning process will now be able to add data disks to a VM.  Using the config provided in the VSphereCloneTemplate, the defined disks will be added to the virtual machine.  Each disk is added to the controller used by the primary disk.  Currently, there are no plans to allow the user to define a new controller (SCSI/IDE/etc) for these disks. 

An example of what the VSphereMachineTemplate looks like when data disks are desired:
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereMachineTemplate 
metadata:
  name: test-machine-template
  namespace: openshift-cluster-api
spec:
  template:
    spec: 
      template: CAPV_2_Disk
      server: vcenter.example.com
      diskGiB: 128
      dataDisks:
      - name: images
        sizeGiB: 50
        provisioningMode: Thin
      - name: swap
        sizeGiB: 90
        provisioningMode: Thick
      cloneMode: linkedClone
      datacenter: cidatacenter
      datastore: /cidatacenter/datastore/vsanDatastore
      folder: /cidatacenter/vm/multi-disk-k96l6
      resourcePool: /cidatacenter/host/cicluster
      numCPUs: 4
      memoryMiB: 16384
      network:
        devices:
        - dhcp4: true
          networkName: ci-vlan-1240
```

An example of what the VSphereMachine looks like when data disks are desired.
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereMachine
metadata:
  name: ngirard-multi-qxkhh-single-def
  namespace: openshift-cluster-api
spec:
  cloneMode: linkedClone
  dataDisks:
  - name: images
    sizeGiB: 50
    provisioningMode: Thin
  - name: swap
    sizeGiB: 90
    provisioningMode: Thick
  datacenter: cidatacenter
  datastore: /cidatacenter/datastore/vsanDatastore
  diskGiB: 128
  folder: /cidatacenter/vm/ngirard-multi-qxkhh
  memoryMiB: 16384
  network:
    devices:
    - dhcp4: true
      networkName: ci-vlan-1240
  numCPUs: 4
  resourcePool: /cidatacenter/host/cicluster
  server: vcenter.ci.ibmc.devcluster.openshift.com
  template: CAPV_2_Disk

```

In the above examples, two data disks will be created during the clone process and placed after the OS disk.  The example shows a 50 GiB with name `images` that will be thin provisioned and a 90 GiB disk with name `swap` that will be thick provisioned being configured and added.

For each `dataDisks` definition, the clone procedure will attempt to generate a device VirtualDeviceConfigSpec that will be used to create the new disk device.  Each disk will be attached in the order in which they are defined in the template.  All disks defined in the vSphere OVA template will come first with the new disks being attached after.

The clone procedure will assign each new disk to the same controller being used by the primary (OS) disk of the OVA template.  The current behavior of the clone procedure will not be able to create any new controllers; however, in the future, the template may be enhanced to define new controllers and then assign them during the disk creation portion of the clone procedure. 

### Notes/Constraints

## Upgrade Strategy

Data disks are optional. Existing clusters will continue to work without changes.  Once existing clusters are upgraded, the new feature will be available for existing machines to be reconfigured / recreated.  

### MachineSets

Existing machineset, that wish to take advantage of the new data disk feature, will need to be reconfigured with data disks and will need to rotate in the new machines.  The modification behavior of all existing custom resources will remain the same.  If an administrator wishes to add data disks to a MachineSet, they need to recreate the existing referenced VSphereMachineTemplate or create a new VSphereMachineTemplate and have the MachineSet reference that new one.

### VSphereMachineTemplates

If an existing template is modified, you will be greeted by an error message similar to the following:

>vspheremachinetemplates.infrastructure.cluster.x-k8s.io "test-machine-template" was not valid:
spec.template.spec: Invalid value: v1beta1.VSphereMachineTemplate{TypeMeta:v1.TypeMeta{Kind:"VSphereMachineTemplate", APIVersion:"infrastructure.cluster.x-k8s.io/v1beta1"}, ObjectMeta:v1.ObjectMeta{Name:"test-machine-template", GenerateName:"", Namespace:"openshift-cluster-api", SelfLink:"", UID:"7467ba3f-758e-4873-afc3-4b8ece9da737", ResourceVersion:"92227", Generation:2, CreationTimestamp:time.Date(2024, time.October, 31, 14, 35, 9, 0, time.Local), DeletionTimestamp:<nil>, DeletionGracePeriodSeconds:(*int64)(nil), Labels:map[string]string(nil), Annotations:map[string]string(nil), OwnerReferences:[]v1.OwnerReference{v1.OwnerReference{APIVersion:"cluster.x-k8s.io/v1beta1", Kind:"Cluster", Name:"ngirard-multi-qxkhh", UID:"7c3030d9-8caa-40e2-a62c-db16d0749fb5", Controller:(*bool)(nil), BlockOwnerDeletion:(*bool)(nil)}}, Finalizers:[]string(nil), ManagedFields:[]v1.ManagedFieldsEntry{v1.ManagedFieldsEntry{Manager:"kubectl-create", Operation:"Update", APIVersion:"infrastructure.cluster.x-k8s.io/v1beta1", Time:time.Date(2024, time.October, 31, 14, 35, 9, 0, time.Local), FieldsType:"FieldsV1", FieldsV1:(*v1.FieldsV1)(0xc000ace450), Subresource:""}, v1.ManagedFieldsEntry{Manager:"cluster-api-controller-manager", Operation:"Update", APIVersion:"infrastructure.cluster.x-k8s.io/v1beta1", Time:time.Date(2024, time.October, 31, 14, 35, 58, 0, time.Local), FieldsType:"FieldsV1", FieldsV1:(*v1.FieldsV1)(0xc000ace4e0), Subresource:""}, v1.ManagedFieldsEntry{Manager:"kubectl-edit", Operation:"Update", APIVersion:"infrastructure.cluster.x-k8s.io/v1beta1", Time:time.Date(2024, time.November, 4, 13, 31, 33, 0, time.Local), FieldsType:"FieldsV1", FieldsV1:(*v1.FieldsV1)(0xc000ace510), Subresource:""}}}, Spec:v1beta1.VSphereMachineTemplateSpec{Template:v1beta1.VSphereMachineTemplateResource{ObjectMeta:v1beta1.ObjectMeta{Labels:map[string]string(nil), Annotations:map[string]string(nil)}, Spec:v1beta1.VSphereMachineSpec{VirtualMachineCloneSpec:v1beta1.VirtualMachineCloneSpec{Template:"CAPV_2_Disk", CloneMode:"linkedClone", Snapshot:"", Server:"vcenter.ci.ibmc.devcluster.openshift.com", Thumbprint:"", Datacenter:"cidatacenter", Folder:"/cidatacenter/vm/ngirard-multi-qxkhh", Datastore:"/cidatacenter/datastore/vsanDatastore", StoragePolicyName:"", ResourcePool:"/cidatacenter/host/cicluster", Network:v1beta1.NetworkSpec{Devices:[]v1beta1.NetworkDeviceSpec{v1beta1.NetworkDeviceSpec{NetworkName:"ci-vlan-1240", DeviceName:"", DHCP4:true, DHCP6:false, Gateway4:"", Gateway6:"", IPAddrs:[]string(nil), MTU:(*int64)(nil), MACAddr:"", Nameservers:[]string(nil), Routes:[]v1beta1.NetworkRouteSpec(nil), SearchDomains:[]string(nil), AddressesFromPools:[]v1.TypedLocalObjectReference(nil), DHCP4Overrides:(*v1beta1.DHCPOverrides)(nil), DHCP6Overrides:(*v1beta1.DHCPOverrides)(nil), SkipIPAllocation:false}}, Routes:[]v1beta1.NetworkRouteSpec(nil), PreferredAPIServerCIDR:""}, NumCPUs:4, NumCoresPerSocket:0, MemoryMiB:16384, DiskGiB:128, AdditionalDisksGiB:[]int32(nil), CustomVMXKeys:map[string]string(nil), TagIDs:[]string(nil), PciDevices:[]v1beta1.PCIDeviceSpec(nil), OS:"", HardwareVersion:"", DataDisks:[]v1beta1.VSphereDisk{v1beta1.VSphereDisk{Name:"", SizeGiB:69}, v1beta1.VSphereDisk{Name:"", SizeGiB:27}}}, ProviderID:(*string)(nil), FailureDomain:(*string)(nil), PowerOffMode:"hard", GuestSoftPowerOffTimeout:(*v1.Duration)(nil)}}}}: VSphereMachineTemplate spec.template.spec field is immutable. Please create a new resource instead.

This is why existing templates will need to be recreated if attempting to add data disks to the definition.
