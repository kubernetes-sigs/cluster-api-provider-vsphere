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
GOVC_SHA256_Darwin_arm64="559430d7691c98172b6b137337cb99c8e0c822512e0ddd170f19bea63cc95e15"
# shellcheck disable=SC2034
GOVC_SHA256_Darwin_x86_64="0c0b1bace57542574584d4e76d097fb81d003c07380469d96bacc1f8eb640042"
# shellcheck disable=SC2034
GOVC_SHA256_Linux_arm64="d813c8bff6f4410332fac97b368235504ecabfcc2baa1aae38bcc868280850e0"
# shellcheck disable=SC2034
GOVC_SHA256_Linux_x86_64="beabfa250fb91f1a9687586448a24c24fd4f95324fa905d157241d0c9efcfeb0"

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
