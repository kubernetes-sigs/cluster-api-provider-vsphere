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

# If you update this file, please follow
# https://suva.sh/posts/well-documented-makefiles

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL := /usr/bin/env bash

.DEFAULT_GOAL := help

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq (,$(strip $(GOPROXY)))
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE := on

# Directories.
BIN_DIR := bin
SRCS := go.mod go.sum

# Binaries.
CONTROLLER_GEN := $(BIN_DIR)/controller-gen
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint
KUSTOMIZE := $(BIN_DIR)/kustomize
CONVERSION_GEN := $(BIN_DIR)/conversion-gen
KIND := $(BIN_DIR)/kind
GOVC := $(BIN_DIR)/govc
GINKGO := $(BIN_DIR)/ginkgo
CONVERSION_VERIFIER := $(BIN_DIR)/conversion-verifier

## --------------------------------------
## Help
## --------------------------------------

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Binaries
## --------------------------------------

controller-gen: $(CONTROLLER_GEN) $(SRCS) ## Build controller-gen
$(CONTROLLER_GEN): go.mod
	go build -tags=tools -o $@ sigs.k8s.io/controller-tools/cmd/controller-gen

conversion-gen: $(CONVERSION_GEN) $(SRCS) ## Build conversion-gen
$(CONVERSION_GEN): go.mod
	go build -tags=tools -o $@ k8s.io/code-generator/cmd/conversion-gen

ginkgo: $(GINKGO) $(SRCS)
$(GINKGO): go.mod
	go build -tags=tools -o $@ github.com/onsi/ginkgo/ginkgo

golangci-lint: $(GOLANGCI_LINT) $(SRCS) ## Build golangci-lint
$(GOLANGCI_LINT): go.mod
	go build -tags=tools -o $@ github.com/golangci/golangci-lint/cmd/golangci-lint

govc: $(GOVC) $(SRCS)
$(GOVC): go.mod
	go build -tags=tools -o $@ github.com/vmware/govmomi/govc

kind: $(KIND) $(SRCS)
$(KIND): go.mod
	go build -tags=tools -o $@ sigs.k8s.io/kind

kustomize: $(KUSTOMIZE) $(SRCS) ## Build kustomize
$(KUSTOMIZE): go.mod
	go build -tags=tools -o $@ sigs.k8s.io/kustomize/kustomize/v4

conversion-verifier: $(CONVERSION_VERIFIER) $(SRCS) ## fetch CAPI's conversion verifier
$(CONVERSION_VERIFIER): go.mod
	go build -tags=tools -o $@ sigs.k8s.io/cluster-api/hack/tools/conversion-verifier

## --------------------------------------
## Generate
## --------------------------------------

.PHONY: modules
modules: ## Runs go mod to ensure proper vendoring
	go mod tidy

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: clean
clean: ## Run all the clean targets
	$(MAKE) clean-bin

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf bin
