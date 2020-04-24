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

# If you update this file, please follow
# https://suva.sh/posts/well-documented-makefiles

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL := /usr/bin/env bash

.DEFAULT_GOAL := help

VERSION ?= $(shell cat clusterctl-settings.json | jq .config.nextVersion -r)

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq (,$(strip $(GOPROXY)))
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE := on

# Directories
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := $(ROOT_DIR)/bin
TOOLS_DIR := $(ROOT_DIR)/hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

# Binaries
MANAGER := $(BIN_DIR)/manager
CLUSTERCTL := $(BIN_DIR)/clusterctl

# Tooling binaries
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/controller-gen)
CONVERSION_GEN := $(TOOLS_BIN_DIR)/conversion-gen
GINKGO := $(TOOLS_BIN_DIR)/ginkgo
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
GOVC := $(TOOLS_BIN_DIR)/govc
KIND := $(TOOLS_BIN_DIR)/kind
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize
TOOLING_BINARIES := $(CONTROLLER_GEN) $(CONVERSION_GEN) $(GINKGO) $(GOLANGCI_LINT) $(GOVC) $(KIND) $(KUSTOMIZE)

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= ./config
CRD_ROOT ?= $(MANIFEST_ROOT)/crd/bases
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac
GC_KIND ?= true
RELEASE_DIR := out
BUILD_DIR := .build
OVERRIDES_DIR := $(HOME)/.cluster-api/overrides/infrastructure-vsphere/$(VERSION)
E2E_ARGS := --e2e.sshAuthKey="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDCbQg7ywSD1oJAwAHhQuemrL6C9wvOIgE7wfZ0PfqolcTEQbLbv7Zxe1/TRzr4B20pb6GMryJ7O3SH9kuCubDYQ4Atw9MF/iAtsg0s3Xs4a3RAqoeaTHA0u401Um27ANDVqLswdTZ0J0Ev+XqRCEgPX+IpGgiNOyiHxfIgwdev/fG1MmMEyCKj8JNlRghFnleBcE+N3Mu0rKb88ascch2mKLY5fGDwbnC3V7d6LE6jWVT5HV391N4IWmjoBjlBt3mfzNslWqJUS+PxRbYR3i7vyVrpb/mkw1YG1jeomTAmkx4kwiV7hSzNVF6pKNIOoB1mpwULJ0VL+UkM8IPEfVJb root@9ae81f510a14"

# Architecture variables
ARCH ?= amd64
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Common docker variables
IMAGE_NAME ?= manager
PULL_POLICY ?= Always
# Hosts running SELinux need :z added to volume mounts
SELINUX_ENABLED := $(shell cat /sys/fs/selinux/enforce 2> /dev/null || echo 0)

ifeq ($(SELINUX_ENABLED),1)
  DOCKER_VOL_OPTS?=:z
endif


# Release docker variables
RELEASE_REGISTRY := gcr.io/cluster-api-provider-vsphere/release
RELEASE_CONTROLLER_IMG := $(RELEASE_REGISTRY)/$(IMAGE_NAME)

# Development Docker variables
DEV_REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
DEV_CONTROLLER_IMG ?= $(DEV_REGISTRY)/vsphere-$(IMAGE_NAME)
DEV_TAG ?= dev
DEV_MANIFEST_IMG := $(DEV_CONTROLLER_IMG)-$(ARCH)


## --------------------------------------
## Help
## --------------------------------------

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: test
test: generate lint-go ## Run tests
	go test -v ./api/... ./controllers/... ./pkg/...

.PHONY: e2e-image
e2e-image: ## Build the e2e manager image
	docker build --tag="capv-manager:e2e" .

.PHONY: e2e
e2e: e2e-image
e2e: $(GINKGO) $(KUSTOMIZE) $(KIND) $(GOVC) ## Run e2e tests
	@echo PATH=$(PATH)
	@echo
	@echo Contents of $(TOOLS_BIN_DIR):
	@ls $(TOOLS_BIN_DIR)
	@echo
	time $(GINKGO) -v ./test/e2e -- --e2e.config="$(abspath test/e2e/e2e.conf)" --e2e.teardownKind=$(GC_KIND) $(E2E_ARGS)

## --------------------------------------
## Binaries
## --------------------------------------

.PHONY: $(MANAGER)
manager: $(MANAGER) ## Build manager binary
$(MANAGER): generate
	go build -o $@ -ldflags '-extldflags "-static" -w -s'

.PHONY: $(CLUSTERCTL)
clusterctl: $(CLUSTERCTL) ## Build clusterctl binary
$(CLUSTERCTL): go.mod
	go build -o $@ sigs.k8s.io/cluster-api/cmd/clusterctl

## --------------------------------------
## Tooling Binaries
## --------------------------------------
tools: $(TOOLING_BINARIES) ## Build tooling binaries
.PHONY: $(TOOLING_BINARIES)
$(TOOLING_BINARIES):
	make -C $(TOOLS_DIR) $(@F)

## --------------------------------------
## Linting and fixing linter errors
## --------------------------------------

.PHONY: lint
lint: ## Run all the lint targets
	$(MAKE) lint-go-full
	$(MAKE) lint-markdown
	$(MAKE) lint-shell

GOLANGCI_LINT_FLAGS ?= --fast=true
.PHONY: lint-go
lint-go: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_FLAGS)

.PHONY: lint-go-full
lint-go-full: GOLANGCI_LINT_FLAGS = --fast=false
lint-go-full: lint-go ## Run slower linters to detect possible issues

.PHONY: lint-markdown
lint-markdown: ## Lint the project's markdown
	docker run --rm -v "$$(pwd)":/build$(DOCKER_VOL_OPTS) gcr.io/cluster-api-provider-vsphere/extra/mdlint:0.17.0 -- /md/lint -i vendor -i contrib/haproxy/openapi .

.PHONY: lint-shell
lint-shell: ## Lint the project's shell scripts
	docker run --rm -t -v "$$(pwd)":/build:ro gcr.io/cluster-api-provider-vsphere/extra/shellcheck

.PHONY: fix
fix: GOLANGCI_LINT_FLAGS = --fast=false --fix
fix: lint-go ## Tries to fix errors reported by lint-go-full target

## --------------------------------------
## Generate
## --------------------------------------

.PHONY: modules
modules: ## Runs go mod to ensure proper vendoring
	go mod tidy
	cd $(TOOLS_DIR); go mod tidy

.PHONY: generate
generate: ## Generate code
	$(MAKE) generate-go
	$(MAKE) generate-manifests

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) $(CONVERSION_GEN) ## Runs Go related generate targets
	go generate ./...
	$(CONTROLLER_GEN) \
		paths=./api/... \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt

	$(CONVERSION_GEN) \
		--input-dirs=./api/v1alpha2 \
		--output-file-base=zz_generated.conversion \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) \
		paths=./api/... \
		crd:crdVersions=v1 \
		output:crd:dir=$(CRD_ROOT) \
		output:webhook:dir=$(WEBHOOK_ROOT) \
		webhook
	$(CONTROLLER_GEN) \
		paths=./controllers/... \
		output:rbac:dir=$(RBAC_ROOT) \
		rbac:roleName=manager-role

## --------------------------------------
## Release
## --------------------------------------

$(RELEASE_DIR):
	@mkdir -p $(RELEASE_DIR)


$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

$(OVERRIDES_DIR):
	@mkdir -p $(OVERRIDES_DIR)

.PHONY: dev-version-check
dev-version-check:
ifndef VERSION
	$(error VERSION must be set)
endif

.PHONY: release-version-check
release-version-check:
ifeq ($(VERSION), 0.0.0)
	$(error VERSION must be >0.0.0 for release)
endif

.PHONY: release-manifests
release-manifests:
	$(MAKE) manifests STAGE=release MANIFEST_DIR=$(RELEASE_DIR) PULL_POLICY=IfNotPresent IMAGE=$(RELEASE_CONTROLLER_IMG):$(VERSION)

.PHONY: release-overrides
release-overrides:
	$(MAKE) manifests STAGE=release MANIFEST_DIR=$(OVERRIDES_DIR) PULL_POLICY=IfNotPresent IMAGE=$(RELEASE_CONTROLLER_IMG):$(VERSION)

.PHONY: dev-manifests
dev-manifests:
	$(MAKE) manifests STAGE=dev MANIFEST_DIR=$(OVERRIDES_DIR) PULL_POLICY=Always IMAGE=$(DEV_CONTROLLER_IMG):$(DEV_TAG)

.PHONY: manifests
manifests:  $(STAGE)-version-check $(STAGE)-flavors $(MANIFEST_DIR) $(BUILD_DIR) $(KUSTOMIZE)
	rm -rf $(BUILD_DIR)/config
	cp -R config $(BUILD_DIR)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' $(BUILD_DIR)/config/manager/manager_pull_policy.yaml
	sed -i'' -e 's@image: .*@image: '"$(IMAGE)"'@' $(BUILD_DIR)/config/manager/manager_image_patch.yaml
	"$(KUSTOMIZE)" build $(BUILD_DIR)/config > $(MANIFEST_DIR)/infrastructure-components.yaml

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: flavors
flavors: $(FLAVOR_DIR)
	go run ./packaging/flavorgen -f multi-host > $(FLAVOR_DIR)/cluster-template.yaml

.PHONY: release-flavors ## Create release flavor manifests
release-flavors: release-version-check
	$(MAKE) flavors FLAVOR_DIR=$(RELEASE_DIR)

.PHONY: dev-flavors ## Create release flavor manifests
dev-flavors:
	$(MAKE) flavors FLAVOR_DIR=$(OVERRIDES_DIR)

.PHONY: overrides ## Generates flavors as clusterctl overrides
overrides: version-check $(OVERRIDES_DIR)
	go run ./packaging/flavorgen -f multi-host > $(OVERRIDES_DIR)/cluster-template.yaml

.PHONY: clean
clean: ## Run all the clean targets
	$(MAKE) clean-bin
	$(MAKE) clean-temporary
	$(MAKE) clean-release
	$(MAKE) clean-examples
	$(MAKE) clean-build

.PHONY: clean-build
clean-build:
	rm -rf $(BUILD_DIR)

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf bin
	$(MAKE) -C $(TOOLS_DIR) clean

.PHONY: clean-temporary
clean-temporary: ## Remove all temporary files and folders
	rm -f minikube.kubeconfig
	rm -f kubeconfig

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: clean-examples
clean-examples: ## Remove all the temporary files generated in the examples folder
	rm -rf examples/_out/
	rm -f examples/provider-components/provider-components-*.yaml

.PHONY: verify
verify: ## Runs all the verify targets
	$(MAKE) verify-boilerplate
	$(MAKE) verify-crds

.PHONY: verify-boilerplate
verify-boilerplate: ## Verifies all sources have appropriate boilerplate
	./hack/verify-boilerplate.sh

.PHONY: verify-crds
verify-crds: ## Verifies the committed CRDs are up-to-date
	./hack/verify-crds.sh

## --------------------------------------
## Check
## --------------------------------------

.PHONY: check
check: ## Verify and lint the project
	$(MAKE) verify
	$(MAKE) lint

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-build
docker-build: ## Build the docker image for controller-manager
	docker build --pull --build-arg ARCH=$(ARCH)  . -t $(DEV_CONTROLLER_IMG):$(DEV_TAG)

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(DEV_CONTROLLER_IMG):$(DEV_TAG)
