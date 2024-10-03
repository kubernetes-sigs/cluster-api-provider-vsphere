# CAPV Additional Data Disks When Creating New Machines

```text
---
title: CAPV Additional Data Disks When Creating New Machines
authors:
  - "@vr4manta"
reviewers:
  - TBD
creation-date: 2024-10-03
last-updated: 2024-10-03
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

As the use of kubernetes clusters grows, admins are needing more and more improvements to the VMs themselves to make sure they run as smoothly as possible.  The number of cores and memory continue to increase for each machine and this is causing the amount of workloads to increase on each virtual machine.  This growth is now causing the base VM image to not provide enough storage for OS needs.  In some cases, users just increase the size of the primary disk using the existing configuration options for machines; however, this does not allows for all desired configuration choices.  Admins are now wanting the ability to add additional disks to these VMs for things such as etcd storage, image storage, container runtime and even swap.  

Before this feature, CAPV only allows for the configuration of additional disks that are found within the vSphere VM Template that is used for cloning.  As clusters stretch failure domains and as clusters contain multiple different machine sets, attempting to just create custom vSphere VM templates will cause the admin to have to manage a large number of vSphere OVA templates.  Instead, it would be ideal if admins can just configure a machine or machineset to add additional disks to a VM during the cloning process that are not part of the template.

This feature enhancement aims to allow admins the ability to configure additional disks that are not present in the VM templates by enhancing the vsphere machine API and adding the capability to the cloning process.

## Motivation

Cluster administrators are asking for the ability to add additional data disks to be used by the OS without having to create custom OVA images to be used by the VM cloning process.

### Goals

* Add new configuration for machines that are used to define new data disks to add to a VM.
* Align new property naming to be similar or even match other providers if possible.
* Do not boil the ocean with the initial implementation of this feature.

### Non-Goals/Future Work

* Add abiltiy to advance configure additions disks (such as define controller type (IDE, scsi, etc) or unit number in the controller)
* Any disk management features such as encryption

## Proposal

### User Stories

#### Story 1

As an admin, I’d like my control plane machines to have an extra disk added so I can dedicate that disk for etcd databse through my own means of mount management

#### Story 2

As an admin, I’d like my compute machine set to have an extra disk added to each machine so that I can use it as a dedicated disk for container image storage.

### Requirements

#### Functional Requirements

### Overall Design

#### For User Story 1

#### For User Story 2

### API Design

### Implementation Details

### Notes/Constraints

## Upgrade Strategy
