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

on_exit() {
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

# Run e2e tests
make e2e
