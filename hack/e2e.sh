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

set -o errexit  # exits immediately on any unexpected error (does not bypass traps)
set -o nounset  # will error if variables are used without first being defined
set -o pipefail # any non-zero exit code in a piped command causes the pipeline to fail with that code

export PATH=${PWD}/hack/tools/bin:${PATH}
REPO_ROOT=$(git rev-parse --show-toplevel)

# shellcheck source=./hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"

on_exit() {
  # release IPClaim
  echo "Releasing IP claim"
  kubectl --kubeconfig="${KUBECONFIG}" delete ipclaim "${IPCLAIM_NAME}" || true

  # kill the VPN
  docker kill vpn
}

trap on_exit EXIT

export VSPHERE_SERVER="${GOVC_URL}"
export VSPHERE_USERNAME="${GOVC_USERNAME}"
export VSPHERE_PASSWORD="${GOVC_PASSWORD}"
export VSPHERE_SSH_AUTHORIZED_KEY="${VM_SSH_PUB_KEY}"
export VSPHERE_SSH_PRIVATE_KEY="/root/ssh/.private-key/private-key"
export E2E_CONF_FILE="${REPO_ROOT}/test/e2e/config/vsphere-ci.yaml"
export ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"

export GC_KIND="false"

# Run the vpn client in container
docker run --rm -d --name vpn -v "${HOME}/.openvpn/:${HOME}/.openvpn/" \
  -w "${HOME}/.openvpn/" --cap-add=NET_ADMIN --net=host --device=/dev/net/tun \
  gcr.io/cluster-api-provider-vsphere/extra/openvpn:latest

# Tail the vpn logs
docker logs vpn

# Sleep to allow vpn container to start running
sleep 30

# Retrieve an IP to be used as the kube-vip IP
KUBECONFIG="/root/ipam-conf/capv-services.conf"
IPCLAIM_NAME="ip-claim-$(date +%s)"
sed "s/IPCLAIM_NAME/${IPCLAIM_NAME}/" "${REPO_ROOT}/hack/ipclaim-template.yaml" | kubectl --kubeconfig=${KUBECONFIG} create -f -

IPADDRESS_NAME=$(kubectl --kubeconfig=${KUBECONFIG} get ipclaim "${IPCLAIM_NAME}" -o=jsonpath='{@.status.address.name}')
CONTROL_PLANE_ENDPOINT_IP=$(kubectl --kubeconfig=${KUBECONFIG} get ipaddresses "${IPADDRESS_NAME}" -o=jsonpath='{@.spec.address}')
export CONTROL_PLANE_ENDPOINT_IP

echo "Acquired Control Plane IP: $CONTROL_PLANE_ENDPOINT_IP"

mkdir -p ${ARTIFACTS}/tempContainers
dockerImage=${ARTIFACTS}/tempContainers/image.tar
docker save gcr.io/k8s-staging-cluster-api/capv-manager:e2e -o dockerImage

# create bucket to store the docker image
BUCKET_NAME=capi-images-oci-images
IMAGE_SHA=$(docker inspect --format='{{index .Id}}' gcr.io/k8s-staging-cluster-api/capv-manager:e2e)
gsutil mb gs://$BUCKET_NAME
gsutil cp _artifacts/tempContainers/image.tar gs://$BUCKET_NAME/$IMAGE_SHA
rm -rf ${ARTIFACTS}/tempContainers

echo $IMAGE_SHA

# Run e2e tests
make e2e
