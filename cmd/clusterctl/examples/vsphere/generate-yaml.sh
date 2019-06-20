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

cd "$(dirname "${BASH_SOURCE[0]}")/../../../.."

OUTPUT_DIR=./out
TEMPLATE_DIR=./cmd/clusterctl/examples/vsphere

MACHINE_TEMPLATE_FILE=${TEMPLATE_DIR}/machines.yaml.template
MACHINE_GENERATED_FILE=${OUTPUT_DIR}/machines.yaml
MACHINESET_TEMPLATE_FILE=${TEMPLATE_DIR}/machineset.yaml.template
MACHINESET_GENERATED_FILE=${OUTPUT_DIR}/machineset.yaml
CLUSTER_TEMPLATE_FILE=${TEMPLATE_DIR}/cluster.yaml.template
CLUSTER_GENERATED_FILE=${OUTPUT_DIR}/cluster.yaml
ADDON_TEMPLATE_FILE=${TEMPLATE_DIR}/addons.yaml.template
ADDON_GENERATED_FILE=${OUTPUT_DIR}/addons.yaml

CLUSTER_API_CRD_PATH=./vendor/sigs.k8s.io/cluster-api/config
VSPHERE_CLUSTER_API_CRD_PATH=./config

PROVIDERCOMPONENT_GENERATED_FILE=${OUTPUT_DIR}/provider-components.yaml
CAPV_MANAGER_TEMPLATE_FILE=${TEMPLATE_DIR}/capv_manager_image_patch.yaml.template
CAPV_MANAGER_GENERATED_FILE=$VSPHERE_CLUSTER_API_CRD_PATH/default/capv_manager_image_patch.yaml

OVERWRITE=0

SCRIPT=$(basename $0)
while test $# -gt 0; do
        case "$1" in
          -h|--help)
            echo "$SCRIPT - generates input yaml files for Cluster API on vSphere"
            echo " "
            echo "$SCRIPT [options]"
            echo " "
            echo "options:"
            echo "-h, --help                show brief help"
            echo "-f, --force-overwrite     if file to be generated already exists, force script to overwrite it"
            exit 0
            ;;
          -f)
            OVERWRITE=1
            shift
            ;;
          --force-overwrite)
            OVERWRITE=1
            shift
            ;;
          *)
            break
            ;;
        esac
done

if [ $OVERWRITE -ne 1 ] && [ -f $MACHINE_GENERATED_FILE ]; then
  echo File $MACHINE_GENERATED_FILE already exists. Delete it manually before running this script.
  exit 1
fi

if [ $OVERWRITE -ne 1 ] && [ -f $CLUSTER_GENERATED_FILE ]; then
  echo File $CLUSTER_GENERATED_FILE already exists. Delete it manually before running this script.
  exit 1
fi

if [ $OVERWRITE -ne 1 ] && [ -f $ADDON_GENERATED_FILE ]; then
  echo File $ADDON_GENERATED_FILE already exists. Delete it manually before running this script.
  exit 1
fi

if [ $OVERWRITE -ne 1 ] && [ -f $PROVIDERCOMPONENT_GENERATED_FILE ]; then
  echo "File $PROVIDERCOMPONENT_GENERATED_FILE already exists. Delete it manually before running this script."
  exit 1
fi

mkdir -p ${OUTPUT_DIR}

# All variables used for yaml generation
# Ensure all variables listed here are also listed under GENERATE_YAML_ENV_VARS in the root Makefile

export CLUSTER_NAME=${CLUSTER_NAME:-capv-mgmt-example}
export SERVICE_CIDR=${SERVICE_CIDR:-100.64.0.0/13}
export CLUSTER_CIDR=${CLUSTER_CIDR:-100.96.0.0/11}

export VSPHERE_USER=${VSPHERE_USER:-}
export VSPHERE_PASSWORD=${VSPHERE_PASSWORD:-}
export VSPHERE_SERVER=${VSPHERE_SERVER:-}
export CAPV_MANAGER_IMAGE=${CAPV_MANAGER_IMAGE:-}
export VSPHERE_DATACENTER=${VSPHERE_DATACENTER:-}
export VSPHERE_DATASTORE=${VSPHERE_DATASTORE:-}
export VSPHERE_NETWORK=${VSPHERE_NETWORK:-}
export VSPHERE_RESOURCE_POOL=${VSPHERE_RESOURCE_POOL:-}
export VSPHERE_FOLDER=${VSPHERE_FOLDER:-}
export VSPHERE_TEMPLATE=${VSPHERE_TEMPLATE:-}
export VSPHERE_DISK=${VSPHERE_DISK:-}
export VSPHERE_DISK_SIZE_GB=${VSPHERE_DISK_SIZE_GB:-20}

# TODO: check if KUBERNETES_VERSION has format "v1.13.6" and
# trim the "v" from the version. Alternatively, have CAPV or CAPI
# handle both 1.13.6 and v1.13.6
export KUBERNETES_VERSION=${KUBERNETES_VERSION:-1.13.6}

# validate all required variables before generating any files
if [ ! "${CAPV_YAML_VALIDATION:-1}" = "0" ]; then
  if [ -z "${VSPHERE_USER}" ]; then
    echo "env var VSPHERE_USER is required" 1>&2
    exit 1
  fi

  if [ -z "${VSPHERE_PASSWORD}" ]; then
    echo "env var VSPHERE_PASSWORD is required" 1>&2
    exit 1
  fi

  if [ -z "${VSPHERE_SERVER}" ]; then
    echo "env var VSPHERE_SERVER is required" 1>&2
    exit 1
  fi

  if [ -z "${CAPV_MANAGER_IMAGE}" ]; then
    echo "env var CAPV_MANAGER_IMAGE is required" 1>&2
    exit 1
  fi

  if [ -z "${VSPHERE_DATACENTER}" ]; then
    echo "env var VSPHERE_DATACENTER is required" 1>&2
    exit 1
  fi

  if [ -z "${VSPHERE_DATASTORE}" ]; then
    echo "env var VSPHERE_DATASTORE is required" 1>&2
    exit 1
  fi

  if [ -z "${VSPHERE_NETWORK}" ]; then
    echo "env var VSPHERE_NETWORK is required" 1>&2
    exit 1
  fi

  if [ -z "${VSPHERE_RESOURCE_POOL}" ]; then
    echo "env var VSPHERE_RESOURCE_POOL is required" 1>&2
    exit 1
  fi

  if [ -z "${VSPHERE_FOLDER}" ]; then
    echo "env var VSPHERE_FOLDER is required" 1>&2
    exit 1
  fi

  if [ -z "${VSPHERE_TEMPLATE}" ]; then
    echo "env var VSPHERE_TEMPLATE is required" 1>&2
    exit 1
  fi

  if [ -z "${VSPHERE_DISK}" ]; then
    echo "env var VSPHERE_DISK is required" 1>&2
    exit 1
  fi

  if [ "${VSPHERE_DISK_SIZE_GB}" -lt "20" ]; then
    echo "env var VSPHERE_DISK_SIZE_GB must be >= 20" 1>&2
    exit 1
  fi
fi

envsubst < $MACHINE_TEMPLATE_FILE > "${MACHINE_GENERATED_FILE}"
echo "Done generating $MACHINE_GENERATED_FILE"
envsubst < $MACHINESET_TEMPLATE_FILE > "${MACHINESET_GENERATED_FILE}"
echo "Done generating $MACHINESET_GENERATED_FILE"
envsubst < $CLUSTER_TEMPLATE_FILE > "${CLUSTER_GENERATED_FILE}"
echo "Done generating $CLUSTER_GENERATED_FILE"
envsubst < $ADDON_TEMPLATE_FILE > "${ADDON_GENERATED_FILE}"
echo "Done generating $ADDON_GENERATED_FILE"

envsubst < $CAPV_MANAGER_TEMPLATE_FILE > "${CAPV_MANAGER_GENERATED_FILE}"

kustomize build $VSPHERE_CLUSTER_API_CRD_PATH/default/ > $PROVIDERCOMPONENT_GENERATED_FILE
echo "---" >> $PROVIDERCOMPONENT_GENERATED_FILE
kustomize build $CLUSTER_API_CRD_PATH/default/ >> $PROVIDERCOMPONENT_GENERATED_FILE

echo "Done generating $PROVIDERCOMPONENT_GENERATED_FILE"

echo
echo "*** Finished creating initial example yamls in ./out"
echo
echo "    You need to update the machines.yaml and cluster.yaml files with information about your"
echo "    vSphere environment and information about the cluster you want to create."
echo
echo Enjoy!
echo
