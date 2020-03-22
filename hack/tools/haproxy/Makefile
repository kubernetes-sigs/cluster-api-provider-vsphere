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

## --------------------------------------
## Help
## --------------------------------------

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Variables
## --------------------------------------

# Initialize the version with the Git version.
VERSION ?= $(shell git describe --always --dirty)

## --------------------------------------
## Packer flags
## --------------------------------------
PACKER_FLAGS += -var='version=$(VERSION)'

# If FOREGROUND=1 then Packer will set headless to false, causing local builds
# to build in the foreground, with a UI. This is very useful when debugging new
# platforms or issues with existing ones.
ifeq (1,$(strip $(FOREGROUND)))
PACKER_FLAGS += -var="headless=false"
endif

# A list of variable files given to Packer.
PACKER_VAR_FILES := $(strip $(foreach f,$(abspath $(PACKER_VAR_FILES)),-var-file="$(f)" ))

# Initialize a list of flags to pass to Packer. This includes any existing flags
# specified by PACKER_FLAGS, as well as prefixing the list with the variable
# files from PACKER_VAR_FILES, with each file prefixed by -var-file=.
#
# Any existing values from PACKER_FLAGS take precendence over variable files.
PACKER_FLAGS := $(PACKER_VAR_FILES) $(PACKER_FLAGS)

## --------------------------------------
## OVA
## --------------------------------------

.PHONY: build
build: ## Builds the HAProxy load balancer OVA
	packer build $(PACKER_FLAGS) packer.json

.PHONY: verify
verify: ## Verifies the packer config
	packer validate $(PACKER_FLAGS) packer.json

.PHONY: build-template
build-template: ## Builds the HAProxy load balancer OVA
	packer build $(PACKER_FLAGS) packer-api.json

.PHONY: verify-template
verify-template: ## Verifies the packer config
	packer validate $(PACKER_FLAGS) packer-api.json

## --------------------------------------
## Clean
## --------------------------------------
.PHONY: clean
clean: ## Cleans all local image caches
	rm -fr ./output

