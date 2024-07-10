# GPU enabled clusters using vGPU

## Overview

You can choose to create a cluster with both worker and control plane nodes having vGPU devices attached to them.

Before we begin, a few important things to note:

- [NVIDIA GPU Operator](https://github.com/NVIDIA/gpu-operator) is used to expose the GPU PCI devices to the workloads running on the cluster.
- The OVA templates used for cluster creation should have the VMX version (Virtual Hardware) set to 17 or higher. This is necessary because Dynamic DirectPath I/O was introduced in this version, which enables the Assignable Hardware intelligence for passthrough devices.
- Since we need the VMX version to be >=17, this way of provisioning clusters with PCI passthrough devices works for vSphere 7.0 and above. This is the ESXi/VMX version [compatibility list](https://kb.vmware.com/s/article/2007240).
- UEFI boot mode is recommended for the OVAs used for cluster creation.
- Most of the setup is similar to [GPU enabled clusters via PCI Passthrough](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/docs/gpu-pci.md#create-the-cluster).

## An example GPU enabled cluster

Let's create a CAPV cluster with vGPU enabled nodes.

### Prerequisites

- Refer the [NVIDIA Virtual GPU Software Quick Start Guide](https://docs.nvidia.com/grid/latest/grid-software-quick-start-guide/index.html) to download and install the vGPU software and configure vGPU licensing.

- Ensure vGPU compatibility for your vSphere installation and the GPU devices using the [VMware Compatibility Guide - Shared Pass-through Graphics](https://www.vmware.com/resources/compatibility/search.php?deviceCategory=vgpu)

- Enable Shared Passthrough for the GPU device on the ESXi Host
  - Browse to a host in the vSphere Client navigator.
  - On the **Configure** tab, expand **Hardware** and click **Graphics**.
  - Under **GRAPHICS DEVICES**, select the GPU device to be used for vGPU, click **EDIT...** and select **Shared Direct**. Repeat this for additional GPU devices as needed.
  - Select **HOST GRAPHICS**, click **EDIT...** and select **Shared Direct** and select a shared passthrough GPU assignment policy, for example **Group VMs on GPU until full (GPU consolidation)**.

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

### Source the vGPU profile(s) for the GPU device

See "2. Choosing the vGPU Profile for the Virtual Machine" at [Using GPUs with Virtual Machines on vSphere](https://blogs.vmware.com/apps/2018/09/using-gpus-with-virtual-machines-on-vsphere-part-3-installing-the-nvidia-grid-technology.html) to see what vGPU profiles are available for your GPU device.

We are using NVIDIA Tesla V100 32GB cards for this example and will use the `grid_v100d-4c` vGPU profile for this card that allocates 4GB GPU memory to the worker node's vGPU device. 

### Create the cluster template

```shell
$ make release-flavors
```

Edit the generated Cluster template (e.g. `out/cluster-template.yaml`) to set the values for the `pciDevices` array. Here we are editing the VSphereMachineTemplate object for the worker nodes. This will create a worker node with a single NVIDIA 16GB vGPU device attached to the VM.

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-worker
  namespace: '${NAMESPACE}'
spec:
  template:
    spec:
      cloneMode: linkedClone
      datacenter: '${VSPHERE_DATACENTER}'
      datastore: '${VSPHERE_DATASTORE}'
      diskGiB: 25
      folder: '${VSPHERE_FOLDER}'
      memoryMiB: 8192
      network:
        devices:
        - dhcp4: true
          networkName: '${VSPHERE_NETWORK}'
      numCPUs: 2
      os: Linux
      powerOffMode: trySoft
      resourcePool: '${VSPHERE_RESOURCE_POOL}'
      server: '${VSPHERE_SERVER}'
      storagePolicyName: '${VSPHERE_STORAGE_POLICY}'
      template: '${VSPHERE_TEMPLATE}'
      thumbprint: '${VSPHERE_TLS_THUMBPRINT}'
      pciDevices:
        - vGPUProfile: "grid_t4-1a" # value from above
```

Set the required values for the other fields and the cluster template is ready for use.
The similar changes can be made to a template generated using `clusterctl generate cluster` command as well.

### Create the cluster

Set the size of the GPU nodes appropriately, since the Nvidia gpu-operator requires additional CPU and memory to install the device drivers on the VMs.

Note: For GPU nodes (PCI Passthrough or vGPU), all memory of the nodes must be reserved. CAPV will automatically do this for nodes that have a PCI Passthrough GPU or a vGPU device in the spec. See "Memory Reservation" at [Using GPUs with Virtual Machines on vSphere](https://blogs.vmware.com/apps/2018/09/using-gpus-with-virtual-machines-on-vsphere-part-2-vmdirectpath-i-o.html)

Apply the manifest from the previous step to your management cluster to have CAPV create a workload cluster with worker nodes that have vGPUs.

From this point on, the setup is exactly the same as [GPU enabled clusters via PCI Passthrough](./gpu-pci.md#create-the-cluster). 
