# Kubernetes cluster-api-provider-vsphere Project

This repository hosts a concrete implementation of a provider for vsphere for the [cluster-api project](https://github.com/kubernetes-sigs/cluster-api).

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

* Join our Cluster API vSphere Provider sessions
  * Bi-weekly on Mondays @ 1:00 PT on [Zoom](https://zoom.us/j/875399243) starting on 8/13
  * Previous meetings: \[ [notes](https://docs.google.com/document/d/1jQrQiOW75uWraPk4b_LWtCTHwT7EZwrWWwMdxeWOEvk/edit?usp=sharing) \]

You can reach the maintainers of this project at:

- [Slack](http://slack.k8s.io/): #cluster-api
- [Mailing List](https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

### Quick Start

Go [here](docs/README.md) for an example of how to get up and going with the cluster api using vSphere.

### Where to get the containers

The containers for this provider are currently hosted at `gcr.io/cnx-cluster-api/`.  Each release of the
container are tagged with the release version appropriately.  Please note, the release tagging changed to
stay uniform with the main cluster api repo.  Also note, these are docker containers.  A container runtime
must pull them.  They cannot simply be downloaded.

| vSphere provider version | container url |
| --- | --- |
| 0.1.0 | gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:v0.1 |
| 0.2.0 | gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:0.2.0 |

| main Cluster API version | container url |
| --- | --- |
| 0.1.0 | gcr.io/k8s-cluster-api/cluster-api-controller:0.1.0 |

To use the appropriate version (instead of `:latest`), replace the version in the generated `provider-components.yaml`,
described in the quick start guide.

### Compatibility Matrix

Below are tables showing the compatibility between versions of the vSphere provider, the main cluster api,
kubernetes versions, and OSes.  Please note, this table only shows version 0.2 of the vSphere provider.  Due
to the way this provider bootstrap nodes (e.g. using Ubuntu package manager to pull some components), there
were changes in some packages that broke version 0.1 (but may get resolved at some point) so the compatibility
tables for that provider version are not provided here.
                              
Compatibility matrix for Cluster API versions and the vSphere provider versions.

| | Cluster API 0.1.0 |
|--- | --- |
| vSphere Provider 0.2.0 | ✓ |

Compatibility matrix for the vSphere provider versions and Kubernetes versions.

| |k8s 1.11.x|k8s 1.12.x|k8s 1.13.x|k8s 1.14.x|
|---|---|---|---|---|
| vSphere Provider 0.2.0 | ✓ | ✓ | ✓ | ✓ |

Compatibility matrix for the vSphere provider versions and node OS.  Further OS support may be added in future releases.

| | Ubuntu Xenial Cloud Image | Ubuntu Bionic Cloud Image |
| --- | --- | --- |
| vSphere Provider 0.2.0 | ✓ | ✓ |

Users may download the cloud images here:

[Ubuntu Xenial (16.04)](https://cloud-images.ubuntu.com/xenial/current/)

[Ubuntu Bionic (18.04)](https://cloud-images.ubuntu.com/bionic/current/)
