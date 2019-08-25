#!/bin/bash

# Copyright 2018 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

# Change directories to the parent directory of the one in which this
# script is located.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

GOHOSTOS="${GOHOSTOS:-$(go env GOHOSTOS)}"
GOHOSTARCH="${GOHOSTARCH:-$(go env GOHOSTARCH)}"
KUBEBUILDER_VERSION="${KUBEBUILDER_VERSION:-2.0.0}"
KUBEBUILDER_HOME="/tmp/kubebuilder_${KUBEBUILDER_VERSION}_${GOHOSTOS}_${GOHOSTARCH}" 
KUBEBUILDER_URL="https://go.kubebuilder.io/dl/${KUBEBUILDER_VERSION}/${GOHOSTOS}/${GOHOSTARCH}"

# Download kubebuilder and extract it to tmp.
echo "downloading ${KUBEBUILDER_URL} to ${KUBEBUILDER_HOME} ..."
curl -sSL "${KUBEBUILDER_URL}" | tar -xz -C /tmp/

# Export the env vars for kubebuilder.
echo "exporting kubebuilder environment ..."
export PATH="${PATH}:${KUBEBUILDER_HOME}/bin"
export TEST_ASSET_KUBECTL="${KUBEBUILDER_HOME}/bin/kubectl"
export TEST_ASSET_KUBE_APISERVER="${KUBEBUILDER_HOME}/bin/kube-apiserver"
export TEST_ASSET_ETCD="${KUBEBUILDER_HOME}/bin/etcd"
export KUBEBUILDER_CONTROLPLANE_START_TIMEOUT=30s

# Run the tests.
echo "running tests ..."
make test
