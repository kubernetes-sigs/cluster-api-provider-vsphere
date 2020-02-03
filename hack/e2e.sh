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

KIND_VERSION="v0.7.0"

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

# dump logs from kind and all the nodes
dump-logs() {
  # log version information
  echo "=== versions ==="
  echo "kind : $(kind version)" || true
  echo "bootstrap cluster:"
  kind get kubeconfig --name mgmt > bootstrap-kubeconfig
  kubectl --kubeconfig=bootstrap-kubeconfig version || true
  echo ""



  # dump all the info from the CAPI related CRDs
  mkdir -p "${ARTIFACTS}"/logs

  # dump images info
  {
     echo "images in docker"
     docker images
  } >> "${ARTIFACTS}/logs/images.info" || true
  {
     echo "images from bootstrap using containerd CLI"
     docker exec mgmt-control-plane ctr -n k8s.io images list
  } >> "${ARTIFACTS}/logs/images.info" || true
  {
     echo "images in bootstrap cluster using kubectl CLI"
     kubectl --kubeconfig="${PWD}"/bootstrap-kubeconfig get pods --all-namespaces -o json \
   | jq --raw-output '.items[].spec.containers[].image' | sort
  } >> "${ARTIFACTS}/logs/images.info" || true

  # dump cluster info for kind
  kubectl --kubeconfig="${PWD}"/bootstrap-kubeconfig cluster-info dump > "${ARTIFACTS}/logs/kind-cluster.info" || true


  # export all logs from kind
  kind "export" logs --name="mgmt" "${ARTIFACTS}/logs" || true

}

on_exit() {
   # dump the logs into the ARTIFACTS directory
   dump-logs

   # remove the kind cluster
   kind delete cluster --name mgmt

   # kill the VPN
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
export VSPHERE_MACHINE_TEMPLATE="centos-7-kube-v1.16.2"
export VSPHERE_HAPROXY_TEMPLATE="capv-haproxy-v0.5.3-77-g224e0ef6"

export CAPI_IMAGE="gcr.io/k8s-staging-cluster-api/cluster-api-controller:v20200103-v0.2.5-497-gdbe789259"
export CAPI_GIT_REF="09949bd397eecbfeac4e011b0d2c29fdbf2ac1ef"

export GC_KIND="false"

# Run the vpn client in container
docker run --rm -d --name vpn  -v "${HOME}/.openvpn/:${HOME}/.openvpn/" \
 -w "${HOME}/.openvpn/" --cap-add=NET_ADMIN --net=host --device=/dev/net/tun  \
 gcr.io/cluster-api-provider-vsphere/extra/openvpn:latest \

# Tail the vpn logs
docker logs vpn 

# Run e2e tests
make e2e
