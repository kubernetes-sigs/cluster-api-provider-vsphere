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
# usage: update-generated.sh
#  This program ensures the generated code for this project is up-to-date.
################################################################################

set -o errexit
set -o nounset
set -o pipefail

# Change directories to the parent directory of the one in which this
# script is located.
cd "$(dirname "${BASH_SOURCE[0]}")/.."

gen-crds() {
  local out="${1-./config/crds}"
  echo "update-generated: gen-crds out=${out}"
  go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go \
    crd \
    --output-dir "${out}"
}

gen-deepcopy() {
  local pkg="${1-}"
  echo "update-generated: deepcopy pkg=${pkg}"
  go run vendor/k8s.io/code-generator/cmd/deepcopy-gen/main.go \
    --go-header-file   hack/boilerplate.go.txt \
    --input-dirs       "${pkg}" \
    --output-file-base zz_generated.deepcopy \
    --bounding-dirs    "${pkg}" \
    -v "${DEEPCOPY_LOG_LEVEL:-1}"
}

gen-rbac() {
  local out="${1-./config/rbac}"
  echo "update-generated: gen-rbac out=${out}"
  go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go \
    rbac \
    --output-dir "${out}"
}

deepcopy-v1alpha1() {
  gen-deepcopy ./pkg/apis/vsphereproviderconfig/v1alpha1
}

deepcopy() { deepcopy-v1alpha1; }
codegen()  { deepcopy; }
crd()      { gen-crds ./config/crds; }
rbac()     { gen-rbac ./config/rbac; }
all()      { codegen && crd && rbac; }

[ "${#}" -eq "0" ] && all

while [ "${#}" -gt "0" ]; do
  case "${1}" in
  all|codegen|deepcopy|crd|rbac)
    eval "${1}"
    shift
    ;;
  *)
    echo "invalid arg: ${1}" 1>&2
    exit 1
    ;;
  esac
done
