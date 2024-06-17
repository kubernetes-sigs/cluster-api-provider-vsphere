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
  # kill the VPN
  docker kill vpn
}

trap on_exit EXIT

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
    curl "https://${GOVC_URL}" --connect-timeout 2 -k && RET=$? || RET=$?
    if [[ "$RET" -eq 0 ]]; then
      break
    fi
    n=$((n + 1))
    sleep 1
  done
  return "$RET"
}
wait_for_vpn_up

# Run e2e tests
make clean-ci
