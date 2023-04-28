# Deploy Workload Clusters Using CAPV Node IPAM

These instructions detail how to use clusterctl, CAPI, CAPV, and
InClusterIPAMProvider on a new management cluster, and then deploy a workload
cluster using Node IPAM features.

Recent versions of CAPV include enhancements whereby when a cluster is
configured to use Node IPAM, the new VM will be cloned and a claim for a new IP
address is created. CAPV will then wait for the claim to have an associated IP
address object to be created. A new component, an IPAM Provider, will watch for
claims that need IP addresses and provide one from a configured pool. Once CAPV
sees that its IP claim has been satisfied, VM creation resumes and the VM
metadata associated with the VM will contain the IP address configuration.

Note that at the time of writing, this feature set only works with the CAPV
infrastructure provider. Also, at the time of writing, the
InClusterIPAMProvider is the only implementation of the CAPI [Node IPAM
proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220125-ipam-integration.md).

As a result of the proposal, CAPI makes available two new resources that will
be used behind the scenes: `IPAddressClaim` and `IPAddress`. Additionally the
In Cluster IPAM Provider makes available a pool custom resource, the
'InClusterIPPool' that will be used to configure what IPs range should be used
to create the workload cluster.

## Requirements

- kubectl
- govc
- kind
- a vSphere network that contains a range of IPs that are not used by a DHCP
  server

## install the clusterctl CLI

```bash
curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.3.3/clusterctl-linux-amd64 -o clusterctl
sudo install -o root -g root -m 0755 clusterctl /usr/local/bin/clusterctl
clusterctl version
```

## obtain a management cluster

```bash
kind create cluster  # automatically sets the kubernetes context
```

## initialze kind cluster as a mangement cluster, with IPAM provider

```bash
export EXP_CLUSTER_RESOURCE_SET=true # feture req'd by this demo
export CLUSTER_TOPOLOGY=true  # enables cluster class features of CAPI
export VSPHERE_USERNAME="xxx" # replace with your creds
export VSPHERE_PASSWORD="yyy"
```

```bash
cat << EOF > clusterctl-config.yaml
---
providers:
  - name: incluster
    url: https://github.com/telekom/cluster-api-ipam-provider-in-cluster/releases/latest/ipam-components.yaml
    type: IPAMProvider
EOF

clusterctl init --infrastructure vsphere --ipam incluster --config clusterctl-config.yaml
```

This will deploy CAPI, CAPV and the In Cluster IPAM Provider to the
management cluster.

## Create an IP Pool

The pool is configured with the details of the network and the range of IPs
that shall be made available for node use.

The namespace of the pool must be in the same namespace as the cluster that
will later be deployed when using an `InClusterIPPool`. Clusters in that
namespace may share a given pool. Ensure the pool has enough IPs to account for
all of the nodes you intend to deploy. It is also important to make the range
slightly larger than the total number of nodes you intend to deploy. When
performing an upgrade a new node(s) is created, then the old node is deleted.
IPs are freed when the nodes are deleted. The number of extra IPs needed will
depend on how CAPI is configured.

Alternatively, a `GlobalInClusterIPPool` can be created, which is a cluster
scoped resource. Clusters in any namespace may make use of a pool of this type.
All other attributes of a `GlobalInClusterIPPool` are the same as an
`InClusterIPPool`.

The `spec.subnet` field describes the actual underlying vSphere network. This
must match the configuration of the underlying network.

The `spec.gateway` field describes the actual underlying vSphere network
gateway. This must match the configuration of the underlying network.

The `spec.start` and `spec.end` are optional fields that describe the range of
IPs that the pool should be restricted to. These fields must describe a subset
of the `spec.subnet`.

The `spec.addresses` field receives an array of IP address that a pool offers.
This allows for a pool to offer a non-contiguous set of IP. This fields must
describe a subset of the `spec.subnet`.

A pool may declare `spec.addresses` or it may declare `spec.start`/`spec.end`.
A pool may not specify both `spec.addresses` and `spec.start`/`spec.end`. A
pool without these fields will offer IPs from the entire subnet range.

**Important** - The InClusterIPAMProvider does not validate against overlapping
pools. If two pools are using the same underlying network and are configured to
offer IPs in the same range, then nodes will have duplicate IPs and bad things
will happen. The onus is on the operator to ensure that pools do not overlap.

Note: This doc does not go into configuring the actual, underlying network. The
featureset assumes control of the range specified. It is important that there
is not a DHCP server using the addresses in the start/end range.

Create and apply a pool:

```bash
kubectl create namespace cluster-ns

cat << EOF > pool.yaml
---
apiVersion: ipam.cluster.x-k8s.io/v1alpha1
kind: InClusterIPPool
metadata:
  name: example-pool
  namespace: cluster-ns
spec:
  subnet: 192.168.117.0/24
  gateway: 192.168.117.1
  start: 192.168.117.152
  end: 192.168.117.180
EOF

kubectl apply -f pool.yaml
```

## Setup Env Variables

Export the needed environment variables for your vSphere env.
These variables were copied from the CAPI quick start guide for vSphere.

Edit & export these example values to match your environment.

```bash
export VSPHERE_SERVER="10.0.0.1"
export VSPHERE_DATACENTER="the-datacenter"
export VSPHERE_DATASTORE="the-datastore"
export VSPHERE_NETWORK="VM Network"
export VSPHERE_RESOURCE_POOL="*/Resources"
export VSPHERE_FOLDER="vm"
export VSPHERE_TEMPLATE="ubuntu-2004-kube-v1.26.2"
export VSPHERE_SSH_AUTHORIZED_KEY="ssh-rsa AAAAB3N..."
export VSPHERE_TLS_THUMBPRINT="97:48:03:8D:78:A9..."
export VSPHERE_STORAGE_POLICY="policy-one"
export CPI_IMAGE_K8S_VERSION=v1.26.0
export NODE_IPAM_POOL_NAME=example-pool
export NODE_IPAM_POOL_API_GROUP=ipam.cluster.x-k8s.io
export NODE_IPAM_POOL_KIND=InClusterIPPool
export NAMESERVER="8.8.8.8"
export CONTROL_PLANE_ENDPOINT_IP="1.2.3.4"
```

## Upload OVA, Mark as template

The CAPV README.md file includes links to ovas.

```bash
govc import.ova https://storage.googleapis.com/capv-templates/v1.26.2/ubuntu-2004-kube-v1.26.2.ova
govc vm.markastemplate ubuntu-2004-kube-v1.26.2
```

## Generate a Workload Cluster Config

Generate a new cluster. Note the `--target-namespace` points
to the same namespace as the IP pool created in earlier steps.

```bash
clusterctl generate cluster ipam-example \
    --infrastructure vsphere \
    --flavor node-ipam \
    --target-namespace cluster-ns \
    --kubernetes-version v1.26.2 \
    --control-plane-machine-count 1 \
    --worker-machine-count 1 > cluster.yaml
```

## Create the Workload Cluster

Apply the cluster YAML to the management cluster. Obtain the cluster's
kubeconfig. Deploy a CNI once the cluster control plane becomes available.

```bash
kubectl apply -f cluster.yaml

clusterctl get kubeconfig ipam-example -n cluster-ns > cluster.kc

kubectl --kubeconfig=cluster.kc \
  apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/addons/calico.yaml
```

At this point, the new workload cluster should have nodes with IPs allocated
from the configured pool.

## Troubleshooting

Watch for new `IPAddressClaim` and `IPAddress` objects. The `VSphereVM` objects
created in the deploy process will recieve `status.condition`
`IPAddressClaimed` updates, describing the state of IPAddress reconcilliation.
CAPV and IPAM Provider logs may also be helpful.

The `Node` objects on the workload cluster should show Internal/External
addresses from the configured pool.

```bash
# on the management cluster
kubectl get ipaddressclaim -n cluster-ns
kubectl get ipaddress -n cluster-ns
kubectl get vspherevm -n cluster-ns -o yaml
kubectl logs -n caip-in-cluster-system caip-in-cluster-controller-manager-bc6ffd66-hp6jm
kubectl logs -n capv-system capv-controller-manager-6f4dc84865-7nh89

# on the new workload cluster
kubectl --kubeconifg cluster.kc get nodes -o wide
```
