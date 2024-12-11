#!/usr/bin/env bash

# Copyright 2024 The Kubernetes Authors.
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

PR_NUMBER="${1:-}"

if [ -z "${PR_NUMBER}" ]; then
  echo "PR_NUMBER must be set"
  exit 1
fi

gh pr view "${PR_NUMBER}" \
  -R kubernetes-sigs/cluster-api-provider-vsphere \
  --json headRepository,headRepositoryOwner,headRefName,baseRefName \
  -q '"https://github.com/team-cluster-api/cluster-api-provider-vsphere/compare/" + .baseRefName + "..." + .headRepositoryOwner.login + ":" + .headRepository.name + ":" + .headRefName'
