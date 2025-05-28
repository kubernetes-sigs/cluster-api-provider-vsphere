#!/bin/bash

# Copyright 2020 The Kubernetes Authors.
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

set -o errexit  # exits immediately on any unexpected error (does not bypass traps)
set -o nounset  # will error if variables are used without first being defined
set -o pipefail # any non-zero exit code in a piped command causes the pipeline to fail with that code

# Fallback for mirror-prow.
if [[ "${GOVC_URL:-}" == "10.2.224.4" ]]; then
  export JANITOR_ARGS
  JANITOR_ARGS="--resource-type=vsphere-project-cluster-api-provider --resource-type=vsphere-project-cloud-provider --resource-type=vsphere-project-image-builder"
fi

# Sanitize input envvars to not contain newline
GOVC_USERNAME=$(echo "${GOVC_USERNAME}" | tr -d "\n")
GOVC_PASSWORD=$(echo "${GOVC_PASSWORD}" | tr -d "\n")
GOVC_URL=$(echo "${GOVC_URL}" | tr -d "\n")
VSPHERE_TLS_THUMBPRINT=$(echo "${VSPHERE_TLS_THUMBPRINT}" | tr -d "\n")
BOSKOS_HOST=$(echo "${BOSKOS_HOST}" | tr -d "\n")

# Run e2e tests
make clean-ci
