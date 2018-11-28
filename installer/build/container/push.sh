#!/bin/sh
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
set -eu

OVA_REV=$(git rev-parse --verify --short=8 HEAD)
IMAGE="cluster-api-ova-build"
# TODO(frapposelli): find a better home for the build container
REPO="docker.io/frapposelli/"

# `docker build` the build container
docker build --pull --force-rm --no-cache -t "$IMAGE:$OVA_REV" -f build/container/Dockerfile .

# tag the build container with latest and a commit hash
docker tag "$IMAGE:$OVA_REV" "$REPO$IMAGE:latest"
docker tag "$IMAGE:$OVA_REV" "$REPO$IMAGE:$OVA_REV"

# push both container tags using gcloud for auth
# gcloud docker -- push "$REPO$IMAGE:latest"
# gcloud docker -- push "$REPO$IMAGE:$OVA_REV"
