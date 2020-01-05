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

set -o errexit
set -o nounset
set -o pipefail

run_openvpn() {
  [ "${#}" -eq "0" ] && exec openvpn --config "$(/bin/ls ./*.ovpn)"
  exec openvpn "${@}"
}

[ "${#}" -eq "0" ] && run_openvpn

{ [ "${1}" = "/bin/bash" ] || \
  [ "${1}" = "bash" ] || \
  [ "${1}" = "/bin/sh" ] || \
  [ "${1}" = "sh" ] || \
  [ "${1}" = "shell" ]; } && exec /bin/bash

run_openvpn "${@}"
