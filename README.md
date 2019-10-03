# Kubernetes Cluster API Provider vSphere

[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/cluster-api-provider-vsphere)](https://goreportcard.com/report/github.com/kubernetes-sigs/cluster-api-provider-vsphere)

<img src="https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png" width="100" height="100" /><a href="https://www.vmware.com/products/vsphere.html"><img height="100" hspace="90px" src="https://i.imgur.com/Wd24COX.png" alt="Powered by VMware vSphere" /></a>

Kubernetes-native declarative infrastructure for vSphere.

## What is the Cluster API Provider vSphere

The [Cluster API][cluster_api] brings declarative, Kubernetes-style APIs to cluster creation, configuration and management. Cluster API Provider for vSphere is a concrete implementation of Cluster API for vSphere.

The API itself is shared across multiple cloud providers allowing for true vSphere hybrid deployments of Kubernetes. It is built atop the lessons learned from previous cluster managers such as [kops][kops] and [kubicorn][kubicorn].

## Launching a Kubernetes cluster on vSphere

Check out the [getting started guide](./docs/getting_started.md) for launching a cluster on vSphere.

## Features

- Native Kubernetes manifests and API
- Manages the bootstrapping of VMs on cluster.
- Choice of Linux distribution between Ubuntu 18.04 and CentOS 7 using VM Templates based on [OVA images](docs/machine_images.md).
- Deploys Kubernetes control planes into provided clusters on vSphere.
- Doesn't use SSH for bootstrapping nodes.
- Installs only the minimal components to bootstrap a control plane and workers.

------

## Compatibility with Cluster API and Kubernetes Versions

This provider's versions are compatible with the following versions of Cluster API:

||Cluster API v1alpha1 (v0.1)|Cluster API v1alpha2 (v0.2)|
|---|:---:|:---:|
| CAPV v1alpha1 (v0.3)|✓|☓|
| CAPV v1alpha1 (v0.4)|✓|☓|
| CAPV v1alpha2 (v0.5, master)|☓|✓|

||Kubernetes 1.13|Kubernetes 1.14|Kubernetes 1.15|
|-|:---:|:---:|:---:|
| CAPV v1alpha1 (v0.3)|✓|✓|✓|
| CAPV v1alpha1 (v0.4)|✓|✓|✓|
| CAPV v1alpha2 (v0.5, master)|✓|✓|✓|

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future modifications to this policy may be made to more closely align with other providers in the Cluster API ecosystem.

## Kubernetes versions with published OVAs

| Kubernetes | CentOS 7 | Ubuntu 18.04 |
|:-:|:-:|:-:|
| v1.13.6 | [ova](http://storage.googleapis.com/capv-images/release/v1.13.6/centos-7-kube-v1.13.6.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.13.6/centos-7-kube-v1.13.6.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.13.6/ubuntu-1804-kube-v1.13.6.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.13.6/ubuntu-1804-kube-v1.13.6.ova.sha256) |
| v1.13.7 | [ova](http://storage.googleapis.com/capv-images/release/v1.13.7/centos-7-kube-v1.13.7.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.13.7/centos-7-kube-v1.13.7.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.13.7/ubuntu-1804-kube-v1.13.7.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.13.7/ubuntu-1804-kube-v1.13.7.ova.sha256) |
| v1.13.8 | [ova](http://storage.googleapis.com/capv-images/release/v1.13.8/centos-7-kube-v1.13.8.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.13.8/centos-7-kube-v1.13.8.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.13.8/ubuntu-1804-kube-v1.13.8.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.13.8/ubuntu-1804-kube-v1.13.8.ova.sha256) |
| v1.13.9 | [ova](http://storage.googleapis.com/capv-images/release/v1.13.9/centos-7-kube-v1.13.9.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.13.9/centos-7-kube-v1.13.9.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.13.9/ubuntu-1804-kube-v1.13.9.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.13.9/ubuntu-1804-kube-v1.13.9.ova.sha256) |
| v1.14.0 | [ova](http://storage.googleapis.com/capv-images/release/v1.14.0/centos-7-kube-v1.14.0.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.0/centos-7-kube-v1.14.0.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.14.0/ubuntu-1804-kube-v1.14.0.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.0/ubuntu-1804-kube-v1.14.0.ova.sha256) |
| v1.14.1 | [ova](http://storage.googleapis.com/capv-images/release/v1.14.1/centos-7-kube-v1.14.1.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.1/centos-7-kube-v1.14.1.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.14.1/ubuntu-1804-kube-v1.14.1.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.1/ubuntu-1804-kube-v1.14.1.ova.sha256) |
| v1.14.2 | [ova](http://storage.googleapis.com/capv-images/release/v1.14.2/centos-7-kube-v1.14.2.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.2/centos-7-kube-v1.14.2.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.14.2/ubuntu-1804-kube-v1.14.2.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.2/ubuntu-1804-kube-v1.14.2.ova.sha256) |
| v1.14.3 | [ova](http://storage.googleapis.com/capv-images/release/v1.14.3/centos-7-kube-v1.14.3.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.3/centos-7-kube-v1.14.3.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.14.3/ubuntu-1804-kube-v1.14.3.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.3/ubuntu-1804-kube-v1.14.3.ova.sha256) |
| v1.14.4 | [ova](http://storage.googleapis.com/capv-images/release/v1.14.4/centos-7-kube-v1.14.4.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.4/centos-7-kube-v1.14.4.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.14.4/ubuntu-1804-kube-v1.14.4.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.4/ubuntu-1804-kube-v1.14.4.ova.sha256) |
| v1.14.5 | [ova](http://storage.googleapis.com/capv-images/release/v1.14.5/centos-7-kube-v1.14.5.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.5/centos-7-kube-v1.14.5.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.14.5/ubuntu-1804-kube-v1.14.5.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.5/ubuntu-1804-kube-v1.14.5.ova.sha256) |
| v1.14.6 | [ova](http://storage.googleapis.com/capv-images/release/v1.14.6/centos-7-kube-v1.14.6.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.6/centos-7-kube-v1.14.6.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.14.6/ubuntu-1804-kube-v1.14.6.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.14.6/ubuntu-1804-kube-v1.14.6.ova.sha256) |
| v1.15.0 | [ova](http://storage.googleapis.com/capv-images/release/v1.15.0/centos-7-kube-v1.15.0.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.15.0/centos-7-kube-v1.15.0.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.15.0/ubuntu-1804-kube-v1.15.0.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.15.0/ubuntu-1804-kube-v1.15.0.ova.sha256) |
| v1.15.1 | [ova](http://storage.googleapis.com/capv-images/release/v1.15.1/centos-7-kube-v1.15.1.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.15.1/centos-7-kube-v1.15.1.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.15.1/ubuntu-1804-kube-v1.15.1.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.15.1/ubuntu-1804-kube-v1.15.1.ova.sha256) |
| v1.15.2 | [ova](http://storage.googleapis.com/capv-images/release/v1.15.2/centos-7-kube-v1.15.2.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.15.2/centos-7-kube-v1.15.2.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.15.2/ubuntu-1804-kube-v1.15.2.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.15.2/ubuntu-1804-kube-v1.15.2.ova.sha256) |
| v1.15.3 | [ova](http://storage.googleapis.com/capv-images/release/v1.15.3/centos-7-kube-v1.15.3.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.15.3/centos-7-kube-v1.15.3.ova.sha256) | [ova](http://storage.googleapis.com/capv-images/release/v1.15.3/ubuntu-1804-kube-v1.15.3.ova), [sha256](http://storage.googleapis.com/capv-images/release/v1.15.3/ubuntu-1804-kube-v1.15.3.ova.sha256) |

A full list of the published machine images for CAPV may be obtained with the following command:

```shell
gsutil ls gs://capv-images/release/*
```

Or, to produce a list of URLs for the same image files (and their checksums), the following command may be used:

```shell
gsutil ls gs://capv-images/release/*/*.{ova,sha256} | sed 's~^gs://~http://storage.googleapis.com/~'
```

## Documentation

Further documentation is available in the `/docs` directory.

## Getting involved and contributing

Are you interested in contributing to cluster-api-provider-vsphere? We, the maintainers and community, would love your suggestions, contributions, and help! Also, the maintainers can be contacted at any time to learn more about how to get involved.

In the interest of getting more new people involved we tag issues with [`good first issue`][good_first_issue]. These are typically issues that have smaller scope but are good ways to start to get acquainted with the codebase.

We also encourage ALL active community participants to act as if they are maintainers, even if you don't have "official" write permissions. This is a community effort, we are here to serve the Kubernetes community. If you have an active interest and you want to get involved, you have real power! Don't assume that the only people who can get things done around here are the "maintainers".

We also would love to add more "official" maintainers, so show us what you can do!

This repository uses the Kubernetes bots.  See a full list of the commands [here][prow].

## Code of conduct

Participating in the project is governed by the Kubernetes code of conduct. Please take some time to read the [code of conduct document][code_of_conduct].

### Implementer office hours

- Bi-weekly on Mondays @ 1:00 PT on [Zoom][zoom_meeting] starting on 8/13
- Previous meetings: \[ [notes][meeting_notes] \]

### Other ways to communicate with the contributors

Please check in with us in the [#cluster-api-vsphere][slack] channel on Slack or email us at our [mailing list][mailing_list]

## Github issues

### Bugs

If you think you have found a bug please follow the instructions below.

- Please spend a small amount of time giving due diligence to the issue tracker. Your issue might be a duplicate.
- Get the logs from the cluster controllers. Please paste this into your issue.
- Follow the helpful tips provided in the [troubleshooting document][troubleshooting] as needed.
- Open a [new issue][new_issue].
- Remember that users might be searching for your issue in the future, so please give it a meaningful title to help others.
- Feel free to reach out to the cluster-api community on the [kubernetes slack][slack_info].

### Tracking new features

We also use the issue tracker to track features. If you have an idea for a feature, or think you can help CAPV become even more awesome follow the steps below.

- Open a [new issue][new_issue].
- Remember that users might be searching for your issue in the future, so please give it a meaningful title to help others.
- Clearly define the use case, using concrete examples. EG: I type `this` and cluster-api-provider-vsphere does `that`.
- Some of our larger features will require some design. If you would like to include a technical design for your feature please include it in the issue.
- After the new feature is well understood, and the design agreed upon, we can start coding the feature. We would love for you to code it. So please open up a **WIP** *(work in progress)* pull request, and happy coding.

<!-- References -->
[cluster_api]: https://github.com/kubernetes-sigs/cluster-api
[code_of_conduct]: https://git.k8s.io/community/code-of-conduct.md
[good_first_issue]: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22
[kops]: https://github.com/kubernetes/kops
[kubicorn]: http://kubicorn.io/
[mailint_list]: https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle
[meeting_notes]: https://docs.google.com/document/d/1jQrQiOW75uWraPk4b_LWtCTHwT7EZwrWWwMdxeWOEvk/edit?usp=sharing
[new_issue]: https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues/new
[prow]: https://go.k8s.io/bot-commands
[slack]: https://kubernetes.slack.com/messages/CKFGK3SSD
[slack_info]: https://github.com/kubernetes/community/tree/master/communication#communication
[troubleshooting]: ./docs/troubleshooting.md
[zoom_meeting]: https://zoom.us/j/875399243
