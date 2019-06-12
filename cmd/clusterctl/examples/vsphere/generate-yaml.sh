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

MACHINE_CONTROLLER_SSH_PUBLIC_FILE=vsphere_tmp.pub
MACHINE_CONTROLLER_SSH_PUBLIC=
MACHINE_CONTROLLER_SSH_PRIVATE_FILE=vsphere_tmp
MACHINE_CONTROLLER_SSH_PRIVATE=
MACHINE_CONTROLLER_SSH_HOME=~/.ssh/

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

# all variables used for yaml generation

export CLUSTER_NAME=${CLUSTER_NAME:-vsphere-cluster}
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

# validate all required variables before generating any files

if [ -z ${VSPHERE_USER} ]; then
  echo "env var VSPHERE_USER is required"
  exit 1
fi

if [ -z ${VSPHERE_PASSWORD} ]; then
  echo "env var VSPHERE_PASSWORD is required"
  exit 1
fi

if [ -z ${VSPHERE_SERVER} ]; then
  echo "env var VSPHERE_SERVER is required"
  exit 1
fi

if [ -z $CAPV_MANAGER_IMAGE ]; then
  echo "env var CAPV_MANAGER_IMAGE is required"
  exit 1
fi

if [ -z $VSPHERE_DATACENTER ]; then
  echo "env var VSPHERE_DATACENTER is required"
  exit 1
fi

if [ -z $VSPHERE_DATASTORE ]; then
  echo "env var VSPHERE_DATASTORE is required"
  exit 1
fi

if [ -z $VSPHERE_NETWORK ]; then
  echo "env var VSPHERE_NETWORK is required"
  exit 1
fi

if [ -z $VSPHERE_RESOURCE_POOL ]; then
  echo "env var VSPHERE_RESOURCE_POOL is required"
  exit 1
fi

if [ -z $VSPHERE_FOLDER ]; then
  echo "env var VSPHERE_FOLDER is required"
  exit 1
fi

if [ -z $VSPHERE_TEMPLATE ]; then
  echo "env var VSPHERE_TEMPLATE is required"
  exit 1
fi

if [ -z $VSPHERE_DISK ]; then
  echo "env var VSPHERE_DISK is required"
  exit 1
fi

if [ ${VSPHERE_DISK_SIZE_GB} -lt 20 ]; then
  echo "env var VSPHERE_DISK_SIZE_GB must be >= 20" 1>&2
  exit 1
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

# Check if the ssh key already exists. If not, generate and copy to the .ssh dir.
if [ ! -f $MACHINE_CONTROLLER_SSH_HOME$MACHINE_CONTROLLER_SSH_PRIVATE_FILE ]; then
  echo "Generating SSH key files for machine controller."
  # This is needed because GetKubeConfig assumes the key in the home .ssh dir.
  ssh-keygen -t rsa -f $MACHINE_CONTROLLER_SSH_HOME$MACHINE_CONTROLLER_SSH_PRIVATE_FILE  -N ""
fi

# With kustomize PR 700 merged, the resources in kustomization.yaml could only be scanned in the sub-folder
# So putting vsphere_tmp and vsphere_tmp.pub in ../config/default folder
cp $MACHINE_CONTROLLER_SSH_HOME$MACHINE_CONTROLLER_SSH_PUBLIC_FILE $VSPHERE_CLUSTER_API_CRD_PATH/default/$MACHINE_CONTROLLER_SSH_PUBLIC_FILE
cp $MACHINE_CONTROLLER_SSH_HOME$MACHINE_CONTROLLER_SSH_PRIVATE_FILE $VSPHERE_CLUSTER_API_CRD_PATH/default/$MACHINE_CONTROLLER_SSH_PRIVATE_FILE

# By default, linux wraps base64 output every 76 cols, so we use 'tr -d' to remove whitespaces.
# Note 'base64 -w0' doesn't work on Mac OS X, which has different flags.
MACHINE_CONTROLLER_SSH_PUBLIC=$(cat $MACHINE_CONTROLLER_SSH_HOME$MACHINE_CONTROLLER_SSH_PUBLIC_FILE | base64 | tr -d '\r\n')
MACHINE_CONTROLLER_SSH_PRIVATE=$(cat $MACHINE_CONTROLLER_SSH_HOME$MACHINE_CONTROLLER_SSH_PRIVATE_FILE | base64 | tr -d '\r\n')

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
