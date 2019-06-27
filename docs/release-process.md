# CAPV Release Process

This document outlines the strategy and process for building and releasing CAPV images.

## Goals

* Define what images/artifacts are built and where they are published
* Define when and how the git repo is tagged
* Define how and where container images are built
* Define what tags are applied to container images and under what scenarios those tags may change

## Non-Goals

* To mandate any specific implementation to achieve Goals
* To place requirements on intermediate/temporary images that are built solely for purposes of CI and E2E testing

## Artifacts

The CAPV project has three main artifacts that are for public consumption

* CAPV manager container image
  * This container image contains the main CAPV manager binary, and is what is deployed to a K8s cluster to make it a CAPI management cluster capable of deploying workload clusters.
* `clusterctl` binary
  * This is a binary that provides the vSphere-capable `clusterctl` CLI, which can be used to bootstrap a management cluster. Use of this tool is not required.
* CAPV machine images
  * These are pre-built VM images intended for use with CAPV. They come pre-loaded with all the binaries and container images needed to run a K8s cluster. An image exists for every combination of Operating System and K8s version that CAPV supports.
  * These images shall be in OVA format for consumption by vSphere infrastructure.

## Container Registry Hosting

The CAPV manager container image is made available via a public container registry service. The image name is `manager`. Within the registry, the image should be tagged according to the following:

* Each image should be tagged with the build version, as described by git with the following command: `git describe --always --dirty --abbrev=8`. This has the side-effect of mandating git annotated tags, which is standard industry practice for tagging releases.
* Images are published in different release "channels", with each channel indicating the target audience of the image. These channels shall be:
  * `release` - Images intended for long-term use that have been thoroughly tested. An example image location would be `{registry}/cluster-api-provider-vspehre/release/manager:v0.3.0`
  * `edge` - Images built from the latest commit on the `master` repo branch. Example: `{registry}/cluster-api-provider-vsphere/edge/manager:v0.3.0-1-gee847d90`
  * `ci` - Images intended solely for the purpose of CI/testing. These images cannot be expected to remain in the registry long term.
* GA releases should be tagged with a `vX.Y.Z` format. This release numbering should follow semantic versioning, and is also the correct format for projects using Go modules.
* The latest release of the highest version should also carry the `latest` tag. For example, if `v1.3.0` is released, but later a bug-fix release for `v1.2.5` is made, `v1.3.x` should maintain the `latest` tag.
* Images that are published to the registry `edge` channel via CI of the most recent build from the git `master` branch should be tagged `latest` in addition to the version tag.

The only image tag that should ever move is `latest`.

## Binary/Image hosting

The `clusterctl` binary and OVA images are not hosted on a container registry, thus they must be made publicly available via a cloud storage solution such as GCS or S3. The locations of these artifacts should be well known, with consistent file naming that clearly indicates a version or other distinguishing information.

### clusterctl

The `clusterctl` binary should be tagged with the same version as the CAPV manager image, e.g. the output from `git describe --always --dirty --abbrev=8`. GA releases should be found in a bucket with the `release` folder in it, along with the version. An example would be a bucket name of `capv-clusterctl/release/v0.3.0`.

TODO: How should we handle the latest `edge` builds? If people are using `edge`, do we assume it's for devs and that they should just built the binary directly?

### Machine Images

The OVAs that are consumed by CAPV must also live in a publicly accessible storage bucket. These images are not built on the same cadence as the other CAPV artifacts. Rather, they are built when a new K8s GA release is made. An image is made for each OS and supported K8s version. To make it clear what each image supports, the image shall be found in a `release/vX.Y.Z` folder, where the `vX.Y.Z` tag indicates the version of K8s contained within the image.

Furthermore, each image name should contain the Operating System it is built with, and the K8s version so that the filename alone is enough to distinguish the image. An example would be `capv-images/release/v1.14.2/centos-7-kube-v1.14.2.ova`.

TODO: We are doing this right now, but the image makes no indication of what version of CAPI or CAPV is in the image. That info is available within the OVA, but do we want it in the filename? The folder structure? Do we picture building multiple images where CAPV is the only difference, for example? Say we released `centos-7-kube-v1.14.2.ova` with CAPV v0.3.0 in it, but then issued a bugfix for CAPV v0.3.1 -- do we build new images? overwrite old ones?

## Repo Tagging

The git repo is tagged to signify a (pre-)release by the project tech lead(s). The tag for a new release should be applied to an existing commit and pushed directly to the repo. The tag must be an annotated tag (e.g. `git tag -a`). For example, `git tag -a v0.3.0` followed by `git push upstream v0.3.0`.

TODO: We could define more explicitly how to handle long-lived release branches for bug fixes, e.g. if master is currently v0.3.x development, but there is a need for a v0.2.x bug fix.

## Artifact Build And Publishing

Artifacts should be built and published by a CI system rather than manually. When a new tag is pushed to the repo, the CI system should react to that push and build a new version of the artifacts that is properly tagged with the version matching the tag. For commits to `master` that are not tagged as a release, artifacts are still published to the `edge` channel with the version set to `git describe --always --dirty --abbrev=8`, and with the `latest` tag.

In the absence of the CI system of choice being able to react to pushes that only contain a tag but no file changes, the project tech lead(s) can build and push manually as a temporary solution.
