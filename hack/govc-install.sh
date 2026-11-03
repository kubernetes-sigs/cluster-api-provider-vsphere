#!/usr/bin/env bash
# Copyright 2021 The Kubernetes Authors.
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

if [ -z "${1}" ]; then
  echo "must provide binary name as first parameter"
  exit 1
fi

if [ -z "${2}" ]; then
  echo "must provide version as second parameter"
  exit 1
fi

if [ -z "${GOBIN}" ]; then
  echo "GOBIN is not set. Must set GOBIN to install the bin in a specified directory."
  exit 1
fi

GOVC_ARCH="x86_64"
if [ "$(go env GOARCH)" == "arm64" ]; then
  GOVC_ARCH="arm64"
fi

GOVC_OS="Linux"
if [ "$(go env GOOS)" == "darwin" ]; then
  GOVC_OS="Darwin"
fi

# shellcheck source=./hack/utils.sh
source "$(dirname "$0")/utils.sh"

# Expected sha256 for each pinned version/OS/ARCH combination. GOVC_VER
# tracks the govmomi version in go.mod, so update these from the release's
# checksums.txt whenever that dependency is bumped.
# Read via indirect expansion below, so shellcheck can't see the usage.
# shellcheck disable=SC2034
GOVC_SHA256_Darwin_arm64="d5b85abf6e2d8ef602aedb35110567d58016edf5bc4ce13f25fb1a4af31957c3"
# shellcheck disable=SC2034
GOVC_SHA256_Darwin_x86_64="665233e84940a659596c84e3ce8bb79ec7e1e142f55cb696974b37db45b2a188"
# shellcheck disable=SC2034
GOVC_SHA256_Linux_arm64="9c34a018300fbb02b943e48f277b9cee8f90ab7a6e6936427ef9f48c1e93af50"
# shellcheck disable=SC2034
GOVC_SHA256_Linux_x86_64="12889f30e42e868487041f6628e9789606c80e13a194fb37c099abffc92d5a59"

rm -f "${GOBIN}/${1}"* || true

ORIGINAL_WORKDIR="$(pwd)"
TMP_DIR="${1}.tmp"

# Create TMP_DIR to download and unpack the govc tarball.
rm -r "${TMP_DIR}" || true
mkdir -p "${TMP_DIR}"
cd "${TMP_DIR}"

# Download govc
GOVC_FILE_NAME="govc_${GOVC_OS}_${GOVC_ARCH}.tar.gz"

GOVC_SHA256_VAR="GOVC_SHA256_${GOVC_OS}_${GOVC_ARCH}"
GOVC_SHA256="${!GOVC_SHA256_VAR:?no known sha256 for govc ${2} on ${GOVC_OS}/${GOVC_ARCH}, add it to $0}"

download_and_verify "https://github.com/vmware/govmomi/releases/download/${2}/${GOVC_FILE_NAME}" "${GOVC_SHA256}" "${GOVC_FILE_NAME}"
tar -xvzf "${GOVC_FILE_NAME}" govc
mv govc "${GOBIN}/${1}-${2}"

# Get back to the original directory and cleanup the temporary directory.
cd "${ORIGINAL_WORKDIR}"
rm -r "${TMP_DIR}"

# Link the unversioned name to the versioned binary.
ln -sf "${GOBIN}/${1}-${2}" "${GOBIN}/${1}"
