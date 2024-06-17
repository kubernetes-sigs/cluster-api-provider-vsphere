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

If you want to use clusterctl with development work, or when working from main,
you will need the following:

* jq
* kind
* Docker
* An OCI registry or Google Cloud project with GCR enabled

### Configuration

If you are using Google Cloud, and have installed gcloud and set up Docker
authentication, there is no more for you to do.

Otherwise, set the `REGISTRY` environment variable to the OCI repository
you want test images uploaded to.

The following environment variables are supported for configuration:

```shell
export VERSION ?= $(shell cat clusterctl-settings.json | jq .config.nextVersion -r)
export PULL_POLICY ?= Always
export IMAGE_NAME ?= cluster-api-vsphere-controller
export REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
export CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
export TAG ?= dev
export ARCH ?= $(shell go env GOARCH)
```

### Building images

The following make targets build and push a test image to your repository:

``` shell
make docker-build DOCKER_BUILD_MODIFY_MANIFESTS=false
make docker-push-all ALL_ARCH=$(go env GOARCH)
```

### Generating clusterctl overrides

Overrides need to be placed in `~/.cluster-api/overrides/infrastructure-vsphere/{version}`

Running the following re-generates the manifests for you and copies them over:

``` shell
mkdir -p "~/.cluster-api/overrides/infrastructure-vsphere/$(cat clusterctl-settings.json | jq .config.nextVersion -r)"
make manifest-modification REGISTRY=$(REGISTRY) RELEASE_TAG=$(TAG) PULL_POLICY=Always
make release-manifests STAGE=dev MANIFEST_DIR="~/.cluster-api/overrides/infrastructure-vsphere/$(cat clusterctl-settings.json | jq .config.nextVersion -r)"
```

For development purposes, if you are using a `major.minor.x` version (see env variable VERSION) which is not present in the `metadata.yml`, include this version entry along with the API contract in the file.

### Using dev manifests

After publishing your test image and generating the manifests, you can use
`clusterctl`, as per the getting-started instructions.

#### Using custom cluster-templates  

In order to generate a custom `custom-template.yaml`, run `make generate-flavors`.  
This command will generate all flavors in the `templates` directory.
To create this custom cluster, use `clusterctl generate cluster --from="<cluster_template_path>" <cluster_name>`  

## Testing e2e

See the [e2e docs](../test/e2e/README.md)
