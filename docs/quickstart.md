# Quickstart Intro

The following is a quick how-to-use guide on using the cluster api on a vCenter infrastructure.  Before beginning, make sure you have the following requirements:

1. vCenter 6.5 cluster
    - You will need to gather some information about the cluster, described below.
2. golang 1.10+
3. a type 2 desktop hypervisor (Vmware Fusion/Workstation or VirtualBox)
4. GOPATH environment set

### Preparing Vmware Fusion/Workstation for minikube

1. If https://github.com/kubernetes/minikube/pull/2606 has been merged, then clone the minikube repo and build.
2. If the above is not true, do the following,
    - **install minikube**, https://github.com/kubernetes/minikube
    - install the PR 2606 and build (shown below)
3. Install docker-machine driver for vmware (shown below)
```
$> git clone https://github.com/kubernetes/minikube $GOPATH/src/k8s.io/minikube
$> cd $GOPATH/src/k8s.io/minikube
$> git fetch origin pull/2606/head:minikube-vmware
$> git checkout minikube-vmware
$> make
$> cd out
$> sudo cp ./minikube /usr/local/bin  (on linux)
   sudo cp ./minikube-darwin-amd64 /usr/local/bin/minikube (on Mac)
   
   
$> export LATEST_VERSION=$(curl -L -s -H 'Accept: application/json' https://github.com/machine-drivers/docker-machine-driver-vmware/releases/latest | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/') \
   && curl -L -o docker-machine-driver-vmware https://github.com/machine-drivers/docker-machine-driver-vmware/releases/download/$LATEST_VERSION/docker-machine-driver-vmware_darwin_amd64 \
   && chmod +x docker-machine-driver-vmware \
   && sudo mv docker-machine-driver-vmware /usr/local/bin/
```

### Building clusterctl

To run clusterctl on vCenter, you must build the CLI from this repo.  The following instruction assumes $GOPATH is defined and $GOPATH/bin is in your current path.

```
$> git clone https://github.com/kubernetes-sigs/cluster-api-provider-vsphere $GOPATH/src/sigs.k8s.io/cluster-api-provider-vsphere
$> cd $GOPATH/src/sigs.k8s.io/cluster-api-provider-vsphere/clusterctl
$> go build && go install
```

### Prepare your vCenter

The current cluster api components and clusterctl CLI assumes two preconditions:

1. Clusters are created inside a resource pool.  Create one if you do not already have a resouce pool.
2. A template of a linux distro with cloud init exist on your vCenter
    - download https://cloud-images.ubuntu.com/releases/16.04/release/ubuntu-16.04-server-cloudimg-amd64.ova
    - deploy the ova
    - create a template for this vm

### Prepare cluster definition files

In *$GOPATH/src/sigs.k8s.io/cluster-api-provider-vsphere/clusterctl/examples/vsphere*, there are some example definition files. There is also a generate-yaml.sh file.  Run the generate script and copy the .template file to the same name without the .template extensions.
```
$> ./generate-yaml.sh
$> cp cluster.yaml.template cluster.yaml
$> cp machines.yaml.template machines.yaml
$> cp machineset.yaml.template machineset.yaml
```

In cluster.yaml, update the name of the cluster and the providerConfig section with your vCenter auth data. Below is an example.
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
    providerConfig:
      value:
        apiVersion: "vsphereproviderconfig/v1alpha1"
        kind: "VsphereClusterProviderConfig"
        vsphereUser: "administrator@vsphere.local"
        vspherePassword: "xxxx"
        vsphereServer: "mycluster.mycompany.com"
```

The machines.yaml file defines the master nodes of your cluster, and the machineset.yaml defines the worker nodes of your cluster.  Edit the providerConfig section of both files.  Below is an example of a modified providerConfig section.
```
      providerConfig:
        value:
          apiVersion: "vsphereproviderconfig/v1alpha1"
          kind: "VsphereMachineProviderConfig"
          vsphereMachine: "standard-node"
          machineVariables:
            datacenter: "dc"
            datastore: "datastore108"
            resource_pool: "kube-resource-pool"
            network: "VM Network"
            num_cpus: "2"
            memory: "2048"
            vm_template: "xenial-server-cloudimg-amd64"
            disk_label: "disk-0"
            disk_size: "15"
            virtual_machine_domain: ""
```

Note, the disk size above in this example needs to be 15GB or higher.

### Create a cluster

The most basic workflow for creating a cluster using *clusterctl* actually ends up creating two clusters.  The first is called the **bootstrap** cluster.  This cluster is created using minikube.  The cluster api components are installed on this cluster.  *Clusterctl* then uses the cluster api server on the bootstrap cluster to create the **target** cluster.  Once the target cluster has been created, *clusterctl* will cleanup by deleting the bootstrap cluster.  There are other workflows to create the target cluster, but for this intro, the most basic workflow is used.  The command is shown below.  Once the CLI has finished, it will put the kubeconfig file for your target cluster in your current folder.  You can use that kubeconfig file to access your new cluster.

```
$> clusterctl create cluster -c cluster.yaml -m machines.yaml -p provider-components.yaml --provider vsphere --vm-driver vmware
$> kubectl --kubeconfig ./kubeconfig get no
```

The above command has created a target cluster with just a master node.  Now, create the worker nodes for the cluster.  For this, you will use kubectl.

```
$> kubectl --kubeconfig ./kubeconfig create -f ./machineset.yaml
$> kubectl --kubeconfig ./kubeconfig get no -w
```

Creating the worker nodes will take some time.  The second command above will monitor the nodes getting created.

### Delete the cluster

To delete an existing target cluster, we use the kubeconfig file created during the create workflow.  Look through the kubeconfig file and look for the name of the cluster.  In this example, the cluster name was 'kubernetes'.  This currently doesn't yet match the name given to the cluster in the cluster.yaml file.

```
$> clusterctl delete cluster --kubeconfig ./kubeconfig --cluster kubernetes -p provider-components.yaml --vm-driver vmware
```

The workflow for deleting the cluster is very similar to the workflow for creating the cluster.  A bootstrap cluster is created using minikube.  The bootstrap cluster is used to then delete the target cluster.  On cleanup, the bootstrap cluster is removed.