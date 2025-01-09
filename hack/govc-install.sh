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

rm -f "${GOBIN}/${1}"* || true

ORIGINAL_WORKDIR="$(pwd)"
TMP_DIR="${1}.tmp"

# Create TMP_DIR to download and unpack the govc tarball.
rm -r "${TMP_DIR}" || true
mkdir -p "${TMP_DIR}"
cd "${TMP_DIR}"

# Download govc

wget "https://github.com/vmware/govmomi/releases/download/${2}/govc_${GOVC_OS}_${GOVC_ARCH}.tar.gz"
tar -xvzf "govc_${GOVC_OS}_${GOVC_ARCH}.tar.gz" govc
mv govc "${GOBIN}/${1}-${2}"

# Get back to the original directory and cleanup the temporary directory.
cd "${ORIGINAL_WORKDIR}"
rm -r "${TMP_DIR}"

# Link the unversioned name to the versioned binary.
ln -sf "${GOBIN}/${1}-${2}" "${GOBIN}/${1}"
