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

all: build

VERSION ?= $(shell git describe --always --dirty)
IMAGE_NAME ?= gcr.io/cluster-api-provider-vsphere/extra/manifests-base
IMAGE_TAG ?= $(IMAGE_NAME):$(VERSION)
KUSTOMIZE_VERSION ?= 2.0.0

build:
	docker build --build-arg KUSTOMIZE_VERSION=$(KUSTOMIZE_VERSION) -t $(IMAGE_TAG) .
	docker tag $(IMAGE_TAG) $(IMAGE_NAME):latest
.PHONY: build

push:
	docker push $(IMAGE_TAG)
	docker push $(IMAGE_NAME):latest
.PHONY: push
