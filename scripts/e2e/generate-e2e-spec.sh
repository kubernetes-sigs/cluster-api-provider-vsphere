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

set -o errexit   # exits immediately on any unexpected error (does not bypass traps)
set -o nounset   # will error if variables are used without first being defined
set -o pipefail  # any non-zero exit code in a piped command causes the pipeline to fail with that code

echo "Creating e2e spec files..."

MANIFESTS_IMAGE="gcr.io/cluster-api-provider-vsphere/pr/manifests:${VERSION}"

echo "Using image: ${MANIFESTS_IMAGE}"

# generate the spec files using the "manifests" docker image
docker run --rm \
    -v "$(pwd)":/out \
    -e VSPHERE_SERVER="${GOVC_URL}" \
    -e VSPHERE_USERNAME="${GOVC_USERNAME}" \
    -e VSPHERE_PASSWORD="${GOVC_PASSWORD}" \
    -e VSPHERE_DATACENTER="SDDC-Datacenter" \
    -e VSPHERE_DATASTORE="WorkloadDatastore" \
    -e VSPHERE_NETWORK="sddc-cgw-network-5" \
    -e VSPHERE_RESOURCE_POOL="clusterapi" \
    -e VSPHERE_FOLDER="clusterapi" \
    -e VSPHERE_TEMPLATE="ubuntu-1804-kube-v1.14.8" \
    -e SSH_AUTHORIZED_KEY="N/A" \
    "${MANIFESTS_IMAGE}" \
    -c "${CLUSTER_NAME}" \
    -m "${VSPHERE_CONTROLLER_VERSION}" \
    -f

