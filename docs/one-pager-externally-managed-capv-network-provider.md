# One-Pager: An ExternallyManaged CAPV Network Provider for the Supervisor Cluster

| Author | ETA | JIRA |
| :--- | :--- | :--- |
| Yifeng Xiao | 2 months | GCM-18278 |

## Business Problem Being Solved

Supervisor Cluster 2.0 is a managed cluster provisioned by VKS on top of the Supervisor bootstrap stack (WCP). A complex, multi-NIC configuration that requires capabilities currently unsupported by existing VKS cluster networking providers:

* VKS needs the ability to directly control the ControlPlaneEndpoint in the CAPI spec for a pre-created Load Balancer.
* VKS needs the ability to configure Supervisor Node VM interfaces using existing Network/VirtualNetwork/Subnet objects, even when those interfaces reference network APIs from different providers in a single VM (e.g., a VPC Subnet on eth0 and a VDS Network on eth1), and must support dual-stack and DHCP-backed networks.

VKS today picks exactly one `NetworkProvider` implementation per Cluster (`VSphereDistributed`, `NSXTier1`, `VPC`) and that provider drives cluster network provisioning, load balancer creation, VM interface construction, and provider-based webhook validation – none of which is appropriate for the Supervisor Cluster.

Rather than threading per-field opt-outs through every existing provider, this design introduces a **new network provider value, ExternallyManaged**, that encodes the Supervisor Cluster’s "bootstrap stack owns the network, VKS just attaches VMs" contract end-to-end. Crucially, ExternallyManaged is a VKS-internal notion, not a Supervisor platform notion. Its selection is assumed by VKS based on the CAPI Cluster CR being in a specifically tagged supervisor namespace (via the [clusters.kubernetes.vmware.com/supervisor-namespace](http://clusters.kubernetes.vmware.com/supervisor-namespace): "true" annotation). This explicit VKS-internal scoping is necessary to avoid confusion for other controllers (like vm-operator, mobility-operator, etc.).

## Requirements From Supervisor Cluster 2.0

The contract the ExternallyManaged provider must implement, derived from Supervisor Cluster 2.0:

1. **VKS provisioning responsibilities – none for networks or LB.**
   * VKS does **not** provision `Network` / `VirtualNetwork` / `Subnet`. The Supervisor bootstrap stack creates them; VKS only references them.
   * VKS does **not** provision the kube-apiserver load balancer or its health check. The Supervisor bootstrap stack creates the Loadbalancer and manages the health check. VKS does not create a `VirtualMachineService` for the control plane.
2. **Interface configuration – pass through from spec.**
   * Primary interface (`eth0`) carries the **workload** network (`Network` / `VirtualNetwork` / `Subnet`) and owns the default route.
   * Secondary interface (`eth1`) carries the **management** network (`Network`).
   * If the referenced workload `Subnet` or management `Network` is dual-stack, the corresponding interface must also be configured as dual-stack on the VM.
   * DHCP-backed networks are supported on any interface; the guest acquires its IP from DHCP.

## Goals

* Introduce a new value ExternallyManaged for the per-Cluster network provider field `VSphereCluster.spec.network.provider`, which is the single source of truth for the per-Cluster network provider.
* Implement an ExternallyManaged `NetworkProvider` in CAPV that satisfies the contract above: no network/LB provisioning, straight propagation of `VSphereMachine.spec.network.interfaces` into `VirtualMachine.spec.network.interfaces`, with dual-stack `ipamModes` derived from the referenced object when it is a `Subnet`.
* Gate selection of the ExternallyManaged provider to the Supervisor’s system namespace via a new namespace annotation introduced for Supervisor 2.0; ordinary workload namespaces cannot pick it.
* Skip provider-based webhook validation on the Supervisor Cluster’s `Cluster`, `VSphereMachine`, and `VSphereMachineTemplate` objects, since they intentionally mix network references across providers.

## Non-Goals

* Changing the behavior of `VSphereDistributed`, `NSXTier1`, or `VPC` providers on workload Clusters.
* Allowing workload Clusters to mix network references across providers, to reference DHCP `SubnetSet` on arbitrary interfaces, or to bypass provider-based validation.
* Re-introducing multi-network on `NSXTier1`, portgroup-based `Network` inside a VPC namespace, or SR-IOV in a VPC namespace.

## Architecture Areas

This design builds on [One-Pager: Support VKS Cluster Transition from VDS / NSX T1 to NSX VPC Network](./one-pager-vks-cluster-transition.md), which introduced the per-Cluster network provider field `VSphereCluster.spec.network.provider` (the single source of truth for the per-Cluster network provider) with the values `VSphereDistributed`, `NSXTier1`, and `VPC`. This design adds one new value to that set:

```yaml
spec:
  network:
    provider: VSphereDistributed | NSXTier1 | VPC | ExternallyManaged
```

### Supervisor System Namespace Annotation

The Supervisor’s system namespace (e.g. `vmware-system-supervisor`) is annotated by the Supervisor bootstrap stack:

```
clusters.kubernetes.vmware.com/supervisor-namespace: "true"
```

This annotation is the single trust signal that "this namespace is owned by the Supervisor bootstrap stack, and the Cluster inside it is the Supervisor Cluster". It is the gate for:
* Selecting the ExternallyManaged network provider on `VSphereCluster.spec.network.provider`.
* Skipping provider-based validation in the GCC Cluster webhook.

Ordinary workload namespaces never carry this annotation, so the ExternallyManaged provider and the validation skips it unlocks are unreachable from a workload Cluster.

### Cluster API Runtime Extension

On the first reconcile of a Cluster’s topology, the runtime extension sets `VSphereCluster.spec.network.provider` from the namespace’s network provider. `spec.network.provider` is immutable once set. This design adds one rule to that initial-set logic:
* If the Cluster’s namespace carries `clusters.kubernetes.vmware.com/supervisor-namespace: "true"`, set `VSphereCluster.spec.network.provider` to ExternallyManaged instead of the value that would otherwise be derived from the namespace’s network provider.
* Otherwise, behavior is unchanged from the VPC transition design.

### Supervisor Custom ClusterClass

If `Cluster.spec.clusterNetwork.pods` contains dual-stack IP ranges, the Supervisor Cluster is deployed as dual-stack. This requires that the Supervisor bootstrap stack's pre-created Control Plane Load Balancer is also dual-stack. Furthermore, the custom ClusterClass must set both the IPv4 and IPv6 VIPs in `KubeadmControlPlane.spec.kubeadmConfigSpec.clusterConfiguration.apiServer.certSANs` to ensure the certificate is signed with both VIPs.

### Guest Cluster Controller – Cluster Webhook

The GCC Cluster validating webhook today runs a large set of provider-based checks against `Cluster.spec.topology.variables[networks].interfaces` (see "References" at the end of this document). All of those checks assume a single workload network provider per Cluster.

New rule:
* If the Cluster’s namespace carries `clusters.kubernetes.vmware.com/supervisor-namespace: "true"`, the webhook **skips all provider-based validation** on `networks.interfaces` and on Pod / Service CIDRs, and runs only schema-level validation (interface name convention, `mtu` max value, structural required fields, etc.).

Because the annotation is set only by the Supervisor bootstrap stack on its own system namespace, this skip cannot be triggered by a workload Cluster.

### Cluster API vSphere Provider (CAPV)

#### New ExternallyManaged `NetworkProvider`

Add a new implementation of the existing [`NetworkProvider`](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/pkg/services/interfaces.go) interface alongside `VSphereDistributed`, `NSXTier1`, and `VPC`.

Behavior of each method:
* **`HasLoadBalancer() bool`** – returns `false`. CAPV will not create a `VirtualMachineService` for the control plane VIP; the Cluster’s `spec.controlPlaneEndpoint` is pre-set to the bootstrap-managed VIP.
* **`SupportsVMReadinessProbe() bool`** – returns `false`. No TCP-based readiness probe is configured on control plane VMs, as this probe is typically used to perform endpoint health checks for the VirtualMachineService-managed Load Balancer.
* **`SkipCreateSupervisorService() bool`** – returns `true`. The `default/supervisor` Service / Endpoints (reconciled by the ServiceDiscovery controller) proxies from a workload VKS Cluster to the Supervisor kube-apiserver, and is consumed by the para-virtualized add-ons vSphere CPI and vSphere CSI that run inside the VKS Cluster. On the Supervisor Cluster itself vSphere CPI is not deployed and vSphere CSI runs in Supervisor mode, so this Service / Endpoints is unnecessary. All other providers return `false`, preserving today’s behavior for workload Clusters.
* **`ProvisionClusterNetwork(ctx, clusterCtx) error`** – does not create any network object. Sets `VSphereClusterNetworkReadyCondition` to `True` with reason `VSphereClusterNetworkReadyReason` and returns `nil`.
* **`GetClusterNetworkName(ctx, clusterCtx) (string, error)`** – returns `"", nil`. There is no VKS-managed cluster-scoped network name to report.
* **`GetVMServiceAnnotations(ctx, clusterCtx) (map[string]string, error)`** – returns `map[string]string{}, nil`. CAPV does not create a `VirtualMachineService`, so these annotations are unused; an empty map keeps the call site uniform with other providers.
* **`ConfigureVirtualMachine(ctx, clusterCtx, machine, vm) error`** – **straight propagation**. Copies `machine.spec.network.interfaces.primary` to `vm.spec.network.interfaces[0]` (named `eth0`) and each entry in `machine.spec.network.interfaces.secondary[*]` to a corresponding entry in `vm.spec.network.interfaces`. For each VM interface:
  * `name`, `mtu`, and `network` (kind / apiVersion / name) are copied verbatim from the `VSphereMachine` interface.
  * Secondary interfaces have `gateway4: "None"` and `gateway6: "None"` set, matching the existing convention so only the primary interface advertises a default route.
  * If the referenced object is a Subnet, writes the derived IPAM from Subnet.spec.ipAddressType (IPv4, IPv6, IPv4IPv6) to VM interface's ipamModes array ([IPv4], [IPv6], or [IPv4, IPv6])
* **`VerifyNetworkStatus(ctx, clusterCtx, obj) error`** – returns `nil`. This method is currently an orphan on the `NetworkProvider` interface and is not invoked by any CAPV call site; since the ExternallyManaged provider creates no network object there is nothing to verify either way.

#### `VSphereMachine` / `VSphereMachineTemplate` Webhook

The CAPV validating webhooks for `VSphereMachine` and `VSphereMachineTemplate` share a `validateNetwork` helper that switches on the configured `NetworkProvider` and applies provider-specific rules (allowed reference kinds, primary / secondary placement, multi-networks feature gate).

New rule:
* When `VSphereCluster.spec.network.provider == "ExternallyManaged"`, the webhooks **skip provider-based validation** of `spec.network.interfaces` and run only schema-level checks (interface name uniqueness, valid name characters, `mtu` bounds, required fields).

This mirrors the GCC Cluster webhook skip and is necessary because the Supervisor Cluster intentionally references network APIs from multiple providers within a single `VSphereMachine`.

#### Provider Selection

The existing provider-selection code path (which today maps `VSphereCluster.spec.network.provider` to a `NetworkProvider` implementation) gains one new branch:

```
provider == "ExternallyManaged"  ->  externallyManagedNetworkProvider
```

No other CAPV code needs to know about the ExternallyManaged value; everything flows through the `NetworkProvider` interface.

#### Feature Gate

Add a CAPV feature gate, ExternallyManagedProvider (alpha, default off). It gates the CAPV side of this feature: ExternallyManaged provider can be set when the gate is enabled.

#### ServiceDiscovery Controller

The ServiceDiscovery controller reconciles a headless `default/supervisor` Service and its Endpoints inside the target Cluster, pointing them at the discovered Supervisor kube-apiserver VIP/FIP. This Service is consumed by the para-virtualized add-ons vSphere CPI and vSphere CSI, which run inside a workload VKS Cluster and use it to reach the Supervisor kube-apiserver.

The Supervisor Cluster does not need this Service: vSphere CPI is not deployed there, and vSphere CSI runs in Supervisor mode (talking to the local kube-apiserver directly). The controller therefore resolves the Cluster’s `NetworkProvider` and, when `np.SkipCreateSupervisorService()` returns `true` (the ExternallyManaged provider), short-circuits the reconcile without creating or patching the `default/supervisor` Service / Endpoints. It marks the `ServiceDiscoveryReady` condition `True` so the skip is an explicit, observable success rather than a silent no-op.

For all other providers the method returns `false` and the controller behavior is unchanged.

### Network-Topology-Aware Addons

When `VSphereCluster.spec.network.provider` is set to ExternallyManaged, addons use NetworkSettings.provider to determine the network provider for their specific logic.

### Capability Flag

A VKS capability flag `enhancement_for_supervisor_network` gates the feature end-to-end: runtime extension behavior, GCC webhook skip, CAPV provider registration, and CAPV webhook skip.

This flag depends on two other flags:
* The VKS capability flag `per_namespace_network_provider` (see [One-Pager: Support VKS Cluster Transition from VDS / NSX T1 to NSX VPC Network](./one-pager-vks-cluster-transition.md)), because the ExternallyManaged provider is selected per namespace alongside the other per-namespace network providers.
* The Supervisor capability flag supports_per_namespace_network_provider, which also indicates that VM Operator supports `VirtualMachine.spec.network.interfaces` referencing multiple network provider APIs in a single VM.

If either dependency is off, the ExternallyManaged provider is not registered and VKS falls back to the standard per-namespace or global network provider.

## End-to-End Example

Supervisor Cluster manifest, in a namespace annotated as the Supervisor system namespace. The workload network is an NSX VPC `Subnet` (dual-stack) on eth0, the management network is a VDS `Network` on eth1, and the kube-apiserver VIP is pre-created by the bootstrap stack:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: vmware-system-supervisor
  annotations:
    clusters.kubernetes.vmware.com/supervisor-namespace: "true"
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: supervisor
  namespace: vmware-system-supervisor
spec:
  controlPlaneEndpoint:
    host: 192.168.0.2     # pre-created by the Supervisor bootstrap stack
    port: 6443
  clusterNetwork:
    pods:
      cidrBlocks:
      - 192.0.2.0/16
      - fd00:100:64::/108
    serviceDomain: managedcluster.local
    services:
      cidrBlocks:
      - 198.51.100.0/12
      - fd00:100:200::/48
  topology:
    class: builtin-generic-v3.8.0
    version: v1.34.4
    controlPlane:
      replicas: 1
    workers:
      machineDeployments:
      - class: node-pool
        name: workers
        replicas: 1
    variables:
    - name: storageClass
      value: wcpglobal-storage-profile
    - name: vmClass
      value: best-effort-small
    - name: networks
      value:
        interfaces:
          primary:                       # eth0 -> workload (default route)
            network:
              apiVersion: crd.nsx.vmware.com/v1alpha1
              kind: Subnet
              name: supervisor-workload
          secondary:
          - name: eth1                   # eth1 -> management
            mtu: 1700
            network:
              apiVersion: netoperator.vmware.com/v1alpha1
              kind: Network
              name: management-network
```

The runtime extension sets `VSphereCluster.spec.network.provider`:

```yaml
apiVersion: vmware.infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereCluster
metadata:
  name: supervisor-769ww
  namespace: vmware-system-supervisor
spec:
  network:
    provider: ExternallyManaged
```

CAPV propagates `networks.interfaces` from the topology variable into `VSphereMachine.spec.network.interfaces`:

```yaml
apiVersion: vmware.infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereMachine
metadata:
  name: supervisor-workers-85kxh-7kp5v
  namespace: vmware-system-supervisor
spec:
  network:
    interfaces:
      primary:
        network:
          apiVersion: crd.nsx.vmware.com/v1alpha1
          kind: Subnet
          name: supervisor-workload
      secondary:
      - name: eth1
        mtu: 1700
        network:
          apiVersion: netoperator.vmware.com/v1alpha1
          kind: Network
          name: management-network
```

The ExternallyManaged `NetworkProvider.ConfigureVirtualMachine` then straight- propagates this into the `VirtualMachine` spec, setting dual-stack `ipamModes` on eth0 because the referenced `Subnet` is dual-stack:

```yaml
apiVersion: vmoperator.vmware.com/v1alpha6
kind: VirtualMachine
metadata:
  name: supervisor-cp-99tgd
  namespace: vmware-system-supervisor
spec:
  network:
    interfaces:
    - name: eth0
      network:
        apiVersion: crd.nsx.vmware.com/v1alpha1
        kind: Subnet
        name: supervisor-workload
      ipamModes:
      - IPv4
      - IPv6
    - name: eth1
      mtu: 1700
      gateway4: None
      gateway6: None
      network:
        apiVersion: netoperator.vmware.com/v1alpha1
        kind: Network
        name: management-network
```

## Dependencies

* **VM Operator** must support `VirtualMachine.spec.network.interfaces` referencing multiple network provider APIs in a single VM, and expose the Supervisor capability flag that this work depends on.
* **VKS** provides the per-Cluster network provider field `VSphereCluster.spec.network.provider` (single source of truth) plumbing this design extends with the ExternallyManaged value.
* **Supervisor bootstrap stack (WCP)** must:
  * Pre-create the workload `Subnet` / `VirtualNetwork` / `Network`, and the management `Network`.
  * Annotate the Supervisor system namespace with `clusters.kubernetes.vmware.com/supervisor-namespace: "true"`.
  * Set the Cluster’s `spec.controlPlaneEndpoint` to the pre-created control plane Loadbalancer VIP.
  * Set Cluster variable `networks.interfaces` with the workload network on the primary interface and the management network on a secondary interface.
  * Implement a custom ClusterClass to ensure the kube-apiserver certificate is signed with both Loadbalancer IPv4 and IPv6 VIPs in dual-stack network.
  * Implement [Namespace-scoped network provider and configurations](https://vmw-confluence.broadcom.net/x/QMlwkQ).

## References: Existing Provider-Based Webhook Validation (Skipped for ExternallyManaged)

For completeness, the validation paths that the ExternallyManaged provider skips. Each item lists the webhook entry point and the provider-driven logic it runs today; under ExternallyManaged only schema-level checks remain.

### GCC Cluster Webhook

File: [`services/guest-cluster-controller/webhooks/capi/validation/capi_validator.go`](https://github-vcf.devops.broadcom.net/vcf/kubernetes-service/blob/main/services/guest-cluster-controller/webhooks/capi/validation/capi_validator.go). Resolves the namespace’s workload network provider via `validation.GetNetworkProvider(...)` and switches on `VDSNetworkProvider`, `NSXNetworkProvider`, or `VPCNetworkProvider` in:
* **Pod / Service CIDR validation** (provider-specific range / overlap rules).
* **`networks.interfaces` shape and reference kind** (allowed kinds, primary / secondary placement, DHCP rules, `SubnetSet` / `Subnet` status checks).
* **VPC NetworkInfo / namespace state validation** (`ValidateVPCNetworkInfo`, no-SNAT / no-LB VPC namespace rules).

### CAPV `VSphereMachine` / `VSphereMachineTemplate` Webhook

File: [`internal/webhooks/vmware/vspheremachine.go`](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/internal/webhooks/vmware/vspheremachine.go). Shared `validateNetwork` helper:
* Rejects `interfaces` outright when the `MultiNetworks` feature gate is disabled.
* Switches on the webhook’s `NetworkProvider`:
  * `NSXVPCNetworkProvider`: primary must be `SubnetSet`; each secondary must be `SubnetSet` or `Subnet`.
  * `VDSNetworkProvider`: primary forbidden; each secondary must be `Network`.
  * Any other provider (e.g. `NSXTier1`): `interfaces` forbidden entirely.
* Validates interface name uniqueness across primary (`eth0`) and secondaries.

Under `provider: ExternallyManaged`, only the structural / name-uniqueness checks remain; The provider-specific rules restricting whether a Primary or Secondary interface can be configured (and their allowed reference types) are skipped. This allows the Supervisor Cluster to freely configure Primary and Secondary interfaces to mix and match networks across providers for a single VM.