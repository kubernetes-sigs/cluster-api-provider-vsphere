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

# Hosts running SELinux need :z added to volume mounts
SELINUX_ENABLED := $(shell cat /sys/fs/selinux/enforce 2> /dev/null || echo 0)

ifeq ($(SELINUX_ENABLED),1)
  DOCKER_VOL_OPTS?=:z
endif


.DEFAULT_GOAL := help

# Tooling
TOOLS_DIR := ../../hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint

# Binaries and images
OPENAPI_GENERATOR_BIN ?= $(shell command -v openapi-generator 2>/dev/null)
OPENAPI_GENERATOR_BIN := $(shell command -v $(OPENAPI_GENERATOR_BIN) 2>/dev/null)
OPENAPI_GENERATOR_IMG ?= openapitools/openapi-generator-cli:v4.2.2

# Sources
SWAGGER_JSON ?= haproxy-dataplaneapi-1.2.json

# The directory in which the client bindings are generated
OUTPUT_DIR ?= ./openapi
TEMPLATE_DIR ?= ./generator-template

# These files will be removed from the OUTPUT_DIR after the client bindings
# are generated.
FILES_TO_DELETE_POST_GEN := go.mod go.sum .travis.yml git_push.sh

## --------------------------------------
## Help
## --------------------------------------

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Tooling Binaries
## --------------------------------------

TOOLING_BINARIES := $(GOLANGCI_LINT)
tools: $(TOOLING_BINARIES) ## Build tooling binaries
.PHONY: $(TOOLING_BINARIES)
$(TOOLING_BINARIES):
	make -C $(TOOLS_DIR) $(@F)

## --------------------------------------
## Build
## --------------------------------------

.PHONY: build
build: ## Builds the HAProxy dataplane API client bindings
	go build -v $(OUTPUT_DIR)

## --------------------------------------
## Generate
## --------------------------------------

.PHONY: generate-with-binary
generate-with-binary: $(SWAGGER_JSON)
generate-with-binary: ## Generates the HAProxy dataplane API client bindings (binary)
	@$(MAKE) --no-print-directory verify-openapi-generator-binary
	$(OPENAPI_GENERATOR_BIN) generate --input-spec $< --generator-name go -t $(TEMPLATE_DIR) --output $(OUTPUT_DIR)
	@rm -fr $(addprefix $(OUTPUT_DIR)/,$(FILES_TO_DELETE_POST_GEN))

.PHONY: generate-with-docker
generate-with-docker: $(SWAGGER_JSON)
generate-with-docker: ## Generates the HAProxy dataplane API client bindings (docker)
	@mkdir -p $(OUTPUT_DIR)
	docker run --rm \
	  -v $(abspath $<):/openapi/swagger.json:ro \
	  -v $(abspath $(OUTPUT_DIR)):/openapi/output$(DOCKER_VOL_OPTS) \
		-v $(abspath $(TEMPLATE_DIR)):/openapi/generator-template$(DOCKER_VOL_OPTS) \
	  $(OPENAPI_GENERATOR_IMG) \
	  generate --input-spec /openapi/swagger.json --generator-name go -t /openapi/generator-template --output /openapi/output
	@rm -fr $(addprefix $(OUTPUT_DIR)/,$(FILES_TO_DELETE_POST_GEN))

.PHONY: generate
ifneq (,$(strip $(OPENAPI_GENERATOR_BIN)))
generate: generate-with-binary
else
generate: generate-with-docker
endif
generate:  ## Generates the HAProxy dataplane API client bindings (auto)
	$(MAKE) --no-print-directory remove-dupe-opts
	$(MAKE) --no-print-directory fix-backend-model
	$(MAKE) --no-print-directory fix-frontend-model

# Please see https://github.com/OpenAPITools/openapi-generator/issues/741
# for more information on the issue of duplicate Opt structures.
.PHONY: remove-dupe-opts
remove-dupe-opts: | $(GOLANGCI_LINT)
remove-dupe-opts: ## Removes duplicate Opt structs from the generated code
	@for struct in $$(grep -h 'type .\{1,\} struct' $(OUTPUT_DIR)/*.go | grep Opts  | sort | uniq -c | grep -v '^      1' | awk '{print $$3}'); do \
	  for f in $$(/bin/ls $(OUTPUT_DIR)/*.go); do \
	    if grep -qF "type $${struct} struct" "$${f}"; then \
	      if eval "test -z \$${$${struct}}"; then \
	        echo "skipping first appearance of $${struct} in file $${f}"; \
	        eval "export $${struct}=1"; \
	      else \
	        echo "removing dupe $${struct} from file $${f}"; \
	        tr '\n' '\r' <"$${f}" | sed 's~// '"$${struct}"'.\{1,\}type '"$${struct}"' struct {[^}]\{1,\}}~~' | tr '\r' '\n' >"$${f}.tmp"; \
	        mv -f "$${f}.tmp" "$${f}"; \
	      fi; \
	    fi \
	  done \
	done
	$(GOLANGCI_LINT) run -v --no-config --fast=false --fix --disable-all --enable goimports $(OUTPUT_DIR)

.PHONY: fix-backend-model
fix-backend-model: | $(GOLANGCI_LINT)
fix-backend-model: $(OUTPUT_DIR)/model_backend.go
fix-backend-model: ## Makes optional fields in the backend model into pointers
	sed -i.bak \
	  -e 's~\(DefaultServer[[:space:]]\{1,\}\)\(DefaultServer\)~\1*\2~' \
	  -e 's~\(Forwardfor[[:space:]]\{1,\}\)\(Forwardfor\)~\1*\2~' \
	  -e 's~\(HashType[[:space:]]\{1,\}\)\(BackendHashType\)~\1*\2~' \
	  -e 's~\(Httpchk[[:space:]]\{1,\}\)\(Httpchk\)~\1*\2~' \
	  -e 's~\(Redispatch[[:space:]]\{1,\}\)\(Redispatch\)~\1*\2~' \
	  -e 's~\(StickTable[[:space:]]\{1,\}\)\(BackendStickTable\)~\1*\2~' \
	  $<
	$(GOLANGCI_LINT) run -v --no-config --fast=false --fix --disable-all --enable goimports $<
	@rm -f $<.bak

.PHONY: fix-frontend-model
fix-frontend-model: | $(GOLANGCI_LINT)
fix-frontend-model: $(OUTPUT_DIR)/model_frontend.go
fix-frontend-model: ## Makes optional fields in the frontend model into pointers
	sed -i.bak \
	  -e 's~\(Forwardfor[[:space:]]\{1,\}\)\(Forwardfor\)~\1*\2~' \
	  $<
	$(GOLANGCI_LINT) run -v --no-config --fast=false --fix --disable-all --enable goimports $<
	@rm -f $<.bak

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: clean
clean: ## Run all the clean targets
	rm -fr $(OUTPUT_DIR)

.PHONY: verify-with-binary
verify-openapi-generator-binary:
	@if [ -z "$(OPENAPI_GENERATOR_BIN)" ]; then \
	  { \
	    echo "openapi-generator is required" && \
	    echo " * use the analogous docker target, or..." && \
	    echo " * install the openapi-generator binary (https://openapi-generator.tech/docs/installation.html)" \
	  ; } 1>&2; \
	  exit 1; \
	fi

.PHONY: verify-with-binary
verify-with-binary: $(SWAGGER_JSON)
verify-with-binary: ## Verifies the HAProxy dataplane API spec file (binary)
	@$(MAKE) --no-print-directory verify-openapi-generator-binary
	$(OPENAPI_GENERATOR_BIN) validate --input-spec $<

.PHONY: verify-with-docker
verify-with-docker: $(SWAGGER_JSON)
verify-with-docker: ## Verifies the HAProxy dataplane spec file (docker)
	@mkdir -p $(OUTPUT_DIR)
	docker run --rm \
	  -v $(abspath $<):/openapi/swagger.json:ro \
	  $(OPENAPI_GENERATOR_IMG) \
	  validate --input-spec /openapi/swagger.json

.PHONY: verify
ifneq (,$(strip $(OPENAPI_GENERATOR_BIN)))
verify: verify-with-binary
else
verify: verify-with-docker
endif
verify:  ## Verifies the HAProxy dataplane spec file (auto)
