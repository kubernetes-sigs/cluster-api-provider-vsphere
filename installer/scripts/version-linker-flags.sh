#!/bin/bash
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
#
set -e -o pipefail +h && [ -n "$DEBUG" ] && set -x

echo "-s -w \
    -X sigs.k8s.io/cluster-api-provider-vsphere/installer/pkg/version.Version=${TAG} \
    -X sigs.k8s.io/cluster-api-provider-vsphere/installer/pkg/version.BuildNumber=\"${BUILD_NUMBER}\" \
    -X sigs.k8s.io/cluster-api-provider-vsphere/installer/pkg/version.BuildDate=`date -u +%Y/%m/%d@%H:%M:%S` \
    -X sigs.k8s.io/cluster-api-provider-vsphere/installer/pkg/version.GitCommit=`git rev-parse --short HEAD` \
    -X sigs.k8s.io/cluster-api-provider-vsphere/installer/pkg/version.State=` \
      if [[ -n $(git ls-files --others --exclude-standard) || \
            ! $(git diff-files --no-ext-diff --quiet) || \
            ! $(git diff-index --no-ext-diff --quiet --cached HEAD) \
     ]]; then echo 'dirty'; fi`"
