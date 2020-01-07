#!/bin/bash

# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -o errexit   # exits immediately on any unexpected error (does not bypass traps)
set -o nounset   # will error if variables are used without first being defined
set -o pipefail  # any non-zero exit code in a piped command causes the pipeline to fail with that code

KIND_VERSION="v0.5.1"

install_govc() {
   GOVC_VERSION=v0.21.0
   GOVC_PKG_NAME=govc_linux_amd64
   curl -L -O https://github.com/vmware/govmomi/releases/download/"${GOVC_VERSION}"/"${GOVC_PKG_NAME}".gz
   gunzip "${GOVC_PKG_NAME}".gz
   mv "${GOVC_PKG_NAME}" /usr/local/bin/govc
   chmod +x /usr/local/bin/govc
}

install_kind() {
   wget "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64" \
      --no-verbose -O /usr/local/bin/kind
   chmod +x /usr/local/bin/kind
}

install_ginkgo() {
   GO111MODULE="on" go get github.com/onsi/ginkgo/ginkgo@v1.11.0
}

install_kustomize() {
 curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash
 mv kustomize /usr/local/bin/
}

on_exit() {
   docker kill vpn
}

trap on_exit EXIT

install_govc

install_kind

install_ginkgo

install_kustomize

export VSPHERE_SERVER="${GOVC_URL}"
export VSPHERE_USERNAME="${GOVC_USERNAME}"
export VSPHERE_PASSWORD="${GOVC_PASSWORD}"
export VSPHERE_DATACENTER="SDDC-Datacenter"
export VSPHERE_FOLDER="clusterapi"
export VSPHERE_RESOURCE_POOL="clusterapi"
export VSPHERE_DATASTORE="WorkloadDatastore"
export VSPHERE_NETWORK="sddc-cgw-network-5"
export VSPHERE_MACHINE_TEMPLATE="ubuntu-1804-kube-v1.16.2"
export VSPHERE_HAPROXY_TEMPLATE="capv-haproxy-v0.5.3-77-g224e0ef6"

export CAPI_IMAGE="gcr.io/k8s-staging-cluster-api/cluster-api-controller:v20200103-v0.2.5-497-gdbe789259"
export CAPI_GIT_REF="09949bd397eecbfeac4e011b0d2c29fdbf2ac1ef"

# Run the vpn client in container
docker run --rm -d --name vpn  -v "${HOME}/.openvpn/:${HOME}/.openvpn/" \
 -w "${HOME}/.openvpn/" --cap-add=NET_ADMIN --net=host --device=/dev/net/tun  \
 gcr.io/cluster-api-provider-vsphere/extra/openvpn:latest \

# Tail the vpn logs
docker logs vpn 

# Run e2e tests
make e2e
