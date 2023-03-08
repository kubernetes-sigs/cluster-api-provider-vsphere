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
- Choice of Linux distribution between Ubuntu 18.04 and CentOS 7 using VM Templates based on [OVA images](#Kubernetes-versions-with-published-OVAs).
- Deploys Kubernetes control planes into provided clusters on vSphere.
- Doesn't use SSH for bootstrapping nodes.
- Installs only the minimal components to bootstrap a control plane and workers.

------

## Compatibility with Cluster API and Kubernetes Versions

This provider's versions are compatible with the following versions of Cluster API:

|                                  | Cluster API v1alpha3 (v0.7) | Cluster API v1alpha4 (v0.8) | Cluster API v1beta1 (v1.0) | Cluster API v1beta1 (v1.3) |
|----------------------------------|:---------------------------:|:---------------------------:|:--------------------------:|:--------------------------:|
| CAPV v1alpha3 (v0.7)(deprecated) |              ✓              |              ✓              |             ✓              |             ✓              |
| CAPV v1alpha4 (v0.8)(deprecated)             |              ☓              |              ✓              |             ✓              |             ✓              |
| CAPV v1beta1 (v1.0)              |              ☓              |              ☓              |             ✓              |             ✓              |
| CAPV v1beta1 (v1.1)              |              ☓              |              ☓              |             ☓              |             ✓              |
| CAPV v1beta1 (v1.2)              |              ☓              |              ☓              |             ☓              |             ✓              |
| CAPV v1beta1 (v1.3, master)      |              ☓              |              ☓              |             ☓              |             ✓              |

|                              | Kubernetes 1.20 | Kubernetes 1.21 | Kubernetes 1.22 |
|------------------------------|:---------------:|:--------------:|:---------------:|
| CAPV v1alpha4 (v0.8)         |        ✓        |       ✓        |        ✓        |
| CAPV v1beta1 (v1.0)          |        ✓        |       ✓        |        ✓        |
| CAPV v1alpha2 (v1.3, master) |        ✓        |       ✓        |        ✓        |

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future modifications to this policy may be made to more closely align with other providers in the Cluster API ecosystem.

## Kubernetes versions with published OVAs

Note: These OVAs are not updated for security fixes and it is recommended to always use the latest patch version for the Kubernetes version you wish to run. For production-like environments, it is highly recommended to build and use your own custom images.

| Kubernetes | Ubuntu 18.04 | Ubuntu 20.04 | Photon 3 | Flatcar Stable |
| :--------: | :----------: | :----------: | :------: | :------------: |
|  v1.23.16  |   [ova](https://storage.googleapis.com/capv-templates/v1.23.16/ubuntu-1804-kube-v1.23.16.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.23.16/ubuntu-1804-kube-v1.23.16.ova.sha256)   |   [ova](https://storage.googleapis.com/capv-templates/v1.23.16/ubuntu-2004-kube-v1.23.16.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.23.16/ubuntu-2004-kube-v1.23.16.ova.sha256)|   [ova](https://storage.googleapis.com/capv-templates/v1.23.16/photon-3-kube-v1.23.16.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.23.16/photon-3-kube-v1.23.16.ova.sha256)   |   n/a   |
|  v1.24.10  |   [ova](https://storage.googleapis.com/capv-templates/v1.24.10/ubuntu-1804-kube-v1.24.10.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.24.10/ubuntu-1804-kube-v1.24.10.ova.sha256)   |   [ova](https://storage.googleapis.com/capv-templates/v1.24.10/ubuntu-2004-kube-v1.24.10.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.24.10/ubuntu-2004-kube-v1.24.10.ova.sha256)|   [ova](https://storage.googleapis.com/capv-templates/v1.24.10/photon-3-kube-v1.24.10.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.24.10/photon-3-kube-v1.24.10.ova.sha256)   |   n/a   |
|  v1.25.6   |   [ova](https://storage.googleapis.com/capv-templates/v1.25.6/ubuntu-1804-kube-v1.25.6.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.25.6/ubuntu-1804-kube-v1.25.6.ova.sha256)   |   [ova](https://storage.googleapis.com/capv-templates/v1.25.6/ubuntu-2004-kube-v1.25.6.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.25.6/ubuntu-2004-kube-v1.25.6.ova.sha256)|   [ova](https://storage.googleapis.com/capv-templates/v1.25.6/photon-3-kube-v1.25.6.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.25.6/photon-3-kube-v1.25.6.ova.sha256)   |   [ova](https://storage.googleapis.com/capv-templates/v1.25.6/flatcar-stable-3374.2.4-kube-v1.25.6.ova), [sha256](https://storage.googleapis.com/capv-templates/v1.25.6/flatcar-stable-3374.2.4-kube-v1.25.6.ova.sha256)   |

A full list of the published machine images for CAPV may be obtained with the following command:

```shell
gsutil ls gs://capv-templates/*
```

Or, to produce a list of URLs for the same image files (and their checksums), the following command may be used:

```shell
gsutil ls gs://capv-templates/*/*.{ova,sha256} | sed 's~^gs://~https://storage.googleapis.com/~'
```

## HAProxy published OVAs

Note: These OVAs are not updated for security fixes and it is recommended to always use the latest patch version for the version you wish to run. For production-like environments, it is highly recommended to build and use your own custom images.

| HAProxy Dataplane API | Photon 3 |
|:--------------------: | :------: |
|  v1.2.4  |  [ova](https://storage.googleapis.com/capv-images/extra/haproxy/release/v1.2.4/photon-3-haproxy-v1.2.4.ova), [sha256](https://storage.googleapis.com/capv-images/extra/haproxy/release/v1.2.4/photon-3-haproxy-v1.2.4.ova.sha256)  |

A full list of the published HAProxy images for CAPV may be obtained with the following command:

```shell
gsutil ls gs://capv-images/extra/haproxy/release/*
```

Or, to produce a list of URLs for the same image files (and their checksums), the following command may be used:

```shell
gsutil ls gs://capv-images/extra/haproxy/release/*/*.{ova,sha256} | sed 's~^gs://~https://storage.googleapis.com/~'
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

- Bi-weekly on [Zoom][zoom_meeting] on Thursdays @ 10:00am Pacific. [Convert to your time zone.][time_zone_converter]
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
[zoom_meeting]: https://zoom.us/j/92253194848?pwd=cVVVNDMxeTl1QVJPUlpvLzNSVU1JZz09
[time_zone_converter]: http://www.thetimezoneconverter.com/?t=08:00&tz=PT%20%28Pacific%20Time%29

<!-- markdownlint-disable-file MD033 -->
