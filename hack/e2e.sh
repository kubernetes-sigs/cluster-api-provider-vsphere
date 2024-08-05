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

# shellcheck source=./hack/ci-e2e-lib.sh
source "${REPO_ROOT}/hack/ci-e2e-lib.sh"

RE_VCSIM='\[vcsim\\]'

# In CI, ARTIFACTS is set to a different directory. This stores the value of
# ARTIFACTS in ORIGINAL_ARTIFACTS and replaces ARTIFACTS by a temporary directory
# which gets cleaned up from credentials at the end of the test.
export ORIGINAL_ARTIFACTS=""
export ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"
if [[ "${ARTIFACTS}" != "${REPO_ROOT}/_artifacts" ]]; then
  ORIGINAL_ARTIFACTS="${ARTIFACTS}"
  ARTIFACTS=$(mktemp -d)
fi

# shellcheck source=./hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"

export BOSKOS_RESOURCE_OWNER=cluster-api-provider-vsphere
if [[ "${JOB_NAME}" != "" ]]; then
  export BOSKOS_RESOURCE_OWNER="${JOB_NAME}/${BUILD_ID}"
fi
export BOSKOS_RESOURCE_TYPE=vsphere-project-cluster-api-provider

on_exit() {
  # Only handle Boskos when we have to (not for vcsim)
  if [[ ! "${GINKGO_FOCUS:-}" =~ $RE_VCSIM ]]; then
    # Stop boskos heartbeat
    [[ -z ${HEART_BEAT_PID:-} ]] || kill -9 "${HEART_BEAT_PID}"

    # If Boskos is being used then release the vsphere project.
    [ -z "${BOSKOS_HOST:-}" ] || docker run -e VSPHERE_USERNAME -e VSPHERE_PASSWORD gcr.io/k8s-staging-capi-vsphere/extra/boskosctl:latest release --boskos-host="${BOSKOS_HOST}" --resource-owner="${BOSKOS_RESOURCE_OWNER}" --resource-name="${BOSKOS_RESOURCE_NAME}" --vsphere-server="${VSPHERE_SERVER}" --vsphere-tls-thumbprint="${VSPHERE_TLS_THUMBPRINT}" --vsphere-folder="${BOSKOS_RESOURCE_FOLDER}" --vsphere-resource-pool="${BOSKOS_RESOURCE_POOL}"
  fi

  # kill the VPN only when we started it (not vcsim)
  if [[ ! "${GINKGO_FOCUS:-}" =~ $RE_VCSIM ]]; then
    docker kill vpn
  fi

  # Cleanup VSPHERE_PASSWORD from temporary artifacts directory.
  if [[ "${ORIGINAL_ARTIFACTS}" != "" ]]; then
    # Delete non-text files from artifacts directory to not leak files accidentially
    find "${ARTIFACTS}" -type f -exec file --mime-type {} \; | grep -v -E -e "text/plain|text/xml|application/json|inode/x-empty" | while IFS= read -r line
    do
      file="$(echo "${line}" | cut -d ':' -f1)"
      mimetype="$(echo "${line}" | cut -d ':' -f2)"
      echo "Deleting file ${file} of type ${mimetype}"
      rm "${file}"
    done || true
    # Replace secret and base64 secret in all files.
    if [ -n "$VSPHERE_PASSWORD" ]; then
      grep -I -r -l -e "${VSPHERE_PASSWORD}" "${ARTIFACTS}" | while IFS= read -r file
      do
        echo "Cleaning up VSPHERE_PASSWORD from file ${file}"
        sed -i "s/${VSPHERE_PASSWORD}/REDACTED/g" "${file}"
      done || true
      VSPHERE_PASSWORD_B64=$(echo -n "${VSPHERE_PASSWORD}" | base64 --wrap=0)
      grep -I -r -l -e "${VSPHERE_PASSWORD_B64}" "${ARTIFACTS}" | while IFS= read -r file
      do
        echo "Cleaning up VSPHERE_PASSWORD_B64 from file ${file}"
        sed -i "s/${VSPHERE_PASSWORD_B64}/REDACTED/g" "${file}"
      done || true
    fi
    # Move all artifacts to the original artifacts location.
    mv "${ARTIFACTS}"/* "${ORIGINAL_ARTIFACTS}/"
  fi
}

trap on_exit EXIT

# NOTE: when running on CI without presets, value for variables are missing: GOVC_URL, GOVC_USERNAME, GOVC_PASSWORD, VM_SSH_PUB_KEY),
#  but this is not an issue when we are targeting vcsim (corresponding VSPHERE_ variables will be injected during test setup).
export VSPHERE_SERVER="${GOVC_URL:-}"
export VSPHERE_USERNAME="${GOVC_USERNAME:-}"
export VSPHERE_PASSWORD="${GOVC_PASSWORD:-}"
export VSPHERE_SSH_AUTHORIZED_KEY="${VM_SSH_PUB_KEY:-}"
export VSPHERE_SSH_PRIVATE_KEY="/root/ssh/.private-key/private-key"
export E2E_CONF_FILE="${REPO_ROOT}/test/e2e/config/vsphere.yaml"
export E2E_CONF_OVERRIDE_FILE=""
export E2E_VM_OPERATOR_VERSION="${VM_OPERATOR_VERSION:-v1.8.6-0-gde75746a}"
export DOCKER_IMAGE_TAR="/tmp/images/image.tar"
export GC_KIND="false"

# Make tests run in-parallel
export GINKGO_NODES=5

# Only run the vpn/check for IPAM when we need them (not for vcsim)
if [[ ! "${GINKGO_FOCUS:-}" =~ $RE_VCSIM ]]; then
  # Run the vpn client in container
  docker run --rm -d --name vpn -v "${HOME}/.openvpn/:${HOME}/.openvpn/" \
    -w "${HOME}/.openvpn/" --cap-add=NET_ADMIN --net=host --device=/dev/net/tun \
    gcr.io/k8s-staging-capi-vsphere/extra/openvpn:latest

  # Tail the vpn logs
  docker logs vpn

  # Wait until the VPN connection is active.
  function wait_for_vpn_up() {
    local n=0
    until [ $n -ge 30 ]; do
      curl "https://${VSPHERE_SERVER}" --connect-timeout 2 -k && RET=$? || RET=$?
      if [[ "$RET" -eq 0 ]]; then
        break
      fi
      n=$((n + 1))
      sleep 1
    done
    return "$RET"
  }
  wait_for_vpn_up

  # If BOSKOS_HOST is set then acquire a vsphere-project from Boskos.
  if [ -n "${BOSKOS_HOST:-}" ]; then
    # Check out the account from Boskos and store the produced environment
    # variables in a temporary file.
    account_env_var_file="$(mktemp)"
    docker run gcr.io/k8s-staging-capi-vsphere/extra/boskosctl:latest acquire --boskos-host="${BOSKOS_HOST}" --resource-owner="${BOSKOS_RESOURCE_OWNER}" --resource-type="${BOSKOS_RESOURCE_TYPE}" 1>"${account_env_var_file}"
    checkout_account_status="${?}"

    # If the checkout process was a success then load the account's
    # environment variables into this process.
    # shellcheck disable=SC1090
    [ "${checkout_account_status}" = "0" ] && . "${account_env_var_file}"
    export BOSKOS_RESOURCE_NAME=${BOSKOS_RESOURCE_NAME}
    export VSPHERE_FOLDER=${BOSKOS_RESOURCE_FOLDER}
    export VSPHERE_RESOURCE_POOL=${BOSKOS_RESOURCE_POOL}
    export E2E_VSPHERE_IP_POOL="${BOSKOS_RESOURCE_IP_POOL}"

    # Always remove the account environment variable file. It contains
    # sensitive information.
    rm -f "${account_env_var_file}"

    if [ ! "${checkout_account_status}" = "0" ]; then
      echo "error getting vsphere project from Boskos" 1>&2
      exit "${checkout_account_status}"
    fi

    # Run the heartbeat to tell boskos periodically that we are still
    # using the checked out account.
    docker run gcr.io/k8s-staging-capi-vsphere/extra/boskosctl:latest heartbeat --boskos-host="${BOSKOS_HOST}" --resource-owner="${BOSKOS_RESOURCE_OWNER}" --resource-name="${BOSKOS_RESOURCE_NAME}" >>"${ARTIFACTS}/boskos-heartbeat.log" 2>&1 &
    HEART_BEAT_PID=$!
  fi
fi

make envsubst kind

# Prepare kindest/node images for all the required Kubernetes version; this implies
# 1. Kubernetes version labels (e.g. latest) to the corresponding version numbers.
# 2. Pre-pulling the corresponding kindest/node image if available; if not, building the image locally.
# Following variables are currently checked (if defined):
# - KUBERNETES_VERSION_MANAGEMENT
k8s::prepareKindestImagesVariables
k8s::prepareKindestImages

# Only pre-pull vm-operator image where running in supervisor mode.
if [[ ${GINKGO_FOCUS:-} =~ \\\[supervisor\\\] ]]; then
   kind::prepullImage "gcr.io/k8s-staging-capi-vsphere/extra/vm-operator:${E2E_VM_OPERATOR_VERSION}"
fi

ARCH="$(go env GOARCH)"

# Only build and upload the image if we run tests which require it to save some $.
# NOTE: the image is required for clusterctl upgrade tests, and those test are run only as part of the main e2e test job (without any focus)
if [[ -z "${GINKGO_FOCUS+x}" ]]; then
  # Save the docker images locally
  make e2e-images
  mkdir -p /tmp/images
  if [[ ${GINKGO_FOCUS:-} =~ \\\[supervisor\\\] ]]; then
    docker save \
      "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-controller-${ARCH}:dev" \
      "gcr.io/k8s-staging-capi-vsphere/cluster-api-net-operator-${ARCH}:dev" \
      > ${DOCKER_IMAGE_TAR}
  else
    docker save \
      "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-controller-${ARCH}:dev" \
      > ${DOCKER_IMAGE_TAR}
  fi
fi

# Run e2e tests
make e2e
