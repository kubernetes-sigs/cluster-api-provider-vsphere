#!/usr/bin/bash
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

# Checks for SSH host keys and generates them if necessary
if [ -f /etc/ssh/ssh_host_key ]; then
  echo 'Keys found.'
else
  ssh-keygen -A
fi

SSH_KEY="$(ovfenv --key appliance.root_ssh_key)"
if echo $SSH_KEY | ssh-keygen -l -f - ; then
  mkdir /root/.ssh
  echo $SSH_KEY >> /root/.ssh/authorized_keys
  # remove all default settings
  sed -i "/^PermitRootLogin.*/d" /etc/ssh/sshd_config
  # permit root login with SSH key
  echo "PermitRootLogin without-password" >> /etc/ssh/sshd_config
fi