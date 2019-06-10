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

################################################################################
# usage: ensure-tools.sh [FLAGS]
#  This program ensures the tools required for building CAPV are present.
################################################################################

set -o errexit
set -o nounset
set -o pipefail

# Go is required.
if ! command -v go >/dev/null 2>&1; then
  echo "Golang binary must be in \$PATH" 1>&2
  exit 1
fi
GOHOSTOS="$(go env GOHOSTOS)"
GOHOSTARCH="$(go env GOHOSTARCH)"

# Run at the project's root directory.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Ensure the BIN_DIR is created.
BIN_DIR="hack/.bin"
mkdir -p "${BIN_DIR}"

ensure_kustomize() {
    echo "ensure-tools: kustomize"

    local _lcl_bin="${BIN_DIR}/kustomize"
    if [ -f "${_lcl_bin}" ]; then return 0; fi

    local _version=2.0.0
    local _url_bin="kustomize_${_version}_${GOHOSTOS}_${GOHOSTARCH}"
    if [ "${GOHOSTOS}" = "windows" ]; then
        _url_bin="${_url_bin}.exe"
    fi
    curl -L -o "${_lcl_bin}" "https://github.com/kubernetes-sigs/kustomize/releases/download/v${_version}/${_url_bin}"
    chmod 0755 "${_lcl_bin}"
}

ensure_envsubst() {
  echo "ensure-tools: envsubst"

  if [ ! envsubst --help 2>/dev/null ]; then
    echo "envsubst must be installed, see https://stackoverflow.com/a/23622446 for install steps"
    exit 1
  fi
}

ensure_kustomize
ensure_envsubst
