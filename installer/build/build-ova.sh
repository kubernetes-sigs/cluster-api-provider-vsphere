#!/bin/bash
# Copyright 2018 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# this file sets vic-product specific variables for the build configuration
set -e -o pipefail +h && [ -n "$DEBUG" ] && set -x
DIR=$(pwd)
CACHE=${DIR}/bin/.cache/
mkdir -p ${CACHE}
KUBERNETES_DEFAULT_VERSION=$(curl -sSL https://dl.k8s.io/release/stable-1.txt)

# Check if it's a file in `scripts`, URL, or REVISION
function setenv() {
  tmpvar=$1
  fallback=$2
  if [ -n "${!tmpvar}" ]; then
    export $1_VERSION="${!tmpvar}"
  else
    export $1_VERSION="${fallback}"
  fi
}

while [[ $# -gt 1 ]]
do
  key="$1"

  case $key in
    --kubernetes-version)
      KUBERNETES="$2"
      shift 2 # past argument
      ;;
    *)
      # unknown
      break; break;
  esac
done

# set Kubernetes Version
setenv KUBERNETES "$KUBERNETES_DEFAULT_VERSION"


ENV_FILE="${CACHE}/installer.env"
touch $ENV_FILE
cat > $ENV_FILE <<EOF
export KUBERNETES_VERSION=${KUBERNETES_VERSION:-}
EOF

echo -e "--------------------------------------------------
building ova with env...\n
$(cat $ENV_FILE | sed 's/export //g')"

echo "--------------------------------------------------"
echo "building make dependencies"
make all

echo "--------------------------------------------------"
echo "building OVA..."
${DIR}/build/bootable/build-main.sh -m "${DIR}/build/ova-manifest.json" -r "${DIR}/bin" -c "${CACHE}" $@