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
    -e KUBERNETES_VERSION="1.16.2" \
    -e VSPHERE_TEMPLATE="centos-7-kube-v1.16.2" \
    -e SSH_AUTHORIZED_KEY="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDCbQg7ywSD1oJAwAHhQuemrL6C9wvOIgE7wfZ0PfqolcTEQbLbv7Zxe1/TRzr4B20pb6GMryJ7O3SH9kuCubDYQ4Atw9MF/iAtsg0s3Xs4a3RAqoeaTHA0u401Um27ANDVqLswdTZ0J0Ev+XqRCEgPX+IpGgiNOyiHxfIgwdev/fG1MmMEyCKj8JNlRghFnleBcE+N3Mu0rKb88ascch2mKLY5fGDwbnC3V7d6LE6jWVT5HV391N4IWmjoBjlBt3mfzNslWqJUS+PxRbYR3i7vyVrpb/mkw1YG1jeomTAmkx4kwiV7hSzNVF6pKNIOoB1mpwULJ0VL+UkM8IPEfVJb root@9ae81f510a14" \
    "${MANIFESTS_IMAGE}" \
    -c "${CLUSTER_NAME}" \
    -m "${VSPHERE_CONTROLLER_VERSION}" \
    -f

