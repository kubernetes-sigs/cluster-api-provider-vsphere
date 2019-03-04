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

OUTPUT_DIR=out
TEMPLATE_DIR=$GOPATH/src/sigs.k8s.io/cluster-api-provider-vsphere/cmd/clusterctl/examples/vsphere

MACHINE_TEMPLATE_FILE=${TEMPLATE_DIR}/machines.yaml.template
MACHINE_GENERATED_FILE=${OUTPUT_DIR}/machines.yaml
MACHINESET_TEMPLATE_FILE=${TEMPLATE_DIR}/machineset.yaml.template
MACHINESET_GENERATED_FILE=${OUTPUT_DIR}/machineset.yaml
CLUSTER_TEMPLATE_FILE=${TEMPLATE_DIR}/cluster.yaml.template
CLUSTER_GENERATED_FILE=${OUTPUT_DIR}/cluster.yaml
ADDON_TEMPLATE_FILE=${TEMPLATE_DIR}/addons.yaml.template
ADDON_GENERATED_FILE=${OUTPUT_DIR}/addons.yaml

PROVIDERCOMPONENT_GENERATED_FILE=${OUTPUT_DIR}/provider-components.yaml

MACHINE_CONTROLLER_SSH_PUBLIC_FILE=vsphere_tmp.pub
MACHINE_CONTROLLER_SSH_PUBLIC=
MACHINE_CONTROLLER_SSH_PRIVATE_FILE=vsphere_tmp
MACHINE_CONTROLLER_SSH_PRIVATE=
MACHINE_CONTROLLER_SSH_HOME=~/.ssh/

CLUSTER_API_CRD_PATH=$GOPATH/src/sigs.k8s.io/cluster-api-provider-vsphere/vendor/sigs.k8s.io/cluster-api/config
VSPHERE_CLUSTER_API_CRD_PATH=$GOPATH/src/sigs.k8s.io/cluster-api-provider-vsphere/config

KUBECON_CONTROLLER='gcr.io/cnx-cluster-api/cluster-api-controller:kubecon2018'
KUBECON_VSPHERE_PROVIDER='gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:kubecon2018'

USE_KUBECON="${USE_KUBECON:-0}"

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
            echo "--kubecon                 Use the containers from the kubecon demo"
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
          --kubecon)
            USE_KUBECON=1
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

if [ $USE_KUBECON = 1 ]; then
  echo "Updating generated provider-components.yaml file with containers from kubecon 2018..."
  CONTROLLER_CONTAINER=$(grep "image:.*cluster-api-controller" ${PROVIDERCOMPONENT_GENERATED_FILE} | awk '{ print $2 }')
  VSPHERE_PROVIDER_CONTAINER=$(grep "image:.*vsphere-cluster-api" ${PROVIDERCOMPONENT_GENERATED_FILE} | awk '{ print $2 }')

  cat $PROVIDERCOMPONENT_GENERATED_FILE | \
  	sed -e "s|$VSPHERE_PROVIDER_CONTAINER|$KUBECON_VSPHERE_PROVIDER|" \
  	> $PROVIDERCOMPONENT_GENERATED_FILE.new

  cat $PROVIDERCOMPONENT_GENERATED_FILE.new | \
  	sed -e "s|$CONTROLLER_CONTAINER|$KUBECON_CONTROLLER|" \
  	> $PROVIDERCOMPONENT_GENERATED_FILE

  rm $PROVIDERCOMPONENT_GENERATED_FILE.new
fi

echo "Done generating $PROVIDERCOMPONENT_GENERATED_FILE"

cat $MACHINE_TEMPLATE_FILE \
  > $MACHINE_GENERATED_FILE

cat $MACHINESET_TEMPLATE_FILE \
  > $MACHINESET_GENERATED_FILE

cat $CLUSTER_TEMPLATE_FILE \
  > $CLUSTER_GENERATED_FILE

cat $ADDON_TEMPLATE_FILE \
  > $ADDON_GENERATED_FILE

echo
echo "*** Finished creating initial example yamls in ./out"
echo
echo "    You need to update the machines.yaml and cluster.yaml files with information about your"
echo "    vSphere environment and information about the cluster you want to create."
echo
echo Enjoy!
echo