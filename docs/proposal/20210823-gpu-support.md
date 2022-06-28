# GPU and PCI passthrough support in CAPV

```text
---
title: GPU and PCI passthrough support in CAPV
authors:
  - "@geetikabatra"
reviewers:
  - "@vijaykumar"
  - "@ankitaswamy"
  - "@sonasingh46"
  - "@pshail"
  - "@srm09"
  - "@yastij"
  
creation-date: 2021-08-23
last-updated: 2021-08-25
status: implementable
```

## Table of Contents

- [GPU and PCI passthrough support in CAPV](#gpu-and-pci-passthrough-support-in-capv)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
  - [Proposal](#proposal)
    - [User Stories](#user-stories)
      - [Story 1 - using vGPU](#story-1---using-vgpu)
      - [Story 2 - GPU Direct implementation](#story-2---gpu-direct-implementation)
      - [Story 3 - PCI passthrough for single node Customer](#story-3---pci-passthrough-for-single-node-customer)
    - [Requirements](#requirements)
      - [Functional Requirements](#functional-requirements)
      - [Non-Functional Requirements](#non-functional-requirements)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
      - [Proposed Changes](#proposed-changes)
      - [Current Scope of vendor](#current-scope-of-vendor)
      - [Additional Webhooks](#additional-webhooks)
      - [Controller Changes](#controller-changes)
  - [Additional Details](#additional-details)
    - [Test Plan](#test-plan)
      - [Subtasks](#subtasks)
    - [Graduation Criteria](#graduation-criteria)
  - [Implementation History](#implementation-history)

## Glossary

- CAPV - An abbreviation of Cluster API Provider vSphere

## Summary

CAPV currently does not support provisioning of GPU-based worker nodes and hence limits the deployment of workloads that needs GPU. This proposal outlines ways to enable GPU support in CAPV. This means that the k8s cluster created via CAPV can have GPU enabled nodes that can be finally consumed by the workloads(pods) that request GPU. The proposed changes will maintain backward compatibility and maintain the existing behaviour without any extra user configuration.

## Motivation

CAPV does not currently support GPU-accelerated workloads which have a limited addressable market. Competitively, all of the hyper-scale cloud providers (AWS, Azure, & GCP) offer GPU-accelerated virtualization and Kubernetes platforms. Many of them additionally offer robust vertically integrated AI/ML solutions building on open source technology. GPU support is a long-requested feature. With this capability, users can deploy GPU accelerated workloads on the Kubernetes cluster provisioned via CAPV

### Goals

1. To allow vGPU support for multi-node clusters.
2. To allow GPU Direct support for multi-node clusters.
3. To allow PCI passthrough support for single-node clusters.

## Proposal

### User Stories

#### Story 1 - using vGPU

Stacy works at an IT department of an organization where AI/ML models are run every hour and the entire staff wants to leverage GPU-supported nodes. VGPU's are able to provide close to bare-metal performance. Stacy needs a lot of sharing GPUs.
vGPU provides a full-fledged cluster from which a GPU can be requested.

#### Story 2 - GPU Direct implementation

Tony is an Engineer at a big organization with multiple centers across the Globe. Tony needs a big GPU cluster to address the needs of his organization. So they intend to use GPU Direct. GPU direct is technically a pool of GPUs connected by a network card in a kind of peer-to-peer network. This is the latest technology and gives the advantage of shared GPUs from a pool. GPU  direct is the technology that helps in linking multiple GPUs. Tony wants to leverage this technology so that it is easier for him to manage resource allocation.

#### Story 3 - PCI passthrough for single node Customer

Alex is an Engineer at a retail organization that requires a single GPU node. They use one node with GPU attached and want to keep things simple. Alex can simply add this GPU-connected machine to the cluster and that should do the job. While selecting nodes, Alex can use appropriate labels to run his AI/ML workload on this particular node. PCI passthrough will provide direct GPU support. The challenge that Alex can face is that using passthrough Alex wouldn't be able to migrate nodes.

### Requirements

#### Functional Requirements

- FR1: CAPV MUST support vGPUs.
- FR2: CAPV MUST support GPUDirect.
- FR3: CAPV MUST support PCI Passthrough.

#### Non-Functional Requirements

- NFR1: Unit tests MUST exist for all 3 supports
- NFR2: e2e tests MUST exist for 3 supports

### Implementation Details/Notes/Constraints

- Using the Vsphere failure domain to dictate where the sphere VM gets created. It needs to have access to GPU; otherwise, cloning will fail. The users' responsibility is to ensure that whatever failure domain they are trying to put the VM on(GPU VM). It must have access to the required physical device.

#### Proposed Changes

```go
// VirtualMachineCloneSpec is information used to clone a virtual machine.
type VirtualMachineCloneSpec struct {
  // PciDevices is the list of pci devices used by the virtual machine.
  // +optional
  PciDevices []PCIDeviceSpec `json:"pcidevices"`
  // VgpuDevices is the list of vgpu devices used by the virtual machine.
  // +optional
  VgpuDevices []VGPUDeviceSpec `json:"vgpudevices"`
}
//VGPUDeviceSpec defines virtual machine's VGPU configuration
type VGPUDeviceSpec struct {
 // ProfileName is the Profile Name of a virtual machine's VGPU, in string.
 // Defaults to the eponymous property value in the template from which the
 // virtual machine is cloned.
 // +optional
 ProfileName string `json:"profileName,omitempty"`
}
//PCIDeviceSpec defines virtual machine's PCI configuration
type PCIDeviceSpec struct {
 // DeviceID is the device ID of a virtual machine's PCI, in integer.
 // Defaults to the eponymous property value in the template from which the
 // virtual machine is cloned.
 // +optional
 DeviceID int32 `json:"deviceId,omitempty"`
 // VendorId is the vendor ID of a virtual machine's PCI, in integer.
 // Defaults to the eponymous property value in the template from which the
 // virtual machine is cloned.
 // +optional
 VendorID int32 `json:"vendorId,omitempty"`
}
```

#### Current Scope of vendor

- Nvidia

#### Additional Webhooks

- Check if `DeviceID` is present or not in `VSphereMachine template`.

#### Controller Changes

- If vGPU is present, the CR populates the ProfileName, which adds the vGPU to the list of devices.
- If GPU is present via PCI passthrough, `device ID`, and `vendor ID`. The CR populates the variables respective to VGPU, i.e., GPUProfileName.
- Similarly, the controller also initializes variables related to Direct devices(for PCI passthrough and GPUDirect). i.e., `GPUDeviceId` and `GPUVendorId`.
- Controller will call a function `getGpuSpecs` device, which will use the information stated above to determine if that specific GPU is present or not. If it is, then VM with that GPU is spawned.
- VSphereVMStatus object will return the Device info to the user stating whether the VM is in ready state or in failed state.
  
## Additional Details

### Test Plan

- Unit tests for cluster controller to test behaviour when vGPU is requested.
- Unit tests for cluster controller to test behaviour if vGPU works as expected.
- Unit tests for cluster controller to test if multiple vGPUs are requested, do all of them behave as expected.
- Unit tests for cluster controller to test GPU's requested from GPU direct work as expected.
- Unit tests for cluster controller to test PCI passthrough works as expected.
- Unit tests for a single node PCI passthrough.
- E2E tests for GPU enabled workload clusters

#### Subtasks

- Enabling GPU based infrastructure, will be hosted behind a VPN.
- Setting up webhooks to trigger the GPU based E2E tests.
- Actual test scenarios
  - E2E test to create a GPU-enabled cluster with one control plane node and one worker node.
    - Wait for a node to have a "nvidia.com/gpu" allocatable resource.
    - E2E test to run a GPU-based calculation.
    - Run a CUDA vector calculation job

### Graduation Criteria

Alpha

- Support VGPU
- Support GPU Direct
- Support PCI PAssthrough

Beta

N/A

Stable

- Two releases since beta.

## Implementation History

- 08/23/2021: Initial Proposal
