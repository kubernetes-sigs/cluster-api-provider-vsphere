# Staging and nightly images

CAPV publishes controller images and provider manifests from CI so developers can try the latest merged code without building locally.

These artifacts are **staging / nightly builds**. They are intended for development and test clusters only, not for production.

## What is published

**Post-submit images** are built on every merge to `main` and to `release-X.Y` branches. Each post-submit also publishes matching provider manifests.

**Nightly images** are an additional dated retag of the latest `main` post-submit. They exist so a specific day's build can be pinned while still tracking unreleased `main`.

Do not use these images in production. Use [released CAPV versions](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/releases) instead.

## Image registry and tags

Images are published to the staging registry `gcr.io/k8s-staging-capi-vsphere`.

The CAPV controller image is:

```text
gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-controller:<tag>
```

Typical tags:

| Tag | Meaning |
| --- | --- |
| `main` | Latest merge to the `main` branch |
| `release-X.Y` | Latest merge to a release branch, for example `release-1.16` |
| git-describe / `vYYYYMMDD-hash` | Immutable post-submit tag for a specific commit |
| `nightly_main_YYYYMMDD` | Nightly retag of `main` for that calendar day |

Prefer the `main`, `release-X.Y`, or `nightly_main_YYYYMMDD` tags together with the matching GCS manifest. The git-describe tag is useful when you need an exact post-submit image.

## Manifest locations

Post-submit and nightly jobs also upload `infrastructure-components.yaml` (and related release files) to GCS bucket `k8s-staging-capi-vsphere`:

```text
https://storage.googleapis.com/k8s-staging-capi-vsphere/components/<tag>/infrastructure-components.yaml
```

Examples:

- Latest `main`: <https://storage.googleapis.com/k8s-staging-capi-vsphere/components/main/infrastructure-components.yaml>
- A nightly: `https://storage.googleapis.com/k8s-staging-capi-vsphere/components/nightly_main_YYYYMMDD/infrastructure-components.yaml`

Staging objects are retained only for a limited time (about 60 days at the time of writing). Older prefixes may 404.

## List recent nightly prefixes

To list recent nightly objects, query the public GCS JSON API (requires `curl` and `jq`):

```shell
curl -sL -H 'Accept: application/json' \
  "https://storage.googleapis.com/storage/v1/b/k8s-staging-capi-vsphere/o" \
  | jq -r '.items | map(select(.name | startswith("components/nightly_main"))) | .[] | [.timeCreated,.mediaLink] | @tsv'
```

The output looks like:

```text
2024-05-03T08:03:09.087Z        https://storage.googleapis.com/download/storage/v1/b/k8s-staging-capi-vsphere/o/components%2Fnightly_main_20240503%2Finfrastructure-components.yaml?...
```

Use the `mediaLink` for the `infrastructure-components.yaml` object you want, or reconstruct the public URL from the nightly prefix:

```text
https://storage.googleapis.com/k8s-staging-capi-vsphere/components/nightly_main_YYYYMMDD/infrastructure-components.yaml
```

## Apply a staging or nightly manifest

The published `infrastructure-components.yaml` already pins the matching controller image. Download it and apply it to a **test** management cluster with `kubectl`:

```shell
curl -L -o infrastructure-components.yaml \
  https://storage.googleapis.com/k8s-staging-capi-vsphere/components/main/infrastructure-components.yaml

kubectl apply -f infrastructure-components.yaml
```

Replace `main` with `release-X.Y` or `nightly_main_YYYYMMDD` as needed.

This only installs or updates the CAPV provider components. The management cluster still needs a compatible Cluster API core (and bootstrap / control-plane providers) as described in the [getting started guide](getting_started.md).

## clusterctl

`clusterctl` can consume local or override provider artifacts during development. See the Cluster API book:

<https://cluster-api.sigs.k8s.io/clusterctl/developers.html>

That page also documents nightly builds for **core Cluster API**. It does not currently provide a CAPV-specific nightly provider entry. For CAPV staging or nightly bits, download `infrastructure-components.yaml` from GCS and apply it with `kubectl` as above, or place the file in a [clusterctl override](development.md#generating-clusterctl-overrides).
