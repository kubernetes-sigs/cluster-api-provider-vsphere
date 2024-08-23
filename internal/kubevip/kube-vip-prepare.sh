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

set -e

# Configure the workaround required for kubeadm init with kube-vip:
# xref: https://github.com/kube-vip/kube-vip/issues/684

# Nothing to do for kubernetes < v1.29 or >= v1.31.
KUBEADM_MINOR="$(kubeadm version -o short | cut -d '.' -f 2)"
if [[ "$KUBEADM_MINOR" -lt "29" ]] || [[ "$KUBEADM_MINOR" -gt "30" ]]; then
  exit 0
fi

IS_KUBEADM_INIT="false"

# cloud-init kubeadm init
if [[ -f /run/kubeadm/kubeadm.yaml ]]; then
  IS_KUBEADM_INIT="true"
fi

# ignition kubeadm init
if [[ -f /etc/kubeadm.sh ]] && grep -q -e "kubeadm init" /etc/kubeadm.sh; then
  IS_KUBEADM_INIT="true"
fi

if [[ "$IS_KUBEADM_INIT" == "true" ]]; then
  sed -i 's#path: /etc/kubernetes/admin.conf#path: /etc/kubernetes/super-admin.conf#' \
    /etc/kubernetes/manifests/kube-vip.yaml
fi
