# CSI

The manifest at `vsphere-csi-driver.yaml` is synced from `https://raw.githubusercontent.com/kubernetes-sigs/vsphere-csi-driver/${VERSION}/manifests/vanilla/vsphere-csi-driver.yaml`.

`kustomization.yaml` patches the synced manifest:

- `replicas: 1` for the `vsphere-csi-controller` Deployment.
- `get`, `list` and `watch` on `volumeattributesclasses` for the `vsphere-csi-controller-role` ClusterRole. csi-resizer v2.0.0 and later needs it on Kubernetes 1.34 and later, and the upstream manifest does not grant it. Remove the patch once upstream adds the rule.
