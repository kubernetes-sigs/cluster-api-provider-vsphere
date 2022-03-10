# Flavors
---
In `clusterctl` the infrastructure provider authors can provide different type of cluster templates, or flavors; use the --flavor flag to specify which flavor to use; e.g

```sh
clusterctl config cluster my-cluster --kubernetes-version v1.22.1 \
    --flavor external-cloud-provider > my-cluster.yaml
```

See [`clustercl` flavors docs](https://cluster-api.sigs.k8s.io/clusterctl/commands/generate-cluster.html#flavors)

# Running flavor clusters as a tilt resource
---

### From Tilt Config

Tilt will auto-detect all available flavors from the `templates` directory.

### Requirements
Please note your tilt-settings.json must contain at minimum the following fields when using tilt resources to deploy cluster flavors:

