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
* [ ] [Remove previously deprecated code](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#remove-previously-deprecated-code)

After the Kubernetes minor release:
* [ ] Bump the Kubernetes version 
  * [ ] Publish new OVA images
    * Build new OVA images via image-builder
    * Make them available in the CI environment
    * Publish them via a GitHub release (e.g. [templates/v1.30.0](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/releases/tag/templates/v1.30.0))
    * Update `README.md` accordingly
  * [ ] Bump e2e tests
    * Add the new image to `internal/test/helpers/vcsim/model.go`
    * Bump template env variables in `test/e2e/config/vsphere.yaml` and `test/e2e/config/config-overrides-example.yaml`
      * Also bump `KUBERNETES_VERSION_*`
      * Also bump `KUBERNETES_VERSION_LATEST_CI` to the next minor version
      * Also bump `CPI_IMAGE_K8S_VERSION`
      * Regenerate `packaging/flavorgen/cloudprovider/cpi/cpi.yaml` by checking out the release tag of `https://github.com/kubernetes/cloud-provider-vsphere` and running `helm template charts/vsphere-cpi`
    * Bump in:
      * `test/e2e/data/infrastructure-vsphere-govmomi/main/clusterclass/patch-vsphere-template.yaml`
      * `test/e2e/data/infrastructure-vsphere-supervisor/main/clusterclass/patch-vsphere-template.yaml` 
  * [ ] Update ProwJob configuration accordingly

Late in the cycle:
* [ ] [Bump dependencies](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#bump-dependencies)
* [ ] [Create the new release branch](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#create-a-release-branch)
* [ ] [opt] [Cut beta/rc releases](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#cut-a-release)
* [ ] [Cut the minor release](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#cut-a-release)

Continuously:
* [Reduce the amount of flaky tests](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#continuously-reduce-the-amount-of-flaky-tests)
