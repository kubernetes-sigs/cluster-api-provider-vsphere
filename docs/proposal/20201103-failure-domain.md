# CAPV ControlPlane Failure Domain

```text
---
title: CAPV ControlPlane Failure Domain
authors:
  - "@jzhoucliqr"
  - "Ben Corrie"
  - "@abhinavnagaraj"
  - "@sadysnaat"
  - "@yastij"
reviewers:
  - "@yastij"
  - "@randomvariable"
  - "Tushar Aggarwal"
  - "max"
  - "scott"
creation-date: 2020-11-03
last-updated: 2020-11-03
status: implementable
---
```

## Table of Contents

* [CAPV ControlPlane Failure Domain](#capv-controlplane-failure-domain)
  * [Table of Contents](#table-of-contents)
  * [Glossary](#glossary)
  * [Summary](#summary)
  * [Motivation](#motivation)
    * [Goals](#goals)
    * [Non-Goals/Future Work](#non-goalsfuture-work)
  * [Proposal](#proposal)
    * [User Stories](#user-stories)
      * [Story 1](#story-1)
      * [Story 2](#story-2)
      * [Story 3](#story-3)
      * [Story 4](#story-4)
    * [Requirements](#requirements)
      * [Functional Requirements](#functional-requirements)
        * [CPI/CSI](#cpicsi)
        * [DRS](#drs)
        * [vMotion](#vmotion)
        * [Across multiple datacenter](#across-multiple-datacenter)
    * [Overall Design](#overall-design)
      * [For User Story 1](#for-user-story-1)
      * [For User Story 2/3](#for-user-story-23)
    * [API Design](#api-design)
    * [Implementation Details](#implementation-details)
    * [Notes/Constraints](#notesconstraints)
      * [Supported topology](#supported-topology)
        * [Region -&gt; Datacenter,  Zone -&gt; ComputeCluster](#region---datacenter--zone---computecluster)
        * [Region -&gt; Country , Zone -&gt; DataCenter](#region---country--zone---datacenter)
        * [Region -&gt; ComputeCluster, Zone -&gt; HostGroup](#region---computecluster-zone---hostgroup)
      * [Shall we support single cluster span across multiple regions](#shall-we-support-single-cluster-span-across-multiple-regions)
      * [What to set into VSphereCluster.Status.FailureDomains](#what-to-set-into-vsphereclusterstatusfailuredomains)
      * [Single Account or Multi Account (multi vcenter)](#single-account-or-multi-account-multi-vcenter)
      * [Single Network or across Multi Networks](#single-network-or-across-multi-networks)
      * [Static IP / Multi Nic](#static-ip--multi-nic)
  * [Upgrade Strategy](#upgrade-strategy)

## Glossary

**Failure Domains**: The infrastructure topology configured by vSphere admin, which represents the physical compute fault domains. Examples include datacenters, compute clusters, hostgroups etc.

**Placement Constraints**: The metadata context that adds further information to the vsphere resource scheduler about the way in which the VM is expected to be deployed within the context of the failure domain. Examples include resources pools, datastores, networks, folders.

**Region/Zone**: The abstraction topology created by attaching tags/labels to the failure domains, which is mainly consumed by Kubernetes (Scheduler/CPI/CSI). Examples includes user defined tags:  k8s-region-us-west / k8s-zone-us-west-az1

**DRS**: vSphere Distributed Resource Scheduler. Manage aggregated resources under one compute cluster. Based on automation level, DRS could automatically migrate VMs to other hosts within the compute cluster. The migration complies with user-defined affinity / anti-affinity rules.

**HostGroup & VMGroup & Affinity Rules**: Used to specify affinity or anti-affinity between a group of virtual machines and a group of hosts. An affinity rule specifies that the members of a selected virtual machine DRS group can or must run on the members of a specific host DRS group. An anti-affinity rule specifies that the members of a selected virtual machine DRS group cannot run on the members of a specific host DRS group.

## Summary

ControlPlane Failure Domain support is added in CAPI with KCP controller as the consumer of the Domains. CAPI cluster controller will copy infracluster.status.FailureDomains to CAPI cluster.status.FailureDomain. If FailureDomain is provided, KCP will try to distribute controlplane nodes across multiple failure domains, by creating a Machine object which has Machine.Spec.FailureDomain specified as the domain it should be created into.

Public clouds FailureDomains are well defined, usually are availability zones within a region. VSphere does not have a native region/zone concept. To utilize the k8s failure domain features which relies on nodes with region/zone labels, vsphere CPI/CSI added support for region/zone deployment options, which use vsphere tags to configure the region/zone topology. This approach gives users full flexibility to configure arbitrary region and zone definitions, which is different from public clouds where region/zones are all pre-defined.

For public cloud providers such as CAPA/CAPZ, the difference between Machines in different FailureDomains is only one parameter, AZ. VSphere has a lot more properties that could be different, such as datacenter/datastore/resourcepool/folder etc. So a single VSphereMachineTemplate for KCP is not able to provide the placement properties for all the FailureDomains. We need to provide those placement information within VSphereCluster.Spec.

## Motivation

Enterprise deployments need to distribute the nodes across multiple failure domains to improve system availability.

### Goals

* Failure domain support for controlplane nodes in CAPV using region/zone topology.
* Align with CPI/CSI for region/zone topology setup.
* Optionally configure region/zone topology for vsphere, if provided with enough permission

### Non-Goals/Future Work

* Failure domains across multiple vcenters
* Replace vsphere DRS for scheduling VM to a specific Host
* Failure domain support for MachineDeployment

## Proposal

### User Stories

#### Story 1

As an admin, I’d like my k8s control plane machines to be placed on to different host groups within a compute cluster. This is for the vsphere setup with one compute cluster which includes multiple fire compartments, each compartment is configured to be a hostgroup.

#### Story 2

As an admin, I’d like my k8s control plane machines to be able to span across multiple compute clusters within a single datacenter.

#### Story 3

As an admin, I’d like my k8s control plane machines to be able to span across multiple datacenters within a single vcenter.

#### Story 4

As an admin, I’d like to use one CAPV to manage multiple k8s clusters which could be deployed into different placement constraints.

### Requirements

#### Functional Requirements

##### CPI/CSI

When control plane machines are across multiple failure domains, when possible, CPI/CSI/Scheduler SHOULD be aware of these failure domains, so that the failure domains could be used to schedule pods/pvc.

##### DRS

CAPV SHOULD let DRS handle the placement (VM -> Host) based on placement constraints, with or without affinity/anti-affinity policies. CAPV should NOT pick a specific host for a VM.

##### vMotion

The k8s cluster SHOULD keep functional during automated or maintenance events with vMotion.

##### Across multiple datacenter

Networks will be different across multiple datacenters. Need to either configure BGP with kube-vip or use an external LB instead of kube-vip.

### Overall Design

A CAPV mgmt level CRD (VSphereDeploymentZone) contains detailed placement constraints for each of the failure domain that this CAPV instance can deploy VMs into.

For a single k8s cluster, within one Zone, we don’t expect for VMs to be placed into different placement constraints. But for different k8s clusters managed by a single CAPV, they could be able to deploy into different placement constraints.

Two options to provide different placement constraints within one zone:

* Pre-define all available placements as CRs, VSphereCluster refer to the CR that it plan to deploy into
* VSphereCluster optionally embed detailed placements constraints

After discussions within the community, we propose to use option 1,  for the following benefits:

* Failure domains and the placement constraints are defined in a single place
* Failure domains feeded into CAPV are decoupled from the infrastructure, with this CR
* Potentially a separate controller with higher level permissions could do discovery/validation, or even the configuration of the failure domains on the CR

#### For User Story 1

Failure domains are host groups within a single compute cluster. Hostgroups need to be pre-configured by vSphere admin.

CAPV will create vm groups matching the host groups, then create vm-host affinity rules between vmgroups and hostgroups, add the CP node to the specific vm group, DRS will schedule the VM to the corresponding hostgroup.

CSI/CPI will NOT be aware of the failure domains, so no failure domain labels will be added to nodes and PVs.

For PV attachment to work successfully in this case, it is expected that all the host groups should have access to the same shared datastore.

#### For User Story 2/3

Failure domains are compute clusters within a single datacenter, or datacenters within one vcenter.

vSphere Admin need to pre-configure the region/zone topology using tags. (examples shown below) CAPV only place VMs into the resourcepool, and let DRS choose the host. There is no affinity rules or vmgroups needed.

CSI/CPI will be configured with region/zone information so nodes and PVs will have correct failure domain labels.

### API Design

```go
type FailureDomainType string

const (
  HostGroupFailureDomain      FailureDomainType = "HostGroup"
  ComputeClusterFailureDomain FailureDomainType = "ComputeCluster"
  DatacenterFailureDomain     FailureDomainType = "Datacenter"
)

// VSphereFailureDomainSpec defines the desired state of VSphereFailureDomain
type VSphereFailureDomainSpec struct {

  // Region defines the name and type of a region
  Region FailureDomain `json:"region"`

  // Zone defines the name and type of a zone
  Zone FailureDomain `json:"zone"`

  // Topology is the what describes a given failure domain using vSphere constructs
  Topology Topology `json:"topology"`
}

type FailureDomain struct {
  // Name is the name of the tag that represents this failure domain
  Name string `json:"name"`

  // Type is the type of failure domain, the current values are "Datacenter", "ComputeCluster" and "HostGroup"
  // +kubebuilder:validation:Enum=Datacenter;ComputeCluster;HostGroup
  Type FailureDomainType `json:"type"`

  // TagCategory is the category used for the tag
  TagCategory string `json:"tagCategory"`

  // AutoConfigure tags the Type which is specified in the Topology
  AutoConfigure *bool `json:"autoConfigure,omitempty"`
}

type Topology struct {
  // The underlying infrastructure for this failure domain
  // Datacenter as the failure domain
  Datacenter string `json:"datacenter"`

  // ComputeCluster as the failure domain
  // +optional
  ComputeCluster *string `json:"computeCluster,omitempty"`

  // HostGroup as the failure domain
  // +optional
  HostGroup *FailureDomainHostGroup `json:"hostGroup,omitempty"`
}

// FailureDomainHostGroup as the failure domain
type FailureDomainHostGroup struct {
  // name of the host group
  Name string `json:"name"`

  // compute cluster that this hostgroup belongs to
  // +optional
  AutoConfigure *bool `json:"autoConfigure,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vspherefailuredomains,scope=Cluster,categories=cluster-api

// VSphereFailureDomain is the Schema for the vspherefailuredomains API
type VSphereFailureDomain struct {
  metav1.TypeMeta   `json:",inline"`
  metav1.ObjectMeta `json:"metadata,omitempty"`

  Spec VSphereFailureDomainSpec `json:"spec,omitempty"`
}

```

we'll also introduce the concept of `VSphereDeploymentZone` which allows us to reference `VSphereFailureDomains`
and combine it with `PlacementConstrains`

```go
// VSphereDeploymentZoneSpec defines the desired state of VSphereDeploymentZone
type VSphereDeploymentZoneSpec struct {

  // Server is the address of the vSphere endpoint.
  Server string `json:"server,omitempty"`

  // failureDomain is the name of the VSphereFailureDomain used for this VSphereDeploymentZone
  FailureDomain string `json:"failureDomain,omitempty"`

  // ControlPlane determines if this failure domain is suitable for use by control plane machines.
  // +optional
  ControlPlane *bool `json:"controlPlane,omitempty"`

// the placement constraints which is used within this failure domain
  PlacementConstaint PlacementConstraint `json:"placementConstraint"`
}

// PlacementConstraint is the context information for VM placements within a failure domain
type PlacementConstraint struct {
  // ResourcePool is the name or inventory path of the resource pool in which
  // the virtual machine is created/located.
  // +optional
  ResourcePool string `json:"resourcePool,omitempty"`

  // Datastore is the name or inventory path of the datastore in which the
  // virtual machine is created/located.
  // +optional
  Datastore string `json:"datastore,omitempty"`

  // Network represents the networking for this depoyment zone
  // +optional
  Network []Network `json:"Network,omitempty"`

  // Folder is the name or inventory path of the folder in which the
  // virtual machine is created/located.
  // +optional
  Folder string `json:"folder,omitempty"`
}

type Network struct {
  // NetworkName is the network name for this machine's VM.
  NetworkName string `json:"networkName,omitempty"`

  // DHCP4 is a flag that indicates whether or not to use DHCP for IPv4
  // +optional
  DHCP4 *bool `json:"dhcp4,omitempty"`

  // DHCP6 indicates whether or not to use DHCP for IPv6
  // +optional
  DHCP6 *bool `json:"dhcp6,omitempty"`
}

type VSphereDeploymentZoneStatus struct {
  // Ready is true when the VSphereDeploymentZone resource is ready.
  // If set to false, it will be ignored by VSphereClusters
  // +optional
  Ready *bool `json:"ready,omitempty"`

  // Conditions defines current service state of the VSphereMachine.
  // +optional
  Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vspheredeploymentzones,scope=Cluster,categories=cluster-api
// +kubebuilder:subresource:status

// VSphereDeploymentZone is the Schema for the vspheredeploymentzones API
type VSphereDeploymentZone struct {
  metav1.TypeMeta   `json:",inline"`
  metav1.ObjectMeta `json:"metadata,omitempty"`

  Spec   VSphereDeploymentZoneSpec   `json:"spec,omitempty"`
  Status VSphereDeploymentZoneStatus `json:"status,omitempty"`
}
```

### Implementation Details

the changes required to the controllers are the following

#### VSphereFailureDomain validating webhook

the validation for `VSphereFailureDomain` should:

* verify that the `.Spec.Topology.Datacenter` is not empty
* if `.Spec.Topology.HostGroup` is not `nil`, verify that `autoconfigure` is not true for both the `hostGroup` and `FailureDomain` struct

#### VSphereDeploymentZone defaulting webhook

the defaultijg for `VSphereDeploymentZone` should:

* check if `controlPlane` is `nil`, if it is default to true

#### the vspheredeploymentzone_controller

This controller would be responsible for:

* Listing `VSphereClusters` and checking if `server` field matches
  * pickup the first matching `VSphereCluster` and extract credentials through reading `.spec.identityRef`
  * No `VSphereCluster` is matching, this means that we fallback to the `capv-controller-manager` credentials
* Verifying the following:
  * being able to create a session
  * the compute cluster exists and has the specified resource pool
  * the network, datastore and folder all exist
  * if autoconfigure is `false` verify that the hostGroup exists
  * if autoconfigure is `false` verify that tags on the elements (compute cluster, datacenter or Hosts) exist
* Depending on autoconfiguration enablement (only ONE of the following can be done):
  * Create the hostGroup and add the tagged hosts
  * List the hosts within the hostGroup and tag them accordingly
* set `.status.Ready` when all of the above is done

note: `.status.Ready` should remain nil, unless we deem the `VSphereDeploymentZone` not ready for consumption

#### the vspherecluster_controller

the following changes are going to be introduced to vspherecluster_controller:

* List the `VSphereDeploymentZones` and match based on `.spec.server`
* check `.status.Ready`
  * if `.status.Ready` is `nil`, add it to the failureDomain map, but  skip setting `.status.InfrastructureReady` on the `VSphereCluster`
  * if `.status.Ready` is `true` add it to the failureDomain map
  * if `.status.Ready` is `false` skip adding `VSphereDeploymentZone` to the map
* copy `.spec.controlPlane` of `VSphereDeploymentZone` into the failureDomain map value
* Before setting `.status.InfrastructureReady` ensure that no matched `VSphereFailureDomain` has `.status.Ready` set to `nil`
* Add a condition to represent the controller waiting for a failure domain to be ready
* Add a condition to represent the controller skipping a matching failure domain
* Add a condition to represent the controller listing all the matched failure domains

#### the vspheremachine_controller

the following changes are going to be introduced to vspheremachine_controller:

* if `.spec.failureDomain` is set:
  * fetch the `VSphereDeploymentZone` that has `.spec.failureDomain` as a name
  * use that to populate the values of the `VSphereVM`
* if `.spec.failureDomain` is not set
  * fallback to reading from the vspheremachine itself

### Notes/Constraints

#### Supported topology

[Example](https://cloud-provider-vsphere.sigs.k8s.io/tutorials/deploying_cpi_and_csi_with_multi_dc_vc_aka_zones.html)

##### Region -> Datacenter,  Zone -> ComputeCluster

```shell
#dcwest,az1
govc tags.attach k8s-region k8s-region-west /dc-west/host/cluster-az1
govc tags.attach k8s-zone k8s-zone-west-1 /dc-west/host/cluster-az1

#dcwest,az2
govc tags.attach k8s-region k8s-region-west /dc-west/host/cluster-az2
govc tags.attach k8s-zone k8s-zone-west-2 /dc-west/host/cluster-az2

#dceast,az1
govc tags.attach k8s-region k8s-region-east /dc-east/host/cluster-az1
govc tags.attach k8s-zone k8s-zone-east-1 /dc-east/host/cluster-az1
```

##### Region -> Country , Zone -> DataCenter

```shell
#dcwest
govc tags.attach k8s-region k8s-region-us /dc-west
govc tags.attach k8s-zone k8s-zone-us-west /dc-west
#dceast
govc tags.attach k8s-region k8s-region-us /dc-east
govc tags.attach k8s-zone k8s-zone-us-east /dc-east
#dceu
govc tags.attach k8s-region k8s-region-eu /dc-eu
govc tags.attach k8s-zone k8s-region-eu-all /dc-eu
```

##### Region -> ComputeCluster, Zone -> HostGroup

```shell
#no tag required
#hostgroups need to be pre-configured
#CAPV need permissions to create vm groups and affinity rules
```

#### Shall we support single cluster span across multiple regions

NO  -> FailureDomain is only for across Zones within a Region

#### What to set into VSphereCluster.Status.FailureDomains

VSphereCluster.Status.FailureDomains will just contain an array of names of VSphereDeploymentZone.

#### Single Account or Multi Account (multi vcenter)

Single account (single vcenter).

#### Single Network or across Multi Networks

If use kube-vip for HA, then multiple failure domains should share a single network.

If use external LB for HA, then there is no such constraint.

#### Static IP / Multi Nic

Should continue to work

## Upgrade Strategy

No Upgrade needed.
Failure domain is optional. Existing clusters do not have failure domain configured, which will continue to work without changes.
