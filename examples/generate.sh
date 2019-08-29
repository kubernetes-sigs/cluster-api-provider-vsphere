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
cd "${WORKDIR:-$(dirname "${BASH_SOURCE[0]}")/..}"
BUILDDIR="${BUILDDIR:-.}"

OUT_DIR="${OUT_DIR:-}"
SRC_DIR="${BUILDDIR}"/examples

OVERWRITE=
CLUSTER_NAME="${CLUSTER_NAME:-capv-mgmt-example}"
ENV_VAR_REQ=':?required'
CAPV_MANAGER_IMAGE="${CAPV_MANAGER_IMAGE:-gcr.io/cluster-api-provider-vsphere/ci/manager:latest}"
CABPK_MANAGER_IMAGE="${CABPK_MANAGER_IMAGE:-gcr.io/kubernetes1-226021/cluster-api-bootstrap-provider-kubeadm:dev}"
export CAPV_MANAGER_IMAGE CABPK_MANAGER_IMAGE

usage() {
  cat <<EOF
usage: ${0} [FLAGS]
  Generates input manifests for the Cluster API Provider for vSphere (CAPV)

FLAGS
  -b    bootstrapper image (default "${CABPK_MANAGER_IMAGE}")
  -c    cluster name (default "${CLUSTER_NAME}")
  -d    disables required environment variables
  -f    force overwrite of existing files
  -h    prints this help screen
  -i    input directory (default ${SRC_DIR})
  -m    manager image (default "${CAPV_MANAGER_IMAGE}")
  -o    output directory (default ${OUT_DIR})
EOF
}

while getopts ':b:c:dfhi:m:o:' opt; do
  case "${opt}" in
  b)
    CABPK_MANAGER_IMAGE="${OPTARG}"
    ;;
  c)
    CLUSTER_NAME="${OPTARG}"
    ;;
  d)
    ENV_VAR_REQ=':-'
    ;;
  f)
    OVERWRITE=1
    ;;
  h)
    usage 1>&2; exit 1
    ;;
  i)
    SRC_DIR="${OPTARG}"
    ;;
  m)
    CAPV_MANAGER_IMAGE="${OPTARG}"
    ;;
  o)
    OUT_DIR="${OPTARG}"
    ;;
  \?)
    { echo "invalid option: -${OPTARG}"; usage; } 1>&2; exit 1
    ;;
  :)
    echo "option -${OPTARG} requires an argument" 1>&2; exit 1
    ;;
  esac
done
shift $((OPTIND-1))

[ -n "${OUT_DIR}" ] || OUT_DIR="./out/${CLUSTER_NAME}"
mkdir -p "${OUT_DIR}"

# Load an envvars.txt file if one is found.
# shellcheck disable=SC1091
[ "${DOCKER_ENABLED-}" ] && [ -e "/envvars.txt" ] && source "/envvars.txt"

# Outputs
COMPONENTS_CLUSTER_API_GENERATED_FILE=${SRC_DIR}/provider-components/provider-components-cluster-api.yaml
COMPONENTS_KUBEADM_GENERATED_FILE=${SRC_DIR}/provider-components/provider-components-kubeadm.yaml
COMPONENTS_VSPHERE_GENERATED_FILE=${SRC_DIR}/provider-components/provider-components-vsphere.yaml

ADDONS_GENERATED_FILE=${OUT_DIR}/addons.yaml
PROVIDER_COMPONENTS_GENERATED_FILE=${OUT_DIR}/provider-components.yaml
CLUSTER_GENERATED_FILE=${OUT_DIR}/cluster.yaml
CONTROLPLANE_GENERATED_FILE=${OUT_DIR}/controlplane.yaml
MACHINEDEPLOYMENT_GENERATED_FILE=${OUT_DIR}/machinedeployment.yaml

ok_file() {
  [ -f "${1}" ] || { echo "${1} is missing" 1>&2; exit 1; }
}

no_file() {
  [ ! -f "${1}" ] || { echo "${1} already exists, overwrite with -f" 1>&2; exit 1; }
}

# Remove the temporary provider components files.
for f in COMPONENTS_CLUSTER_API COMPONENTS_KUBEADM COMPONENTS_VSPHERE; do \
  eval "rm -f \"\${${f}_GENERATED_FILE}\""
done

# Ensure that the actual outputs are only overwritten if the flag is provided.
for f in ADDONS PROVIDER_COMPONENTS CLUSTER CONTROLPLANE MACHINEDEPLOYMENT; do
  [ -n "${OVERWRITE}" ] || eval "no_file \"\${${f}_GENERATED_FILE}\""
done

require_if_defined() {
  while [ "${#}" -gt "0" ]; do
    eval "[ ! \"\${${1}+x}\" ] || ${1}=\"\${${1}${ENV_VAR_REQ}}\""
    shift
  done
}

require_if_defined CABPK_MANAGER_IMAGE \
                   CAPV_MANAGER_IMAGE \
                   VSPHERE_DATACENTER \
                   VSPHERE_DATASTORE \
                   VSPHERE_RESOURCE_POOL \
                   VSPHERE_FOLDER \
                   VSPHERE_TEMPLATE

# All variables used for yaml generation
EXPORTED_ENV_VARS=
record_and_export() {
  eval "EXPORTED_ENV_VARS=\"${EXPORTED_ENV_VARS} -e ${1}\"; \
        export ${1}=\"\${${1}${2}}\""
}
record_and_export CLUSTER_NAME          ':-capv-mgmt-example'
record_and_export SERVICE_CIDR          ':-100.64.0.0/13'
record_and_export CLUSTER_CIDR          ':-100.96.0.0/11'
record_and_export CABPK_MANAGER_IMAGE   ':-'
record_and_export CAPV_MANAGER_IMAGE    ':-'
record_and_export VSPHERE_USERNAME      "${ENV_VAR_REQ}"
record_and_export VSPHERE_PASSWORD      "${ENV_VAR_REQ}"
record_and_export VSPHERE_SERVER        "${ENV_VAR_REQ}"
record_and_export VSPHERE_DATACENTER    ':-'
record_and_export VSPHERE_DATASTORE     ':-'
record_and_export VSPHERE_NETWORK       "${ENV_VAR_REQ}"
record_and_export VSPHERE_RESOURCE_POOL ':-'
record_and_export VSPHERE_FOLDER        ':-'
record_and_export VSPHERE_TEMPLATE      ':-'
record_and_export SSH_AUTHORIZED_KEY    ':-'

verify_cpu_mem_dsk() {
  eval "[[ \${${1}-} =~ [[:digit:]]+ ]] || ${1}=\"${2}\"; \
        [ \"\${${1}}\" -ge \"${2}\" ] || { echo \"${1} must be >= ${2}\" 1>&2; exit 1; }; \
        record_and_export ${1} ':-\"\${${1}}\"'"
}
verify_cpu_mem_dsk VSPHERE_NUM_CPUS 2
verify_cpu_mem_dsk VSPHERE_MEM_MIB  2048
verify_cpu_mem_dsk VSPHERE_DISK_GIB 20

# TODO: check if KUBERNETES_VERSION has format "v1.14.4" and
# trim the "v" from the version. Alternatively, have CAPV or CAPI
# handle both 1.14.4 and v1.14.4
[[ ${KUBERNETES_VERSION-} =~ ^v?[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+([\+\.\-](.+))?$ ]] || KUBERNETES_VERSION="1.14.4"
record_and_export KUBERNETES_VERSION ":-${KUBERNETES_VERSION}"

# Base64 encode the credentials and unset the plain-text values.
VSPHERE_B64ENCODED_USERNAME="$(printf '%s' "${VSPHERE_USERNAME}" | base64)"
VSPHERE_B64ENCODED_PASSWORD="$(printf '%s' "${VSPHERE_PASSWORD}" | base64)"
export VSPHERE_B64ENCODED_USERNAME VSPHERE_B64ENCODED_PASSWORD
unset VSPHERE_USERNAME VSPHERE_PASSWORD

envsubst() {
  python -c 'import os,sys;[sys.stdout.write(os.path.expandvars(l)) for l in sys.stdin]'
}

# Generate the addons file.
envsubst >"${ADDONS_GENERATED_FILE}" <"${SRC_DIR}/addons.yaml"
echo "Generated ${ADDONS_GENERATED_FILE}"

# Generate cluster resources.
kustomize build "${SRC_DIR}/cluster" | envsubst >"${CLUSTER_GENERATED_FILE}"
echo "Generated ${CLUSTER_GENERATED_FILE}"

# Generate controlplane resources.
kustomize build "${SRC_DIR}/controlplane" | envsubst >"${CONTROLPLANE_GENERATED_FILE}"
echo "Generated ${CONTROLPLANE_GENERATED_FILE}"

# Generate machinedeployment resources.
kustomize build "${SRC_DIR}/machinedeployment" | envsubst >"${MACHINEDEPLOYMENT_GENERATED_FILE}"
echo "Generated ${MACHINEDEPLOYMENT_GENERATED_FILE}"

# Generate Cluster API provider components file.
kustomize build "github.com/kubernetes-sigs/cluster-api/config/default/?ref=master" >"${COMPONENTS_CLUSTER_API_GENERATED_FILE}"
echo "Generated ${COMPONENTS_CLUSTER_API_GENERATED_FILE}"

# Generate Kubeadm Bootstrap Provider components file.
kustomize build "github.com/kubernetes-sigs/cluster-api-bootstrap-provider-kubeadm/config/default/?ref=master" >"${COMPONENTS_KUBEADM_GENERATED_FILE}"
echo "Generated ${COMPONENTS_KUBEADM_GENERATED_FILE}"

# Generate VSphere Infrastructure Provider components file.
kustomize build "${SRC_DIR}/../config/default" | envsubst >"${COMPONENTS_VSPHERE_GENERATED_FILE}"
echo "Generated ${COMPONENTS_VSPHERE_GENERATED_FILE}"

# Generate a single provider components file.
kustomize build "${SRC_DIR}/provider-components" | envsubst >"${PROVIDER_COMPONENTS_GENERATED_FILE}"
echo "Generated ${PROVIDER_COMPONENTS_GENERATED_FILE}"
echo "WARNING: ${PROVIDER_COMPONENTS_GENERATED_FILE} includes vSphere credentials"

# If running in Docker then ensure the contents of the OUT_DIR have the
# the same owner as the volume mounted to the /out directory.
[ "${DOCKER_ENABLED-}" ] && chown -R "$(stat -c '%u:%g' /out)" "${OUT_DIR}"
