# Multi VCenter support

Cluster API Provider vSphere (CAPV) supports multiple VCenter for a single. Therefore CAPV is allowing to define the used identity for each machine. CAPV will check on every Machine first, if there is a local identity otherwise it fallback on the default selection method. 

In order to run a CAPV cluster in multiple VCenter, you have to configure CPI & CSI to support multi VCenter, see [guide](https://docs.vmware.com/en/VMware-vSphere-Container-Storage-Plug-in/3.0/vmware-vsphere-csp-getting-started/GUID-8B3B9004-DE37-4E6B-9AA1-234CDA1BD7F9.html). Trivia, `VSphereCluster` can be only in single VCenter. This will just used as a fallback, if you haven't configured a different identity for a `VSphereMachine``.

## Examples

Deploy a `VSphereMachine` with a custom identityRef:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereMachine
metadata:
  name: new-workload-cluster
spec:
  server: vcenter
  identityRef:
    kind: VSphereClusterIdentity
    name: identityName
...
```

Deploy a `VSphereMachineTemplate` with a custom identityRef:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereMachineTemplate
metadata:
  name: new-workload-cluster
spec:
  template:
    spec:
       server: vcenter
      identityRef:
        kind: VSphereClusterIdentity
        name: identityName
...
```