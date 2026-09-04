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
  * [ ] PR main:
* [ ] Bump to CAPI main
  * Prereq: CAPI already bumped to the next controller-runtime / k8s.io/* minor version
  * For details, see core CAPI issue: "Tasks to bump to Kubernetes v1.x" (section "Using new Kubernetes dependencies")
  * [ ] PR main:
* [ ] [Remove previously deprecated code](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#remove-previously-deprecated-code)

Release-specific tasks:

Late in the cycle:
* [ ] When cutting RC.0: [Create the new release branch](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#create-a-release-branch)
* [ ] [Cut beta/rc/GA releases](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/release/release-tasks.md#cut-a-release)
  * [ ] Bump to CAPI beta/rc/ga
    * Bump `CAPI_HACK_TOOLS_VER`
    * Bump dependencies in go.mod
    * Bump core CAPI provider versions in `test/e2e/config/vsphere.yaml`  (for CAPI ga bump use `go://` for latest stable CAPI)
    * PR main:
    * PR release-1.xx: (n-1):
  * [ ] After minor release: Update README.md
    * PR main:
* [ ] Continuously modify testing to use newer versions of the upcoming Kubernetes release (betas, rcs and GA):
  * Bump `KUBERNETES*` and if needed `KUBERNETES_VERSION_CHAINED_UPGRADE_FROM` in vsphere.yaml
  * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/4167
  * PRs main:
  * PRs release-1.xx: (n-1):
  * PRs release-1.xx: (n-2):
* [ ] Continuously modify testing to use newer versions of CPI (betas, rcs and GA):
  * Bump `CPI_IMAGE_K8S_VERSION` in vsphere.yaml & regenerate `packaging/flavorgen/cloudprovider/cpi/cpi.yaml` by
    checking out the release tag of `https://github.com/kubernetes/cloud-provider-vsphere` and running:
    `helm template charts/vsphere-cpi --namespace kube-system > ../../sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/cloudprovider/cpi/cpi.yaml`
  * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/4172
  * PRs main:
  * PRs release-1.xx: (n-1):
  * PRs release-1.xx: (n-2):

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
      * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/4170
      * PR main:
  * [ ] Use new images in CI:
    * Add the new image to `internal/test/helpers/vcsim/model.go`
    * Bump template env variables in `test/e2e/config/vsphere.yaml` and `test/e2e/config/config-overrides-example.yaml`:
      * Bump `VSPHERE_CONTENT_LIBRARY_ITEMS`, `VSPHERE_IMAGE_NAME`, `VSPHERE_TEMPLATE`, `FLATCAR_VSPHERE_TEMPLATE`
    * Bump in:
      * `test/e2e/data/infrastructure-vsphere-govmomi/main/clusterclass/patch-vsphere-template.yaml`
      * `test/e2e/data/infrastructure-vsphere-supervisor/main/clusterclass/patch-vsphere-template.yaml`
      * `test/extension/handlers/topologymutation/handler.go`
    * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/4171
    * PR main:
    * PR release-1.xx: (n-1):
    * PR release-1.xx: (n-2):
  * [ ] Bump envtest
    * Bump `KUBEBUILDER_ENVTEST_KUBERNETES_VERSION` in `Makefile`
    * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/4173
    * PR main:
    * PR release-1.xx: (n-1):
  * [ ] Start testing with next Kubernetes release on main
    * Bumping KUBERNETES_VERSION_LATEST_CI in vsphere.yaml
    * Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/4174
    * PR main:
    * PR release-1.xx: (n-1):
  * [ ] Update ProwJobs to use the kube-kins image of the new Kubernetes GA release + add upgrade jobs for next release
    * Prior art: https://github.com/kubernetes/test-infra/pull/37778
    * PR:
* [ ] Check the core CAPI Kubernetes bump issue to ensure we do everything necessary in CAPV main as well
  * [ ] Section "Issues specific to the Kubernetes minor release":
  * [ ] Section "Using new Kubernetes dependencies": (kubekins-e2e & envtest are already covered above)
  * [ ] Prior art: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/4195
  * [ ] PR main:

* [ ] Update the GitHub template for this issue if necessary
