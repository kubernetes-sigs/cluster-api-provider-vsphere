# GPU enabled clusters via PCI Passthrough

## Overview

Starting with the v1.3 release, Kubernetes clusters with nodes having GPU devices can be created on vSphere. You can choose to create a cluster with both worker and control plane nodes having GPU devices attached to them.

Before we begin, a couple of important things of note:

- [NVIDIA GPU Operator](https://github.com/NVIDIA/gpu-operator) is used to expose the GPU PCI devices to the workloads running on the cluster.
- The OVA templates used for cluster creation should have the VMX version (Virtual Hardware) set to 17. This is necessary because Dynamic DirectPath I/O was introduced in this version, which enables the Assignable Hardware intelligence for passthrough devices.
- Since we need the VMX version to be >=17, this way of provisioning clusters with PCI passthrough devices works for vSphere 7.0 and above. This is the ESXi/VMX version [compatibility list](https://kb.vmware.com/s/article/2007240).
- UEFI boot mode is recommended for the OVAs used for cluster creation.

## An example GPU enabled cluster

Let's create a CAPV cluster with GPU enabled via PCI passthrough mode and run a GPU-powered vector calculation.

### Prerequisites

- Enable Passthrough for the GPU device on the ESXi Host
  - Browse to a host in the vSphere Client navigator.
  - On the **Configure** tab, expand **Hardware** and click **PCI Devices**.
  - Select the GPU device to be used for passthrough and click **TOGGLE PASSTHROUGH**. This sets the device to be available in the passthrough mode.
  <img width="1673" alt="image" src="https://user-images.githubusercontent.com/8758225/178333983-0dcc9771-ba41-4c90-918f-388795d77846.png">

- Find the Device ID and Vendor ID of the PCI device.
  - Browse to a host in the vSphere Client navigator.
  - On the **Configure** tab, expand **Hardware** and click **PCI Devices**.
  - Click on the **PASSTHROUGH-ENABLED DEVICES** tab and select the device you want to use.
  - As shown below, the **General Information** section lists out the Device ID and Vendor ID information.
  <img width="1675" alt="image" src="https://user-images.githubusercontent.com/8758225/178334149-def48b35-1142-4c05-b455-fefd15b1e41a.png">

  **Note**: The device and vendor ID combination is the same for a single family of GPU cards. So, for all the Tesla T4 cards, the values would be the ones listed above.

- Build an OVA template
   We can build a custom OVA template using the [image-builder](https://github.com/kubernetes-sigs/image-builder) project. We will build a Ubuntu 20.04 OVA with UEFI boot mode. More documentation on how to use image-builder can be found in the [image-builder book](https://image-builder.sigs.k8s.io/capi/providers/vsphere.html)
  - Clone the repo locally and go to the `./images/capi/` directory.
  - Create a `packer-vars.json` file with the following content.

    ```shell
    $ cat packer-vars.json
    {
        "vmx_version": 17
    }
    ```

  - Run the make file target associated to ubuntu 20.04 UEFI OVA as follows:

    ```shell
    > PACKER_VAR_FILES=packer-vars.json make build-node-ova-vsphere-ubuntu-2004-efi
    ```

### Source the DeviceID and VendorID for the PCI device

We are using Nvidia Tesla T4 cards for the purpose of this example.

- To use the above values during cluster creation, you need to convert them from Hexadecimal format to Decimal format. For example, the above screenshot lists the device ID and vendor ID for Tesla T4 card as **1EB8** and **10DE** respectively. The corresponding values in decimal formats are **7864** and **4318** respectively.

### Create the cluster template

```shell
$ make dev-flavors
/Library/Developer/CommandLineTools/usr/bin/make generate-flavors FLAVOR_DIR=/Users/muchhals/.cluster-api/overrides/infrastructure-vsphere/v1.4.0
go run ./packaging/flavorgen -f vip > /Users/muchhals/.cluster-api/overrides/infrastructure-vsphere/v1.4.0/cluster-template.yaml
```

Edit the generated cluster template (`cluster-template.yaml`) to set the values for the PCI devices array. Here we are editing the VSphereMachineTemplate object for the worker nodes. This will create a worker node with a single Tesla T4 card attached to the VM.

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-worker
  namespace: ${NAMESPACE}
spec:
  template:
    spec:
      cloneMode: linkedClone
      datacenter: ${VSPHERE_DATACENTER}
      datastore: ${VSPHERE_DATASTORE}
      diskGiB: 25
      folder: ${VSPHERE_FOLDER}
      memoryMiB: 8192
      network:
        devices:
        - dhcp4: true
          networkName: ${VSPHERE_NETWORK}
      numCPUs: 2
      pciDevices:
      - deviceId: 7864                 <============ value from above
        vendorId: 4318                 <============ value from above
      resourcePool: ${VSPHERE_RESOURCE_POOL}
      server: ${VSPHERE_SERVER}
      storagePolicyName: ${VSPHERE_STORAGE_POLICY}
      template: ${VSPHERE_TEMPLATE}
      thumbprint: ${VSPHERE_TLS_THUMBPRINT}
```

Set the required values for the other fields and the cluster template is ready for use. The similar changes can be made to a template generated using clusterctl generate cluster command as well.

### Create the cluster

Set the size of the GPU nodes appropriately, since the Nvidia gpu-operator requires additional CPU and memory to install the device drivers on the VMs.

Apply the manifest from the previous step to your management cluster to have CAPV create a
workload cluster:

```shell
$ kubectl apply -f /tmp/vsp-cluster.yml
cluster.cluster.x-k8s.io/gpumaxpro unchanged
vspherecluster.infrastructure.cluster.x-k8s.io/gpumaxpro configured
vspheremachinetemplate.infrastructure.cluster.x-k8s.io/gpumaxpro created
vspheremachinetemplate.infrastructure.cluster.x-k8s.io/gpumaxpro-worker created
kubeadmcontrolplane.controlplane.cluster.x-k8s.io/gpumaxpro configured
kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io/gpumaxpro-md-0 created
machinedeployment.cluster.x-k8s.io/gpumaxpro-md-0 created
clusterresourceset.addons.cluster.x-k8s.io/gpumaxpro-crs-0 created
secret/gpumaxpro configured
secret/vsphere-csi-controller created
configmap/vsphere-csi-controller-role created
configmap/vsphere-csi-controller-binding created
secret/csi-vsphere-config created
configmap/csi.vsphere.vmware.com created
configmap/vsphere-csi-node created
configmap/vsphere-csi-controller created
secret/cloud-controller-manager created
secret/cloud-provider-vsphere-credentials created
configmap/cpi-manifests created
```

Wait until the cluster and nodes are finished provisioning.

```shell
$ kubectl get cluster gpumaxpro
NAME        PHASE
gpumaxpro   Provisioned
$ kubectl get machines
NAME                              CLUSTER     NODENAME                          PROVIDERID                                       PHASE      AGE   VERSION
gpumaxpro-jdf24                   gpumaxpro   gpumaxpro-jdf24                   vsphere://420f169d-d225-2781-8744-5bcdd313751c   Running   1d   v1.22.9
gpumaxpro-md-0-784fd759bb-62tjg   gpumaxpro   gpumaxpro-md-0-784fd759bb-62tjg   vsphere://420ffe2a-2fd7-61fe-4d78-24b1c497ebc7   Running   1d   v1.22.9
gpumaxpro-md-0-784fd759bb-dfqs7   gpumaxpro                                                                                      Running   1d   v1.22.9
```

Install a [CNI](https://cluster-api.sigs.k8s.io/user/quick-start.html#deploy-a-cni-solution) of your choice.
Once the nodes are `Ready`, run the following commands against the workload cluster to install the `gpu-operator`:

```shell
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia \
   && helm repo update
helm install --kubeconfig=./<workload-kubeconf>.conf  --wait --generate-name -n gpu-operator --create-namespace nvidia/gpu-operator
```

Then run the following commands against the workload cluster to verify that the
[NVIDIA GPU operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/getting-started.html#install-the-gpu-operator)
has initialized and the `gpu-operator` resource is available:

```shell
kubectl --kubeconfig=./<cluster-name>-kube.conf get pods -A
NAMESPACE         NAME                                                              READY   STATUS     RESTARTS   AGE
gpu-operator      gpu-feature-discovery-4p6rs                                       1/1     Running    0          10s
gpu-operator      gpu-operator-1656477030-node-feature-discovery-master-56457lp8r   1/1     Running    0          50s
gpu-operator      gpu-operator-1656477030-node-feature-discovery-worker-9g2cm       1/1     Running    0          50s
gpu-operator      gpu-operator-1656477030-node-feature-discovery-worker-l296w       1/1     Running    0          50s
gpu-operator      gpu-operator-6688b48999-zssxv                                     1/1     Running    0          50s
gpu-operator      nvidia-container-toolkit-daemonset-r6nzz                          1/1     Running    0          10s
gpu-operator      nvidia-dcgm-exporter-m2vt8                                        1/1     Running    0          10s
gpu-operator      nvidia-device-plugin-daemonset-tp6qx                              1/1     Running    0          10s
```

### Run a test app

Let's create a pod manifest for the `cuda-vector-add` example from the Kubernetes documentation and
deploy it:

```shell
$ cat > cuda-vector-add.yaml << EOF
apiVersion: v1
kind: Pod
metadata:
  name: cuda-vector-add
spec:
  restartPolicy: OnFailure
  containers:
    - name: cuda-vector-add
      # https://github.com/kubernetes/kubernetes/blob/v1.7.11/test/images/nvidia-cuda/Dockerfile
      image: "registry.k8s.io/cuda-vector-add:v0.1"
      resources:
        limits:
          nvidia.com/gpu: 1 # requesting 1 GPU
EOF
$ kubectl apply -f cuda-vector-add.yaml
```

The container will download, run, and perform a [CUDA](https://developer.nvidia.com/cuda-zone)
calculation with the GPU.

```shell
$ kubectl get po cuda-vector-add
cuda-vector-add   0/1     Completed   0          91s
$ kubectl logs cuda-vector-add
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

If you see output like the above, your GPU cluster is working!
