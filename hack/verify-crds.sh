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
# usage: verify-crds.sh
#  This program ensures the commited CRDs match generated CRDs
################################################################################

set -o errexit
set -o nounset
set -o pipefail

# Change directories to the parent directory of the one in which this
# script is located.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

_diff_log="$(mktemp)"
_output_dir="$(mktemp -d)"

echo "verify-crds: generating crds"
  go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go \
    crd \
    --output-dir "${_output_dir}"

_exit_code=0
echo "verify-crds: comparing crds"
echo
for f in $(/bin/ls "${_output_dir}"); do
  diff "${_output_dir}/${f}" "./config/crds/${f}" >"${_diff_log}" || _exit_code="${?}"
  if [ "${_exit_code}" -ne "0" ]; then
    echo "${f}" 1>&2
    cat "${_diff_log}" 1>&2
    echo 1>&2
  fi
done

exit "${_exit_code}"
