---
name: 🚋 Release cycle tracking
about: Create a new release cycle tracking issue for a CAPV minor release
title: Tasks for v<release-tag> release cycle
labels: ''
assignees: ''

---

Please see the corresponding sections in [release-tasks.md](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md) for documentation of individual tasks.  

## Tasks

Early in the cycle:
* [ ] [Prepare main branch for development of the new release](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#prepare-main-branch-for-development-of-the-new-release)
* [ ] Bump to CAPI main
  * Prereq: CAPI already bumped to the next controller-runtime / k8s.io/* minor version
  * For details, see core CAPI issue: "Tasks to bump to Kubernetes v1.x" (section "Using new Kubernetes dependencies")
* [ ] [Remove previously deprecated code](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#remove-previously-deprecated-code)

Release-specific tasks:

Late in the cycle:
* [ ] [Bump dependencies](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#bump-dependencies)
* [ ] When cutting RC.0: [Create the new release branch](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#create-a-release-branch)
* [ ] [opt] [Cut beta/rc releases](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#cut-a-release)
  * [ ] Before every release ensure we bumped to latest CPI & CAPI & Kubernetes pre-releases
    * Bump `CAPI_HACK_TOOLS_VER`
    * Bump dependencies in go.mod
    * Bump core CAPI provider versions in `test/e2e/config/vsphere.yaml`
    * Bump `KUBERNETES*` and `CPI_IMAGE_K8S_VERSION` to latest pre-releases
    * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/3424
  * [ ] Cherry-pick if the release branch was already cut

After the Kubernetes minor release:
* [ ] Bump the Kubernetes version 
  * [ ] Publish new OVA images
    * [ ] Build new OVA images with image-builder via a temporary VM in vCenter
    * [ ] Publish images to the GCVE CI environment via the vCenter UI:
      * Go to the folder view
        * [ ] Move `ubuntu-2404` and `flatcar` from `Workload VMs` to `prow/templates`
          * If this is not possible for some reason, you can upload `ubuntu-2404` and `flatcar` manually:
            * Right-click on the `prow/templates` folder | Deploy OVF Template
              * Select local file
              * Select a name and folder: `prow/templates`
              * Select a compute resource: `k8s-gcve-cluster`
              * Select storage: `vsanDatastore`
              * Select networks: Destination Network: `k8s-ci`
              * Finish & wait for upload to complete (under tasks)
        * [ ] Right-click the templates and `Clone to library` to `capv`
    * [ ] Publish them via a GitHub release (e.g. [templates/v1.30.0](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/releases/tag/templates/v1.30.0))
    * [ ] Delete the temporary VM used to build the images
    * [ ] Delete `ubuntu-2204` and `photon` templates from `Workload VMs`
    * [ ] Update `README.md` accordingly
      * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/3437
  * [ ] Bump unit & e2e tests to use the new Kubernetes version
    * Add the new image to `internal/test/helpers/vcsim/model.go`
    * Bump `KUBEBUILDER_ENVTEST_KUBERNETES_VERSION` in `Makefile`
    * Bump template env variables in `test/e2e/config/vsphere.yaml` and `test/e2e/config/config-overrides-example.yaml`
      * Bump `KUBERNETES_VERSION_*`
      * Bump `KUBERNETES_VERSION_LATEST_CI` to the next minor version
      * Bump `VSPHERE_CONTENT_LIBRARY_ITEMS`, `VSPHERE_IMAGE_NAME`, `VSPHERE_TEMPLATE`, `FLATCAR_VSPHERE_TEMPLATE`
      * Bump `CPI_IMAGE_K8S_VERSION` & regenerate `packaging/flavorgen/cloudprovider/cpi/cpi.yaml` by
        checking out the release tag of `https://github.com/kubernetes/cloud-provider-vsphere` and running:
        `helm template charts/vsphere-cpi --namespace kube-system > ../../sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/cloudprovider/cpi/cpi.yaml`
    * Bump in:
      * `test/extension/handlers/topologymutation/handler.go`
      * `test/e2e/data/infrastructure-vsphere-govmomi/main/clusterclass/patch-vsphere-template.yaml`
      * `test/e2e/data/infrastructure-vsphere-supervisor/main/clusterclass/patch-vsphere-template.yaml`
    * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/3294
    * [ ] Cherry-pick
  * [ ] Bump CPI GA release as soon as available
    * [ ] Cherry-pick
  * [ ] Update ProwJobs to use the Kubernetes GA release
    * Prior art: https://github.com/kubernetes/test-infra/pull/34688
  * [ ] Start using next Kubernetes release on main
    * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/3438

Prepare & publish the release:
* [ ] Bump to latest CAPI version (Kubernetes & CPI if necessary) 
  * Bump `CAPI_HACK_TOOLS_VER`
  * Bump dependencies in go.mod
  * Bump core CAPI provider versions in `test/e2e/config/vsphere.yaml`
  * Bump `KUBERNETES*` and `CPI_IMAGE_K8S_VERSION` to latest releases, if necessary
  * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/3291
  * [ ] Cherry-pick
* [ ] [Cut the minor release](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#cut-a-release)

Continuously:
* [Reduce the amount of flaky tests](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#continuously-reduce-the-amount-of-flaky-tests)
