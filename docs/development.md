# Development

## Using Tilt for rapid iteration

Follow the instructions to use [Tilt with Cluster API](https://cluster-api.sigs.k8s.io/developer/tilt.html)

For example, given a directory layout of

``` shell
~/capi-dev
  +
  |
  +-+ cluster-api
  |
  +-+ cluster-api-provider-vsphere
```

your `~/capi-dev/cluster-api/tilt-settings.json` should include `VSPHERE_USERNAME`
and `VSPHERE_PASSWORD` in `kustomize_substitutions`:

``` json
  "allowed_contexts": [
    "kind-kind"
  ],
  "default_registry": "gcr.io/my-project-123",
  "provider_repos": [
    "../cluster-api-provider-vsphere"
  ],
  "enable_providers": [
    "vsphere",
    "kubeadm-bootstrap",
    "kubeadm-control-plane"
  ],
  "kustomize_substitutions": {
    "VSPHERE_USERNAME": "administrator@vsphere.local",
    "VSPHERE_PASSWORD": "admin!23",
  }
```

## Developing in conjunction with clusterctl

If you want to use clusterctl with development work, or when working from master,
you will need the following:

* jq
* kind
* Docker
* An OCI registry or Google Cloud project with GCR enabled

### Configuration

If you are using Google Cloud, and have installed gcloud and set up Docker
authentication, there is no more for you to do.

Otherwise, set the `DEV_REGISTRY` environment variable to the OCI repository
you want test images uploaded to.

The following environment variables are supported for configuration:

```shell
export VERSION ?= $(shell cat clusterctl-settings.json | jq .config.nextVersion -r)
export IMAGE_NAME ?= manager
export PULL_POLICY ?= Always
export DEV_REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
export DEV_CONTROLLER_IMG ?= $(DEV_REGISTRY)/vsphere-$(IMAGE_NAME)
export DEV_TAG ?= dev
```

### Building images

The following make targets build and push a test image to your repository:

``` shell
make docker-build docker-push
```

### Generating clusterctl overrides

Overrides need to be placed in `~/.cluster-api/overrides/infrastructure-vsphere/{version}`

Running the following generates these overrides for you:

``` shell
make dev-manifests
```

For development purposes, if you are using a `major.minor.x` version (see env variable VERSION) which is not present in the `metadata.yml`, include this version entry along with the API contract in the file.

### Using dev manifests

After publishing your test image and generating the manifests, you can use
`clusterctl`, as per the getting-started instructions.

## Testing e2e

See the [e2e docs](../test/e2e/README.md)
