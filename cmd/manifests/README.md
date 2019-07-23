# Manifests

The `manifests` command generates the YAML manifests required to use CAPV. It has several benefits over the previous method:

* Links to Kustomize as a library, obviating the requirement for the `kustomize` binary
* Supports [_virtual patches_](#Virtual-patches) for Kustomize
* The `cluster`, `machines`, or `machineset` manifests are generated from the version of the API types with which the program is built, removing the risk of drift between templates and API versions
* Avoids general issues associated with shell scripts

## Getting started

Running the `manifests` command requires access to both the CAPI provider's and CAPI framework's config directories. While more options are available, these are the only requirements. For example:

```shell
./manifests -config-dir "$(pwd)/config/default" \
            -config-dir "$(pwd)/vendor/sigs.k8s.io/cluster-api/config/default"
```

```shell
$ ls -al
-rw-r--r--  1 akutz  staff    19K Jul 23 12:00 addons.yaml
-rw-r--r--  1 akutz  staff   484B Jul 23 12:00 cluster.yaml
-rw-r--r--  1 akutz  staff   640B Jul 23 12:00 machines.yaml
-rw-r--r--  1 akutz  staff   936B Jul 23 12:00 machineset.yam
-rw-r--r--  1 akutz  staff    99K Jul 23 12:00 provider-components.yaml
```

For a full list of the flags and options, use `./manifests -?`.

## Virtual patches

The `manifests` program introduces the concept of _virtual patches_ for Kustomize.

_A virtual patch is any file located in a kustomization root that ends with `.template`._

For example, CAPV's `kustomization.yaml` includes three patches:

```yaml
patches:
- manager_image_patch.yaml
- manager_log_level_patch.yaml
- rbac_role_binding_patch.yaml
```

However, the kustomization root does not include those files, instead, it includes those files ending with `.template`. The file `manager_image_patch.yaml.template` has the following contents:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: controller-manager
  namespace: system
spec:
  template:
    spec:
      containers:
      - image: "{{ .ManagerImage }}"
        name: manager
```

When `manifests` generates the provider components it scans all provided kustomization roots for any files that end with `.template` and interpolate them as Go templates. The data provided to the template engine is all of the CLI flags converted to camel-case. For example:

* `-kubernetes-version` becomes `KubernetesVersion`
* `-manager-image` becomes `ManagerImage`

Thus the `{{ .ManagerImage }}` in `manager_image_patch.yaml.template` is transformed into the value of the `-manager-image` flag.

The result of the transformation is stored in a hybrid, virtual filesystem as `path/to/manager_image_patch.yaml` given to the Kustomize engine. Therefore, when Kustomize looks for `path/to/manager_image_patch.yaml`, it will find the interpolated version created from the template and stored in-memory.

## Development

The `manifests` program is designed to be reusable by any CAPI provider.

1. Import `sigs.k8s.io/cluster-api-provider-vsphere/cmd/manifests/pkg/app`
2. Define a type that adheres to the following interface:

    ```golang
    type Provider interface {
        GetClusterProviderSpec() (runtime.Object, error)
        GetMachineProviderSpec() (runtime.Object, error)
    }
    ```

3. Call `app.Run(myProvider)`

That's it!

There's a basic example in [`./pkg/examples/basic-manifest-program/main.go`](./pkg/examples/basic-manifest-program/main.go) that illustrates how simple it is to produce a variation of the `manifests` program.
