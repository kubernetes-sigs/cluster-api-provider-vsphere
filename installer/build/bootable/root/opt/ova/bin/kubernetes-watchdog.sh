#!/usr/bin/bash

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

set -euf -o pipefail

DCUI_SOCKET="UNIX:/var/run/dcui.sock"

function log_to_dcui() {
  local msg=$1
  echo -n "$msg" | socat $DCUI_SOCKET -
}

function check_api_endpoint() {
  local interval=$1
  local max_retries=$2
  local is_ready=$3
  local retries=0
  local JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
  while [ $retries -lt "$max_retries" ]; do
    if kubectl --kubeconfig=/etc/kubernetes/admin.conf get nodes -o jsonpath="$JSONPATH" | grep "Ready=${is_ready}"; then
      return 0
    fi
    (( retries=retries+1 ))
    sleep "$interval"
  done
  return 1
}

# If kubelet is running and ready, report that is running and quit
if check_api_endpoint 2 5 True; then
  log_to_dcui "[RUNNING](fg-green)"
  exit 0
fi

# If kubelet is running and ready, report that is running and quit
if check_api_endpoint 2 5 False; then
  log_to_dcui "[NODE NOT READY](fg-red)"
  exit 0
fi
