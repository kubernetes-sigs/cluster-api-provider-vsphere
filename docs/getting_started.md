# Getting Started

This is a guide on how to get started with CAPV (Cluster API Provider vSphere).
To learn more about cluster API in more depth, check out the the [cluster api docs page](https://cluster-api.sigs.k8s.io/).

Table of Contents
=================

   * [Getting Started](#getting-started)
   * [Table of Contents](#table-of-contents)
      * [Bootstrapping a Management Cluster with clusterctl](#bootstrapping-a-management-cluster-with-clusterctl)
         * [Install Requirements](#install-requirements)
            * [Docker](#docker)
            * [Kind](#kind)
            * [clusterctl](#clusterctl)
            * [kubectl](#kubectl)
         * [vSphere Requirements](#vsphere-requirements)
            * [vCenter Credentials](#vcenter-credentials)
            * [Uploading the CAPV Machine Image](#uploading-the-capv-machine-image)
         * [Generating YAML for the Bootstrap Cluster](#generating-yaml-for-the-bootstrap-cluster)
         * [Using clusterctl](#using-clusterctl)
      * [Managing Workload Clusters using the Management Cluster](#managing-workload-clusters-using-the-management-cluster)

## Bootstrapping a Management Cluster with clusterctl

`clusterctl` is a command line tool used for [bootstrapping](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/book/GLOSSARY.md#bootstrap) your [Management Cluster](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/book/GLOSSARY.md#management-cluster).
Your management cluster stores resources such as `clusters` and `machines` using the Kubernetes API. This is the cluster you use to provision and manage multiple clusters going forward.
Before diving into the bootstrap process, it's worth noting that a Management Cluster is just another Kubernetes cluster with special addons and CRDs. You are not required to use `clusterctl` to
provision your management cluster, however, this guide will be focused on using `clusterctl` to bootstrap your management cluster using [Kind](https://github.com/kubernetes-sigs/kind).

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
$ GO111MODULE="on" go get -mod readonly sigs.k8s.io/kind@v0.3.0
```

#### clusterctl

A version of `clusterctl` that is built from **this** repository must be installed. You can build `clusterctl` with docker using
the `clusterctl-in-docker` make target:

```
# set GOOS based on your environment (linux, darwin, windows, etc)
$ GOOS=linux make clusterctl-in-docker
```

The `clusterctl-in-docker` make target installs `clusterctl` in `bin/clusterctl`. Temporarily add this folder to your PATH to use it
for the rest of this guide:
```
$ export PATH="$PATH:$(pwd)/bin"
```

#### kubectl

`kubectl` is required to use `clusterctl`. See [Install and Set Up kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) for install instructions.

### vSphere Requirements

#### vCenter Credentials

In order for `clusterctl` to bootstrap a management cluster on vSphere, it must be able to connect and authenticate to vCenter.
Ensure you have credentials to your vCenter server (user, password and server URL).

#### Uploading the CAPV Machine Image

It is required that machines provisioned by CAPV use one of the official CAPV machine images as a VM template. The machine images are retrievable from
public URLs. CAPV currently supports machine images based on Ubuntu 18.04 and CentOS 7. You can find the full list of available
machine images on the [Machine Images](./machine_images.md) page. For this guide we'll be deploying Kubernetes v1.13.6 on Ubuntu 18.04, the
machine image is available at https://storage.googleapis.com/capv-images/release/v1.13.6/ubuntu-1804-kube-v1.13.6.ova

[Create a VM template](https://docs.vmware.com/en/VMware-vSphere/6.7/com.vmware.vsphere.vm_admin.doc/GUID-17BEDA21-43F6-41F4-8FB2-E01D275FE9B4.html) using the downloadable OVA URL above. The rest of the guide will assume you named the VM template `ubuntu-1804-kube-v1.13.6`.

### Generating YAML for the Bootstrap Cluster

The bootstrapping process in `clusterctl` requires a few configuration files:
* **cluster.yaml**: a yaml file defining the cluster resource for your management cluster
* **machine.yaml**: a yaml file defining the machine resource for the initial control plane node of your management cluster
* **provider-components.yaml**: a yaml file which adds all the Cluster API and CAPV-specific resources to your management cluster
* **addons.yaml**: a yaml file indicating any additional addons you want on your management cluster (e.g. CNI plugin)

The project Makefile provides a convenient make target to generate the yaml files. Before attempting to generate the above yaml file,
the following environment variables should be set based on your vSphere environment:

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
export VSPHERE_MEM_MIB="2048"                       # (optional) The memory (in MiB) for control plane nodes in your management cluster, defaults to 208 if not set
export SSH_AUTHORIZED_KEY="ssh-rsa AAAAB3N..."      # (optional) The public ssh authorized key on all machines in this cluster

# Kubernetes configs
export KUBERNETES_VERSION=1.13.6       # (optional) The Kubernetes version to use, defaults to 1.13.6
export CLUSTER_NAME="my-cluster"       # (optional) The name for the management cluster, defaults to "capv-mgmt-example"
export SERVICE_CIDR="100.64.0.0/13"    # (optional) The service CIDR of the management cluster, defaults to "100.64.0.0/13"
export CLUSTER_CIDR="100.96.0.0/11"    # (optional) The cluster CIDR of the management cluster, defaults to "100.96.0.0/11"
```

With the above environment variables set, you can now run the generate yaml make target in the root directory of this project:
```
$ make prod-yaml

done generating ./out/addons.yaml
done generating ./config/default/capv_manager_image_patch.yaml
done generating ./out/cluster.yaml
done generating ./out/machines.yaml
done generating ./out/machineset.yaml
Done generating ./out/provider-components.yaml

*** Finished creating initial example yamls in ./out

    The files ./out/cluster.yaml and ./out/machines.yaml need to be updated
    with information about the desired Kubernetes cluster and vSphere environment
    on which the Kubernetes cluster will be created.

Enjoy!
```

### Using clusterctl

Once `make prod-yaml` has succeeded, all the necessary yaml files required for the bootstrapping process
should be in the `out/` directory. Go to the `out` directory and run the following `clusterctl` command to bootstrap
your management cluster

```bash
$ cd out
$ clusterctl create cluster --provider vsphere --bootstrap-type kind -c cluster.yaml -m machines.yaml -p provider-components.yaml --addon-components addons.yaml
```

Once `clusterctl` has successfully bootstrapped your management cluster, it should have left a `kubeconfig` file in the same
path that it ran (i.e. `out/kubeconfig`). This is the **admin** kubeconfig file for your management cluster, you can use it
going forward to spin up multiple clusters using Cluster API, however, it is recommended that you create dedicated roles
with limited access before doing so.

## Managing Workload Clusters using the Management Cluster

With your management cluster bootstrapped, it's time to reap the benefits of Cluster API. From this point forward,
clusters and machines (belonging to a cluster) are simply provisioned by creating `cluster`, `machine` and `machineset` resources.

Taking the generated `out/cluster.yaml` and `out/machine.yaml` file from earlier as a reference, you can create a cluster with the
initial control plane node by just editing the name of the cluster and machine resource. For example, the following cluster and
machine resource will provision a cluster named "prod-workload" with 1 initial control plane node:

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
        apiVersion: "vsphereproviderconfig/v1alpha1"
        kind: "VsphereClusterProviderConfig"
        vsphereUser: "<REDACTED>"
        vspherePassword: "<REDACTED>"
        vsphereServer: "<REDACTED>"
        vsphereCredentialSecret: ""
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
      apiVersion: vsphereproviderconfig/v1alpha1
      kind: VsphereMachineProviderConfig
      machineSpec:
        datacenter: "SDDC-Datacenter"
        datastore: "DefaultDatastore"
        resourcePool: "*/CAPV"
        vmFolder: "Workloads"
        network:
          devices:
          - networkName: "vm-network-1"
            dhcp4: true
            dhcp6: true
        numCPUs: 2
        memoryMB: 2048
        diskGiB: 20
        template: "ubuntu-1804-kube-v1.13.6"
  versions:
    kubelet: "1.13.6"
    controlPlane: "1.13.6"
```

To add 3 additional worker nodes to your cluster, create a machineset like the following:

```yaml
apiVersion: "cluster.k8s.io/v1alpha1"
kind: MachineSet
metadata:
  name: prod-workload-machineset
spec:
  replicas: 3
  selector:
    matchLabels:
      node-type: worker-node
      cluster.k8s.io/cluster-name: prod-workload
  template:
    metadata:
      labels:
        node-type: worker-node
        cluster.k8s.io/cluster-name: prod-workload
    spec:
      providerSpec:
        value:
          apiVersion: vsphereproviderconfig/v1alpha1
          kind: VsphereMachineProviderConfig
          machineSpec:
            datacenter: "SDDC-Datacenter"
            datastore: "DefaultDatastore"
            resourcePool: "*/CAPV"
            vmFolder: "Workloads"
            network:
              devices:
              - networkName: "vm-network-1"
                dhcp4: true
                dhcp6: true
            numCPUs: 2
            memoryMB: 2048
            diskGiB: 20
            template: "ubuntu-1804-kube-v1.13.6"
      versions:
        kubelet: "1.13.6"
        controlPlane: "1.13.6"
```

Run `kubectl apply -f` to apply the above files on your management cluster and it should start provisioning the new cluster.
Clusters that are provisioned by the management cluster that run your application workloads are called [Workload Clusters](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/book/GLOSSARY.md#workload-cluster).

The `kubeconfig` file to access workload clusters should be accessible as a Kubernetes Secret on the management cluster. As of today, the
Secret resource is named `<cluster-name>-kubeconfig` in the same namespace as the cluster to which the Secret belongs. For the example above,
you can fetch the kubeconfig by finding the Secret `prod-workload-kubeconfig` like so:
```
$ kubectl -n default get secrets prod-workload-kubeconfig -o yaml
```

Now that you have the `kubeconfig` for your Workload Cluster, you can start deploying your applications there.
