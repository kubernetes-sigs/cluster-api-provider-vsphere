# CAPV IPAM Support

```text
---
title: CAPV IPAM Support
authors:
  - "@christianang"
  - "@flawedmatrix"
  - "@adobley"
  - "@tylerschultz"
reviewers:
  - "@yastij"
  - "@srm09"
  - "@vrabbi"
  - "@schrej"
creation-date: 2022-9-13
last-updated: 2022-10-11
status: Implementable
---
```

## Table of Contents

- [CAPV IPAM Support](#capv-ipam-support)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals/Future Work](#non-goalsfuture-work)
  - [Proposal](#proposal)
    - [User Stories](#user-stories)
      - [Story 1](#story-1)
      - [Story 2](#story-2)
      - [Story 3](#story-3)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
      - [API Changes](#api-changes)
      - [VSphereVM Controller Changes](#vspherevm-controller-changes)
      - [Owner Reference and Finalizer](#owner-reference-and-finalizer)
      - [Flavors](#flavors)
      - [Validation](#validation)
      - [Nameservers](#nameservers)
    - [Security Model](#security-model)
    - [Risks and Mitigations](#risks-and-mitigations)
  - [Alternatives](#alternatives)
  - [Upgrade Strategy](#upgrade-strategy)
  - [Additional Details](#additional-details)
    - [Test Plan](#test-plan)
    - [Graduation Criteria](#graduation-criteria)
    - [Version Skew Strategy](#version-skew-strategy)
  - [References](#references)
  - [Implementation History](#implementation-history)

## Glossary

Refer to the [Cluster API IPAM Integration Glossary](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220125-ipam-integration.md#glossary)

## Summary

This proposal adds CAPV support for the IPAM API contract described in the
[IPAM Integration
proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220125-ipam-integration.md).
CAPV will be able to allocate/release IP Addresses from an IPAM provider's IP
Pool to be used by `VSphereMachine`s.

## Motivation

The initial IPAM integration proposal explains the motivation for IPAM pretty well:

> IP address management for machines is currently left to the infrastructure
> providers for CAPI. Most on-premise providers (e.g. CAPV) allow the use of
> either DHCP or statically configured IPs. Since Machines are created from
> templates, static allocation would require a single template for each machine,
> which prevents dynamic scaling of nodes without custom controllers that create
> new templates.
>
> While DHCP is a viable solution for dynamic IP assignment which also enables
> scaling, it can cause problems in conjunction with CAPI. Especially in smaller
> networks rolling cluster upgrades can exhaust the network in use. When multiple
> machines are replaced in quick succession, each of them will get a new DHCP
> lease. Unless the lease time is very short, at least twice as many IPs as the
> maximum number of nodes has to be allocated to a cluster.
>
> Metal3 has an ip-address-manager component that allows for in-cluster
> management of IP addresses through custom resources, allowing to avoid DHCP.
> CAPV allows to omit the address from the template while having DHCP disabled,
> and will wait until it is set manually, allowing to implement custom
> controllers to take care of IP assignments. At DTAG we've extended metal3's
> ip-address-manager and wrote a custom controller for CAPV to integrate both
> with Infoblox, our IPAM solution of choice. The CAPV project has shown
> interest, and there has been a proposal to allow integrating metal3's
> ip-address-manager with external systems.
>
> All on-premise providers have a need for IP Address Management, since they
> can't leverage SDN features that are available in clouds such as AWS, Azure or
> GCP. We therefore propose to add an API contract to CAPI, which allows
> infrastructure providers to integrate with different IPAM systems. A similar
> approach to Kubernetes' PersistentVolumes should be used where infrastructure
> providers create Claims that reference a specific IP Pool, and IPAM providers
> fulfill claims to Pools managed by them.

### Goals

- Implement support for the IPAM API contract in CAPV
- Support both IPV4 and IPv6
- Support dual-stack
- Allow dynamically allocating and releasing addresses
- Allow running multiple IPAM providers in parallel
- Support running any IPAM provider

### Non-Goals/Future Work

- Configuring nameservers from Pool
- Handling updates to the devices on `VSphereMachine` objects
- Handling updates to the `addressesFromPools` list on individual devices
- User direct manipulation of `IPAddressClaim`s
- IP Address Management – this is the job of the IPAM provider
- Support for specific IPs for specific machines - a machine will receive a
  random IP in the pool. Implementation of this feature can be left to
  future work.

## Proposal

Following the contract specified by the IPAM integration proposal we plan to:

- Add the `addressesFromPools` field to the `spec.network.devices` section of
  the `VSphereMachine` and `VSphereMachineTemplate` CRDs. The
  `addressesFromPools` field will allow a user to specify that the network
  device's IP address should be claimed from an IP pool.
- If a `VSphereMachine` has been specified to claim an IP address from an
  IP pool, then CAPV will create an `IPAddressClaim`, which the IPAM provider
  will use to allocate an `IPAddress`. This `IPAddress` can then be used by the
  `VSphereMachine`.

### User Stories

#### Story 1

As a user, I want to be able to create a `VSphereMachine` where the IP address
for the VM comes from an IPAM Provider instead of from DHCP.

#### Story 2

As a user I want to be able to allocate either IPv4 or IPv6 addresses for my
`VSphereMachine`.

#### Story 3

As a user I want to be able to release my IP address allocation when the
`VSphereMachine` is deleted for future `VSphereMachine`s to use.

### Implementation Details/Notes/Constraints

#### API Changes

Adding `addressesFromPools` to the `VSphereMachineTemplate`

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: VSphereMachineTemplate
metadata:
  name: example
  namespace: vsphere-site1
spec:
  template:
    spec:
      cloneMode: FullClone
      numCPUs: 8
      memoryMiB: 8192
      diskGiB: 45
      network:
        devices:
        - dhcp4: false
          dhcp6: false
          networkName: VM Network
          addressesFromPools:
            # reference to the pool
            - apiGroup: ipam.cluster.x-k8s.io
              kind: InClusterIPPool
              name: testpool4
            - apiGroup: ipam.cluster.x-k8s.io
              kind: InClusterIPPool
              name: testpool6
```

Add `AddressesFromPools` to the `NetworkDeviceSpec`:

```go
// NetworkDeviceSpec defines the network configuration for a virtual machine's
// network device.
type NetworkDeviceSpec struct {
 // FromPools is a list of references to the pools from which IP addresses should be claimed.
 AddressesFromPools []corev1.TypedLocalObjectReference `json:"addressesFromPools,omitempty"`
}
```

#### VSphereVM Controller Changes

1. If the `addressesFromPools` field is set on a device then this controller
   will create a new `IPAddressClaim` for each pool configured on the device.
   This `IPAddressClaim` will be created in the same namespace as the
   `VSphereVM` / `VSphereMachine` / `VSphereMachineTemplate`. The `IPAddressClaim`
   will be named according to the device index and the pool index in the
   `VSphereVM` spec. The controller will not continue the VM creation process
   until all `IPAddressClaim`s have been fulfilled.

    ```yaml
    apiVersion: ipam.cluster.x-k8s.io/v1alpha1
    kind: IPAddressClaim
    metadata:
      name: "example-claim-0-0"
      namespace: "vsphere-site1"
      finalizers: ["ipam.cluster.x-k8s.io/ip-claim-protection"]
      ownerReferences:
      - apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1"
        kind: "VSphereVM"
        name: "prod-fdftj"
        uid: "097ccb1e-959f-4357-b0bf-cb3eccb2a185"
    spec:
      poolRef:
        apiGroup: ipam.cluster.x-k8s.io
        kind: InClusterIPPool
        name: testpool4
    status:
      addressRef: null
    ```

1. It will set the `VSphereVM` Status to reflect that an `IPAddressClaim`(s)
   has been created and that it is waiting for an `IPAddress`(es) to be bound
   to the claim(s). The Status should include information on how many addresses
   are bound out of how many are requested:

    ```yaml
    status:
      conditions:
        - lastTransitionTime: "2022-09-14T14:00:00Z"
          type: IPAddressClaimed
          status: "False"
          reason: "WaitingForIPAddress"
          message: "Waiting for IPAddressClaim to have an IPAddress bound."
    ```

1. The controller would watch `IPAddressClaim`s. When an `IPAddress` is bound
   to the claim it can continue the reconciliation of the `VSphereVM`.

    ```yaml
    ---
    apiVersion: ipam.cluster.x-k8s.io/v1alpha1
    kind: IPAddressClaim
    metadata:
      name: "example-claim-0-0"
      namespace: "vsphere-site1"
      finalizers: ["ipam.cluster.x-k8s.io/ip-claim-protection"]
      ownerReferences:
      - apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1"
        kind: "VSphereVM"
        name: "prod-fdftj"
        uid: "097ccb1e-959f-4357-b0bf-cb3eccb2a185"
    spec:
      poolRef:
        apiGroup: ipam.cluster.x-k8s.io
        kind: InClusterIPPool
        name: testpool4
    status:
      addressRef:
        name: "example-claim-0-0-ip-qw3rt"
    ---
    apiVersion: ipam.cluster.x-k8s.io/v1alpha1
    kind: IPAddress
    metadata:
      name: "example-claim-0-0-ip-qw3rt"
      namespace: "vsphere-site1"
      finalizers: ["ipam.cluster.x-k8s.io/ip-claim-protection"]
      ownerReferences:
      - apiVersion: "infrastructure.cluster.x-k8s.io/v1beta1"
        kind: "VSphereVM"
        name: "prod-fdftj"
        uid: "097ccb1e-959f-4357-b0bf-cb3eccb2a185"
    spec:
      address: "10.10.10.100"
      prefix: 24
      gateway: "10.10.10.1"
      claimRef:
        name: "example-claim-0-0"
      poolRef:
        apiGroup: ipam.cluster.x-k8s.io
        kind: InClusterIPPool
        name: testpool4
    ```

1. The controller can continue the VM creation process once all
   `IPAddressClaim`s have been bound. It will read the `IPAddressClaim` status
   for the bound `IPAddress` to be passed into the VM metadata. The metadata
   template will need to be updated to accept this IP address(es) and populate
   the addresses field with the IPs for the corresponding device.

   The device index on the `IPAddressClaim` name will be used to configure the
   IP address on the correct device.

1. Update the `VSphereVM` status to reflect the network device state. Update
   the `IPAddressClaim` condition.

    ```yaml
    ---
    status:
      conditions:
        - lastTransitionTime: "2022-09-14T14:00:00Z"
          type: IPAddressClaimed
          status: "True"
      network:
        - connected: true
          ipAddrs:
            - 10.10.10.100
          macAddr: 00:01:02:03:04:05
          networkName: VM Network
    ```

#### Owner Reference and Finalizer

When a `IPAddressClaim` is created, a finalizer named
`ipam.cluster.x-k8s.io/ip-claim-protection` will be added, as well as an owner
reference linking back to the `VSphereVM`.

The finalizer will prevent users from deleting `IPAddressClaim`s that are in-use
by VMs.

During `VSphereVM` delete reconcilliation, after the VM is deleted from the
VSphere API, the `IPAddressClaim`'s finalizer will be removed. When the
`VSphereVM` is deleted in the Kubernetes API, the garbage collector will delete
the referencing `IPAddressClaim`s. When the VM is deleted from the VSphere
API, it is safe to release the IP for use on another VM.

#### Flavors

An IPAM flavor will be added to support configuring an array of group, kind,
name, VM Network objects. When the IPAM flavor is specified, the devices in the
network spec of the Nodes will have DHCP turned off. Each of the device's
fromPool in the network spec will be populated with one of the above mentioned
configuration objects. The templating and configurability of flavors may
restrict the possible ways IPAM can be configured through flavors.

#### Validation

In the `NetworkDeviceSpec`:

The `IPAddrs` field is required if DHCP4 or DHCP6 is false. The new
`addressesFromPools` field allows for a third option and therefore we need to
update validation to ensure one of the three options is specified for a network
device.

#### Nameservers

Nameservers can come from DHCP if enabled. Therefore when DHCP is disabled and
the `addressesFromPools` field is set  a user will have to supply their own nameservers
in the device spec.

### Security Model

- Following the original IPAM integration proposal, the `IPAddressClaim`,
  `IPAddress`, and IP pool are required to be in the same namespace as the
  `VSphereMachine`.
- For `IPAddressClaim` and `IPAddress` this is enforced by the lack of a
  namespace field on the reference
- The `addressesFromPools` field on the
  `VSphereMachineTemplate`/`VSphereMachine` also lacks a namespace field on
  the reference.

### Risks and Mitigations

- Unresponsive IPAM providers can prevent successful creation of a cluster
- Any issues with fulfilling the IPAddressClaim can prevent successful creation of a cluster
  - This is partially by design i.e we don't want the cluster to be created if
    it can't fulfill the ip address claim, but this is just added complexity
    that a user has to be aware of.
- Wrong IP address allocations (e.g. duplicates) can brick a cluster’s network
- Incorrectly assigning IP address claims to the wrong device
  - We assume the devices and pools in the VSphereVM spec are static. If a user
    decides to change the spec at runtime, this will cause undefined behavior.
- A user could cause an IP address to be allocated to another machine/device
  while in use by deleting the IPAddressClaim.
  - This can be mitigated by adding a finalizer similar to the
    `kubernetes.io/pvc-protection` finalizer.

## Alternatives

Continuing to use some DHCP based solution, but DHCP has proven to have many
limitations (see motivation) that users want to avoid.

Considering the IPAM integration proposal has been accepted we didn't consider
any alternative approaches to IPAM outside of what has been defined by the
proposal.

## Upgrade Strategy

The previous API for the previous behavior (using DHCP or static IP
assignments) is preserved, so if the addressesFromPools field is not used, the behavior
will remain the same.

In order to migrate from a machine using the previous behavior (e.g. DHCP
enabled) to IPAM, a new VSphereMachineTemplate can be created with the
`addressesFromPools` field and existing machines based from the original template can be
recreated.

## Additional Details

### Test Plan

Note: Section not required until targeted at a release.

### Graduation Criteria

Note: Section not required until targeted at a release.

### Version Skew Strategy

IPAM support would depend on the version of CAPI supporting IPAM e.g we expect
CAPI to install the CRDs so CAPV can create and watch IPAddressClaims. The CAPV
controller could check if the CRD exists to determine if it can create/watch
IPAddressClaims. If the CRD is not present, but the user is attempting to use
addressesFromPools, then the VSphereMachine should error and the controller should log
an error.

```yaml
status:
  conditions:
    - lastTransitionTime: "2022-09-14T14:00:00Z"
      type: IPAddressClaimed
      status: "False"
      reason: "FailedToCreateIPAddressClaim"
      message: "The IPAddressClaim CRD is not found. Check the version of CAPI is >= 1.2.0."
```

## References

[Cluster API IPAM Provider In Cluster](https://github.com/telekom/cluster-api-ipam-provider-in-cluster)
[IPAM integration proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220125-ipam-integration.md)
[Historical implementation of static IP support for CAPV using M3](https://github.com/spectrocloud/cluster-api-provider-vsphere-static-ip)

## Implementation History

- 09/13/2022: Compile a Google Doc following the CAEP template
- 09/15/2022: First round of feedback from community
- 9/16/2022: Revisions made to proposal
- Updated how we intend to correlate IP Address Claims to devices in implementation details.
- 9/29/2022: Present proposal at a community meeting
- 9/29/2022: Open proposal PR
