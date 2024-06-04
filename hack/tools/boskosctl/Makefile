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

#
# Go.
#
GO_VERSION ?= 1.22.3
GO_CONTAINER_IMAGE ?= docker.io/library/golang:$(GO_VERSION)

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

all: build

VERSION ?= $(shell git describe --always --dirty)
IMAGE_NAME ?= gcr.io/k8s-staging-capi-vsphere/extra/boskosctl
IMAGE_TAG ?= $(IMAGE_NAME):$(VERSION)

build:
	cd ../../../; DOCKER_BUILDKIT=1 docker build --build-arg builder_image=docker.io/library/golang:$(GO_VERSION) --build-arg goproxy=$(GOPROXY) . -t $(IMAGE_TAG) --file hack/tools/boskosctl/Dockerfile
	docker tag $(IMAGE_TAG) $(IMAGE_NAME):latest
.PHONY: build

push:
	docker push $(IMAGE_TAG)
	docker push $(IMAGE_NAME):latest
.PHONY: push
