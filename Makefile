# Copyright 2018 The Kubernetes Authors.
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

all: build test

# Store the original working directory.
CWD := $(abspath .)

# Ensure that the Makefile targets execute from the GOPATH due to the K8s tools
# failing if executed outside the GOPATH. This work-around:
#   1. Creates a sub-directory named ".gopath" to act as a new GOPATH
#   2. Symlinks the current directory into ".gopath/src/sigs.k8s.io/cluster-api-provider-vsphere"
#   3. Sets the Makefile's SHELL variable to "hack/shell-with-gopath.sh" to
#      cause all sub-shells opened by this Makefile to execute from inside the
#      nested GOPATH.
SHELL := hack/shell-with-gopath.sh

# Image URL to use all building/pushing image targets
PRODUCTION_IMG ?= gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:0.3.0-alpha.1
CI_IMG ?= gcr.io/cnx-cluster-api/vsphere-cluster-api-provider
CLUSTERCTL_CI_IMG ?= gcr.io/cnx-cluster-api/clusterctl
DEV_IMG ?= # <== NOTE:  outside dev, change this!!!

# Retrieves the git hash
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
	   git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

# Build manager binary
manager: fmt vet
	go build -o bin/manager ./cmd/manager

# Build the clusterctl binary
clusterctl: fmt vet
	go build -o bin/clusterctl ./cmd/clusterctl

clusterctl-in-docker:
	docker run --rm -v $(CWD):/go/src/sigs.k8s.io/cluster-api-provider-vsphere \
	  -w /go/src/sigs.k8s.io/cluster-api-provider-vsphere \
	  -e GOOS -e GOHOSTOS golang:1.12 \
	  go build -o bin/clusterctl ./cmd/clusterctl
.PHONY: clusterctl-in-docker

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	hack/update-generated.sh crd rbac

# Run go fmt against code
fmt: | generate
ifneq (,$(strip $(shell command -v goformat 2>/dev/null)))
	goformat -s -w ./pkg ./cmd
else
	go fmt ./pkg/... ./cmd/...
endif

# Run go vet against code
vet: | generate
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	hack/update-generated.sh codegen

# Regenerating vendor cannot happen in a symlink due to the way certain Go
# commands traverse the file structure. Fore more information please see
# https://github.com/golang/go/issues/17451.
vendor: export SHELL_WITH_GOPATH=0
vendor: export GO111MODULE=on
vendor:
	go mod tidy -v
	go mod vendor -v
	go mod verify
	_src="$$(go list -f '{{.Dir}}' sigs.k8s.io/cluster-api/cmd/clusterctl 2>/dev/null)/../../config" && \
	_dst=./vendor/sigs.k8s.io/cluster-api/ && \
	mkdir -p "$${_dst}" && \
	cp -rf --no-preserve=mode "$${_src}" "$${_dst}"
.PHONY: vendor

####################################
# DEVELOPMENT Build and Push targets
####################################

# Create YAML file for deployment
dev-yaml:
	CAPV_MANAGER_IMAGE=$(DEV_IMG) hack/generate-yaml.sh

# Build the docker image
dev-build: #test
	docker build . -t $(DEV_IMG)

# Push the docker image
dev-push:
	docker push $(DEV_IMG)

.PHONY: dev-yaml dev-build dev-push

###################################
# PRODUCTION Build and Push targets
###################################

# Create YAML file for deployment
prod-yaml:
	CAPV_MANAGER_IMAGE=$(PRODUCTION_IMG) hack/generate-yaml.sh

# Build the docker image
prod-build: test
	docker build . -t $(PRODUCTION_IMG)

# Push the docker image
prod-push:
	@echo "logging into gcr.io registry with key file"
	@docker login -u _json_key --password-stdin gcr.io <"$(GCR_KEY_FILE)"
	docker push $(PRODUCTION_IMG)

.PHONY: prod-yaml prod-build prod-push

###################################
# CI Build and Push targets
###################################

# Create YAML file for deployment into CI
ci-yaml:
	CAPV_MANAGER_IMAGE=$(CI_IMG) hack/generate-yaml.sh

ci-image: generate fmt vet manifests
	docker build . -t "$(CI_IMG):$(VERSION)"
	docker build . -f cmd/clusterctl/Dockerfile -t "$(CLUSTERCTL_CI_IMG):$(VERSION)"

ci-push: ci-image
# Log into the registry with a service account file.  In CI, GCR_KEY_FILE contains the content and not the file name.
	@echo "logging into gcr.io registry with key file"
	@echo $$GCR_KEY_FILE | docker login -u _json_key --password-stdin gcr.io
	docker push "$(CI_IMG):$(VERSION)"
	docker push "$(CLUSTERCTL_CI_IMG):$(VERSION)"
	@echo docker logout gcr.io

.PHONY: ci-yaml ci-image ci-push

################################################################################
##                          The default targets                               ##
################################################################################

# The default build target
build: manager clusterctl
.PHONY: build

# The default test target
test: build manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out
.PHONY: test
