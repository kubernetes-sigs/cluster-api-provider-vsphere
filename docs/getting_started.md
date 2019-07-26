# Getting Started

This is a guide on how to get started with CAPV (Cluster API Provider vSphere). To learn more about cluster API in more depth, check out the the [cluster api docs page](https://cluster-api.sigs.k8s.io/).

* [Getting Started](#Getting-Started)
  * [Bootstrapping a Management Cluster with clusterctl](#Bootstrapping-a-Management-Cluster-with-clusterctl)
    * [Install Requirements](#Install-Requirements)
      * [clusterctl](#clusterctl)
      * [Docker](#Docker)
      * [Kind](#Kind)
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

#### clusterctl

Please download `clusterctl` from the GitHub [releases page](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/releases).

#### Docker

Docker is required for the bootstrap cluster using `clusterctl`. See the [docker documentation](https://docs.docker.com/glossary/?term=install) for install instructions.

#### Kind

`clusterctl` uses [Kind](https://github.com/kubernetes-sigs/kind) to provision the bootstrap cluster. Please see the [kind documentation](https://kind.sigs.k8s.io) for install instructions.

#### kubectl

`kubectl` is required to use `clusterctl`. See [Install and Set Up kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) for install instructions.

### vSphere Requirements

#### vCenter Credentials

In order for `clusterctl` to bootstrap a management cluster on vSphere, it must be able to connect and authenticate to vCenter. Ensure you have credentials to your vCenter server (user, password and server URL).

#### Uploading the CAPV Machine Image

It is required that machines provisioned by CAPV use one of the official CAPV machine images as a VM template. The machine images are retrievable from public URLs. CAPV currently supports machine images based on Ubuntu 18.04 and CentOS 7. You can find the full list of available machine images on the [Machine Images](./machine_images.md) page. For this guide we'll be deploying Kubernetes v1.13.6 on Ubuntu 18.04 (link to [machine image](https://storage.googleapis.com/capv-images/release/v1.13.6/ubuntu-1804-kube-v1.13.6.ova)).

[Create a VM template](https://docs.vmware.com/en/VMware-vSphere/6.7/com.vmware.vsphere.vm_admin.doc/GUID-17BEDA21-43F6-41F4-8FB2-E01D275FE9B4.html) using the OVA URL above. The rest of the guide will assume you named the VM template `ubuntu-1804-kube-v1.13.6`.

### Generating YAML for the Bootstrap Cluster

The bootstrapping process in `clusterctl` requires a few manifest files:

| Name | Description |
|------|-------------|
| `cluster.yaml` | The cluster resource for the target cluster |
| `machines.yaml` | The machine resource for the initial control plane node for the target cluster |
| `provider-components.yaml` | The CAPI and CAPV reources for the target cluster |
| `addons.yaml` | Additional add-ons to apply to the management cluster (ex. CNI) |

Before attempting to generate the above manifests, the following environment variables should be set based on your vSphere environment:

```shell
$ cat <<EOF >envvars.txt
# vCenter config/credentials
export VSPHERE_SERVER='10.0.0.1'                # (required) The vCenter server IP or UR
export VSPHERE_USERNAME='viadmin@vmware.local'  # (required) The vCenter user to login with
export VSPHERE_PASSWORD='some-secure-password'  # (required) The vCenter password to login with

# vSphere deployment configs
export VSPHERE_DATACENTER='SDDC-Datacenter'         # (required) The vSphere datacenter to deploy the management cluster on
export VSPHERE_DATASTORE='DefaultDatastore'         # (required) The vSphere datastore to deploy the management cluster on
export VSPHERE_NETWORK='vm-network-1'               # (required) The VM network to deploy the management cluster on
export VSPHERE_RESOURCE_POOL='Resources'            # (required) The vSphere resource pool for your VMs
export VSPHERE_FOLDER='vm'                          # (optional) The VM folder for your VMs, defaults to the root vSphere folder if not set.
export VSPHERE_TEMPLATE='ubuntu-1804-kube-v1.13.6'  # (required) The VM template to use for your management cluster.
export VSPHERE_DISK_GIB='50'                        # (optional) The VM Disk size in GB, defaults to 20 if not set
export VSPHERE_NUM_CPUS='2'                         # (optional) The # of CPUs for control plane nodes in your management cluster, defaults to 2 if not set
export VSPHERE_MEM_MIB='2048'                       # (optional) The memory (in MiB) for control plane nodes in your management cluster, defaults to 2048 if not set
export SSH_AUTHORIZED_KEY='ssh-rsa AAAAB3N...'      # (optional) The public ssh authorized key on all machines in this cluster

# Kubernetes configs
export KUBERNETES_VERSION='1.13.6'        # (optional) The Kubernetes version to use, defaults to 1.13.6
export SERVICE_CIDR='100.64.0.0/13'       # (optional) The service CIDR of the management cluster, defaults to "100.64.0.0/13"
export CLUSTER_CIDR='100.96.0.0/11'       # (optional) The cluster CIDR of the management cluster, defaults to "100.96.0.0/11"
EOF
```

With the above environment variable file it is now possible to generate the manifests needed to bootstrap the management cluster. The following command uses Docker to run an image that has all of the necessary templates and tools to generate the YAML manifests. Additionally, the `envvars.txt` file created above is mounted inside the the image in order to provide the generation routine with its default values:

```shell
$ docker run --rm \
  -v "$(pwd)":/out \
  -v "$(pwd)/envvars.txt":/envvars.txt:ro \
  gcr.io/cluster-api-provider-vsphere/release/manifests:latest \
  -c management-cluster

done generating ./out/management-cluster/addons.yaml
done generating ./config/default/manager_image_patch.yaml
done generating ./out/management-cluster/cluster.yaml
done generating ./out/management-cluster/machines.yaml
done generating ./out/management-cluster/machineset.yaml
done generating ./out/management-cluster/provider-components.yaml

*** Finished creating initial example yamls in ./out

    The files ./out/management-cluster/cluster.yaml and ./out/management-cluster/machines.yaml need to be updated
    with information about the desired Kubernetes cluster and vSphere environment
    on which the Kubernetes cluster will be created.

Enjoy!
```

### Using clusterctl

Once the manifests are generated, `clusterctl` may be used to create the management cluster:

```shell
clusterctl create cluster \
  --provider vsphere \
  --bootstrap-type kind \
  --cluster ./out/management-cluster/cluster.yaml \
  --machines ./out/management-cluster/machines.yaml \
  --provider-components ./out/management-cluster/provider-components.yaml \
  --addon-components ./out/management-cluster/addons.yaml \
  --kubeconfig-out ./out/management-cluster/kubeconfig
```

Once `clusterctl` has completed successfully, the file `./out/management-cluster/kubeconfig` may be used to access the new management cluster. This is the **admin** `kubeconfig` for the management cluster, and it may be used to spin up additional clusters with Cluster API. However, the creation of roles with limited access, is recommended before creating additional clusters.

**NOTE**: From this point forward `clusterctl` is no longer required to provision new clusters. Workload clusters should be provisioned by applying Cluster API resources directly on the management cluster using `kubectl`.

## Managing Workload Clusters using the Management Cluster

With your management cluster bootstrapped, it's time to reap the benefits of Cluster API. From this point forward, clusters and machines (belonging to a cluster) are simply provisioned by creating `cluster`, `machine` and `machineset` resources.

Using the same Docker command as above, generate resources for a new cluster, this time with a different name:

```shell
$ docker run --rm \
  -v "$(pwd)":/out \
  -v "$(pwd)/envvars.txt":/envvars.txt:ro \
  gcr.io/cluster-api-provider-vsphere/release/manifests:latest \
  -c workload-cluster-1
```

**NOTE**: The above step is not required to manage your Cluster API resources at this point but is used to simplify this guide. You should manage your Cluster API resources in the same way you would manage your Kubernetes application manifests. Please use the generated manifests only as a reference.

The Cluster and Machine resource in `./out/workload-cluster-1/cluster.yaml` and `./out/workload-cluster-1/machines.yaml` defines the workload cluster with the initial control plane node:

```yaml
---
apiVersion: "cluster.k8s.io/v1alpha1"
kind: Cluster
metadata:
  name: workload-cluster-1
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
  name: "workload-cluster-1-controlplane-1"
  labels:
    cluster.k8s.io/cluster-name: "workload-cluster-1"
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

To add 3 additional worker nodes to your cluster, see the generated machineset file `./out/workload-cluster-1/machineset.yaml`:

```yaml
apiVersion: "cluster.k8s.io/v1alpha1"
kind: MachineSet
metadata:
  name: workload-cluster-1-machineset
spec:
  replicas: 3
  selector:
    matchLabels:
      machineset-name: workload-cluster-1-machineset
      cluster.k8s.io/cluster-name: workload-cluster-1
  template:
    metadata:
      labels:
        machineset-name: workload-cluster-1-machineset
        cluster.k8s.io/cluster-name: workload-cluster-1
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

Use `kubectl` with the `kubeconfig` for the management cluster to provision the new workload cluster:

1. Export the management cluster's `kubeconfig` file:

    ```shell
    export KUBECONFIG="$(pwd)/out/management-cluster/kubeconfig"
    ```

2. Create the workload cluster by applying the cluster manifest:

    ```shell
    $ kubectl apply -f ./out/workload-cluster-1/cluster.yaml
    cluster.cluster.k8s.io/workload-cluster-1 created
    ```

3. Create the control plane nodes for the workload cluster by applying the machines manifest:

    ```shell
    $ kubectl apply -f ./out/workload-cluster-1/machines.yaml
    machine.cluster.k8s.io/workload-cluster-1-controlplane-1 created
    ```

4. Create the worker nodes for the workload cluster by applying the machineset manifest:

    ```shell
    $ kubectl apply -f ./out/workload-cluster-1/machineset.yaml
    machineset.cluster.k8s.io/workload-cluster-1-machineset-1 created
    ```

Clusters that are provisioned by the management cluster that run your application workloads are called [Workload Clusters](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/book/GLOSSARY.md#workload-cluster).

The `kubeconfig` file to access workload clusters should be accessible as a Kubernetes Secret on the management cluster. As of today, the Secret resource is named `<cluster-name>-kubeconfig` in the same namespace as the cluster to which the Secret belongs. For the example above, you can list all the kubeconfig files and then retrive the corresponding kubeconfig like so:

1. Retrieve a list of the secrets stored in the management cluster:

    ```shell
    $ kubectl get secrets
    NAME                            TYPE                                  DATA   AGE
    management-cluster-kubeconfig   Opaque                                1      18h
    workload-cluster-1-kubeconfig   Opaque                                1      17h
    ```

2. Get the KubeConfig secret for the `workload-cluster-1` cluster and decode it from base64:

    ```shell
    kubectl get secret workload-cluster-1-kubeconfig -o=jsonpath='{.data.value}' | \
    { base64 -d 2>/dev/null || base64 -D; } >./out/workload-cluster-1/kubeconfig
    ```

The new `./out/workload-cluster-1/kubeconfig` file may now be used to access the workload cluster.

**NOTE**: Workload clusters do not have any addons applied aside from those added by kubeadm. Nodes in your workload clusters will be in the `NotReady` state until you apply a CNI addon. The `addons.yaml` files generated above have a default Calico addon which you can use, otherwise apply custom addons based on your use-case.
