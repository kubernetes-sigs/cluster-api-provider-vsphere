# Getting Started

This is a guide on how to get started with CAPV (Cluster API Provider vSphere). To learn more about cluster API in more depth, check out the the [cluster api docs page](https://cluster-api.sigs.k8s.io/).

* [Getting Started](#Getting-Started)
  * [Bootstrapping a Management Cluster with clusterctl](#Bootstrapping-a-Management-Cluster-with-clusterctl)
    * [Install Requirements](#Install-Requirements)
      * [Docker](#Docker)
      * [Kind](#Kind)
      * [clusterctl](#clusterctl)
      * [kubectl](#kubectl)
    * [vSphere Requirements](#vSphere-Requirements)
      * [vCenter Credentials](#vCenter-Credentials)
      * [Uploading the CAPV Machine Image](#Uploading-the-CAPV-Machine-Image)
    * [Generating YAML for the Bootstrap Cluster](#Generating-YAML-for-the-Bootstrap-Cluster)
    * [Using clusterctl](#Using-clusterctl)
  * [Managing Workload Clusters using the Management Cluster](#Managing-Workload-Clusters-using-the-Management-Cluster)

## Bootstrapping a Management Cluster with clusterctl

`clusterctl` is a command line tool used for [bootstrapping](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/book/GLOSSARY.md#bootstrap) your [Management Cluster](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/book/GLOSSARY.md#management-cluster). Your management cluster stores resources such as `clusters` and `machines` using the Kubernetes API. This is the cluster you use to provision and manage multiple clusters going forward. Before diving into the bootstrap process, it's worth noting that a Management Cluster is just another Kubernetes cluster with special addons and CRDs. You are not required to use `clusterctl` to provision your management cluster, however, this guide will be focused on using `clusterctl` to bootstrap your management cluster using [Kind](https://github.com/kubernetes-sigs/kind).

### Install Requirements

#### Docker

Docker is required for the bootstrap cluster using `clusterctl`. See the [docker documentation](https://docs.docker.com/glossary/?term=install) for install instructions.

#### Kind

`clusterctl` uses [Kind](https://github.com/kubernetes-sigs/kind) to provision the bootstrap cluster.

You can install Kind with the following:

```bash
# Linux
$ curl -Lo ./kind-linux-amd64 https://github.com/kubernetes-sigs/kind/releases/download/v0.3.0/kind-linux-amd64
$ chmod +x ./kind-linux-amd64
$ mv ./kind-linux-amd64 /usr/local/bin/kind

# Darwin
$ curl -Lo ./kind-darwin-amd64 https://github.com/kubernetes-sigs/kind/releases/download/v0.3.0/kind-darwin-amd64
$ chmod +x ./kind-darwin-amd64
$ mv ./kind-darwin-amd64 /usr/local/bin/kind
```

If you have a Go installed on your machine, you can install Kind with the following:

```bash
GO111MODULE="on" go get -mod readonly sigs.k8s.io/kind@v0.3.0
```

#### clusterctl

Install the official release binary of `clusterctl` for CAPV with the following:

```shell
# Linux
$ curl -Lo ./clusterctl.linux_amd64 https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/releases/download/v0.3.0/clusterctl.linux_amd64
$ chmod +x ./clusterctl.linux_amd64
$ sudo mv ./clusterctl.linux_amd64 /usr/local/bin/clusterctl

# Darwin
$ curl -Lo ./clusterctl.darwin_amd64 https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/releases/download/v0.3.0/clusterctl.darwin_amd64
$ chmod +x ./clusterctl.darwin_amd64
$ sudo mv ./clusterctl.darwin_amd64 /usr/local/bin/clusterctl
```

#### kubectl

`kubectl` is required to use `clusterctl`. See [Install and Set Up kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) for install instructions.

### vSphere Requirements

#### vCenter Credentials

In order for `clusterctl` to bootstrap a management cluster on vSphere, it must be able to connect and authenticate to vCenter. Ensure you have credentials to your vCenter server (user, password and server URL).

#### Uploading the CAPV Machine Image

It is required that machines provisioned by CAPV use one of the official CAPV machine images as a VM template. The machine images are retrievable from public URLs. CAPV currently supports machine images based on Ubuntu 18.04 and CentOS 7. You can find the full list of available machine images on the [Machine Images](./machine_images.md) page. For this guide we'll be deploying Kubernetes v1.13.6 on Ubuntu 18.04 (link to [machine image](https://storage.googleapis.com/capv-images/release/v1.13.6/ubuntu-1804-kube-v1.13.6.ova)).

[Create a VM template](https://docs.vmware.com/en/VMware-vSphere/6.7/com.vmware.vsphere.vm_admin.doc/GUID-17BEDA21-43F6-41F4-8FB2-E01D275FE9B4.html) using the downloadable OVA URL above. The rest of the guide will assume you named the VM template `ubuntu-1804-kube-v1.13.6`.

### Generating YAML for the Bootstrap Cluster

The bootstrapping process in `clusterctl` requires a few configuration files:

* **cluster.yaml**: a yaml file defining the cluster resource for your management cluster
* **machines.yaml**: a yaml file defining the machine resource for the initial control plane node of your management cluster
* **provider-components.yaml**: a yaml file which adds all the Cluster API and CAPV-specific resources to your management cluster
* **addons.yaml**: a yaml file indicating any additional addons you want on your management cluster (e.g. CNI plugin)

The project Makefile provides a convenient make target to generate the yaml files. Before attempting to generate the above yaml file, the following environment variables should be set based on your vSphere environment:

```bash
# vCenter config/credentials
export VSPHERE_SERVER=10.0.0.1                # (required) The vCenter server IP or UR
export VSPHERE_USER=viadmin@vmware.local      # (required) The vCenter user to login with
export VSPHERE_PASSWORD=some-secure-password  # (required) The vCenter password to login with

# vSphere deployment configs
export VSPHERE_DATACENTER="SDDC-Datacenter"         # (required) The vSphere datacenter to deploy the management cluster on
export VSPHERE_DATASTORE="DefaultDatastore"         # (required) The vSphere datastore to deploy the management cluster on
export VSPHERE_NETWORK="vm-network-1"               # (required) The VM network to deploy the management cluster on
export VSPHERE_RESOURCE_POOL="*/CAPV"               # (required) The vSphere resource pool for your VMs
export VSPHERE_FOLDER="Workloads"                   # (optional) The VM folder for your VMs, defaults to the root vSphere folder if not set.
export VSPHERE_TEMPLATE="ubuntu-1804-kube-v1.13.6"  # (required) The VM template to use for your management cluster.
export VSPHERE_DISK_GIB="50"                        # (optional) The VM Disk size in GB, defaults to 20 if not set
export VSPHERE_NUM_CPUS="2"                         # (optional) The # of CPUs for control plane nodes in your management cluster, defaults to 2 if not set
export VSPHERE_MEM_MIB="2048"                       # (optional) The memory (in MiB) for control plane nodes in your management cluster, defaults to 2048 if not set
export SSH_AUTHORIZED_KEY="ssh-rsa AAAAB3N..."      # (optional) The public ssh authorized key on all machines in this cluster

# Kubernetes configs
export KUBERNETES_VERSION=1.13.6       # (optional) The Kubernetes version to use, defaults to 1.13.6
export CLUSTER_NAME="my-cluster"       # (optional) The name for the management cluster, defaults to "capv-mgmt-example"
export SERVICE_CIDR="100.64.0.0/13"    # (optional) The service CIDR of the management cluster, defaults to "100.64.0.0/13"
export CLUSTER_CIDR="100.96.0.0/11"    # (optional) The cluster CIDR of the management cluster, defaults to "100.96.0.0/11"
```

With the above environment variables set, you can now run the generate yaml make target in the root directory of this project:

```shell
$ make prod-yaml

done generating ./out/my-cluster/addons.yaml
done generating ./config/default/capv_manager_image_patch.yaml
done generating ./out/my-cluster/cluster.yaml
done generating ./out/my-cluster/machines.yaml
done generating ./out/my-cluster/machineset.yaml
Done generating ./out/my-cluster/provider-components.yaml

*** Finished creating initial example yamls in ./out/my-cluster

    The files ./out/my-cluster/cluster.yaml and ./out/my-cluster/machines.yaml need to be updated
    with information about the desired Kubernetes cluster and vSphere environment
    on which the Kubernetes cluster will be created.

Enjoy!
```

### Using clusterctl

Once `make prod-yaml` has succeeded, all the necessary yaml files required for the bootstrapping process should be in the `out/` directory. Go to the `out` directory and run the following `clusterctl` command to bootstrap your management cluster

```shell
cd out/my-cluster && clusterctl create cluster --provider vsphere --bootstrap-type kind -c cluster.yaml -m machines.yaml -p provider-components.yaml --addon-components addons.yaml
```

Once `clusterctl` has successfully bootstrapped your management cluster, it should have left a `kubeconfig` file in the same
path that it ran (i.e. `out/kubeconfig`). This is the **admin** kubeconfig file for your management cluster, you can use it
going forward to spin up multiple clusters using Cluster API, however, it is recommended that you create dedicated roles
with limited access before doing so.

Note that from this point forward, you no longer need to use `clusterctl` to provision clusters since your management cluster
(the cluster used to manage workload clusters) has been created. Workload clusters should be provisioned by applying Cluster API resources
directly on the management cluster using `kubectl`. More on this below.

## Managing Workload Clusters using the Management Cluster

With your management cluster bootstrapped, it's time to reap the benefits of Cluster API. From this point forward,
clusters and machines (belonging to a cluster) are simply provisioned by creating `cluster`, `machine` and `machineset` resources.

Using the same `prod-yaml` make target, generate Cluster API resources for a new cluster, this time with a different name:

```shell
CLUSTER_NAME=prod-workload make prod-yaml
```

**NOTE**: The `make prod-yaml` target is not required to manage your Cluster API resources at this point but is used to simplify this guide.
You should manage your Cluster API resources in the same way you would manage your application yaml files for Kubernetes. Use the
generated yaml files from `make prod-yaml` as a reference.

The Cluster and Machine resource in `out/prod-workload/cluster.yaml` and `out/prod-workload/machines.yaml` defines your workload
cluster with the initial control plane.

```yaml
---
apiVersion: "cluster.k8s.io/v1alpha1"
kind: Cluster
metadata:
  name: prod-workload
spec:
  clusterNetwork:
    services:
      cidrBlocks: ["100.64.0.0/13"]
    pods:
      cidrBlocks: ["100.96.0.0/11"]
    serviceDomain: "cluster.local"
  providerSpec:
    value:
      apiVersion: "vsphere.cluster.k8s.io/v1alpha1"
      kind: "VsphereClusterProviderSpec"
      server: "<REDACTED>"
      username: "<REDACTED>"
      password: "<REDACTED>"
---
apiVersion: cluster.k8s.io/v1alpha1
kind: Machine
metadata:
  name: "prod-workload-controlplane-1"
  labels:
    cluster.k8s.io/cluster-name: "prod-workload"
spec:
  providerSpec:
    value:
      apiVersion: vsphere.cluster.k8s.io/v1alpha1
      kind: VsphereMachineProviderSpec
      datacenter: "SDDC-Datacenter"
      datastore: "DefaultDatastore"
      resourcePool: "Resources"
      folder: "vm"
      network:
        devices:
        - networkName: "vm-network-1"
          dhcp4: true
          dhcp6: false
      numCPUs: 2
      memoryMiB: 2048
      diskGiB: 20
      template: "ubuntu-1804-kube-v1.13.6"
  versions:
    kubelet: "1.13.6"
    controlPlane: "1.13.6"
```

To add 3 additional worker nodes to your cluster, see the generated machineset file `out/prod-workload/machineset.yaml`:

```yaml
apiVersion: "cluster.k8s.io/v1alpha1"
kind: MachineSet
metadata:
  name: prod-workload-machineset
spec:
  replicas: 3
  selector:
    matchLabels:
      machineset-name: prod-workload-machineset
      cluster.k8s.io/cluster-name: prod-workload
  template:
    metadata:
      labels:
        machineset-name: prod-workload-machineset
        cluster.k8s.io/cluster-name: prod-workload
    spec:
      providerSpec:
        value:
          apiVersion: vsphere.cluster.k8s.io/v1alpha1
          kind: VsphereMachineProviderSpec
          datacenter: "SDDC-Datacenter"
          datastore: "DefaultDatastore"
          resourcePool: "Resources"
          folder: "vm"
          network:
            devices:
            - networkName: "vm-network-1"
              dhcp4: true
              dhcp6: false
          numCPUs: 2
          memoryMiB: 2048
          diskGiB: 20
          template: "ubuntu-1804-kube-v1.13.6"
      versions:
        kubelet: "1.13.6"
        controlPlane: "1.13.6"
```

Run `kubectl apply -f` to apply the above files on your management cluster and it should start provisioning the new cluster:

```shell
$ cd out/prod-workload
$ kubectl apply -f cluster.yaml
cluster.cluster.k8s.io/prod-workload created
$ kubectl apply -f machines.yaml
machine.cluster.k8s.io/prod-workload-controlplane-1 created
$ kubectl apply -f machineset.yaml
machineset.cluster.k8s.io/prod-workload-machineset-1 created
```

Clusters that are provisioned by the management cluster that run your application workloads are called [Workload Clusters](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/book/GLOSSARY.md#workload-cluster).

The `kubeconfig` file to access workload clusters should be accessible as a Kubernetes Secret on the management cluster. As of today, the
Secret resource is named `<cluster-name>-kubeconfig` in the same namespace as the cluster to which the Secret belongs. For the example above,
you can list all the kubeconfig files and then retrive the corresponding kubeconfig like so:

```bash
$ kubectl get secrets
NAME                     TYPE                                  DATA   AGE
my-cluster-kubeconfig   Opaque                                1      18h
prod-workload-kubeconfig   Opaque                                1      17h

$ kubectl get secret prod-workload-kubeconfig -o=jsonpath='{.data.value}' | base64 -d > prod-workload-kubeconfig # Darwin users will want to use the -D flag for base64
```

Now that you have the `kubeconfig` for your Workload Cluster, you can start deploying your applications there.

**NOTE**: workload clusters do not have any addons applied aside from those added by kubeadm. Nodes in your workload clusters
will be in the `NotReady` state until you apply a CNI addon. The `addons.yaml` file generated from `make prod-yaml` has a default calico
addon which you can use, otherwise apply custom addons based on your use-case.
