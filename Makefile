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

E2E_CONF_FILE  ?= "$(abspath test/e2e/config/vsphere-dev.yaml)"
INTEGRATION_CONF_FILE ?= "$(abspath test/integration/integration-dev.yaml)"
E2E_TEMPLATE_DIR := "$(abspath test/e2e/data/infrastructure-vsphere/)"

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
CONVERSION_VERIFIER := $(abspath $(TOOLS_BIN_DIR)/conversion-verifier)
TOOLING_BINARIES := $(CONTROLLER_GEN) $(CONVERSION_GEN) $(GINKGO) $(GOLANGCI_LINT) $(GOVC) $(KIND) $(KUSTOMIZE) $(CONVERSION_VERIFIER)
ARTIFACTS_PATH := $(ROOT_DIR)/_artifacts

# Set --output-base for conversion-gen if we are not within GOPATH
ifneq ($(abspath $(ROOT_DIR)),$(shell go env GOPATH)/src/sigs.k8s.io/cluster-api-provider-vsphere)
	OUTPUT_BASE := --output-base=$(ROOT_DIR)
endif

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= ./config
CRD_ROOT ?= $(MANIFEST_ROOT)/default/crd/bases
SUPERVISOR_CRD_ROOT ?= $(MANIFEST_ROOT)/supervisor/crd
VMOP_CRD_ROOT ?= $(MANIFEST_ROOT)/deployments/integration-tests/crds
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac
GC_KIND ?= true
RELEASE_DIR := out
BUILD_DIR := .build
OVERRIDES_DIR := $(HOME)/.cluster-api/overrides/infrastructure-vsphere/$(VERSION)

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

# Set build time variables including git version details
LDFLAGS := $(shell hack/version.sh)

## --------------------------------------
## Help
## --------------------------------------

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: test
test: $(GOVC)
	$(MAKE) generate lint-go
	source ./hack/fetch_ext_bins.sh; fetch_tools; setup_envs; export GOVC_BIN_PATH=$(GOVC); go test -v ./apis/... ./controllers/... ./pkg/...

.PHONY: e2e-image
e2e-image: ## Build the e2e manager image
	docker build --build-arg ldflags="$(LDFLAGS)" --tag="gcr.io/k8s-staging-cluster-api/capv-manager:e2e" .

.PHONY: e2e-templates
e2e-templates: ## Generate e2e cluster templates
	$(MAKE) release-manifests
	cp $(RELEASE_DIR)/cluster-template.yaml $(E2E_TEMPLATE_DIR)/kustomization/base/cluster-template.yaml
	"$(KUSTOMIZE)" build $(E2E_TEMPLATE_DIR)/kustomization/base > $(E2E_TEMPLATE_DIR)/cluster-template.yaml
	"$(KUSTOMIZE)" build $(E2E_TEMPLATE_DIR)/kustomization/remote-management > $(E2E_TEMPLATE_DIR)/cluster-template-remote-management.yaml

.PHONY: test-integration
test-integration: e2e-image
test-integration: $(GINKGO) $(KUSTOMIZE) $(KIND)
	time $(GINKGO) -v ./test/integration -- --config="$(INTEGRATION_CONF_FILE)" --artifacts-folder="$(ARTIFACTS_PATH)"

GINKGO_FOCUS ?=

.PHONY: e2e
e2e: e2e-image e2e-templates
e2e: $(GINKGO) $(KUSTOMIZE) $(KIND) $(GOVC) ## Run e2e tests
	@echo PATH="$(PATH)"
	@echo
	@echo Contents of $(TOOLS_BIN_DIR):
	@ls $(TOOLS_BIN_DIR)
	@echo
	time $(GINKGO) -v -skip="ClusterAPI Upgrade Tests" ./test/e2e -- --e2e.config="$(E2E_CONF_FILE)" --e2e.artifacts-folder="$(ARTIFACTS_PATH)"

.PHONY: e2e-upgrade
e2e-upgrade: e2e-image e2e-templates
e2e-upgrade: $(GINKGO) $(KUSTOMIZE) $(KIND) $(GOVC) ## Run only upgrade e2e tests
	@echo PATH="$(PATH)"
	@echo
	@echo Contents of $(TOOLS_BIN_DIR):
	@ls $(TOOLS_BIN_DIR)
	@echo
	time $(GINKGO) -v -focus="ClusterAPI Upgrade Tests" ./test/e2e -- --e2e.config="$(E2E_CONF_FILE)"
## --------------------------------------
## Binaries
## --------------------------------------

.PHONY: $(MANAGER)
manager: $(MANAGER) ## Build manager binary
$(MANAGER): generate
	go build -o $@ -ldflags "$(LDFLAGS) -extldflags '-static' -w -s"

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
	docker run --rm -v "$$(pwd)":/build$(DOCKER_VOL_OPTS) gcr.io/cluster-api-provider-vsphere/extra/mdlint:0.23.2 -- /md/lint -i vendor -i contrib/haproxy/openapi .

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
		paths=./apis/... \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt

	$(CONVERSION_GEN) \
		--input-dirs=./apis/v1alpha3 \
		--input-dirs=./apis/v1alpha4 \
		--output-file-base=zz_generated.conversion $(OUTPUT_BASE) \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) \
		paths=./apis/v1alpha3 \
		paths=./apis/v1alpha4 \
		paths=./apis/v1beta1 \
		crd:crdVersions=v1 \
		output:crd:dir=$(CRD_ROOT) \
		output:webhook:dir=$(WEBHOOK_ROOT) \
		webhook
	$(CONTROLLER_GEN) \
		paths=./controllers/... \
		output:rbac:dir=$(RBAC_ROOT) \
		rbac:roleName=manager-role
	$(CONTROLLER_GEN) \
		paths=./apis/vmware/v1beta1 \
		crd:crdVersions=v1 \
		output:crd:dir=$(SUPERVISOR_CRD_ROOT)
	# vm-operator crds are loaded to be used for integration tests.
	$(CONTROLLER_GEN) \
		paths=github.com/vmware-tanzu/vm-operator-api/api/... \
		crd:crdVersions=v1 \
		output:crd:dir=$(VMOP_CRD_ROOT)
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
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: release-overrides
release-overrides:
	$(MAKE) manifests STAGE=release MANIFEST_DIR=$(OVERRIDES_DIR) PULL_POLICY=IfNotPresent IMAGE=$(RELEASE_CONTROLLER_IMG):$(VERSION)

.PHONY: dev-manifests
dev-manifests:
	$(MAKE) manifests STAGE=dev MANIFEST_DIR=$(OVERRIDES_DIR) PULL_POLICY=Always IMAGE=$(DEV_CONTROLLER_IMG):$(DEV_TAG)
	cp metadata.yaml $(OVERRIDES_DIR)/metadata.yaml

.PHONY: manifests
manifests:  $(STAGE)-version-check $(STAGE)-flavors $(MANIFEST_DIR) $(BUILD_DIR) $(KUSTOMIZE)
	rm -rf $(BUILD_DIR)/config
	cp -R config $(BUILD_DIR)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' $(BUILD_DIR)/config/base/manager_pull_policy.yaml
	sed -i'' -e 's@image: .*@image: '"$(IMAGE)"'@' $(BUILD_DIR)/config/base/manager_image_patch.yaml
	"$(KUSTOMIZE)" build $(BUILD_DIR)/config/default > $(MANIFEST_DIR)/infrastructure-components.yaml
	"$(KUSTOMIZE)" build $(BUILD_DIR)/config/supervisor > $(MANIFEST_DIR)/infrastructure-components-supervisor.yaml

.PHONY: verify-conversions
verify-conversions: $(CONVERSION_VERIFIER)  ## Verifies expected API conversion are in place
	$(CONVERSION_VERIFIER)

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: flavors
flavors: $(FLAVOR_DIR)
	go run ./packaging/flavorgen -f vip > $(FLAVOR_DIR)/cluster-template.yaml
	go run ./packaging/flavorgen -f external-loadbalancer > $(FLAVOR_DIR)/cluster-template-external-loadbalancer.yaml


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

.PHONY: verify-gen
verify-gen: generate  ## Verfiy go generated files are up to date
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi

.PHONY: verify-modules
verify-modules: modules  ## Verify go modules are up to date
	@if !(git diff --quiet HEAD -- go.sum go.mod $(TOOLS_DIR)/go.mod $(TOOLS_DIR)/go.sum); then \
		git diff; \
		echo "go module files are out of date"; exit 1; \
	fi
	@if (find . -name 'go.mod' | xargs -n1 grep -q -i 'k8s.io/client-go.*+incompatible'); then \
		find . -name "go.mod" -exec grep -i 'k8s.io/client-go.*+incompatible' {} \; -print; \
		echo "go module contains an incompatible client-go version"; exit 1; \
	fi

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
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg ldflags="$(LDFLAGS)"  . -t $(DEV_CONTROLLER_IMG):$(DEV_TAG)

.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(DEV_CONTROLLER_IMG):$(DEV_TAG)
