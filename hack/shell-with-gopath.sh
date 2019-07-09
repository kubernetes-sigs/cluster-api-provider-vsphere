#!/bin/sh

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

# If SHELL_WITH_GOPATH is 0 then do not change directories.
[ "${SHELL_WITH_GOPATH}" = "0" ] && exec /bin/sh "${@}"

export GOPATH="${PWD}/.gopath"

REPO_ORG=sigs.k8s.io
REPO_NAME=cluster-api-provider-vsphere
NEW_ROOT="${GOPATH}/src/${REPO_ORG}/${REPO_NAME}"

[ -L "${NEW_ROOT}" ] && [ ! -e "${NEW_ROOT}" ] && rm -fr "${GOPATH}"
[ -d "${GOPATH}/src/${REPO_ORG}" ] || mkdir -p "${GOPATH}/src/${REPO_ORG}" 1>/dev/null
[ -L "${NEW_ROOT}" ] || ln -s "${PWD}" "${NEW_ROOT}" >/dev/null
cd "${NEW_ROOT}" && exec /bin/sh "${@}"
