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

# Fetch current local Kubernetes version
KUBEVERSION=$(kubectl version --client -o json | jq -r .clientVersion.gitVersion)
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
  echo "max retry has passed... exiting..."
  exit 1
}


# Pull images
log_to_dcui "[PULLING IMAGES](fg-yellow)"
kubeadm config images pull --kubernetes-version "$KUBEVERSION"
log_to_dcui "[INITIALIZING](fg-yellow)"
kubeadm init --pod-network-cidr=10.244.0.0/16 --kubernetes-version "$KUBEVERSION"

# Wait for API server to respond
log_to_dcui "[STARTING](fg-yellow)"
# Check if the node is up and Not Ready
check_api_endpoint 1 100 False

# Deploy flannel
log_to_dcui "[DEPLOYING NETWORK](fg-yellow)"
kubectl --kubeconfig=/etc/kubernetes/admin.conf apply -f https://raw.githubusercontent.com/coreos/flannel/bc79dd1505b0c8681ece4de4c0d86c5cd2643275/Documentation/kube-flannel.yml

log_to_dcui "[CONFIGURING](fg-yellow)"
# Set up kubeconfig for root user
mkdir -p "/root/.kube"
cp -i /etc/kubernetes/admin.conf "/root/.kube/config"
chown "$(id -u)":"$(id -g)" "/root/.kube/config"

# Check if the node is up and Ready
check_api_endpoint 1 100 True
log_to_dcui "[RUNNING](fg-green)"

# Set kubernetes provisioning datetime
date -u +"%Y-%m-%dT%H:%M:%SZ" > /etc/vmware/kubeadm.provisioned
