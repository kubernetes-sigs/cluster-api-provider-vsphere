#!/usr/bin/env bash

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

# MIN_GO_VERSION is the minimum, supported Go version.
# Note: Enforce only the minor version as we can't guarantee that
# the images we use in ProwJobs already use the latest patch version.
MIN_GO_VERSION="go${MIN_GO_VERSION:-1.20}"

# Ensure the go tool exists and is a viable version.
verify_go_version() {
  if ! command -v go >/dev/null 2>&1; then
    cat <<EOF
Can't find 'go' in PATH, please fix and retry.
See http://golang.org/doc/install for installation instructions.
EOF
    return 2
  fi

  local go_version
  IFS=" " read -ra go_version <<<"$(go version)"
  if [ "${go_version[2]}" != 'devel' ] && \
     [ "${MIN_GO_VERSION}" != "$(printf "%s\n%s" "${MIN_GO_VERSION}" "${go_version[2]}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1)" ]; then
    cat <<EOF
Detected go version: ${go_version[*]}.
Kubernetes requires ${MIN_GO_VERSION} or greater.
Please install ${MIN_GO_VERSION} or later.
EOF
    return 2
  fi
}

verify_go_version

# Explicitly opt into go modules.
export GO111MODULE=on
