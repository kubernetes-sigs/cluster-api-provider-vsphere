# Support for Node anti-affinity

```text
---
title: Support for Node anti-affinity
authors:
  - "@srm09"
reviewers:
  - "@yastij"
creation-date: 2022-09-07
last-updated: 2022-10-07
status: proposed
---
```

## Table of Contents

* [Glossary](#glossary)
* [Summary](#summary)
* [Motivation](#motivation)
  * [Goals](#goals)
  * [Future Work](#future-work)
  * [Non-Goals](#non-goals)
* [Proposal](#proposal)
  * [User Stories](#user-stories)
    * [Story 1](#story-1)
    * [Story 2](#story-2)
    * [Story 3](#story-3)
  * [Requirements](#requirements)
    * [Functional Requirements](#functional-requirements)
    * [Non-Functional Requirements](#non-functional-requirements)
  * [Overall Design](#overall-design)
    * [Cluster Modules](#cluster-modules)
  * [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
    * [API changes](#api-changes)
      * [VSphereCluster CR](#vspherecluster-cr)
      * [VSphereVM CR](#vspherevm-cr)
    * [Implementation Details](#implementation-details)
      * [VSphereCluster controller changes](#vspherecluster-controller-changes)
      * [VSphereVM controller changes](#vspherevm-controller-changes)
      * [VSphereMachine controller changes](#vspheremachine-controller-changes)
      * [Node label controller changes](#node-label-controller-changes)
    * [Feature flags](#feature-flags)
  * [Upgrade Strategy](#upgrade-strategy)

## Glossary

**Node**:
A node is a worker machine in kubernetes and may be a physical or virtual machine. In this scenario it is a VM.

**Node pool**:
A node pool is a group of nodes within a Kubernetes cluster that have the same configuration.

**Affinity**:
Affinity between two nodes implies that they should share the same ESXI host.

**Anti-affinity (AAF)**:
Anti-affinity between two nodes implies that they should not share the same ESXI host.

**ESXi**:
VMware ESXi is a bare metal hypervisor that directly installs onto a physical server. With direct access and control of underlying resources, ESXI effectively partitions hardware to cut costs and consolidate applications.

**vCenter**:
VMware vCenter is a server application allowing the management of ESXI and VMs on ESXI.

**DRS**:
vSphere Distributed Resource Scheduler. Manage aggregated resources under one compute cluster. Its purpose is to optimize the performance of VMs. Based on automation level, DRS could automatically migrate VMs to other hosts within the compute cluster. The migration complies with user-defined affinity / anti-affinity rules.

## Summary

ControlPlane Failure Domain support is added in CAPI with KCP controller as the consumer of the Domains. CAPI cluster controller will copy infracluster.status.FailureDomains to CAPI cluster.status.FailureDomain. If FailureDomain is provided, KCP will try to distribute controlplane nodes across multiple failure domains, by creating a Machine object which has Machine.Spec.FailureDomain specified as the domain it should be created into.

Public clouds FailureDomains are well defined, usually are availability zones within a region. VSphere does not have a native region/zone concept. To utilize the k8s failure domain features which relies on nodes with region/zone labels, vsphere CPI/CSI added support for region/zone deployment options, which use vsphere tags to configure the region/zone topology. This approach gives users full flexibility to configure arbitrary region and zone definitions, which is different from public clouds where region/zones are all pre-defined.

For public cloud providers such as CAPA/CAPZ, the difference between Machines in different FailureDomains is only one parameter, AZ. VSphere has a lot more properties that could be different, such as datacenter/datastore/resourcepool/folder etc. So a single VSphereMachineTemplate for KCP is not able to provide the placement properties for all the FailureDomains. We need to provide those placement information within VSphereCluster.Spec.

## Motivation

Today, it is possible for two nodes to live on the same ESXI host. This means that if the ESXI host fails, both those nodes go down. This is an issue if all pod replicas of an application have been placed on those nodes, or if those nodes belong to the control plane.

This problem is especially important to solve for Telco customers, who have to ensure Carrier Grade redundancy for end user services.

**Why kubernetes zones do not address this issue:**

Customers can achieve application resiliency by implementing kubernetes zones, in that they can portion a cluster across multiple AZ regions. However, nodes in a node pool have to live in the same AZ (a node pool can only exist in one AZ). Customers schedule workloads at the node pool level (i.e., they spread their pod replicas across nodes in a node pool), and therefore, multi-AZ does not solve the issues for the customers in consideration, because it does not ensure that workers in the same node pool live on different hardware.

### Goals

* Avoid scheduling two control plane nodes of a given cluster on a single host, unless there are fewer hosts than control plane nodes.
* Avoid scheduling two worker nodes of a given machine deployment of a cluster on a single host, unless there are fewer hosts than nodes.
* Provide visibility as to which nodes are running on the same host within the cluster.
* Ensure that we have a path forward, as enhancements are made to vSphere.

### Future Work

* Ensure that worker nodes of a given machine deployment are spread as evenly as possible when there are more nodes than hosts.
* Allow a user to indicate that the nodes of two or more machine deployments should be considered together for the purpose of anti-affinity.

### Non goals

* Support for vSphere 6.7.
* Allow the user to express anti-affinity between nodes belonging to different clusters.
* Allow the user to express affinity between nodes, or between nodes and hosts.
* Optimize resource utilization of the hosts.

## Proposal

### User Stories

#### Story 1

As an admin, I’d like my Kubernetes control plane VMs to be placed on to different hosts within a single compute cluster to ensure the availability of the Kubernetes control plane in the evnt of host failures.

#### Story 2

As an admin, I’d like my Kubernetes nodes within a machine deployment to be placed on separate hosts within a single compute cluster.

#### Story 3

As a cluster user, the ESXi host information on which the VM is placed should be exposed on the Kubernetes node.

### Requirements

#### Functional Requirements

* FR1: For a cluster, control plane VMs must be placed on separate ESXi hosts when possible.
* FR2: For a cluster, worker VMs belonging to a machine deployment must be placed on separate ESXi hosts when possible.

#### Non-Functional Requirements

* NFR1: Unit tests MUST exist for all new APIs
* NFR2: e2e tests MUST exist to verify anti affinity.

### Overall Design

The design centers around using the vCenter construct of Cluster Modules _(described below)_ to ensure soft anti affinity constraints amongst VMs belonging to a single cluster-api (CAPI) custom resource.

#### Cluster Modules

Cluster modules is a construct that supports "soft" anti-affinity between its members. It tries to place the VMs on separate hosts, but allows violations if that cannot be possible due to limited infrastructure resources or other resource challenges such as hosts in maintenance mode. This is an internal API currently in use by Tanzu on Supervisor and is prone to API changes.

Following are the results on some experiments run for creating anti-affine nodes using cluster modules,

* When the number of nodes <= number of ESXi hosts, the VM placement strictly adheres anti-affine behavior.
* When the number of nodes > number of ESXi hosts, some VMs are stacked on the same ESXi hosts.

  * When the number of nodes are scaled down so that the number of ESXi hosts and nodes are the same, the VM placement is not automatically rebalanced to guarantee anti-affine behavior.

**Note**:

1. No support for affinity between VMs is supported.
2. VMotion needs to be enabled to make sure that VMs can be moved over as part of the rebalancing operations.
3. This API is supported in vCenter 7.0 and above.

### Implementation Details/Notes/Constraints

#### API changes

##### VSphereCluster CR

The `VSphereCluster` CR has the following changes:

The spec is used to hold the information of the cluster modules created for each CAPI object associated to the cluster capable of creating infrastructure machines. This slice is populated by the CAPV controllers and should not be edited by the user.

```go
type VSphereClusterSpec struct {
    ...
    // ClusterModules hosts information regarding the anti-affinity vSphere constructs
    //for each of the objects responsible for creation of VM objects belonging to the cluster.
    // +optional
    ClusterModules []ClusterModule `json:"clusterModules,omitempty"`
}

// ClusterModule holds the anti affinity construct `ClusterModule` identifier
// in use by the VMs owned by the object referred by the TargetObjectName field.
type ClusterModule struct {
    // ControlPlane indicates whether the referred object is responsible for control plane nodes.
    // Currently, only the KubeadmControlPlane objects have this flag set to true.
    // Only a single object in the slice can have this value set to true.
    ControlPlane bool `json:"controlPlane"`

    // TargetObjectName points to the object that uses the Cluster Module information to enforce
    // anti-affinity amongst its descendant VM objects.
    TargetObjectName string `json:"targetObjectName"`

    // ModuleUUID is the unique identifier of the `ClusterModule` used by the object.
    ModuleUUID string `json:"moduleUUID"`
}
```

The status adds a new field to expose the vCenter version details since the support for cluster modules is present in vCenter 7.0 and above.

```go
// VCenterVersion conveys the API version of the vCenter instance.
type VCenterVersion string

type VSphereClusterStatus struct {
    ...
    // VCenterVersion defines the version of the vCenter server defined in the spec.
    VCenterVersion VCenterVersion `json:"vCenterVersion,omitempty"`
}
```

##### VSphereVM CR

The `VSphereVM` CR has the following changes:

The status has a new variable which hold the UUID of the cluster module that the VM is a part of.

```go
type VSphereVMStatus struct {
    ...
    // Host describes the hostname or IP address of the infrastructure host
    // that the VSphereVM is residing on.
    // +optional
    Host string `json:"host,omitempty"`

    // ModuleUUID is the unique identifier for the vCenter cluster module construct
    // which is used to configure anti-affinity. Objects with the same ModuleUUID
    // will be anti-affined, meaning that the vCenter DRS will best effort schedule
    // the VMs on separate hosts.
    // +optional
    ModuleUUID *string `json:"moduleUUID,omitempty"`
}
```

#### Implementation Details

##### VSphereCluster controller changes

1. The cluster module reconciliation logic is gated by the vCenter version set on the object. The major version of the vCenter API should be >= 7. For clusters pointing to vCenter versions < 7, the cluster module reconcile logic is skipped entirely.
1. During each reconcile loop, the controller lists out all the CAPI objects (not marked for deletion) capable of creating VSphereMachine objects. Currently, this proposal only takes into consideration the `KubeadmControlPlane` and `MachineDeployment` objects.
1. For each of these objects,
   1. It checks whether a cluster module has already been created using the information contained in `.spec.clusterModules` slice.
   2. A new cluster module is created and the information is added to the `.spec.clusterModules` slice if the cluster module does not exist already.
1. For all the remaining entries in the `.spec.clusterModules` which do not have a CAPI object associated, either it does not exist or marked for deletion, the cluster module is deleted using the moduleUUID contained in the slice.

##### VSphereVM controller changes

1. The controller checks for the cluster module information set on the `VSphereCluster` object, using the name of the CAPI object it is owned by.
   1. For control plane nodes, the owner hierarchy is traced as **VSphereVM -> VSphereMachine -> Machine -> KubeadmControlPlane**
   2. For worker nodes, the owner hierarchy is traced as **VSphereVM -> VSphereMachine -> Machine -> MachineSet -> MachineDeployment**
1. For normal VM reconcile loop, the cluster module ID is used to add the VM to the cluster module.
1. For delete VM reconcile loop, the cluster module ID is used to remove the VM from the cluster module.
1. The information of the host that the VM is placed on is fetched and added to the `VSphereVMStatus` object.

##### VSphereMachine controller changes

1. The controller checks for the ESXi host information that the corresponding `VSphereVM` object is placed on.
1. This information is added to the Machine object via the label `node.cluster.x-k8s.io/esxi-host: <host-info>`

##### Node label controller changes

A new controller is added which is responsible for propagating the labels with a specific prefix `node.cluster.x-k8s.io` present on the `Machine` object to the Kubernetes node. This functionality will eventually be available in CAPI natively, at which point we can remove this controller.

#### Feature flags

Two new feature flags are introduced to control the availability of these new behaviors:

1. **NodeAntiAffinity** which is set to `false` by default. This controls the creation of cluster modules to dictate anti affinity for VM placement.
2. **NodeLabeling** which is set to `false` by default. This controls the propagation of labels with a special prefix from Machine to Node objects. Starting from v1.7.0 release, this feature flag is deprecated and this functionality will not be provided by CAPV. CAPI v1.4.0 natively supports this feature. Starting with v1.8.0 the feature gate has been removed.

## Upgrade Strategy

No inputs are needed from the ser when upgrading to a newer CAPV version. If the feature flag is enabled, the CAPV controllers automatically create and/or delete cluster modules corresponding to the CAPI objects related to the cluster.

This section calls out the behavior of the clusters in terms of CAPV and vSphere version upgrades. This is necessary since cluster modules, which is the building block for node anti-affinity is a vSphere 7.0 construct. The various upgrade scenarios would operate in the following fashion:

1. **User on vSphere 7 upgrades from 1.3 to 1.4** => Post CAPV upgrade, cluster modules will be created for each MD/KCP. This might lead to some vMotion events, which the VI admin in partial DRS mode and would be automatic in full DRS automation. No config change for anti-affinity is needed by the user.
1. **User on vSphere 6.7 upgrades from 1.3 to 1.4 and then upgrades to vSphere 7** => No anti-affinity changes would occur after the 1.4 installation, but the Cluster objects will show a condition that anti-affinity is not possible. Existing VM setup will not be altered. After vSphere 7.0 upgrade, cluster modules will start getting created eventually and some vMotion events will be seen as listed in point 1.
