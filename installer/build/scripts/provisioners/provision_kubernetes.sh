#!/usr/bin/bash
set -euf -o pipefail

echo "Kubernetes version being provisioned: $KUBERNETES_VERSION"
# TODO(frapposelli): parametrize CNI / CRICTL version
CNI_VERSION="v0.6.0"
CRICTL_VERSION="v1.11.1"

mkdir -p /opt/cni/bin
mkdir -p /opt/bin
curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-amd64-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz
curl -L "https://github.com/kubernetes-incubator/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-amd64.tar.gz" | tar -C /opt/bin -xz

curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${KUBERNETES_VERSION}/bin/linux/amd64/{kubeadm,kubelet,kubectl}
chmod +x {kubeadm,kubelet,kubectl}
mv {kubeadm,kubelet,kubectl} /opt/bin
curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/kubelet.service" | sed "s:/usr/bin:/opt/bin:g" > /etc/systemd/system/kubelet.service
mkdir -p /etc/systemd/system/kubelet.service.d
curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}/build/debs/10-kubeadm.conf" | sed "s:/usr/bin:/opt/bin:g" > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
