# Identity Management

## Identity types

Cluster API Provider vSphere (CAPV) supports multiple methods to provide vCenter credentials and authorize workload clusters to use them. This guide will go through the different types and provide examples for each. The 3 ways to provide credentials:

* CAPV Manager bootstrap credentials: The vCenter username and password provided via `VSPHERE_USERNAME` `VSPHERE_PASSWORD` will be injected into the CAPV manager binary. These credentials will act as the fallback method should the other two credential methods not be utilized by a workload cluster.
* Credentials via a Secret: Credentials can be provided via a `Secret` that could then be referenced by a `VSphereCluster`. This will create a 1:1 relationship between the VSphereCluster and Secret and the secret cannot be utilized by other clusters.
* Credentials via a VSphereClusterIdentity: `VSphereClusterIdentity` is a cluster-scoped resource and enables multiple VSphereClusters to share the same set of credentials. The namespaces that are allowed to use the VSphereClusterIdentity can also be configured via a `LabelSelector`.

## Examples

### CAPV Manager Credentials

Setting `VSPHERE_USERNAME` and `VSPHERE_PASSWORD` before initializing the management cluster will ensure the credentials are injected into the manager's binary. More information can be found in the [Cluster API quick start guide](https://cluster-api.sigs.k8s.io/user/quick-start.html)

### Credentials via Secret

Deploy a `Secret` with the credentials in the VSphereCluster's namespace:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: secretName
  namespace: <Namespace of VSphereCluster>
stringData:
  username: <Username>
  password: <Password>
```

`Note: The secret must reside in the same namespace as the VSphereCluster`

Reference the Secret in the VSphereCluster Spec:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: VSphereCluster
metadata:
  name: new-workload-cluster
spec:
  identityRef:
    kind: Secret
    name: secretName
...
```

Once the VSphereCluster reconciles, it will set itself as the owner of the Secret and no other VSphereClusters will use the same secret. When a cluster is deleted, the secret will also be deleted.

### Credentials via VSphereClusterIdentity

Deploy a `Secret` with the credentials in the CAPV manager namespace (capv-system by default):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: secretName
  namespace: capv-system
stringData:
  username: <Username>
  password: <Password>
```

Deploy a `VSphereClusterIdentity` that references the secret. The `allowedNamespaces` LabelSelector can also be used to dictate which namespaces are allowed to use the identity. Setting `allowedNamespaces` to nil will block all namespaces from using the identity, while setting it to an empty selector will allow all namespaces to use the identity. The following example uses an empty selector.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: VSphereClusterIdentity
metadata:
  name: identityName
spec:
  secretName: secretName
  allowedNamespaces:
    selector:
      matchLabels: {}
```

Once the VSphereClusterIdentity reconciles, it will set itself as the owner of the Secret and the Secret cannot be used by other identities or VSphereClusters. The Secret will also be deleted if the VSphereClusterIdentity is deleted.

Reference the VSphereClusterIdentity in the VSphereCluster.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha3
kind: VSphereCluster
metadata:
  name: new-workload-cluster
spec:
  identityRef:
    kind: VSphereClusterIdentity
    name: identityName
...
```

`Note: VSphereClusterIdentity cannot be used in conjunction with the WatchNamespace set for the CAPV manager`
