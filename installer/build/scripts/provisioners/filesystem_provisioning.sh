#!/usr/bin/bash
# Copyright 2018 VMware, Inc. All Rights Reserved.
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

if [ -z "${BUILD_OVA_REVISION}" ]; then
  echo "BUILD_OVA_REVISION must be set"
  exit 1
fi

# Create directories
mkdir -p /usr/lib/systemd/system/getty@tty2.service.d
mkdir -p /root/.ssh

# Write version files
echo "appliance=${BUILD_OVA_REVISION}" > /etc/vmware/version
