# Single Controller Multitenancy

```text
---
title: Single Controller Multitenancy
authors:
  - "@gab-satchi"
reviewers:
  - "@sedefsavas"
  - "@davigned"
  - "@nader-ziada"
  - "@yastij"
  - "@fabriziopandini"
creation-date: 2021-03-11
last-updated: 2021-03-11
status: implementable
see-also:
  - https://github.com/kubernetes-sigs/cluster-api-provider-aws/blob/master/docs/proposal/20200506-single-controller-multitenancy.md
  - https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/master/docs/proposals/20200720-single-controller-multitenancy.md
---
```

## Table of Contents

* [Single Controller Multitenancy](#single-controller-multitenancy)
  * [Glossary](#glossary)
  * [Summary](#summary)
  * [Motivation](#motivation)
    * [Goals](#goals)
  * [Proposal](#proposal)
    * [User Stories](#user-stories)
      * [Story 1 - Deploying to multiple vCenters](#story-1---deploying-to-multiple-vcenters)
      * [Story 2 - Deploying multiple clusters from a single account](#story-2---deploying-multiple-clusters-from-a-single-account)
      * [Story 3 - Legacy behaviour](#story-3---legacy-behaviour)
    * [Requirements](#requirements)
      * [Functional Requirements](#functional-requirements)
      * [Non-Functional Requirements](#non-functional-requirements)
    * [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
      * [Current State](#current-state)
      * [Proposed Changes](#proposed-changes)
      * [Controller Changes](#controller-changes)
      * [Clusterctl Changes](#clusterctl-changes)
    * [Security Model](#security-model)
      * [Roles](#roles)
      * [RBAC](#rbac)
        * [Write Permissions](#write-permissions)
        * [Namespace Restrictions](#namespace-restrictions)
        * [CAPV Controller Requirements](#capv-controller-requirements)
    * [Risks and Mitigations](#risks-and-mitigations)
      * [Caching](#caching)
  * [Alternatives](#alternatives)
    * [Using only secrets to specify vSphere accounts](#using-only-secrets-to-specify-vsphere-accounts)
      * [Benefits](#benefits)
      * [Mitigations for current proposal](#mitigations-for-current-proposal)
  * [Upgrade Strategy](#upgrade-strategy)
  * [Additional Details](#additional-details)
    * [Test Plan](#test-plan)
    * [Graduation Criteria](#graduation-criteria)
  * [Implementation History](#implementation-history)

## Glossary

* CAPV - An abbreviation of Cluster API Provider vSphere

## Summary

The CAPV controller is capable of managing infrastructure resources on a vCenter using the credentials it was provided during initialization. The credentials are provided via environment variables to clusterctl that get saved onto a secret that's used by the CAPV deployment.

Credentials provided are used for the entire lifetime of the CAPV deployment which means a CAPV cluster can become broken if the provisioning CAPV deployment were to be reconfigred for another set of credentials.

This proposal outlines new capabilities for CAPV that can assume different credentials, at runtime, on a per-cluster basis. The proposed changes will maintain backwards compatibility and maintain the existing behaviour without any extra user configuration.  

## Motivation

Larger organizations often need to separate management and workload spaces by utilizing separate credentials for each. The tooling (CAPV in this case) may run in the mangement account with the intent of provisioning infrastructure in the workload accounts. For CAPV to be most useful within these organizations, it will need to support multi-acocunt models.

vSphere can also be deployed in edge scenarios. With CAPV's current capabilities, a management cluster will need to be deployed in each edge environment. With the new capabilities, a single management cluster can manage multiple edge deployments.  

### Goals

1. To enable VSphereCluster resources reconciliation using per cluster vSphere credentials.
1. To allow sets of clusters to use the same set of vSphere credentials.
1. To maintain backwards compatibility for users who do not intend to use these capabilities and want to continue specifying credentials through the CAPV deployment.

## Proposal

### User Stories

#### Story 1 - Deploying to multiple vCenters

Alex is an engineer in an organization that enforces strict vSphere account and environment architectures. They use a management vCenter and account for the management cluster where CAPV is running. Alex has a workload account in a dedicated vCenter.
Alex can provision a new cluster in the workload account by creating Cluster API resources in the management cluster. The CAPV controller will utilize the workload credentials to provision the cluster in Alex's environment.

#### Story 2 - Deploying multiple clusters from a single account

Stacy works at an organization where the cloud admin provides them with a namespace to use on a management cluster. Stacy can deploy workload clusters without having to know or specify the account details. If Stacy tried to deploy into a namespace that isn't authorized to use the account, the cluster will fail to deploy. Stacy can still create clusters into their dev environments by providing the account details in a Secret.

#### Story 3 - Legacy behaviour

Erin is an engineer in a smaller less strict organization. They use a single vCenter and keep their management cluster up-to-date. Erin can create new vSphere clusters while omitting the `vSphereCluster.Spec.IdentityRef` field. The CAPV controller will use the credentials it was given at initialization to create new vSphere clusters.

### Requirements

#### Functional Requirements

* FR1: CAPV MUST support credentials provided through a `VSphereClusterIdentity` and referenced by `VSphereCluster.IdentityRef` field (Stories 1,2)
* FR2: CAPV MUST support credentials provided through a `Secret` and referenced by `VSphereCluster.IdentityRef` field (Story 1)
* FR3: CAPV MUST support static credentials (Story 3)
* FR4: CAPV MUST support clusterctl move scenarios
* FR5: CAPV MUST prevent privilege escalation allowing users to create clusters in accounts they should not be able to (Story 2)

#### Non-Functional Requirements

* NFR1: Unit tests MUST exist for cluster and machine controllers that utilize the credentials
* NFR2: e2e tests MUST exist for multi account scenarios

### Implementation Details/Notes/Constraints

#### Current State

CAPV currently uses a session manager to create and cache sessions. Sessions are created (or retrieved from the cache) during cluster and vSphere VM reconcile loops.
vSphere cluster uses the session as a sanity check to ensure connectivity. vSphere VM uses the session for VM lifecycle tasks on vcenter and the session is stored in a VMContext.

```go
type VMContext struct {
    *ControllerContext
    VSphereVM   *v1alpha3.VSphereVM
    PatchHelper *patch.Helper
    Logger      logr.Logger
    Session     *session.Session
}
```

#### Proposed Changes

The proposed changes below allow users to specify the account to use for clusters during runtime. The credentials can be provided by two approaches:

* Referencing a cluster scoped `VSphereClusterIdentity` to be used for credentials.
* Referencing a `Secret` that contains the credentials in the same namespace as the cluster.

Changed Resources

* `VSphereCluster`

New Resources

* A cluster-scoped `VSphereClusterIdentity` represents the vCenter account to use for reconciliation. This type should also contain references to namespaces that are allowed to use the account.

Changes to VSphereCluster
A new field is added to the VSphereClusterSpec to reference the `VSphereClusterIdentity`. We intend to use a `VSphereIdentityReference` type, similar to that of `corev1.TypedLocalObjectReference` in order to ensure the only objects that can be referenced are either in the same namespace or are scoped globally.

```go
// VSphereClusterIdentity is the account to be used for vcenter actions
type VSphereClusterIdentity struct {
    metav1.TypeMeta `json:`",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec VSphereClusterIdentitySpec `json:"spec,omitempty"`
    Status VSphereClusterIdentityStatus `json:"status, omitempty"`
}

type VSphereClusterIdentitySpec struct {
    // secretsRef references a secret in the CAPV controller namespace with the credentials to use
    secretRef `json:"secretRef,omitempty"`

    // AllowedNamespaces is used to identify which namespaces are allowed to use this account.
    // Namespaces can be selected either by using a label selector.
    // If this object is nil, no namespaces will be allowed.
    // If this object is empty {}, clusters can use this identity from any namespace.
    //
    // +optional
    AllowedNamespaces *AllowedNamespaces `json:"allowedNamespaces"`
}

type AllowedNamespaces struct {
    // Selector is a standard Kubernetes LabelSelector.
    // This is a standard Kubernetes LabelSelector, a label query over a set of resources.
    // The result of matchLabels and matchExpressions are ANDed.
    // +optional
    Selector metav1.LabelSelector `json:"selector"`
}

type VSphereIdentityReference struct {
    // Kind of the identity.
    // +kubebuilder:validation:Enum=VSphereClusterIdentity,Secret
    Kind string

    // Name of the identity
    // +kubebuilder:validation:MinLength=1
    Name string
}

type VSphereClusterSpec struct {
    ...
    // +optional
    IdentityRef *VSphereIdentityReference `json:"IdentityRef,omitempty"`
}
```

#### Controller Changes

* If IdentityRef is specified, and it references a `VSphereClusterIdentity`, the CRD is fetched and used to create a session
* The secret referenced within `VSphereClusterIdentity` must be in the controller namespace
* If IdentityRef is specified, and it references a `Secret`, the `Secret` is used to create a session. The secret must be in the same namespace as the `VSphereCluster`
* The controller will add the VSphereCluster to OwnerReferences on a Secret that contains the account credentials
* If IdentityRef is not specified, the controller will fallback to using credentials provided in the controller deployment
* The session will be cached using the existing logic where the key is server + username + datacenter
* The `IdentityRef` on a `VSphereCluster` will be mutable. Setting the `IdentityRef` to nil will cause the controller to fallback to the static credentials
* The controller will fail to delete a `VSphereClusterIdentity` with an error if the account is used by `VSphereClusters`

#### Clusterctl Changes

Today, clusterctl move operates by tracking objectreferences within the same namespace, since we are now proposing to use cluster-scoped resources, we will need to add requisite support to clusterctl's object graph to track cluster-scoped resources that are used by the source cluster, and ensure they are moved. We will naively not delete cluster-scoped resources during a move, as they maybe referenced across namespaces. If a cluster uses a `Secret` for account credentials, the OwnerReference will get set by the controller and the `Secret` will be moved to the target cluster.

### Security Model

The intended RBAC model mirrors that for Service APIs:

#### Roles

For the purposes of this security model, 3 common roles have been identified:

* **Infrastructure Provider**: The infrastructure provider (infra) is responsible for the overall environment that
  the cluster(s) are operating in or the PaaS provider in a company.

* **Management Cluster Operator**: The cluster operator (ops) is responsible for
  administration of the Cluster API management cluster. They manage policies, network access,
  application permissions.

* **Workload Cluster Operator**: The workload cluster operator (dev) is responsible for
  management of the cluster relevant to their particular applications .

There are two primary components to the Service APIs security model: RBAC and namespace restrictions.

#### RBAC

RBAC (role-based access control) is the standard used for Kubernetes
authorization. This allows users to configure who can perform actions on
resources in specific scopes. RBAC can be used to enable each of the roles
defined above. In most cases, it will be desirable to have all resources be
readable by most roles, so instead we'll focus on write access for this model.

##### Write Permissions

|                              | VSphereClusterIdentity | Secret | Cluster |
| ---------------------------- | ---------------------- | ------ | ------- |
| Infrastructure Provider      | Yes                    | Yes    | Yes     |
| Management Cluster Operators | Yes                    | Yes    | Yes     |
| Workload Cluster Operator    | No                     | Yes    | Yes     |

#### Namespace Restrictions

* To prevent workload cluster operators from using cluster-scoped `VSphereClusterIdentity` that they should not be using, the `allowedNamespaces` will be used to dictate which namespaces are allowed to use the `VSphereClusterIdentity`.
An empty slice indicates that the `VSphereClusterIdentity` can be used by any `VSphereCluster`

#### CAPV Controller Requirements

The CAPV controller will need to:

* Populate conditions when cluster is misconfigured. `VSphereClusterIdentity` is not found or isn't compatible with the cluster due to namespace restrictions.
* Not implement an invalid configuration. If a cluster is attempting to use the `VSphereClusterIdentity` from an invalid namespace, ignore it and indicate the issue through conditions.
* Respond to changes in a `VSphereClusterIdentity` spec.

### Risks and Mitigations

#### Caching

With multi-tenant support, a single CAPV instance may reconcile multiple clusters across many accounts.
There's existing caching in the session manager that should be sufficient to handle the extra sessions that will get created.

## Alternatives

### Using only secrets to specify vSphere accounts

The VSphereCluster will reference the Secret via an ObjectReference

#### Benefits

* Re-using secrets ensures encryption by default and provides a clear UX signal to end users that the data is meant to be secure
* Keeps clusterctl move straightforward with the 1:1 cluster -> credential relationship

#### Mitigations for current proposal

* There are use cases where users would like to reuse the same account for multiple namespaces. See [Story 2](#story-2---deploying-multiple-clusters-from-a-single-account)

## Upgrade Strategy

The data changes are additive and optional. VSphereClusters that aren't configured with a `VSphereClusterIdentity` or `Secret` will default to the credentials initialized in the CAPV deployment.

## Additional Details

### Test Plan

* Unit tests for cluster controller to test behaviour when `VSphereClusterIdentity` is provided, missing or misconfigured.
* Unit tests for cluster controller to test behaviour for credentials provided through a `Secret`, missing or misconfigured.
* If it can be supported in the Prow environment, additional e2e test which can use a different vSphere account.
* clusterctl e2e that tests a move of a cluster

### Graduation Criteria

Alpha

* Support managing clusters while defining the `VSphereClusterIdentity` to use.
* Support workload clusters that specify the account to use via a `Secret`
* Ensure `clusterctl move` works.

Beta

* Full e2e coverage.

Stable

* Two releases since beta.

## Implementation History

* 03/11/2021: Initial Proposal
