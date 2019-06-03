# Building Base Images

This directory contains tooling for building base images for use as nodes in Kubernetes Clusters. [Packer](https://www.packer.io) is used for building these images. This tooling has been forked and extended from the [Wardroom](https://github.com/heptiolabs/wardroom) project.

## Prerequisites

### Hypervisor

The images may be built using one of the following hypervisors:

| OS | Builder |
|----|---------|
| Linux | VMware Workstation |
| macOS | VMware Fusion |

The `vmware-iso` builder supports building against a remote VMware ESX server, but is untested with this project.

### Tools

- [Packer](https://www.packer.io/intro/getting-started/install.html)
- [Ansible](http://docs.ansible.com/ansible/latest/intro_installation.html) version >= 2.8.0
- [goss](https://github.com/YaleUniversity/packer-provisioner-goss)

The program `../../hack/image-tools.sh` can be used to download and install the goss plug-in.

## Building Images

### Configuration

The `config` directory includes several JSON files that define the configuration for the images:

| File | Description |
|------|-------------|
| `config/kubernetes.json` | The version of Kubernetes to install |
| `config/centos-7.json` | The settings for the CentOS 7 image |
| `config/ubuntu-1804.json` | The settings for the Ubuntu 1804 image |

### Limiting Images to Build

To see a list of which images may be built, use Make's tab completion:

```shell
$ make build<tab><tab>
build              build-centos-7     build-ubuntu-1804
```

### Building the Images

To build the Ubuntu and CentOS images:

```shell
$ make
```

The images are built and located in `packer/output/BUILD_NAME+kube-KUBERNETES_VERSION`

## Uploading Images

The images are uploaded to the GCS bucket `capv-images`. The path to the image depends on the version of Kubernetes:

| Build type | Upload location |
|------------|-----------------|
| CI | `gs://capv-images/ci/KUBERNETES_VERSION/BUILD_NAME+kube-KUBERNETES_VERSION.ova` |
| Release | `gs://capv-images/release/KUBERNETES_VERSION/BUILD_NAME+kube-KUBERNETES_VERSION.ova` |

Uploading the images requires the `gcloud` and `gsutil` programs, an active Google Cloud account, or a service account with an associated key file. The latter may be specified via the environment variable `KEY_FILE`.

```shell
$ ../../hack/image-upload.py --key-file KEY_FILE BUILD_DIR 
```

First the images are checksummed (SHA256). If a matching checksum already exists remotely then the image is not re-uploaded. Otherwise the images are uploaded to the GCS bucket.

### Listing Available Images

Once uploaded the available images may be listed using the `gsutil` program, for example:

```shell
$ gsutil ls gs://capv-images/release
```

### Downloading Images

Images may be downloaded via HTTP:

| Build type | Download location |
|------------|-----------------|
| CI | `http://storage.googleapis.com/capv-images/ci/KUBERNETES_VERSION/BUILD_NAME+kube-KUBERNETES_VERSION.ova` |
| Release | `http://storage.googleapis.com/capv-images/release/KUBERNETES_VERSION/BUILD_NAME+kube-KUBERNETES_VERSION.ova` |

## Testing Images

### Accessing a locked-down image

Cloud-init restricts access to the images upon boot. Only SSH access via public key is allowed. To gain access to an image, perform the following steps using either VMware Workstation or VMware Fusion:

1. Create a virtual machine from the image
2. Run `make -C cloudinit` to generate `cloudinit/cidata.iso`
3. Connect `cloudinit/cidata.iso` to the virtual machine with the image being tested
4. Boot the image
5. Cloud-init will find the mounted ISO with user-data, and copy the provided SSH keys to the image's default user
6. SSH into the image with `../../hack/image-ssh.sh BUILD_DIR`

### Initialize a CNI

As root:

(copied from [containernetworking/cni](https://github.com/containernetworking/cni#how-do-i-use-cni))

```shell
mkdir -p /etc/cni/net.d
curl -LO https://github.com/containernetworking/plugins/releases/download/v0.7.0/cni-plugins-amd64-v0.7.0.tgz
tar -xzf cni-plugins-amd64-v0.7.0.tgz --directory /etc/cni/net.d
cat >/etc/cni/net.d/10-mynet.conf <<EOF
{
    "cniVersion": "0.2.0",
    "name": "mynet",
    "type": "bridge",
    "bridge": "cni0",
    "isGateway": true,
    "ipMasq": true,
    "ipam": {
        "type": "host-local",
        "subnet": "10.22.0.0/16",
        "routes": [
            { "dst": "0.0.0.0/0" }
        ]
    }
}
EOF
cat >/etc/cni/net.d/99-loopback.conf <<EOF
{
    "cniVersion": "0.2.0",
    "name": "lo",
    "type": "loopback"
}
EOF
```

### Run the e2e node conformance tests

As a non-root user:

```shell
curl -LO https://dl.k8s.io/$(</etc/kubernetes_community_ami_version)/kubernetes-test.tar.gz
tar -zxvf kubernetes-test.tar.gz kubernetes/platforms/linux/amd64
cd kubernetes/platforms/linux/amd64
sudo ./ginkgo --nodes=8 --flakeAttempts=2 --focus="\[Conformance\]" --skip="\[Flaky\]|\[Serial\]|\[sig-network\]|Container Lifecycle Hook" ./e2e_node.test -- --k8s-bin-dir=/usr/bin --container-runtime=remote --container-runtime-endpoint unix:///var/run/containerd/containerd.sock --container-runtime-process-name /usr/local/bin/containerd --container-runtime-pid-file= --kubelet-flags="--cgroups-per-qos=true --cgroup-root=/ --runtime-cgroups=/system.slice/containerd.service" --extra-log="{\"name\": \"containerd.log\", \"journalctl\": [\"-u\", \"containerd\"]}"
```
