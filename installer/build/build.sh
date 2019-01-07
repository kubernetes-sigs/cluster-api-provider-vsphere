#!/bin/bash
# Copyright 2018 The Kubernetes Authors.
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

# this file is responsible for parsing cli args and spinning up a build container
# wraps bootable/build-main.sh in a docker container

DEBUG=${DEBUG:-}
set -e -o pipefail +h && [ -n "$DEBUG" ] && set -x
ROOT_DIR="$(go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-vsphere"
ROOT_WORK_DIR="/go/src/sigs.k8s.io/cluster-api-provider-vsphere"

ROOT_INSTALLER_DIR="${ROOT_DIR}/installer"
ROOT_INSTALLER_WORK_DIR="${ROOT_WORK_DIR}/installer"

# TODO(frapposelli): hardcoding version for now, this will be removed once a
#                    tagging strategy is established for the project
# TAG=${DRONE_TAG:-$(git describe --abbrev=0 --tags)} # e.g. `v0.9.0`
TAG="v0.0.1"

REV=$(git rev-parse --short=8 HEAD)
BUILD_OVA_REVISION="${TAG}-${REV}"

function usage() {
    echo -e "Usage:
      <ova-dev|ova-ci>
      [passthrough args for ./bootable/build-main.sh, eg. '-b bin/.ova-appliance-base.tar.gz']
    ie: $0 ova-dev" >&2
    exit 1
}

[ $# -gt 0 ] || usage
step=$1; shift
[ ! "$step" == "ova-ci" ] || [ ! "$step" == "ova-dev" ] || usage

echo "--------------------------------------------------"
if [ "$step" == "ova-dev" ]; then
  echo "starting docker dev build container..."
  docker run -it --rm --privileged -v /dev:/dev \
    -v ${ROOT_DIR}:${ROOT_WORK_DIR}:ro \
    -v ${ROOT_INSTALLER_DIR}/bin:${ROOT_INSTALLER_WORK_DIR}/bin \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -e DEBUG=${DEBUG} \
    -e BUILD_OVA_REVISION=${BUILD_OVA_REVISION} \
    -e TAG=${TAG} \
    -e BUILD_NUMBER=${BUILD_NUMBER} \
    -e TERM -w ${ROOT_INSTALLER_WORK_DIR} \
    gcr.io/cnx-cluster-api/cluster-api-ova-build:latest ./build/build-ova.sh "$@"
elif [ "$step" == "ova-ci" ]; then
  echo "starting ci build..."
  export DEBUG=${DEBUG}
  export BUILD_OVA_REVISION=${BUILD_OVA_REVISION}
  export TAG=${TAG}
  export BUILD_NUMBER=${BUILD_NUMBER}
  ./build/build-ova.sh "$@"
else
  usage
fi


