---
title: supervisor-based APIs for CAPV
authors:
  - "@yastij"
reviewers:
  - "@MaxRink"
  - "@vrabbi"
creation-date: 2021-10-06
last-updated: 2021-10-06
status: implementable
---
## supervisor-based APIs for CAPV

## Table of Contents

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
  - [Security Model](#security-model)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Alternatives](#alternatives)
- [Upgrade Strategy](#upgrade-strategy)
- [Implementation History](#implementation-history)

## Glossary

**Supervisor cluster:** The supervisor is a management cluster provided by vsphere, it is deployed differently depending on the networking solution. The supervisor cluster uses vSphere VMs for its control plane nodes and ESXi hosts as its worker nodes. When deployed using VDS networking, there are zero worker nodes.

**Supervisor APIs:** The set of APIs offered by the services running within the supervisor cluster, this includes VM Operator and Network Operator

## Summary

As vSphere is moving towards heavily relying on the supervisor APIs, CAPV needs to start leveraging those.
This proposal is adding support for supervisor-based APIs to provision Infrastructure for Kubernetes clusters. While ensuring we’re still maintaining through bug and security fixes the current APIs, we’ll also actively review any submitted PRs and keep a release cadence for the current APIs.

## Motivation

Today CAPV has to deal with a layer of govmomi code to provision infrastructure for the kubernetes cluster, which can prove challenging and has limitations as we’re expanding the number of  supported features. We’re also bringing some of the learnings we’ve had downstream to the upstream community.
All of this is aligning with the approach vSphere is taking toward relying on the supervisor services as the new common infrastructure LCM APIs.

### Goals

- Keep supporting the existing set of APIs of CAPV
- Add support for the supervisor APIs to provision infrastructure
- Ensure we have a single release cadence for both APIs
- Introducing these APIs should be iterative, which means both APIs will live next to each other, until vSphere versions which do not support the supervisor APIs reach End of General Support
- Remove the need to store vSphere credentials in workload clusters by introducing a paravirtual model to CAPV
- Add a support grid between vsphere and CAPV versions in the readme

### Non-Goals/Future Work

- Bring feature parity between the two sets of APIs
- Support running the new supervisor APIs in a management cluster other than the supervisor cluster

## Proposal

In order to bring some of the innovations done downstream around infrastructure providers leveraging the supervisor-based APIs, we will have to:

- Add a new set of APIs for VSphereMachines, VSphereMachineTemplates and VSphereClusters (those will either live in either a new API Group or branch, this is discussed below)
- Enable/add controllers that reconcile those new APIs
- For VSphereMachines generate VirtualMachines instead of the home-grown VSphereVM objects
- Ensure that we have a good controller test coverage
- Enable testing against VSphere 7+ with supervisor enabled (experimental support)

### Release engineering

Using a separate API Group such as `vmware.infrastructure.cluster.x-k8s.io` would allow hosting VSphereMachines/VSphereClusters/VSphereMachineTemplates in the same branch.
controllers would be watching both API Group resources and would branch off separate code paths depending on the API Group/Kind being reconciled. To avoid situations where we are blocked from releasing the content of one API Group because of the other not being ready, it will be required to have the main branch in a state that is always shippable.
RBAC-wise, we will be aggregating the clusterRoles to the CAPI manager through aggregated RBACs so that the capi-controller-manager has the right permission to reconcile infrastructure resources.

### User Stories

#### Story 1

- As a user I want to be able to deploy vim API based CAPV without any changes to my setup to be able to pick up bug and security fixes without introducing new APIs or behaviors

#### Story 2

- As a user I want to deploy to vSphere environments where supervisor isn’t enabled to leverage existing environments (6.7, 7.0 without supervisor) and use CAPV’s features

### Story 3

- As a user I want to deploy to vSphere environments where supervisor is enabled to leverage supervisor's features

### Implementation Details/Notes/Constraints

We will be adding the following API Group, and follow kubebuilder's handling of multi-apigroup setups:

```golang
package v1beta1

import (
   "k8s.io/apimachinery/pkg/runtime/schema"
   "sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
   // Version is the API version.
   Version = "v1beta1"

   // GroupName is the name of the API group.
   GroupName = "vmware.infrastructure.cluster.x-k8s.io"
)

var (
   // GroupVersion is group version used to register these objects
   GroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

   // SchemeBuilder is used to add go types to the GroupVersionKind scheme
   SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

   // AddToScheme adds the types in this group-version to the given scheme.
   AddToScheme = SchemeBuilder.AddToScheme
)

```

To this API Group we're adding the following infrastructure resources:

```golang
package v1beta1

import (
 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
 clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
 // ClusterFinalizer allows reconciliation to clean up vSphere
 // resources associated with VSphereCluster before removing it from the
 // API server.
 ClusterFinalizer = "vspherecluster.vmware.infrastructure.cluster.x-k8s.io"
)

// VSphereClusterSpec defines the desired state of VSphereCluster
type VSphereClusterSpec struct {
 ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// VSphereClusterStatus defines the observed state of VSphereClusterSpec
type VSphereClusterStatus struct {
 // Ready indicates the infrastructure required to deploy this cluster is
 // ready.
 // +optional
 Ready bool `json:"ready"`

 // ResourcePolicyName is the name of the VirtualMachineSetResourcePolicy for
 // the cluster, if one exists
 // +optional
 ResourcePolicyName string `json:"resourcePolicyName,omitempty"`

 // Conditions defines current service state of the VSphereCluster.
 // +optional
 Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vsphereclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// VSphereCluster is the Schema for the VSphereClusters API
type VSphereCluster struct {
 metav1.TypeMeta   `json:",inline"`
 metav1.ObjectMeta `json:"metadata,omitempty"`

 Spec   VSphereClusterSpec   `json:"spec,omitempty"`
 Status VSphereClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VSphereClusterList contains a list of VSphereCluster
type VSphereClusterList struct {
 metav1.TypeMeta `json:",inline"`
 metav1.ListMeta `json:"metadata,omitempty"`
 Items           []VSphereCluster `json:"items"`
}

func (r *VSphereCluster) GetConditions() clusterv1.Conditions {
 return r.Status.Conditions
}

func (r *VSphereCluster) SetConditions(conditions clusterv1.Conditions) {
 r.Status.Conditions = conditions
}

func init() {
 SchemeBuilder.Register(&VSphereCluster{}, &VSphereClusterList{})
}

```

the VSphereMachine types would be:

```golang
package v1beta1

import (
 v1 "k8s.io/api/core/v1"
 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
 clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
 "sigs.k8s.io/cluster-api/errors"
)

const (
 // MachineFinalizer allows Reconciliation to clean up
 // resources associated with VSphereMachine before removing it from the
 // API Server.
 MachineFinalizer = "vspheremachine.infrastructure.cluster.vmware.com"
)

// VSphereMachineVolume defines a PVC attachment
type VSphereMachineVolume struct {
 // Name is suffix used to name this PVC as: VSphereMachine.Name + "-" + Name
 Name string `json:"name"`
 // Capacity is the PVC capacity
 Capacity v1.ResourceList `json:"capacity"`
 // StorageClass defaults to VSphereMachineSpec.StorageClass
 // +optional
 StorageClass string `json:"storageClass,omitempty"`
}

// VSphereMachineSpec defines the desired state of VSphereMachine
type VSphereMachineSpec struct {
 // ProviderID is the virtual machine's BIOS UUID formated as
 // vsphere://12345678-1234-1234-1234-123456789abc.
 // This is required at runtime by CAPI. Do not remove this field.
 // +optional
 ProviderID *string `json:"providerID,omitempty"`

 // ImageName is the name of the base image used when specifying the
 // underlying virtual machine
 ImageName string `json:"imageName"`

 // ClassName is the name of the class used when specifying the underlying
 // virtual machine
 ClassName string `json:"className"`

 // StorageClass is the name of the storage class used when specifying the
 // underlying virtual machine.
 // +optional
 StorageClass string `json:"storageClass,omitempty"`

 // Volumes is the set of PVCs to be created and attached to the VSphereMachine
 // +optional
 Volumes []VSphereMachineVolume `json:"volumes,omitempty"`
}

// VSphereMachineStatus defines the observed state of VSphereMachine
type VSphereMachineStatus struct {
 // Ready is true when the provider resource is ready.
 // This is required at runtime by CAPI. Do not remove this field.
 // +optional
 Ready bool `json:"ready"`

 // Addresses contains the instance associated addresses.
 Addresses []v1.NodeAddress `json:"addresses,omitempty"`

 // ID is used to identify the virtual machine.
 // +optional
 ID *string `json:"vmID,omitempty"`

 // IPAddr is the IP address used to access the virtual machine.
 // +optional
 IPAddr string `json:"vmIp,omitempty"`

 // FailureReason will be set in the event that there is a terminal problem
 // reconciling the Machine and will contain a succinct value suitable
 // for machine interpretation.
 //
 // This field should not be set for transitive errors that a controller
 // faces that are expected to be fixed automatically over
 // time (like service outages), but instead indicate that something is
 // fundamentally wrong with the Machine's spec or the configuration of
 // the controller, and that manual intervention is required. Examples
 // of terminal errors would be invalid combinations of settings in the
 // spec, values that are unsupported by the controller, or the
 // responsible controller itself being critically misconfigured.
 //
 // Any transient errors that occur during the reconciliation of Machines
 // can be added as events to the Machine object and/or logged in the
 // controller's output.
 // +optional
 FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

 // FailureMessage will be set in the event that there is a terminal problem
 // reconciling the Machine and will contain a more verbose string suitable
 // for logging and human consumption.
 //
 // This field should not be set for transitive errors that a controller
 // faces that are expected to be fixed automatically over
 // time (like service outages), but instead indicate that something is
 // fundamentally wrong with the Machine's spec or the configuration of
 // the controller, and that manual intervention is required. Examples
 // of terminal errors would be invalid combinations of settings in the
 // spec, values that are unsupported by the controller, or the
 // responsible controller itself being critically misconfigured.
 //
 // Any transient errors that occur during the reconciliation of Machines
 // can be added as events to the Machine object and/or logged in the
 // controller's output.
 // +optional
 FailureMessage *string `json:"failureMessage,omitempty"`

 // VMStatus is used to identify the virtual machine status.
 // +optional
 VMStatus VirtualMachineState `json:"vmstatus,omitempty"`

 // Conditions defines current service state of the VSphereMachine.
 // +optional
 Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// VSphereMachine is the Schema for the vspheremachines API
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vspheremachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="IPAddr",type="string",JSONPath=".status.vmIp",description="IP address"
type VSphereMachine struct {
 metav1.TypeMeta   `json:",inline"`
 metav1.ObjectMeta `json:"metadata,omitempty"`

 Spec   VSphereMachineSpec   `json:"spec,omitempty"`
 Status VSphereMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VSphereMachineList contains a list of VSphereMachine
type VSphereMachineList struct {
 metav1.TypeMeta `json:",inline"`
 metav1.ListMeta `json:"metadata,omitempty"`
 Items           []VSphereMachine `json:"items"`
}

func (r *VSphereMachine) GetConditions() clusterv1.Conditions {
 return r.Status.Conditions
}

func (r *VSphereMachine) SetConditions(conditions clusterv1.Conditions) {
 r.Status.Conditions = conditions
}

func init() {
 SchemeBuilder.Register(&VSphereMachine{}, &VSphereMachineList{})
}

```

The VSphereMachineTemplate types would be:

```golang
package v1beta1

import (
 metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VSphereMachineTemplateResource describes the data needed to create a VSphereMachine from a template
type VSphereMachineTemplateResource struct {
 // Spec is the specification of the desired behavior of the machine.
 Spec VSphereMachineSpec `json:"spec"`
}


// VSphereMachineTemplateSpec defines the desired state of VSphereMachineTemplate
type VSphereMachineTemplateSpec struct {
 Template VSphereMachineTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=vspheremachinetemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// VSphereMachineTemplate is the Schema for the vspheremachinetemplates API
type VSphereMachineTemplate struct {
 metav1.TypeMeta   `json:",inline"`
 metav1.ObjectMeta `json:"metadata,omitempty"`

 Spec VSphereMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// VSphereMachineTemplateList contains a list of VSphereMachineTemplate
type VSphereMachineTemplateList struct {
 metav1.TypeMeta `json:",inline"`
 metav1.ListMeta `json:"metadata,omitempty"`
 Items           []VSphereMachineTemplate `json:"items"`
}

func init() {
 SchemeBuilder.Register(&VSphereMachineTemplate{}, &VSphereMachineTemplateList{})
}

```

#### Controller changes

This section will outline the common controller changes between the API Groups, and which new controllers we will need to add to CAPV in order to support supervisor-based APIs in CAPV.

Common controllers should be the controllers that help fulfill the Cluster API contract, those should be the vspherecluster and vspheremachine controller. They need to be refactored at two levels, the first one is the controller setup, the second is at reconciliation logic level.

For the vsphereMachine controller is fairly similar, we should only have reconciler, the differences are going to be:

- Depending on the API Group we would instantiate a different VM Service (either the govmomi layer or the one provided for VM Service)
- Audit reconcileProviderID() and ReconcileNetwork() to be flexible from where they read the information they need

For the vspherecluster controller, the logic of reconciliation is mostly different, so they would remain independent in different reconciler files. The vspherecluster controller should only have AddClusterControllerToManager() which would be detailed in the controller setup below.

we also need to re-work the controllers in a bi-modal fashion, through the following steps:
Refactor `AddClusterControllerToManager()` function that adds the vspherecluster/vspheremachine controllers into a bi-modal fashion: depending on the parameters the function needs to setup a controller that watches for a different set of types
Pass the reconciler
Refactor the main.go to call AddClusterControllerToManager() for each API Group (if installed)

#### Newly added Controllers

ProviderServiceAccount controller

This controller is responsible for provisioning service account tokens into target clusters. This is needed for the CPI and CSI in the new paravirtual model of vsphere 7. The API for the ProviderServiceAccount is the following:

```golang
package v1beta1

import (
   corev1 "k8s.io/api/core/v1"
   rbacv1 "k8s.io/api/rbac/v1"
   metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderServiceAccountSpec defines the desired state of ProviderServiceAccount.
type ProviderServiceAccountSpec struct {
   // Ref specifies the reference to the Cluster for which the ProviderServiceAccount needs to be realized.
   Ref *corev1.ObjectReference `json:"ref"`

   // Rules specify the privileges that need to be granted to the service account.
   Rules []rbacv1.PolicyRule `json:"rules"`

   // TargetNamespace is the namespace in the target cluster where the secret containing the generated service account
   // token needs to be created.
   TargetNamespace string `json:"targetNamespace"`

   // TargetSecretName is the name of the secret in the target cluster that contains the generated service account
   // token.
   TargetSecretName string `json:"targetSecretName"`
}

// ProviderServiceAccountStatus defines the observed state of ProviderServiceAccount.
type ProviderServiceAccountStatus struct {
   Ready    bool   `json:"ready,omitempty"`
   ErrorMsg string `json:"errorMsg,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=providerserviceaccounts,scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=.spec.ref.name
// +kubebuilder:printcolumn:name="TargetNamespace",type=string,JSONPath=.spec.targetNamespace
// +kubebuilder:printcolumn:name="TargetSecretName",type=string,JSONPath=.spec.targetSecretName
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ProviderServiceAccount is the schema for the ProviderServiceAccount API.
type ProviderServiceAccount struct {
   metav1.TypeMeta   `json:",inline"`
   metav1.ObjectMeta `json:"metadata,omitempty"`

   Spec ProviderServiceAccountSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderServiceAccountList contains a list of ProviderServiceAccount
type ProviderServiceAccountList struct {
   metav1.TypeMeta `json:",inline"`
   metav1.ListMeta `json:"metadata,omitempty"`
   Items           []ProviderServiceAccount `json:"items"`
}

func init() {
   SchemeBuilder.Register(&ProviderServiceAccount{}, &ProviderServiceAccountList{})
}

```

#### ServiceDiscovery controller

The ServiceDiscovery controller is a controller that provides a headless service in the workload cluster that proxies the supervisor cluster’s API Server. This controller will be needed for the components and addons that needs to talk to the supervisor API

#### CI and test coverage

CI signals will be first implemented through downstream testing. We will be relying on a downstream Jenkins that polls github periodically to trigger tests. Long term, a native integration with Prow will be achieved through using AWS SNS where Prow jobs queue tests and where downstream reads and starts tests.

### Security Model

From a security perspective, this proposal is going to enable users of the new APIs to remove the need of having vcenter credentials stored in the workload clusters.

### Risks and Mitigations

One of the identified risks here is that the main branch should be in releasable state across both API Groups, this is mainly mitigated by good hygiene (e.g. ensuring we’re not merging API changes without the corresponding controller changes, continuing on increasing the test coverage etc..)

## Alternatives

### Use a separate branch to host the new APIs and controllers

This means that if we're hosting the code base into different branches, the new branch would host the new supervisor-based API code and would be released as CAPV v1.0, This has the advantage of keeping the release cycle decoupled in both APIs. On the other hand, this model can bring the burden of managing multiple branches  (e.g. doing work twice for housekeeping and common fixes)

### Add new fields to current API and heavily rely on webhook for field compatibility

This option adds new fields to the current APIs and makes those mutually exclusive by relying heavily on validating webhook. This option has the benefit of keeping everything within the same branch, but this will get increasingly complicated to manage our APIs and webhooks. The UX wouldn’t be ideal for users trying to explain the resources.

## Upgrade Strategy

The upgrade will be covered in a follow up document that will outline the upgrade process from one API Group to the other

## Implementation History

- [ ] MM/DD/YYYY: Proposed idea in an issue or [community meeting]
- [ ] MM/DD/YYYY: Compile a Google Doc following the CAEP template (link here)
- [ ] MM/DD/YYYY: First round of feedback from community
- [ ] MM/DD/YYYY: Present proposal at a [community meeting]
- [ ] MM/DD/YYYY: Open proposal PR

<!-- Links -->
[community meeting]: https://docs.google.com/document/d/1Ys-DOR5UsgbMEeciuG0HOgDQc8kZsaWIWJeKJ1-UfbY
