# Quickstart Intro

The following is a quick how-to-use guide on using the cluster api on a vCenter infrastructure along with troubleshooting information to use if you get stuck.  Before beginning, make sure you have the following requirements:

1. vCenter 6.5+ cluster or ESXi 6.5+
    - You will need to gather some information about the cluster, described below.
    - [ESXi provisioning](./esx_provisioning.md) should be used for development purposes only.
2. golang 1.12+
3. a type 2 desktop hypervisor (Vmware Fusion/Workstation or VirtualBox)
4. GOPATH environment set
5. dep installed (https://github.com/golang/dep)
6. kustomize V2 installed (https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md)
7. kubebuilder installed (https://book.kubebuilder.io/quick_start.html)

**Be aware, the current repo supports deploying Kubernetes 1.11.x and above.**

### Decide on whether to use minikube or kind as your bootstrapper

In the self-service workflow, clusterctl will create a bootstrap cluster to kick off the process.  You have two options for bootstrappers: minikube or kind.  Kind is a relatively new project that starts a cluster within Docker on your local machine.  This removes the need to have a desktop hypervisor installed on your local machine.  If you intend to use the kind bootstrapper, you may skip the next section on Fusion and minikube.

### Preparing VMware Fusion/Workstation for minikube

1. Download a release build from the [minikube release page](https://github.com/kubernetes/minikube/releases)
2. Install docker-machine driver for vmware (shown below)
```
$> curl -Lo minikube https://storage.googleapis.com/minikube/releases/v0.33.1/minikube-darwin-amd64 && chmod +x minikube && sudo cp minikube /usr/local/bin/ && rm minikube

$> export LATEST_VERSION=$(curl -L -s -H 'Accept: application/json' https://github.com/machine-drivers/docker-machine-driver-vmware/releases/latest | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/') \
   && curl -L -o docker-machine-driver-vmware https://github.com/machine-drivers/docker-machine-driver-vmware/releases/download/$LATEST_VERSION/docker-machine-driver-vmware_darwin_amd64 \
   && chmod +x docker-machine-driver-vmware \
   && sudo mv docker-machine-driver-vmware /usr/local/bin/
```

### Prepare kind

To install kind, simply tell Go to download and build it.

```
$> go get sigs.k8s.io/kind
```


### Building clusterctl

To run clusterctl on vCenter, you must build the CLI from this repo.  The following instruction assumes $GOPATH is defined and $GOPATH/bin is in your current path.

```
$> go get sigs.k8s.io/cluster-api-provider-vsphere
$> cd $GOPATH/src/sigs.k8s.io/cluster-api-provider-vsphere/cmd/clusterctl
$> go build && go install
```

### Prepare your vCenter

The current cluster api components and clusterctl CLI assumes two preconditions:

1. Clusters are created inside a resource pool.  Create one if you do not already have a resource pool.
2. A template of a linux distro with cloud init exist on your vCenter
    - download https://cloud-images.ubuntu.com/releases/16.04/release/ubuntu-16.04-server-cloudimg-amd64.ova
    - deploy the ova
    - create a template for this vm

### Prepare cluster definition files

In *$GOPATH/src/sigs.k8s.io/cluster-api-provider-vsphere*, the makefile has targets to create base definition files, build the controller container, and push the container up to a repo.  For this example, use the production controller containers that have already been built.  For developers that want to modify the controller, there is a section below on makefile targets.  For now, follow the following commands:
```
$> make prod-yaml
```

Once that command has been executed, there should now be an `out/` folder in the repo's root path.  This folder should now contain 4 base yaml files, provider-components.yaml, machines.yaml, cluster.yaml, addons.yaml.

In cluster.yaml, update the name of the cluster and the providerSpec section with your vCenter auth data. Below is an example.
```
apiVersion: "cluster.k8s.io/v1alpha1"
kind: Cluster
metadata:
  name: test1
spec:
    clusterNetwork:
        services:
            cidrBlocks: ["10.96.0.0/12"]
        pods:
            cidrBlocks: ["192.168.0.0/16"]
        serviceDomain: "cluster.local"
    providerSpec:
      value:
        apiVersion: "vsphereproviderconfig/v1alpha1"
        kind: "VsphereClusterProviderConfig"
        vsphereUser: "administrator@vsphere.local"
        vspherePassword: "xxxx"
        vsphereServer: "mycluster.mycompany.com"
```

The machines.yaml file defines the master nodes of your cluster, and the machineset.yaml defines the worker nodes of your cluster.  Edit the providerSpec section of both files.  Below is an example of a modified providerSpec section.
```
items:
- apiVersion: "cluster.k8s.io/v1alpha1"
  kind: Machine
  metadata:
    generateName: vs-master-
    labels:
      cluster.k8s.io/cluster-name: test1
      set: master
  spec:
      providerSpec:
        value:
          apiVersion: "vsphereproviderconfig/v1alpha1"
          kind: "VsphereMachineProviderConfig"
        machineSpec:
          datacenter: "mydc"
          datastore: "mydatastore"
          resourcePool: "my-resource-pool"
          networks:
          - networkName: "VM Network"
            ipConfig:
              networkType: static
              ip: ""
              netmask: ""
              gateway: ""
              dns:
              - xxxx
              - yyyy
          numCPUs: 2
          memoryMB: 2048
          template: "xenial-server-cloudimg-amd64"
          disks:
          - diskLabel: "Hard disk 1"
            diskSizeGB: 20
          preloaded: false
          trustedCerts:
          - zzzz
          - aaaa
    versions:
      kubelet: 1.11.1
      controlPlane: 1.11.1
    roles:
    - Master
```

Note, the disk size above in this example needs to be 15GB or higher.  Also note, the machine yaml allow the network to be setup to use either DHCP or static IP.  However, for the machine to be created to use static IP, the VM template used must be derived from a custom cloud init image with the Guestinfo Datasource installed.  The instructions for creating such an image can be found [here](cloud_init_with_guestinfo_datasource.md).

### Create a *target cluster*

The most basic workflow for creating a cluster using *clusterctl* actually ends up creating two clusters.  The first is called the **bootstrap** cluster.  This cluster is created using minikube.  The cluster api components are installed on this cluster.  *Clusterctl* then uses the cluster api server on the bootstrap cluster to create the **target** cluster.  Once the target cluster has been created, *clusterctl* will cleanup by deleting the bootstrap cluster.  There are other workflows to create the target cluster, but for this intro, the most basic workflow is used.  The command is shown below.  Once the CLI has finished, it will put the kubeconfig file for your target cluster in your current folder.  You can use that kubeconfig file to access your new cluster.

Using minikube:
```
$> clusterctl create cluster --provider vsphere --bootstrap-type minikube --bootstrap-flags "vm-driver=vmware" -c cluster.yaml -m machines.yaml -p provider-components.yaml
$> kubectl --kubeconfig ./kubeconfig get no
```

Using KIND:
```
$> clusterctl create cluster --provider vsphere --bootstrap-type kind -c cluster.yaml -m machines.yaml -p provider-components.yaml
$> kubectl --kubeconfig ./kubeconfig get no

```

The above command has created a target cluster with just a master node.  Now, create the worker nodes for the cluster.  For this, you will use kubectl.

```
$> kubectl --kubeconfig ./kubeconfig create -f ./machineset.yaml
$> kubectl --kubeconfig ./kubeconfig get no -w
```

Creating the worker nodes will take some time.  The second command above will monitor the nodes getting created.

### Create a *Target Cluster* using an initial *existing cluster*

The clusterctl CLI has an ability to create a cluster **without** creating a bootstrap cluster.  If there is an existing cluster that can serve as the bootstrap cluster, it is possible to provide clusterctl with it's kubeconfig file.  The following example uses minikube to create a kubernetes cluster and then use clusterctl with the `-e` option to let the CLI know about this cluster.


```
// Create a minikube cluster with control plane 1.11.3
$> minikube start --bootstrapper=kubeadm --vm-driver=vmware --kubernetes-version=v1.11.3

// Create a cluster using the kubeconfig created by minikube above
$> clusterctl create cluster -c cluster.yaml -m machines.yaml -p provider-components.yaml --provider vsphere -e $HOME/.kube/config
```

Notice in the example above, the `-e` option was used and the `--vm-driver` option was left out.  Recall, that option is only used to create a bootstrap cluster, and there is no need to create a bootstrap cluster in this example.  Also, notice above, minikube was instructed to install kubernetes 1.11.3.

### Delete the cluster

`Clusterctl delete cluster` currently is unable to delete the cluster.  To delete a cluster created using the above instructions, use kubectl and perform the steps in reverse.  For this to work, the workflow to create a cluster using an existing cluster must be used as this step requires a cluster containing the api objects backing the target cluster.

```
$> minikube start --bootstrapper=kubeadm --vm-driver=vmware --kubernetes-version=v1.11.3
$> clusterctl create cluster -e <existing cluster kubeconfig> ...

...

$> kubectl delete -f machineset.yaml
$> kubectl delete -f machines.yaml
$> kubectl delete -f cluster.yaml
```

### Makefile targets

The makefile in the repo contains a few useful targets described below.

| Target | Description |
| --- | --- |
| make manager | builds the controller binary in cmd/manager.  This target is useful to test building the controller code without the lengthy build process to create the container version. |
| make clusterctl | builds the clusterctl binary cmd/clusterctl.  Like the above, this is useful to test building the code but does not install the binary. |
| make prod-build | builds the container version of the controller.  This target really isn't meant for users or devs who do not have access to the production container's registry. |
| make prod-push | pushes the container version of the container to the production registry. |
| make prod-yaml | creates the base yaml files. |
| make ci-build | used in CI to build the container version of the controller.  This target isn't meant for users.  It is used only by the CI system. |
| make ci-push | used in CI to push the container version of the controller to a registry that the CI uses for testing. |
| make ci-yaml | creates the base yaml files. |
| make dev-build | *used by developers to create their own container version of the controller. |
| make dev-push | *used by developers to push their own container version of the controller. |
| make dev-yaml | *creates the base yaml files. |

Note, for developers who want to test their changes, use the dev targets.  Use the build targets before the yaml targets.  Also, modify the makefile and update the DEV_IMG with the desired registry and container name.

```
# Image URL to use all building/pushing image targets
PRODUCTION_IMG ?= gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:latest
CI_IMG ?= gcr.io/cnx-cluster-api/vsphere-cluster-api-provider
DEV_IMG ?= # <== NOTE:  outside dev, change this!!!
```

During the build targets, the necessary config file gets updated with this image name.  Then once the yaml targets are use, provider-components.yaml will contain the desired controller container image.

## Troubleshooting Resources to Aid in Resolving Provisioning Failures

1. Ensure `provider-components.yaml` specifies a container image for the `vsphere-provider-controller-manager` Statefulset which has ClusterAPI Provider vSphere v0.2.0. or later.  If one is not available then one can be created using the `make dev-build` process.
2. After running `clusterctl create cluster...`, verify the two ClusterAPI Provider vSphere pods `vsphere-provider-controller-manager-0` and `cluster-api-controller-manager-0` are running in the bootstrap cluster:
    ```shell
    $> kubectl get pods --all-namespaces
    NAMESPACE                 NAME                                         READY   STATUS    RESTARTS   AGE
    cluster-api-system        cluster-api-controller-manager-0             1/1     Running   0          5d1h
    kube-system               coredns-fb8b8dccf-bs2v9                      1/1     Running   0          5d1h
    kube-system               coredns-fb8b8dccf-hhc4b                      1/1     Running   0          5d1h
    kube-system               etcd-kind-control-plane                      1/1     Running   0          5d1h
    kube-system               ip-masq-agent-m5jkt                          1/1     Running   0          5d1h
    kube-system               kindnet-5s6tz                                1/1     Running   1          5d1h
    kube-system               kube-apiserver-kind-control-plane            1/1     Running   0          5d1h
    kube-system               kube-controller-manager-kind-control-plane   1/1     Running   0          5d1h
    kube-system               kube-proxy-x57n5                             1/1     Running   0          5d1h
    kube-system               kube-scheduler-kind-control-plane            1/1     Running   0          5d1h
    vsphere-provider-system   vsphere-provider-controller-manager-0        1/1     Running   0          5d1h
    ```
2. If any are failing then view the pod logs to review errors: 
    ```shell
    $> kubectl logs <pod name> --namespace <pod namespace>
    ```
3. After the bootstrap pods have been created, vSphere will create one or more VMs in accordance to the machine yaml files specified with the `clusterctl create cluster` command.
4. Should the `clusterctl create cluster` command fail to retrieve the `admin.conf` file the following steps can be used:
	1. Connect to the manager pod in the bootstrap cluster: 
    ```shell
    $> kubectl exec vsphere-provider-controller-manager-0 -it /bin/bash --namespace vsphere-provider-system
    ```
	2. SSH on the provisioned master VM within vSphere from the manager pod: 
    ```shell
    $> ssh -i ~/.ssh/vsphere_tmp ubuntu@<vm ip address>
    ```
	3. Verify if the following file exists: `/etc/kuberenetes/admin.conf`.  Please note it may take a couple minutes for cloud-init to process and create these files.
        ```shell
        $> ls /etc/kubernetes/
        admin.conf  cloud-config  controller-manager.conf  kubeadm_config.yaml  kubelet.conf  manifests  pki  scheduler.conf
        ```
	4. If either the file or folder do not exist then check the following log files for failed commands: `/var/log/cloud-init.log` and `/var/log/cloud-init-output.log`.
	5. If the log files are still being appended to then cloud-init has not finished processing and may need more time to run.
	6. An example failure which may be listed in `/var/log/cloud-init.log` is `2019-05-06 18:22:41,691 - util.py[WARNING]: Failed loading yaml blob. unacceptable character #xdccf: special characters are not allowed`.  This error indicates an incorrect entry in `machines.yaml` or `machineset.yaml` which was specified in the `clusterctl create cluster` command.  Commonly this could be leaving in the `- xxxx` values in the `machines.yaml` for sections such as `DNS` and `trustedCerts`.
5. From the location of where the clusterctl command was run, once a kubeconfig file is generated, check the status of the nodes: 
    ```shell
    $> kubectl --kubeconfig kubeconfig get nodes
    ```
    1. If the master never enters ready state then check to see if any pods are failing: 
    ```shell
    $> kubectl --kubeconfig kubeconfig get pods --all-namespaces
    ```
    2. Use the logs command to check logs of a failing pod, example: 
    ```shell
    $> kubectl --kubeconfig kubeconfig logs weave-net-dl2bn -c weave --namespace kube-system
    ```
	3. If the `weave-net` pod is indeed failing then you may have specified a network range within `cluster.yaml` under the pods `cidrBlocks` which overlaps an existing network on the provisioned Kubernetes nodes.  For example if the VM IP addresses are within 192.168.0.0/16 then the default `cidrBlock` value will need to be changed.
