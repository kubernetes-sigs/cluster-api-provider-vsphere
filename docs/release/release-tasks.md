# Release Tasks

**Notes**:

- The examples in this document are based on the v1.8 release cycle.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Prepare main branch for development of the new release](#prepare-main-branch-for-development-of-the-new-release)
- [Remove previously deprecated code](#remove-previously-deprecated-code)
- [[Optional] Bump the Kubernetes version](#optional-bump-the-kubernetes-version)
- [Bump dependencies](#bump-dependencies)
- [Create a release branch](#create-a-release-branch)
- [Cut a release](#cut-a-release)
- [[Continuously] Reduce the amount of flaky tests](#continuously-reduce-the-amount-of-flaky-tests)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Prepare main branch for development of the new release

TODO(sbueringer): Finalize this section once we do this for the first time

The goal of this issue is to bump the versions on the main branch so that the upcoming release version
is used for e.g. local development and e2e tests. We also modify tests so that they are testing the previous release.

This comes down to changing occurrences of the old version to the new version, e.g. `v1.7` to `v1.8`:

1. Setup E2E tests for the new release:
   1. Goal is that our clusterctl upgrade tests are testing the right versions. For `v1.8` this means:
      - v1beta1: `v1.8` (will change with each new release)
      - v1beta1: `v1.7` (will change with each new release)
   2. Update providers in `vsphere-ci.yaml`, `vsphere-dev.yaml`, `integration-dev.yaml`:
      1. Add a new `v1.7.0` entry.
      2. Remove providers that are not used anymore in clusterctl upgrade tests.
      3. Change `v1.7.99` to `v1.8.99`.
   3. Adjust `metadata.yaml`'s:
      1. Add new release to the top-level `metadata.yaml`
      2. Create a new `v1.7` `metadata.yaml` (`test/e2e/data/shared/v1.7/v1beta1_provider/metadata.yaml`) by copying
        `test/e2e/data/shared/main/v1beta1_provider/metadata.yaml`
      3. Add the new v1.8 release to the main `metadata.yaml` (`test/e2e/data/shared/main/v1beta1_provider/metadata.yaml`).
      4. Remove old `metadata.yaml`'s that are not used anymore in clusterctl upgrade tests.
   4. Adjust cluster templates in `test/e2e/data/infrastructure-vsphere`:
      1. Create a new `v1.7` folder. It should be created based on the `main` folder and only contain the templates
        we use in the clusterctl upgrade tests (as of today `remote-management`).
      2. Remove old folders that are not used anymore in clusterctl upgrade tests.
   5. Modify the test specs in `test/e2e/capi_clusterctl_upgrade_test.go` (according to the versions we want to test described above).
      Please note that both `InitWithKubernetesVersion` and `WorkloadKubernetesVersion` should be the highest mgmt cluster version supported by the respective Cluster API version.
2. Update `clusterctl-settings.json`: `v1.7.99` => `v1.8.99`.
3. Make sure all tests are green (also run `pull-cluster-api-provider-vsphere-e2e-full-main` and `pull-cluster-api-provider-vsphere-conformance-main`).

Prior art: TODO(sbueringer): link example PR

## Remove previously deprecated code

The goal of this task is to remove all previously deprecated code that can be now removed.

1. Check for deprecated code and remove it.
   **Note**: We can't just remove all code flagged with `Deprecated`. In some cases like e.g. in API packages
   we have to keep the old code.

Prior art: TODO(sbueringer): link example PR

## [Optional] Bump the Kubernetes version

TODO(sbueringer): Write this when we do it the next time

## Bump dependencies

The goal of this task is to ensure that we have relatively up-to-date dependencies at the time of the release.
This reduces the risk that CVEs are found in outdated dependencies after our release.

We should take a look at the following dependencies:

- Go dependencies in the `go.mod` file. (usually dependabot takes care of that)
- Tools used in our Makefile (e.g. kustomize).

## Create a release branch

The goal of this task is to ensure we have a release branch with test coverage and results in testgrid. While we
add test coverage for the new release branch we will also drop the tests for old release branches if necessary.
The milestone applier should also apply milestones accordingly.
From this point forward changes which should land in the release have to be cherry-picked into the release branch.

1. Create the release branch locally based on the latest commit on main and push it.

   ```bash
      # Create the release branch
      git checkout -b release-1.8

      # Push the release branch
      # Note: `upstream` must be the remote pointing to `github.com/kubernetes-sigs/cluster-api-provider-vsphere`.
      git push -u upstream release-1.8
     ```

2. Create the milestone for the new release via [GitHub UI](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/milestones/new).
3. Update the [milestone applier config](https://github.com/kubernetes/test-infra/blob/151bab62dc023525f592e6d1fdc2a8de5305cd01/config/prow/plugins.yaml#L523) accordingly (e.g. `release-1.8: v1.8`
   and `main: v1.9`)
4. Create new jobs based on the jobs running against our `main` branch:
   1. Copy `config/jobs/kubernetes-sigs/cluster-api-provider-vsphere/cluster-api-provider-vsphere-periodics-main.yaml` to `config/jobs/kubernetes-sigs/cluster-api-provider-vsphere/cluster-api-provider-vsphere-periodics-release-1.8.yaml`.
   2. Copy `config/jobs/kubernetes-sigs/cluster-api-provider-vsphere/cluster-api-provider-vsphere-presubmits-main.yaml` to `config/jobs/kubernetes-sigs/cluster-api-provider-vsphere/cluster-api-provider-vsphere-presubmits-release-1.8.yaml`.
   3. Modify the following:
      1. Rename the jobs, e.g.: `periodic-cluster-api-provider-vsphere-test-main` => `periodic-cluster-api-provider-vsphere-test-release-1-8`.
      2. Change `annotations.testgrid-tab-name`, e.g. `periodic-test-main` => `periodic-test-release-1-8`.
      3. For periodics additionally:
         - Change `extra_refs[].base_ref` to `release-1.8` (for repo: `cluster-api-provider-vsphere`).
      4. For presubmits additionally:
         - Adjust branches: `^main$` => `^release-1.8$`.
5. Remove tests for old release branches if necessary
6. Verify the jobs and dashboards a day later by taking a look at [testgrid](https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-vsphere)
7. Update `.github/workflows/weekly-security-scan.yaml` - to setup Trivy and govulncheck scanning - `.github/workflows/weekly-md-link-check.yaml` - to setup link checking in the CAPI book - and `.github/workflows/weekly-test-release.yaml` - to verify the release target is working - for the currently supported branches.
8. Update the [PR markdown link checker](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/.github/workflows/pr-md-link-check.yaml) accordingly (e.g. `main` -> `release-1.8`).
   - Prior art: [Update branch for link checker](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/pull/2242)

## Cut a release

1. Ensure via testgrid that CI is stable before cutting the release
   Note: special attention should be given to image scan results, so we can avoid cutting a release with CVEs or document known CVEs in release notes.
2. Create and push the release tags to the GitHub repository:

   ```bash
      # Export the tag of the release to be cut, e.g.:
      export RELEASE_TAG=v1.8.0-beta.0
      # Create tags locally
      # Warning: The test tag MUST NOT be an annotated tag.
      git tag -s -a ${RELEASE_TAG} -m ${RELEASE_TAG}

      # Push tags
      # Note: `upstream` must be the remote pointing to `github.com/kubernetes-sigs/cluster-api-provider-vsphere`.
      git push upstream ${RELEASE_TAG}
   ```

   This will automatically trigger:
   - a [GitHub Action](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/actions/workflows/release.yaml) to create a draft release and
   - a [ProwJob](https://prow.k8s.io/?repo=kubernetes-sigs%2Fcluster-api-provider-vsphere&job=post-cluster-api-provider-vsphere-push-images) to publish images to the staging image repository.
3. Promote images from the staging repository to the production registry (`registry.k8s.io/cluster-api-vsphere`):
   1. Wait until images for the tag have been built and pushed to the [staging repository](https://console.cloud.google.com/gcr/images/k8s-staging-capi-vsphere/global/cluster-api-vsphere-controller) by
      the [push images job](https://prow.k8s.io/?repo=kubernetes-sigs%2Fcluster-api-provider-vsphere&job=post-cluster-api-provider-vsphere-push-images).
   2. If you don't have a GitHub token, create one by going to your GitHub settings in [Personal access tokens](https://github.com/settings/tokens). Make sure you give the token the `repo` scope.
   3. Create a PR to promote the images to the production registry:

      ```bash
      # Export the tag of the release to be cut, e.g.:
      export RELEASE_TAG=v1.8.0-beta.0
      export GITHUB_TOKEN=<your GH token>
      make promote-images
      ```

      **Notes**:
       - `make promote-images` target tries to figure out your Github user handle in order to find the forked [k8s.io](https://github.com/kubernetes/k8s.io) repository.
       If you have not forked the repo, please do it before running the Makefile target.
       - if `make promote-images` fails with an error like `FATAL while checking fork of kubernetes/k8s.io` you may be able to solve it by manually setting the USER_FORK variable
       i.e.  `export USER_FORK=<personal GitHub handle>`
       - `kpromo` uses `git@github.com:...` as remote to push the branch for the PR. If you don't have `ssh` set up you can configure
       git to use `https` instead via `git config --global url."https://github.com/".insteadOf git@github.com:`.
       - This will automatically create a PR in [k8s.io](https://github.com/kubernetes/k8s.io) and assign the CAPV maintainers.
4. Merge the PR (/lgtm + /hold cancel) and verify the images are available in the production registry:
    - Wait for the [promotion prow job](https://prow.k8s.io/?repo=kubernetes%2Fk8s.io&job=post-k8sio-image-promo) to complete successfully. Then verify that the production images are accessible:

     ```bash
     docker pull registry.k8s.io/cluster-api-vsphere/cluster-api-vsphere-controller:${RELEASE_TAG}
     ```

5. Publish the release.
   1. Finalize release notes
      1. Pay close attention to the `## :question: Sort these by hand` section, as it contains items that need to be manually sorted.
      2. Ensure consistent formatting of entries (e.g. prefix).
      3. Merge dependency bump PR entries for the same dependency into a single entry.
      4. Move minor changes into a single line at the end of each section.
      5. Sort entries within a section alphabetically.
      6. Write highlights section based on the initial release notes doc. (for minor releases and important changes only)
      7. **For minor releases** Modify `Changes since v1.x.y` to `Changes since v1.x`
         **Note**: The release notes tool includes all merges since the previous release branch was branched of.
   2. Publish the release and ensure release is flagged as `pre-release` for all `beta` and `rc` releases or `latest` for a new release in the most recent release branch.
6. **For minor releases** Update supported versions in README.md.

- Cutting a release as of today requires permissions to:
  - Create a release tag on the GitHub repository.
  - Create/update/publish GitHub releases.

## [Continuously] Reduce the amount of flaky tests

The CAPV tests are pretty stable, but there are still some flaky tests from time to time. To reduce the amount of flakes please periodically:

1. Take a look at recent CI failures via [k8s-triage](https://storage.googleapis.com/k8s-triage/index.html?job=.*cluster-api-provider-vsphere.*)
2. Open issues for occurring flakes and ideally fix them or find someone who can.
