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

KUSTOMIZE_VERSION ?= 2.0.0
IMAGE_NAME ?= akutz/kustomize
IMAGE_TAG ?= $(IMAGE_NAME):$(KUSTOMIZE_VERSION)

build:
	docker build --build-arg KUSTOMIZE_VERSION=$(KUSTOMIZE_VERSION) -t $(IMAGE_TAG) .
.PHONY: build

push:
	docker push $(IMAGE_TAG)
.PHONY: push
