#!/bin/bash

# Copyright 2019 The Kubernetes Authors.
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

# TODO(akutz) This needs to go into the project's Makefile once the
# old targets are no longer needed by CI.

# Change directories to the parent directory of the one in which this
# script is located.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

VERSION="${VERSION:-$(git describe --dirty)}"
CAPV_MANAGER_IMAGE_NAME="${CAPV_MANAGER_IMAGE_NAME:-gcr.io/cnx-cluster-api/vsphere-cluster-api-provider}"
CAPV_GENERATE_YAML_IMAGE_NAME="${CAPV_GENERATE_YAML_IMAGE_NAME:-gcr.io/cnx-cluster-api/generate-yaml}"

usage() {
  cat <<EOF
usage: ${0} [FLAGS]
  Builds the CAPV container images.

FLAGS
  -g    generate yaml image name (default "${CAPV_GENERATE_YAML_IMAGE_NAME}")
  -h    prints this help screen
  -m    manager image name (default "${CAPV_MANAGER_IMAGE_NAME}")
  -v    version (default "${VERSION}")
EOF
}

while getopts ':g:hm:v:' opt; do
  case "${opt}" in
  g)
    CAPV_GENERATE_YAML_IMAGE_NAME="${OPTARG}"
    ;;
  h)
    usage 1>&2; exit 1
    ;;
  m)
    CAPV_MANAGER_IMAGE_NAME="${OPTARG}"
    ;;
  v)
    VERSION="${OPTARG}"
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

CAPV_MANAGER_IMAGE="${CAPV_MANAGER_IMAGE_NAME}:${VERSION}"
CAPV_GENERATE_YAML_IMAGE="${CAPV_GENERATE_YAML_IMAGE_NAME}:${VERSION}"

build_capv_manager_image() {
  echo "building capv manager image"
  docker build \
    -f Dockerfile \
    -t "${CAPV_MANAGER_IMAGE}" \
    .
}

build_capv_generate_yaml_image() {
  echo "building capv generate yaml image"
  docker build \
    -f hack/tools/generate-yaml/Dockerfile \
    -t "${CAPV_GENERATE_YAML_IMAGE}" \
    --build-arg "CAPV_MANAGER_IMAGE=${CAPV_MANAGER_IMAGE}" \
    .
}

build_capv_manager_image
build_capv_generate_yaml_image
