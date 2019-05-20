# Cluster API Provider vSphere, OVA Installer Build process

The build process uses a collection of bash scripts to launch a docker container on your local machine
where we provision a linux OS, install dependencies, and extract the filesystem to make the OVA.

- [Cluster API Provider vSphere, OVA Installer Build process](#cluster-api-provider-vsphere-ova-installer-build-process)
  - [Usage](#usage)
    - [Prerequisites](#prerequisites)
    - [Build bundle and OVA](#build-bundle-and-ova)
      - [Build script](#build-script)
  - [Deploy](#deploy)
  - [Using the bootstrap cluster](#using-the-bootstrap-cluster)

## Usage

The build process is controlled from a central script, `build.sh`. This script
launches the build docker container and controls our provisioning and ova
extraction through `make`.

### Prerequisites

The build machine must have `docker`.

- `docker` for Mac: https://www.docker.com/docker-mac
- `docker` for Windows: https://www.docker.com/docker-windows

### Build bundle and OVA

#### Build script

The build script pulls the desired versions of each included component into the build container.

> *Note:* You must specify build step `ova-dev` when calling `build.sh` from a development machine.

If called without any values, `build.sh` will get Kubernetes version: `stable-1`
```
./build/build.sh ova-dev
```

If called with the values below, `build.sh` will include Kubernetes version v1.12.1
```
./build/build.sh ova-dev --kubernetes-version v1.12.1
```

The `build.sh` script can also prefill default values for root password and root ssh key, this is used by our CI when deploying the OVA on VMC for testing
```
./build/build.sh ova-dev --ci-root-password '<my password>' --ci-root-ssh-key 'ssh-rsa <my rsa key> <my comment>'
```

## Deploy

The OVA must be deployed to a vCenter.
Deploying to ESX host is not tested at the moment.

The recommended method for deploying the OVA:
- Access the vCenter Web UI, click `vCenter Web Client` (HTML5 or Flash)
- Right click on the desired cluster or resource pool
- Click `Deploy OVF Template`
- Select the URL or file and follow the prompts
- Power on the deployed VM
- Open the VM Console and wait for the Kubernetes cluster status to be: RUNNING

Alternative, deploying with `govc` (this also requires [`jq`](https://stedolan.github.io/jq/)):

First create a spec to populate the OVA fields during deployment:
```console
$ govc import.spec bin/cluster-api-xxxxxx.ova | jq -r . > cluster-api-spec.json
```

Now edit the file and populate the properties (Root password and SSH key at minimum)
```console
$ vi cluster-api-spec.json
```

Now you can import the OVA on your vSphere infrastructure
```console
$ export GOVC_URL='<username>:<password>@<vcenter.fqdn>'
$ export GOVC_INSECURE=1 # This is needed if your vSphere certificate is not valid or self signed
$ govc import.ova --options=cluster-api-spec.json cluster-api-xxxxxx.ova
```

And power it on and open the VM Console.
```console
$ govc vm.power -on cluster-api
$ govc vm.console cluster-api
```

## Using the bootstrap cluster

Once the cluster is in RUNNING state, you can use the `scp` command to retrieve the kubeconfig file from the Cluster API VM, this kubeconfig will be passed to clusterctl to launch the Cluster API provisioner.

The Kubeconfig file inside the deployed VM lives at `/etc/kubernetes/admin.conf`.
