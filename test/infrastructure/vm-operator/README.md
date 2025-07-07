# vm-operator

In this folder we are maintaining code for building the vm-operator manifest.

vm-operator is a component of vCenter supervisor.
CAPV, when running in supervisor mode, delegates the responsibility to create and manage VMs to the vm-operator.

**NOTE:** The vm-operator manifest in this folder and everything else described in this page is **not** designed for
production use and is intended for CAPV development and test only.

## "limited version of the supervisor"

This project has the requirement to test CAPV in supervisor mode using  all the supported versions of
CAPI, CAPV and vCenter supervisor, and also for all the version built from open PRs.

In order to achieve this without incurring the cost/complexity of creating multiple, ad-hoc vCenter distributions, 
we are using a "limited version of the supervisor", composed by the vm-operator and a minimal version of the net-operator.

This "limited version of the supervisor" is considered enough to provide a signal for CAPV development and test; 
however, due to the necessary trade-offs required to get a simple and cheap test environment, the solution described below
does not fit to use for other use cases.

The picture explains how this works in detail:

![Architecture](architecture-part1.drawio.svg)

NOTE: net-operator is not represented for sake of simplicity, it is complementary to the vm-operator.

As you might notice, it is required to have an additional component taking care of setting up the management cluster
and vCenter as required by the vm-operator. This component exist in different variants according to the use cases
described in following paragraphs.

## Tilt for CAPV in supervisor mode using vcsim

NOTE: As of today we are not supporting Tilt development of CAPV in supervisor mode when targeting a real vCenter.

Before reading this paragraph, please familiarize with [vcsim](../vcsim/README.md) documentation.

To use vsphere in supervisor mode it is required to add it to the list of enabled providers in your `tilt-setting.yaml/json`
(note that we are also adding `vsphere-supervisor`, which is a variant that deploys the supervisor's CRDs);
in this case, it is also required to add both the `vm-operator` and `vcsim.

NOTE: before using `vm-operator` for the first time, you have to run `make vm-operator-manifest-build` in the CAPV folder.

```yaml
...
provider_repos:
  - ../cluster-api-provider-vsphere
  - ../cluster-api-provider-vsphere/test/infrastructure/net-operator
  - ../cluster-api-provider-vsphere/test/infrastructure/vcsim
enable_providers:
  - kubeadm-bootstrap
  - kubeadm-control-plane
  - vsphere-supervisor
  - vm-operator
  - net-operator
  - vcsim
extra_args:
  vcsim:
    - "--v=2"
    - "--logging-format=json"
debug:
  vcsim:
    continue: true
    port: 30040
...
```

Note: the default configuration does not allow to debug the vm-operator.

While starting tilt, the vcsim controller will also automatically setup the `default` namespace with
all the pre requisites for the vm-operator to reconcile machines created in it.

If there is the need to create machines in different namespace, it is required to create manually
`VMOperatorDependencies` resource to instruct the vcsim controller to setup additional namespaces too.

The following image summarizes all the moving parts involved in this scenario.

![Architecture](architecture-part2.drawio.svg)

NOTE: net-operator is not represented for sake of simplicity, it is complementary to the vm-operator.

## E2E tests for CAPV in supervisor mode

A subset of CAPV E2E tests can be executed using the supervisor mode by setting `GINKGO_FOCUS="\[supervisor\]"`.

See [Running the end-to-end tests locally](https://cluster-api.sigs.k8s.io/developer/core/testing#running-the-end-to-end-tests-locally) for more details.

Note: The code responsible for E2E tests setup will take care of ensuring the management cluster
and vCenter have all the dependencies required by the vm-operator; The only exception is the Content Library with
Machine templates, that must be created before running the tests.

Note: the operation above (ensure vm-operator dependencies) considers the values provided via
env variables or via test config variables, thus making it possible to run E2E test on the VMC instance used
for vSphere CI as well as on any other vCenter.

## E2E tests for CAPV in supervisor mode using vcsim

A subset of CAPV E2E tests can be executed using the supervisor mode and vcsim as a target infrastructure by setting
`GINKGO_FOCUS="\[vcsim\]\s+\[supervisor\]"`.

Note: The code responsible for E2E tests setup will take care of creating the `VCenterSimulator`, the `ControlPlaneEndpoint`
and to grab required variables from the corresponding `EnvVar`. On top of that, the setup code will also 
create the required `VMOperatorDependencies` resource for configuring the test namespace.

## Building and pushing the VM-operator manifest

Run `make release-vm-operator` to build & publish vm-operator manifest and image to the CAPV staging bucket.

Note: we are maintaining a copy of those artefacts to ensure CAPV test isolation and to allow small customizations
that makes it easier to run the vm-operator in the "limited version of the supervisor", but this might change in the future.
