# Implementation Plan: ExternallyManaged Network Provider (CAPV only)

This plan covers the CAPV portion of
[One-Pager: An ExternallyManaged CAPV Network Provider for the Supervisor Cluster](./one-pager-externally-managed-capv-network-provider.md).

All the plumbing from the VPC-transition design already exists (the
`VSphereCluster.spec.network.provider` field, the `ClusterNetworkProvider` feature gate,
the `NetworkProviderFactory`, and per-cluster webhook provider resolution), so this feature
layers cleanly on top.

Decisions incorporated from design review:

* CAPV's VSphereCluster webhook only checks the `ExternallyManagedProvider` feature gate;
  namespace gating (the supervisor-namespace annotation) is enforced by the VKS runtime
  extension and GCC webhook, not by CAPV.
* `ConfigureVirtualMachine` requires the primary interface to be defined (error if missing).
* Interface `routes` are NOT propagated to the VM spec.
* The `MultiNetworks` feature-gate check in `validateNetwork` still applies under
  ExternallyManaged; only provider-specific rules are skipped.
* ipamModes derivation from `spec.ipAddressType` applies to both `Subnet` and `SubnetSet`
  references, and is gated by the `IPv6DualStack` feature gate.
* The new `NetworkProvider` method uses positive polarity: `SupportsSupervisorService()`
  (instead of the one-pager's `SkipCreateSupervisorService()`), consistent with the
  existing predicate style (`SupportsVMReadinessProbe`, `SupportsIPv6DualStack`).

## Step 1 — Feature gate

**`feature/feature.go`**

* Add a new `ExternallyManagedProvider` feature gate constant (alpha, default off), with a
  doc comment noting it gates setting `VSphereCluster.spec.network.provider: ExternallyManaged`
  and registering the ExternallyManaged provider.
* Register it in the `supervisorGates` map (`Default: false, PreRelease: featuregate.Alpha`).
  It does not depend on a vm-operator API version from CAPV's perspective (the VM Operator
  capability is checked by VKS), so `supervisorVersionedGates` is not needed.

## Step 2 — Provider name constant and selection

**`pkg/manager/network.go`**

* Add an `ExternallyManagedNetworkProvider = "ExternallyManaged"` constant next to
  `NSXVPCNetworkProvider` / `NSXNetworkProvider` / `VDSNetworkProvider`.
* Add a case for it in the `GetNetworkProvider` switch, returning the new provider
  (constructed with the client, which it needs to resolve `Subnet` / `SubnetSet` objects).
  Do not add a legacy-name mapping — this value never existed as a `--network-provider`
  flag value.

**`pkg/manager/network_factory.go`**

* In `NewPerClusterNetworkProviderFactory`, append `ExternallyManagedNetworkProvider` to the
  registry name list only when `feature.Gates.Enabled(feature.ExternallyManagedProvider)`.
  When the gate is off, `ForCluster` then naturally rejects the value with the existing
  `unknown network provider` error. The static factory path is untouched (ExternallyManaged
  is per-cluster only and requires `ClusterNetworkProvider` to be reachable at all).

## Step 3 — Interface change: `SupportsSupervisorService()`

**`pkg/services/interfaces.go`**

* Add `SupportsSupervisorService() bool` to the `NetworkProvider` interface, with a comment
  explaining it controls whether the ServiceDiscovery controller reconciles the
  `default/supervisor` headless Service/Endpoints in the target cluster. This is the
  one-pager's `SkipCreateSupervisorService()` renamed to positive polarity to match the
  existing predicate style.

**Existing implementations** — add the method returning `true` to each (preserving today's
behavior for workload Clusters):

* `pkg/services/network/netop_provider.go`
* `pkg/services/network/nsxt_provider.go`
* `pkg/services/network/nsxt_vpc_provider.go`
* `pkg/services/network/dummy_provider.go` (the dummy; `dummyLBNetworkProvider` embeds it)

## Step 4 — New ExternallyManaged provider

**New file `pkg/services/network/externally_managed_provider.go`** implementing
`services.NetworkProvider`, struct holding a `client.Client`, plus a constructor following
the `NSXTVpcNetworkProvider(client)` pattern:

* `SupportsIPv6DualStack()` → `false`, with a comment that this method is unreachable for
  this provider (only consumed by the VMService LB path and ServiceDiscovery address
  discovery, both skipped) and dual-stack is instead derived per-interface from the
  referenced `Subnet` / `SubnetSet`.
* `HasLoadBalancer()` → `false`. This makes `CPService.ReconcileControlPlaneEndpointService`
  a no-op and, because the Supervisor bootstrap stack pre-sets
  `Cluster.spec.controlPlaneEndpoint`, the first branch of `reconcileControlPlaneEndpoint`
  in `controllers/vmware/vspherecluster_reconciler.go` already handles condition-marking —
  no reconciler changes needed.
* `SupportsVMReadinessProbe()` → `false` (consumed at
  `controllers/vspheremachine_controller.go` where
  `ConfigureControlPlaneVMReadinessProbe` is set).
* `SupportsSupervisorService()` → `false`.
* `ProvisionClusterNetwork(...)` → creates nothing; marks
  `VSphereClusterNetworkReadyCondition` `True` with `VSphereClusterNetworkReadyReason` plus
  the deprecated `ClusterNetworkReadyV1Beta1Condition`, mirroring
  `netopNetworkProvider.ProvisionClusterNetwork`.
* `GetClusterNetworkName(...)` → `"", nil`.
* `GetVMServiceAnnotations(...)` → empty map, `nil`.
* `VerifyNetworkStatus(...)` → `nil` (orphan method, nothing to verify).
* `ConfigureVirtualMachine(...)` → straight propagation:
  * Error if `machine.Spec.Network.Interfaces.Primary` is not defined (the Supervisor
    contract requires the workload network on `eth0`).
  * Primary → `vm.Spec.Network.Interfaces[0]` named `PrimaryInterfaceName` (`eth0`), copying
    `NetworkRef` kind/apiVersion/name and MTU.
  * Secondaries → subsequent entries copying name, network ref, and MTU, with
    `Gateway4`/`Gateway6` set to `"None"`. Note: the shared `setVMSecondaryInterfaces`
    helper applies one `ipamModes` slice to all secondaries and propagates routes, which
    doesn't fit here; write a provider-local loop instead.
  * Do NOT propagate interface `routes` to the VM spec.
  * Per-interface ipamModes (gated by the `IPv6DualStack` feature gate; when the gate is
    disabled, leave `IPAMModes` unset on all interfaces): when an interface's `NetworkRef`
    GVK equals `NetworkGVKNSXTVPCSubnet` or `NetworkGVKNSXTVPCSubnetSet`
    (`pkg/services/network/constants.go`), `Get` the `Subnet` / `SubnetSet` from the
    machine's namespace and map its `spec.ipAddressType` → `IPAMModes`:
    `IPv4` → `[IPv4]`, `IPv6` → `[IPv6]`, `IPv4IPv6` → `[IPv4, IPv6]` (empty/unset defaults
    to `[IPv4]` per the CRD default). For other refs (`Network`, `VirtualNetwork`,
    DHCP-backed networks) leave `IPAMModes` unset.
  * Propagate `machine.Spec.Network.VLANs` to the VM spec (same as VPC; gated by
    `VLANSubinterface`).

**RBAC**: the provider now `Get`s `subnets.crd.nsx.vmware.com` (`subnetsets` RBAC already
exists). Add a `+kubebuilder:rbac` marker for `subnets;subnets/status` (get;list;watch) —
the natural home is next to the existing subnetsets marker in
`controllers/vmware/vspherecluster_reconciler.go` — then regenerate
`config/rbac/role.yaml` (`make generate`).

## Step 5 — Webhook changes

**`internal/webhooks/vmware/vspheremachine.go` — `validateNetwork`** (shared by the
`VSphereMachine` and `VSphereMachineTemplate` webhooks, so one change covers both):

* Keep the `MultiNetworks` feature-gate rejection as the outer check (unchanged).
* In the provider switch, add a `manager.ExternallyManagedNetworkProvider` case that
  performs no reference-kind or primary/secondary-placement validation (i.e., an empty
  case, so it doesn't fall into the `default` "interfaces can not be set" rejection).
* The interface-name-uniqueness check after the switch already runs for every provider —
  leave it; that plus the CRD schema markers (name pattern, MTU bounds, required fields)
  constitute the remaining schema-level validation.
* `validateVLANs` allows VLANs for both VPC and ExternallyManaged (still gated by
  `VLANSubinterface`).

**`internal/webhooks/vmware/vspherecluster.go` — `validateClusterNetwork`**:

* After the provider value is resolved (the `ClusterNetworkProvider`-gated branch), reject
  `provider == manager.ExternallyManagedNetworkProvider` with a `field.Forbidden` when
  `feature.Gates.Enabled(feature.ExternallyManagedProvider)` is false ("provider
  ExternallyManaged can only be set when feature gate ExternallyManagedProvider is
  enabled"). No namespace-annotation check in CAPV — that trust gate lives in the VKS
  runtime extension / GCC webhook.
* The existing `nsxVPC can only be set when provider is VPC` check already covers
  ExternallyManaged correctly.

No changes needed in `internal/webhooks/vmware/network_provider.go`
(`resolveNetworkProvider` returns the provider value as-is) or `vsphereclustertemplate.go`.

## Step 6 — ServiceDiscovery controller

**`controllers/vmware/servicediscovery_controller.go`**:

* In `Reconcile`, immediately after the `NetworkProviderFactory.ForCluster` call succeeds
  and before `clusterCache.GetClient` (no reason to wait for guest-cluster connectivity
  when we won't touch it): if `!np.SupportsSupervisorService()`, mark
  `VSphereClusterServiceDiscoveryReadyCondition` `True` with
  `VSphereClusterServiceDiscoveryReadyReason` (and the deprecated
  `ServiceDiscoveryReadyV1Beta1Condition`), log the skip, and return without error. The
  deferred `patch` persists the condition, making the skip an explicit observable success.

## Step 7 — Tests

* **`pkg/services/network/network_test.go`** (+ suite): tests for the new provider —
  `HasLoadBalancer` / `SupportsVMReadinessProbe` / `SupportsSupervisorService` /
  `GetClusterNetworkName` / `GetVMServiceAnnotations` return values;
  `ProvisionClusterNetwork` creates nothing and sets conditions; `ConfigureVirtualMachine`
  covering: error on missing primary; verbatim propagation of name/MTU/network; routes NOT
  propagated; secondary `gateway4/gateway6: None`; ipamModes derivation for IPv4, IPv6,
  dual-stack, and defaulted `Subnet` and `SubnetSet` with the `IPv6DualStack` gate enabled;
  no ipamModes when the gate is disabled; no ipamModes for `Network`/`VirtualNetwork` refs;
  error when a referenced `Subnet`/`SubnetSet` is missing; mixed-provider refs in one VM
  (the end-to-end example from the one-pager: VPC `Subnet` on eth0 + VDS `Network` on
  eth1); VLAN propagation when `VLANSubinterface` is enabled.
* **`pkg/manager/network_factory_test.go` / `network_test.go`**: registry contains
  ExternallyManaged only when the gate is enabled; `GetNetworkProvider` returns the new
  provider for the name.
* **`internal/webhooks/vmware/vspheremachine_test.go` / `vspheremachinetemplate_test.go`**:
  under provider ExternallyManaged, cross-provider interface mixes and primary+secondary
  combinations are accepted; duplicate interface names still rejected; interfaces still
  rejected when MultiNetworks is off; VLANs accepted (same rules as VPC).
* **`internal/webhooks/vmware/vspherecluster_test.go`**: `provider: ExternallyManaged`
  rejected with `ExternallyManagedProvider` gate off, accepted with it on (with
  `ClusterNetworkProvider` also on).
* **`controllers/vmware/servicediscovery_controller_unit_test.go`**: with a provider whose
  `SupportsSupervisorService` is false, no Service/Endpoints created and
  `ServiceDiscoveryReady` condition is `True`.

## Step 8 — Generated artifacts and docs

* `make generate` for RBAC (Step 4) — no API type changes, so no CRD/conversion
  regeneration expected.
* Optionally mention the new gate in the feature-gate section of module docs; no `config/`
  manifest changes beyond generated RBAC.

## Explicitly not changed

* `controllers/vmware/vspherecluster_reconciler.go` control-plane-endpoint logic (the
  pre-set `Cluster.spec.controlPlaneEndpoint` already short-circuits it).
* `controllers/vspheremachine_controller.go` (flows through the `NetworkProvider`
  interface).
* The behavior of the `VSphereDistributed`, `NSXTier1`, and `VPC` providers.
* The govmomi (non-supervisor) side of CAPV.

## Suggested implementation / PR order

Each step below keeps every intermediate state compiling and shippable:

1. Steps 1–3: feature gate, provider name constant, interface method with `true`
   implementations everywhere.
2. Step 4: the new provider, its registration, and RBAC.
3. Step 5: webhook changes.
4. Step 6: ServiceDiscovery skip.

Tests (Step 7) land alongside each step.

## Deviations from the one-pager

* The one-pager's `SkipCreateSupervisorService()` (ExternallyManaged returns `true`, others
  `false`) is implemented as `SupportsSupervisorService()` with inverted polarity
  (ExternallyManaged returns `false`, others `true`), matching the existing predicate style.
* The one-pager derives ipamModes only when the referenced object is a `Subnet`; this plan
  extends the same derivation to `SubnetSet` references, and gates it behind the
  `IPv6DualStack` feature gate.
* The one-pager doesn't mention `SupportsIPv6DualStack` at all — this plan sets it to
  `false` with an explanatory comment since both call sites are unreachable for this
  provider.
