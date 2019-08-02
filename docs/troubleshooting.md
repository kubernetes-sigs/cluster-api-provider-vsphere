# Troubleshooting

This is a guide on how to troubleshoot issues related to the Cluster API provider for vSphere (CAPV).

- [Troubleshooting](#troubleshooting)
  - [Debugging issues](#debugging-issues)
    - [Bootstrapping with logging](#bootstrapping-with-logging)
      - [Adjusting log levels](#adjusting-log-levels)
        - [Adjusting the CAPI manager log level](#adjusting-the-capi-manager-log-level)
        - [Adjusting the CAPV manager log level](#adjusting-the-capv-manager-log-level)
        - [Adjusting the `clusterctl` log level](#adjusting-the-clusterctl-log-level)
      - [Accessing the logs in the bootstrap cluster](#accessing-the-logs-in-the-bootstrap-cluster)
        - [Exporting the kubeconfig](#exporting-the-kubeconfig)
        - [Following the CAPI manager logs](#following-the-capi-manager-logs)
        - [Following the CAPV manager logs](#following-the-capv-manager-logs)
        - [Following Kubernetes core component logs](#following-kubernetes-core-component-logs)
          - [The API server](#the-api-server)
          - [The controller manager](#the-controller-manager)
          - [The scheduler](#the-scheduler)
  - [Common issues](#common-issues)
    - [Ensure prerequisites are up to date](#ensure-prerequisites-are-up-to-date)
    - [`envvars.txt` is a directory](#envvarstxt-is-a-directory)
    - [Failed to retrieve kubeconfig secret](#failed-to-retrieve-kubeconfig-secret)
    - [Timed out while failing to retrieve kubeconfig secret](#timed-out-while-failing-to-retrieve-kubeconfig-secret)
      - [Cannot access the vSphere endpoint](#cannot-access-the-vsphere-endpoint)
      - [A VM with the same name already exists](#a-vm-with-the-same-name-already-exists)
      - [A static IP address must include the segment length](#a-static-ip-address-must-include-the-segment-length)
      - [Multiple networks](#multiple-networks)
        - [Multiple default routes](#multiple-default-routes)
        - [Preferring an IP address](#preferring-an-ip-address)

## Debugging issues

This section describes how to debug issues tha occur while trying to deploy a new cluster with `clusterctl` and CAPV.

### Bootstrapping with logging

The first step to figuring out what went wrong is to increase the logging.

#### Adjusting log levels

There are three places to adjust the log level when bootstrapping cluster.

##### Adjusting the CAPI manager log level

The following steps may be used to adjust the CAPI manager's log level:

1. Open the `provider-components.yaml` file, ex. `./out/management-cluster/provider-components.yaml`
2. Search for `cluster-api/cluster-api-controller`
3. Modify the pod spec for the CAPI manager to indicate where to send logs and the log level:

    ```yaml
    spec:
      containers:
      - args:
        - --logtostderr
        - -v=6
        command:
        - /manager
        image: us.gcr.io/k8s-artifacts-prod/cluster-api/cluster-api-controller:v0.1.7
        name: manager
    ```

A log level of six should provided additional information useful for figuring out most issues.

##### Adjusting the CAPV manager log level

1. Open the `provider-components.yaml` file, ex. `./out/management-cluster/provider-components.yaml`
2. Search for `cluster-api-provider-vsphere`
3. Modify the pod spec for the CAPV manager to indicate the log level:

    ```yaml
    spec:
      containers:
      - args:
        - --logtostderr
        - -v=6
        command:
        - /manager
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        image: gcr.io/cluster-api-provider-vsphere/ci/manager:latest
        name: manager
    ```

A log level of six should provided additional information useful for figuring out most issues.

##### Adjusting the `clusterctl` log level

The `clusterctl` log level may be specified when running `clusterctl`:

```shell
clusterctl create cluster \
  -a ./out/management-cluster/addons.yaml \
  -c ./out/management-cluster/cluster.yaml \
  -m ./out/management-cluster/machines.yaml \
  -p ./out/management-cluster/provider-components.yaml \
  --kubeconfig-out ./out/management-cluster/kubeconfig \
  --provider vsphere \
  --bootstrap-type kind \
  -v 6
```

The last line of the above command, `-v 6`, tells `clusterctl` to log messages at level six. This should provide additional information that may be used to diagnose issues.

#### Accessing the logs in the bootstrap cluster

The `clusterctl` logs are client-side only. The more interesting information is occurring inside of the bootstrap cluster. This section describes how to access the logs in the bootstrap cluster.

##### Exporting the kubeconfig

To make the subsequent steps easier, please go ahead and export a `KUBECONFIG` environment variable to point to the bootstrap cluster that is or will be running via Kind:

```shell
export KUBECONFIG=$(kind get kubeconfig-path --name=clusterapi)
```

##### Following the CAPI manager logs

The following command may be used to follow the logs from the CAPI manager:

```shell
$ while ! kubectl \
  -n cluster-api-system logs cluster-api-controller-manager-0 -f || \
  true; do sleep 1; done
The connection to the server localhost:8080 was refused - did you specify the right host or port?
The connection to the server localhost:8080 was refused - did you specify the right host or port?
I0726 18:52:10.267212       1 main.go:65] Registering Components
I0726 18:52:10.269442       1 controller.go:121] kubebuilder/controller "level"=0 "msg"="Starting EventSource"  "controller"="machinedeployment-controller" "source"={"Type":{"metadata":{"creationTimestamp":null},"spec":{"selector":{},"template":{"metadata":{"creationTimestamp":null},"spec":{"metadata":{"creationTimestamp":null},"providerSpec":{},"versions":{"kubelet":""}}}},"status":{}}}
I0726 18:52:10.269819       1 controller.go:121] kubebuilder/controller "level"=0 "msg"="Starting EventSource"  "controller"="machinedeployment-controller" "source"={"Type":{"metadata":{"creationTimestamp":null},"spec":{"selector":{},"template":{"metadata":{"creationTimestamp":null},"spec":{"metadata":{"creationTimestamp":null},"providerSpec":{},"versions":{"kubelet":""}}}},"status":{"replicas":0}}}
```

The above command immediately begins trying to follow the CAPI manager log, even before the bootstrap cluster and the CAPI manager pod exist. Once the latter is finally available, the command will start following its log.

##### Following the CAPV manager logs

To tail the logs from the CAPV manager image, use the following command:

```shell
$ while ! kubectl \
  -n vsphere-provider-system logs vsphere-provider-controller-manager-0 \
  -f || true; do sleep 1; done
The connection to the server localhost:8080 was refused - did you specify the right host or port?
The connection to the server localhost:8080 was refused - did you specify the right host or port?
Error from server (NotFound): namespaces "vsphere-provider-system" not found
Error from server (NotFound): namespaces "vsphere-provider-system" not found
Error from server (NotFound): pods "vsphere-provider-controller-manager-0" not found
Error from server (NotFound): pods "vsphere-provider-controller-manager-0" not found
Error from server (BadRequest): container "manager" in pod "vsphere-provider-controller-manager-0" is waiting to start: ContainerCreating
Error from server (BadRequest): container "manager" in pod "vsphere-provider-controller-manager-0" is waiting to start: ContainerCreating
I0726 18:30:17.988872       1 main.go:92] Cluster-api objects are synchronized every 10m0s
I0726 18:30:17.989473       1 main.go:93] The default requeue period is 20s
I0726 18:30:18.066304       1 round_trippers.go:405] GET https://10.96.0.1:443/api?timeout=32s 200 OK in 76 milliseconds
```

The above command immediately begins trying to follow the CAPV manager log, even before the bootstrap cluster and the CAPV manager pod exist. Once the latter is finally available, the command will start following its log.

##### Following Kubernetes core component logs

Solving issues may also require accessing the logs from the bootstrap cluster's core components:

###### The API server

```shell
kubectl -n kube-system logs kube-apiserver-clusterapi-control-plane -f
```

###### The controller manager

```shell
kubectl -n kube-system logs kube-controller-manager-clusterapi-control-plane -f
```

###### The scheduler

```shell
kubectl -n kube-system logs kube-scheduler-clusterapi-control-plane -f
```

## Common issues

This section contains issues commonly encountered by people using CAPV.

### Ensure prerequisites are up to date

The [Getting Started](getting_started.md) guide lists the prerequisites for deploying clusters with CAPV. Make sure those prerequisites, such as `clusterctl`, `kubectl`, `kind`, etc. are up to date.

### `envvars.txt` is a directory

When generating the YAML manifest the following error may occur:

```shell
$ docker run --rm \
>   -v "$(pwd)":/out \
>   -v "$(pwd)/envvars.txt":/envvars.txt:ro \
>   gcr.io/cluster-api-provider-vsphere/release/manifests:latest \
>   -c management-cluster
/build/hack/generate-yaml.sh: line 90: source: /envvars.txt: is a directory
```

This means that `"$(pwd)/envvars.txt"` does not refer to an existing file on the localhost. So instead of bind mounting a file into the container, Docker created a new directory on the localhost at the path `"$(pwd)/envvars.txt"` and bind mounted *it* into the container.

Make sure the path to the `envvars.txt` file is correct before using it to generate the YAML manifests.

### Failed to retrieve kubeconfig secret

When bootstrapping the management cluster, the vSphere manager log may emit errors similar to the following:

```shell
E0726 17:12:54.812485       1 actuator.go:217] [cluster-actuator]/cluster.k8s.io/v1alpha1/default/v0.4.0-beta.2 "msg"="target cluster is not ready" "error"="unable to get client for target cluster: failed to retrieve kubeconfig secret for Cluster \"management-cluster\" in namespace \"default\": secret not found"
```

The above error does not mean there is a problem. Kubernetes components operate in a reconciliation model -- a message loops attempts to reconcile the desired state over and over until it is achieved or a timeout occurs.

The error message simply indicates that the first control plane node for the target cluster has not yet come online and provided the information necessary to generate the `kubeconfig` for the target cluster.

It is quite typical to see many errors in Kubernetes service logs, from the API server, to the controller manager, to the kubelet -- the errors are eventually reconciled as the expected configurations are provided and the desired state is reconciled.

### Timed out while failing to retrieve kubeconfig secret

When `clusterctl` times out waiting for the management cluster to come online, and the vSphere manager log repeats `failed to retrieve kubeconfig secret for Cluster` over and over again, it means there was an error bringing the management cluster's first control plane node online. Possible resaons include:

#### Cannot access the vSphere endpoint

Two common causes for a failed deployment are related to accessing the remote vSphere endpoint:

1. The host from which `clusterctl` is executed must have access to the vSphere endpoint to which the management cluster is being deployed.
2. The provided vSphere credentials are invalid.

A quick way to validate both access and the credentials is using the program [`govc`](https://github.com/vmware/govmomi/tree/master/govc) or its container image, [`vmware/govc`](https://hub.docker.com/r/vmware/govc):

```shell
# Define the vSphere's endpoint and access information.
$ export GOVC_URL="myvcenter.com" GOVC_USERNAME="username" GOVC_PASSWORD="password"

# Use "govc" to list the contents of the vSphere endpoint.
$ docker run --rm \
  -e GOVC_URL -e GOVC_USERNAME -e GOVC_PASSWORD \
  vmware/govc \
  ls -k
/my-datacenter/vm
/my-datacenter/network
/my-datacenter/host
/my-datacenter/datastore
```

If the above command fails then there is an issue with accessing the vSphere endpoint, and it must be corrected before `clusterctl` will succeed.

#### A VM with the same name already exists

Deployed VMs get their names from the names of the machines in `machines.yaml` and `machineset.yaml`. If a VM with the same name already exists in the same location as one of the VMs that would be created by a new cluster, then the new cluster will fail to deploy and the CAPV manager log will include an error similar to the following:

```shell
I0726 18:52:48.920975       1 util.go:195] default-logger/cluster.k8s.io/v1alpha1/default/v0.4.0-beta.2/management-cluster-controlplane-1/task-231288 "level"=2 "msg"="task failed"  "description-id"="VirtualMachine.clone"
```

Use the `govc` image to check to see if there is a VM with the same name:

```shell
$ docker run --rm \
  -e GOVC_URL -e GOVC_USERNAME -e GOVC_PASSWORD \
  vmware/govc \
  vm.info -k management-cluster-controlplane-1
Name:           management-cluster-controlplane-1
  Path:         /my-datacenter/vm/management-cluster-controlplane-1
  UUID:         4230a650-c92a-d99a-d9f7-fa2fd770e536
  Guest name:   Other 3.x or later Linux (64-bit)
  Memory:       2048MB
  CPU:          2 vCPU(s)
  Power state:  poweredOn
  Boot time:    <nil>
  IP address:   5.6.7.8
  Host:         1.2.3.4
```

#### A static IP address must include the segment length

Another common error is to omit the segment length when using a static IP address. For example:

```yaml
network:
  devices:
  - networkName: "sddc-cgw-network-6"
    gateway4: 192.168.6.1
    ipAddrs:
    - 192.168.6.20/24
    nameservers:
    - 1.1.1.1
    - 1.0.0.1
```

The above network configuration defines a static IP address, `192.168.6.20`, but also includes the required segment length. Without this, `clusterctl` will timeout waiting for the control plane to come online.

#### Multiple networks

A machine with multiple networks may cause the bootstrap process to fail for various reasons.

##### Multiple default routes

A machine that defines two networks may lead to failure if both networks use DHCP and two default routes are defined on the guest. For example:

```yaml
network:
  devices:
  - networkName: "sddc-cgw-network-5"
    dhcp4: true
  - networkName: "sddc-cgw-network-6"
    dhcp4: true
```

The above network configuratoin from a machine definition includes two network devices, both using DHCP. This likely causes two default routes to be defined on the guest, meaning it's not possible to determine the default IPv4 address that should be used by Kubernetes.

##### Preferring an IP address

Another reason a machine with two networks can lead to failure is because the order in which IP addresses are returned externally from a VM is not guaranteed to be the same order as they are when inspected inside the guest. The solution for this is to define a preferred CIDR -- the network segment that contains the IP that the `kubeadm` bootstrap process selected for the API server. For example:

```yaml
network:
  devices:
  preferredAPIServerCidr: "192.168.5.0/24"
  - networkName: "sddc-cgw-network-6"
    ipAddrs:
    - 192.168.6.20/24
  - networkName: "sddc-cgw-network-5"
    dhcp4: true
```

The above network definition specifies the CIDR to which the IP address belongs that is bound to the Kubernetes API server on the guest.
