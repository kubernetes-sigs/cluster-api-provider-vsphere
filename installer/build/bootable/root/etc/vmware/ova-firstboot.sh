#!/usr/bin/env bash
# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
set -euf -o pipefail

declare -r mask="*******"

umask 077

function firstboot() {
  set +e
  local tmp
  tmp="$(ovfenv --key appliance.root_pwd)"
  if [[ "$tmp" == "$mask" ]]; then
    return
  fi

  echo "root:$tmp" | chpasswd
  # Reset password expiration to 90 days by default
  chage -d "$(date +"%Y-%m-%d")" -m 0 -M 90 root
  set -e

  # Set firstboot datetime
  date -u +"%Y-%m-%dT%H:%M:%SZ" > /etc/vmware/firstboot
}

function clearPrivate() {
  # We then obscure the root password, if the VM is reconfigured with another
  # password after deployment, we don't act on it and keep obscuring it.
  if [[ "$(ovfenv --key appliance.root_pwd)" != "$mask" ]]; then
    ovfenv --key appliance.root_pwd --set "$mask"
  fi
}

# Only run on first boot
if [[ ! -f /etc/vmware/firstboot ]]; then
  firstboot
fi
# Remove private values from ovfenv
clearPrivate