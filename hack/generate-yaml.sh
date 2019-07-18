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

OUT_DIR="${OUT_DIR:-}"
TPL_DIR=./cmd/clusterctl/examples/vsphere

OVERWRITE=
CLUSTER_NAME="${CLUSTER_NAME:-capv-mgmt-example}"
ENV_VAR_REQ=':?required'
CAPV_MANAGER_IMAGE="${CAPV_MANAGER_IMAGE:-gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:latest}"

usage() {
  cat <<EOF
usage: ${0} [FLAGS]
  Generates input manifests for the Cluster API Provider for vSphere (CAPV)

FLAGS
  -c    cluster name (default "${CLUSTER_NAME}")
  -d    disables required environment variables
  -f    force overwrite of existing files
  -h    prints this help screen
  -i    input directory (default ${TPL_DIR})
  -m    manager image (default "${CAPV_MANAGER_IMAGE}")
  -o    output directory (default ${OUT_DIR})
EOF
}

while getopts ':c:dfhi:m:o:' opt; do
  case "${opt}" in
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
    TPL_DIR="${OPTARG}"
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

export MANAGER_IMAGE="${CAPV_MANAGER_IMAGE}"

[ -n "${OUT_DIR}" ] || OUT_DIR="./out/${CLUSTER_NAME}"
mkdir -p "${OUT_DIR}"

# Load an envvars.txt file if one is found.
# shellcheck disable=SC1090
[ -e "${OUT_DIR}/envvars.txt" ] && source "${OUT_DIR}/envvars.txt"

# shellcheck disable=SC2034
ADDON_TPL_FILE="${TPL_DIR}"/addons.yaml.template
# shellcheck disable=SC2034
ADDON_OUT_FILE="${OUT_DIR}"/addons.yaml
# shellcheck disable=SC2034
CLUSTER_TPL_FILE="${TPL_DIR}"/cluster.yaml.template
CLUSTER_OUT_FILE="${OUT_DIR}"/cluster.yaml
# shellcheck disable=SC2034
MACHINES_TPL_FILE="${TPL_DIR}"/machines.yaml.template
MACHINES_OUT_FILE="${OUT_DIR}"/machines.yaml
# shellcheck disable=SC2034
MACHINESET_TPL_FILE="${TPL_DIR}"/machineset.yaml.template
# shellcheck disable=SC2034
MACHINESET_OUT_FILE="${OUT_DIR}"/machineset.yaml

CAPI_CFG_DIR=./vendor/sigs.k8s.io/cluster-api/config
CAPV_CFG_DIR=./config

COMP_OUT_FILE="${OUT_DIR}"/provider-components.yaml
# shellcheck disable=SC2034
CAPV_MANAGER_TPL_FILE="${CAPV_CFG_DIR}"/default/manager_image_patch.yaml.template
# shellcheck disable=SC2034
CAPV_MANAGER_OUT_FILE="${CAPV_CFG_DIR}"/default/manager_image_patch.yaml

ok_file() {
  [ -f "${1}" ] || { echo "${1} is missing" 1>&2; exit 1; }
}

no_file() {
  [ ! -f "${1}" ] || { echo "${1} already exists, overwrite with -f" 1>&2; exit 1; }
}

for f in ADDON CLUSTER MACHINES MACHINESET; do
  eval "ok_file \"\${${f}_TPL_FILE}\""
  [ -n "${OVERWRITE}" ] || eval "no_file \"\${${f}_OUT_FILE}\""
done

require_if_defined() {
  while [ "${#}" -gt "0" ]; do
    eval "[ ! \"\${${1}+x}\" ] || ${1}=\"\${${1}${ENV_VAR_REQ}}\""
    shift
  done
}

require_if_defined CAPV_MANAGER_IMAGE \
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

# TODO: check if KUBERNETES_VERSION has format "v1.13.6" and
# trim the "v" from the version. Alternatively, have CAPV or CAPI
# handle both 1.13.6 and v1.13.6
[[ ${KUBERNETES_VERSION-} =~ ^v?[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+([\+\.\-](.+))?$ ]] || KUBERNETES_VERSION="1.13.6"
record_and_export KUBERNETES_VERSION ":-${KUBERNETES_VERSION}"

do_envsubst() {
  python hack/envsubst.py >"${2}" <"${1}"
  echo "done generating ${2}"
}

# Create the output files by substituting the templates with envrionment vars.
for f in ADDON CAPV_MANAGER CLUSTER MACHINES MACHINESET; do
  eval "do_envsubst \"\${${f}_TPL_FILE}\" \"\${${f}_OUT_FILE}\""
done

# Run kustomize on the patches.
{ kustomize build "${CAPV_CFG_DIR}"/default/ && \
  echo "---" && \
  kustomize build "${CAPI_CFG_DIR}"/default/; } >"${COMP_OUT_FILE}"

cat <<EOF
Done generating ${COMP_OUT_FILE}

*** Finished creating initial example yamls in ${OUT_DIR}

    The files ${CLUSTER_OUT_FILE} and ${MACHINES_OUT_FILE} need to be updated
    with information about the desired Kubernetes cluster and vSphere environment
    on which the Kubernetes cluster will be created.

Enjoy!
EOF
