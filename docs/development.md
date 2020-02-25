# Development

## Using Tilt

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
