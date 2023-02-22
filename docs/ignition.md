# Deploying Workload Clusters With Ignition-Based Distros

This guide explains how to deploy a workload cluster using an [Ignition][1]-enabled Linux distro.
The guide uses [Flatcar Container Linux][2] for demonstration purposes, however other distros which
support Ignition could likely be used as well.

## Requirements

- A vSphere environment
- A Flatcar OVA template (see blow)
- Docker
- [Kind][3] version `0.14.0` or higher
- kubectl
- [clusterctl][4]

## Deployment

Ensure your vSphere environment has a Flatcar OVA template either by [building an image][5] using
image-builder (recommended) or by [importing a prebuilt image][6] using a published [OVA URL][7].
Ensure the built/imported image is marked as a template. Use the template name as the value of the
`VSPHERE_TEMPLATE` variable below.

Set required environment variables:

```shell
export VSPHERE_SERVER="my-vcenter.example.com"
export VSPHERE_USERNAME="foo"
export VSPHERE_PASSWORD="bar"
export VSPHERE_DATACENTER="DC"
export VSPHERE_FOLDER="my-stuff"
export VSPHERE_RESOURCE_POOL="my-pool"
export VSPHERE_DATASTORE="Datastore"
export VSPHERE_NETWORK="VM Network"
export VSPHERE_TEMPLATE="flatcar-stable-3139.2.3-kube-v1.23.5"
export VSPHERE_SSH_AUTHORIZED_KEY="ssh-rsa AAAAB3N..."
export VSPHERE_TLS_THUMBPRINT="D5:D0:E2:72:00:AF:46:74:B7:9C:30:58:85:B8:6C:34:AA:BF:45:D2"
export CONTROL_PLANE_ENDPOINT_IP="10.1.2.3"
export VSPHERE_STORAGE_POLICY=""
```

Create a management cluster:

```shell
# Create a Kind cluster
kind create cluster

# Enabled required feature flags
export EXP_CLUSTER_RESOURCE_SET=true
export EXP_KUBEADM_BOOTSTRAP_FORMAT_IGNITION=true

# Initialize Cluster API
clusterctl init -i vsphere
```

Wait for all pods to converge:

```shell
kubectl get pods -A
```

Sample output:

```shell
NAMESPACE                           NAME                                                             READY   STATUS    RESTARTS   AGE
capi-kubeadm-bootstrap-system       capi-kubeadm-bootstrap-controller-manager-8668dd96c9-kjdt6       1/1     Running   0          4m22s
capi-kubeadm-control-plane-system   capi-kubeadm-control-plane-controller-manager-64f549b6d4-dzqww   1/1     Running   0          4m21s
capi-system                         capi-controller-manager-5d95c8dbc-fhqdm                          1/1     Running   0          4m22s
capv-system                         capv-controller-manager-76d7dd6c9-vdrt4                          1/1     Running   0          4m21s
cert-manager                        cert-manager-6dd9658548-7hvmx                                    1/1     Running   0          5m13s
cert-manager                        cert-manager-cainjector-5987875fc7-jw4fv                         1/1     Running   0          5m13s
cert-manager                        cert-manager-webhook-7b4c5f579b-xrtjz                            1/1     Running   0          5m13s
kube-system                         coredns-6d4b75cb6d-8tq8s                                         1/1     Running   0          5m57s
kube-system                         coredns-6d4b75cb6d-9fdk5                                         1/1     Running   0          5m57s
kube-system                         etcd-kind-control-plane                                          1/1     Running   0          6m13s
kube-system                         kindnet-hng9v                                                    1/1     Running   0          5m58s
kube-system                         kube-apiserver-kind-control-plane                                1/1     Running   0          6m13s
kube-system                         kube-controller-manager-kind-control-plane                       1/1     Running   0          6m15s
kube-system                         kube-proxy-gvdbj                                                 1/1     Running   0          5m58s
kube-system                         kube-scheduler-kind-control-plane                                1/1     Running   0          6m13s
local-path-storage                  local-path-provisioner-9cd9bd544-v6lm5                           1/1     Running   0          5m57s
```

Deploy a workload cluster:

```shell
# Generate a workload cluster manifest
clusterctl generate cluster my-cluster -f ignition --kubernetes-version v1.23.5 --worker-machine-count 1 > cluster.yaml

# Create the workload cluster
kubectl apply -f cluster.yaml
```

Wait for all machines to enter the `provisioned` state:

```shell
kubectl get machine
```

Sample output:

```shell
NAME                               CLUSTER      NODENAME   PROVIDERID   PHASE          AGE   VERSION
my-cluster-m6q4g                   my-cluster                           Provisioning   5s    v1.23.5
my-cluster-md-0-677cbb5f87-vsmrp   my-cluster                           Pending        8s    v1.23.5
```

Get the kubeconfig for the workload cluster:

```shell
clusterctl get kubeconfig my-cluster > kubeconfig
```

In a new shell, use the kubeconfig to communicate with the workload cluster:

```shell
export KUBECONFIG=$(pwd)/kubeconfig
kubectl get nodes
```

Sample output:

```shell
NAME                               STATUS     ROLES                  AGE     VERSION
my-cluster-7nkns                   NotReady   control-plane,master   3m31s   v1.23.5
my-cluster-md-0-677cbb5f87-mhhst   NotReady   <none>                 37s     v1.23.5
```

Deploy a CNI to the workload cluster (we use Calico in this guide):

```shell
kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.24.1/manifests/calico.yaml
```

Ensure all pods are up:

```shell
kubectl get pods -A
```

Sample output:

```shell
NAMESPACE     NAME                                       READY   STATUS             RESTARTS      AGE
kube-system   calico-kube-controllers-66966888c4-6r5rx   1/1     Running            0             2m41s
kube-system   calico-node-fhg9r                          1/1     Running            0             2m43s
kube-system   calico-node-sgjtc                          1/1     Running            0             2m43s
kube-system   coredns-64897985d-4w8b9                    1/1     Running            0             7m56s
kube-system   coredns-64897985d-lzlhc                    1/1     Running            0             7m56s
kube-system   etcd-my-cluster-7nkns                      1/1     Running            0             8m4s
kube-system   kube-apiserver-my-cluster-7nkns            1/1     Running            0             8m5s
kube-system   kube-controller-manager-my-cluster-7nkns   1/1     Running            0             8m5s
kube-system   kube-proxy-6lvkt                           1/1     Running            0             5m17s
kube-system   kube-proxy-sxp96                           1/1     Running            0             7m56s
kube-system   kube-scheduler-my-cluster-7nkns            1/1     Running            0             8m5s
kube-system   kube-vip-my-cluster-7nkns                  1/1     Running            0             8m4s
kube-system   vsphere-cloud-controller-manager-6snb2     1/1     Running            0             5m17s
kube-system   vsphere-cloud-controller-manager-f5kjv     1/1     Running            0             7m53s
kube-system   vsphere-csi-controller-bc4676cd9-8zpwd     5/5     Running            0             7m56s
kube-system   vsphere-csi-node-4xl6w                     3/3     Running            0             5m17s
kube-system   vsphere-csi-node-q7x8q                     3/3     Running            0             7m56s
```

## Cleanup

Delete the workload cluster by running the following on the *management* cluster:

```shell
kubectl delete cluster my-cluster
```

Wait for the machines to get terminated on vSphere, then delete the management cluster:

```shell
kind delete cluster
```

[1]: https://www.flatcar.org/docs/latest/provisioning/ignition/
[2]: https://www.flatcar.org/
[3]: https://kind.sigs.k8s.io/
[4]: https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl
[5]: https://image-builder.sigs.k8s.io/capi/providers/vsphere.html
[6]: https://docs.vmware.com/en/VMware-vSphere/7.0/com.vmware.vsphere.vm_admin.doc/GUID-17BEDA21-43F6-41F4-8FB2-E01D275FE9B4.html
[7]: https://storage.googleapis.com/capv-templates/v1.25.6/flatcar-stable-3374.2.4-kube-v1.25.6.ova
