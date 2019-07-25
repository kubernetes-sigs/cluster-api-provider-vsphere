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

# This script is used build new container images of the CAPV manager and
# clusterctl. When invoked without arguments, the default behavior is to build
# new ci images

set -o errexit
set -o nounset
set -o pipefail

# BASE_REPO is the root path of the image repository
readonly BASE_IMAGE_REPO=gcr.io/cluster-api-provider-vsphere

# Release images
readonly CAPV_MANAGER_IMAGE_RELEASE=${BASE_IMAGE_REPO}/release/manager
readonly CAPV_MANIFESTS_IMAGE_RELEASE=${BASE_IMAGE_REPO}/release/manifests

# PR images
readonly CAPV_MANAGER_IMAGE_PR=${BASE_IMAGE_REPO}/pr/manager
readonly CAPV_MANIFESTS_IMAGE_PR=${BASE_IMAGE_REPO}/pr/manifests

# CI images
readonly CAPV_MANAGER_IMAGE_CI=${BASE_IMAGE_REPO}/ci/manager
readonly CAPV_MANIFESTS_IMAGE_CI=${BASE_IMAGE_REPO}/ci/manifests

AUTH=
PUSH=
LATEST=
MANAGER_IMAGE_NAME=
MANIFESTS_IMAGE_NAME=
VERSION=$(git describe --dirty --always 2>/dev/null)
GCR_KEY_FILE="${GCR_KEY_FILE:-}"

BUILD_RELEASE_TYPE="${BUILD_RELEASE_TYPE-}"

# If BUILD_RELEASE_TYPE is not set then check to see if this is a PR
# or release build. This may still be overridden below with the "-t" flag.
if [ -z "${BUILD_RELEASE_TYPE}" ]; then
  if hack/match-release-tag.sh >/dev/null 2>&1; then
    BUILD_RELEASE_TYPE=release
  else
    BUILD_RELEASE_TYPE=ci
  fi
fi

USAGE="
usage: ${0} [FLAGS]
  Builds and optionally pushes new images for Cluster API Provider vSphere (CAPV)

FLAGS
  -h    show this help and exit
  -k    path to GCR key file. Used to login to registry if specified
        (defaults to: ${GCR_KEY_FILE})
  -l    tag the images as \"latest\" in addition to their version
        when used with -p, both tags will be pushed
  -p    push the images to the public container registry
  -t    the build/release type (defaults to ${BUILD_RELEASE_TYPE})
        one of [ci,pr,release]
"

# Change directories to the parent directory of the one in which this
# script is located.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

function error() {
  local exit_code="${?}"
  echo "${@}" 1>&2
  return "${exit_code}"
}

function fatal() {
  error "${@}" || exit 1
}

function build_images() {
  case "${BUILD_RELEASE_TYPE}" in
    ci)
      # A non-PR, non-release build. This is usually a build off of master
      MANAGER_IMAGE_NAME=${CAPV_MANAGER_IMAGE_CI}
      MANIFESTS_IMAGE_NAME=${CAPV_MANIFESTS_IMAGE_CI}
      ;;
    pr)
      # A PR build
      MANAGER_IMAGE_NAME=${CAPV_MANAGER_IMAGE_PR}
      MANIFESTS_IMAGE_NAME=${CAPV_MANIFESTS_IMAGE_PR}
      ;;
    release)
      # On an annotated tag
      MANAGER_IMAGE_NAME=${CAPV_MANAGER_IMAGE_RELEASE}
      MANIFESTS_IMAGE_NAME=${CAPV_MANIFESTS_IMAGE_RELEASE}
      ;;
  esac

  # Manager image
  echo "building ${MANAGER_IMAGE_NAME}:${VERSION}"
  docker build \
    -f Dockerfile \
    -t "${MANAGER_IMAGE_NAME}":"${VERSION}" \
    .
  if [ "${LATEST}" ]; then
    echo "tagging image ${MANAGER_IMAGE_NAME}:${VERSION} as latest"
    docker tag "${MANAGER_IMAGE_NAME}":"${VERSION}" "${MANAGER_IMAGE_NAME}":latest
  fi

  # Manifests image
  echo "building ${MANIFESTS_IMAGE_NAME}:${VERSION}"
  docker build \
    -f hack/tools/generate-yaml/Dockerfile \
    -t "${MANIFESTS_IMAGE_NAME}":"${VERSION}" \
    --build-arg "CAPV_MANAGER_IMAGE=${MANAGER_IMAGE_NAME}:${VERSION}" \
    .
  if [ "${LATEST}" ]; then
    echo "tagging image ${MANIFESTS_IMAGE_NAME}:${VERSION} as latest"
    docker tag "${MANIFESTS_IMAGE_NAME}":"${VERSION}" "${MANIFESTS_IMAGE_NAME}":latest
  fi
}

function logout() {
  if [ "${AUTH}" ]; then
    gcloud auth revoke
  fi
}

function login() {
  # If GCR_KEY_FILE is set, use that service account to login
  if [ "${GCR_KEY_FILE}" ]; then
    trap logout EXIT
    gcloud auth configure-docker --quiet || fatal "unable to add docker auth helper"
    gcloud auth activate-service-account --key-file "${GCR_KEY_FILE}" || fatal "unable to login"
    docker login -u _json_key --password-stdin https://gcr.io <"${GCR_KEY_FILE}"
    AUTH=1
  fi
}

function push_images() {
  [ "${MANAGER_IMAGE_NAME}" ] || fatal "MANAGER_IMAGE_NAME not set"
  [ "${MANIFESTS_IMAGE_NAME}" ] || fatal "MANIFESTS_IMAGE_NAME not set"

  login

  # Manager image
  echo "pushing ${MANAGER_IMAGE_NAME}:${VERSION}"
  docker push "${MANAGER_IMAGE_NAME}":"${VERSION}"
  if [ "${LATEST}" ]; then
    echo "also pushing ${MANAGER_IMAGE_NAME}:${VERSION} as latest"
    docker push "${MANAGER_IMAGE_NAME}":latest
  fi

  # Manifests image
  echo "pushing ${MANIFESTS_IMAGE_NAME}:${VERSION}"
  docker push "${MANIFESTS_IMAGE_NAME}":"${VERSION}"
  if [ "${LATEST}" ]; then
    echo "also pushing ${MANIFESTS_IMAGE_NAME}:${VERSION} as latest"
    docker push "${MANIFESTS_IMAGE_NAME}":latest
  fi
}

function build_clusterctl() {
  for os in linux darwin; do
    # Don't build Darwin for PRs
    if [ "${BUILD_RELEASE_TYPE}" = "pr" ] && [ "${os}" == "darwin" ]; then
      continue
    fi
    echo "building clusterctl for ${os}"
    GOOS="${os}" GOARCH=amd64 make clusterctl-in-docker
  done
}

function sha_sum() {
  { sha256sum "${1}" || shasum -a 256 "${1}"; } 2>/dev/null > "${1}.sha256"
}

function push_clusterctl() {
  local bucket
  case "${BUILD_RELEASE_TYPE}" in
    ci)
      bucket="capv-ci"
      ;;
    pr)
      bucket="capv-pr"
      ;;
    release)
      bucket="capv-release"
      ;;
  esac

  for os in linux darwin; do
    # Don't build Darwin for PRs
    if [ "${BUILD_RELEASE_TYPE}" = "pr" ] && [ "${os}" == "darwin" ]; then
      continue
    fi
    sha_sum "bin/clusterctl.${os}_amd64"
    echo "copying clusterctl version ${VERSION} for ${os} to ${bucket}"
    gsutil cp "bin/clusterctl.${os}_amd64" "gs://${bucket}/${VERSION}/bin/${os}/amd64/clusterctl"
    gsutil cp "bin/clusterctl.${os}_amd64.sha256" "gs://${bucket}/${VERSION}/bin/${os}/amd64/clusterctl.sha256"
  done
}

# Start of main script
while getopts ":hk:lpt:" opt; do
  case ${opt} in
    h)
      error "${USAGE}" && exit 1
      ;;
    k)
      GCR_KEY_FILE="${OPTARG}"
      ;;
    l)
      LATEST=1
      ;;
    p)
      PUSH=1
      ;;
    t)
      BUILD_RELEASE_TYPE="${OPTARG}"
      ;;
    \?)
      error "invalid option: -${OPTARG} ${USAGE}" && exit 1
      ;;
    :)
      error "option -${OPTARG} requires an argument" && exit 1
      ;;
  esac
done
shift $((OPTIND-1))

# Verify the GCR_KEY_FILE exists if defined
if [ "${GCR_KEY_FILE}" ]; then
  [ -e "${GCR_KEY_FILE}" ] || fatal "key file ${GCR_KEY_FILE} does not exist"
fi

# Validate build/release type.
case "${BUILD_RELEASE_TYPE}" in
  ci|pr|release)
    # do nothing
    ;;
  *)
    fatal "invalid BUILD_RELEASE_TYPE: ${BUILD_RELEASE_TYPE}"
    ;;
esac

# make sure that Docker is available
docker ps >/dev/null 2>&1 || fatal "Docker not available"

# build container images
build_images

# build clusterctl
build_clusterctl

# Optionally push artifacts
if [ "${PUSH}" ]; then
  push_images
  push_clusterctl
fi
