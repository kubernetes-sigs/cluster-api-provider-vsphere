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

# In CI, ARTIFACTS is set to a different directory. This stores the value of
# ARTIFACTS i1n ORIGINAL_ARTIFACTS and replaces ARTIFACTS by a temporary directory
# which gets cleaned up from credentials at the end of the test.
export ORIGINAL_ARTIFACTS=""
export ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"
if [[ "${ARTIFACTS}" != "${REPO_ROOT}/_artifacts" ]]; then
  ORIGINAL_ARTIFACTS="${ARTIFACTS}"
  ARTIFACTS=$(mktemp -d)
fi

# shellcheck source=./hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"

on_exit() {
  # kill the VPN
  docker kill vpn

  # logout of gcloud
  if [ "${AUTH}" ]; then
    gcloud auth revoke
  fi

  # Cleanup VSPHERE_PASSWORD from temporary artifacts directory.
  if [[ "${ORIGINAL_ARTIFACTS}" != "" ]]; then
    grep -r -l -e "${VSPHERE_PASSWORD}" "${ARTIFACTS}" | while IFS= read -r file
    do
      echo "Cleaning up VSPHERE_PASSWORD from file ${file}"
      sed -i "s/${VSPHERE_PASSWORD}/REDACTED/g" "${file}"
    done
    # Move all artifacts to the original artifacts location.
    mv "${ARTIFACTS}"/* "${ORIGINAL_ARTIFACTS}/"
  fi
}

trap on_exit EXIT

function login() {
  # If GCR_KEY_FILE is set, use that service account to login
  if [ "${GCR_KEY_FILE}" ]; then
    gcloud auth activate-service-account --key-file "${GCR_KEY_FILE}" || fatal "unable to login"
    AUTH=1
  fi
}

AUTH=
E2E_IMAGE_SHA=
GCR_KEY_FILE="${GCR_KEY_FILE:-}"
export VSPHERE_SERVER="${GOVC_URL}"
export VSPHERE_USERNAME="${GOVC_USERNAME}"
export VSPHERE_PASSWORD="${GOVC_PASSWORD}"
export VSPHERE_SSH_AUTHORIZED_KEY="${VM_SSH_PUB_KEY}"
export VSPHERE_SSH_PRIVATE_KEY="/root/ssh/.private-key/private-key"
export E2E_CONF_FILE="${REPO_ROOT}/test/e2e/config/vsphere.yaml"
export E2E_CONF_OVERRIDE_FILE=""
export E2E_CAPV_MODE="${CAPV_MODE:-govmomi}"
export E2E_TARGET_TYPE="${TARGET_TYPE:-vmc}"
export ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"
export DOCKER_IMAGE_TAR="/tmp/images/image.tar"
export GC_KIND="false"

# Make tests run in-parallel
export GINKGO_NODES=5
# Set the kubeconfig to the IPAM cluster so the e2e tests can claim ip addresses
# for kube-vip.
export E2E_IPAM_KUBECONFIG="/root/ipam-conf/capv-services.conf"

# Run the vpn client in container
docker run --rm -d --name vpn -v "${HOME}/.openvpn/:${HOME}/.openvpn/" \
  -w "${HOME}/.openvpn/" --cap-add=NET_ADMIN --net=host --device=/dev/net/tun \
  gcr.io/k8s-staging-capi-vsphere/extra/openvpn:latest

# Tail the vpn logs
docker logs vpn

# Wait until the VPN connection is active and we are able to reach the ipam cluster
function wait_for_ipam_reachable() {
  local n=0
  until [ $n -ge 30 ]; do
    kubectl --kubeconfig="${E2E_IPAM_KUBECONFIG}" --request-timeout=2s  get inclusterippools.ipam.cluster.x-k8s.io && RET=$? || RET=$?
    if [[ "$RET" -eq 0 ]]; then
      break
    fi
    n=$((n + 1))
    sleep 1
  done
  return "$RET"
}
wait_for_ipam_reachable

make envsubst

ARCH="$(go env GOARCH)"

# # Only build and upload the image if we run tests which require it to save some $.
# if [[ -z "${GINKGO_FOCUS+x}" ]]; then
#   # Save the docker image locally
#   make e2e-images
#   mkdir -p /tmp/images
#   docker save "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-controller-${ARCH}:dev" -o "$DOCKER_IMAGE_TAR"

#   # Store the image on gcs
#   login
#   E2E_IMAGE_SHA=$(docker inspect --format='{{index .Id}}' "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-controller-${ARCH}:dev")
#   export E2E_IMAGE_SHA
#   gsutil cp ${DOCKER_IMAGE_TAR} gs://capv-ci/"$E2E_IMAGE_SHA"
# fi

# Run e2e tests
make e2e
