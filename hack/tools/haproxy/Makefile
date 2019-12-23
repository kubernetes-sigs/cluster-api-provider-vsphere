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

VERSION ?= $(shell git describe --always --dirty)
PACKER_FLAGS += -var 'version=$(VERSION)'

## --------------------------------------
## Help
## --------------------------------------

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## OVA
## --------------------------------------

.PHONY: build
build: ## Builds the haproxy OVA
	packer build $(PACKER_FLAGS) packer.json

.PHONY: build-fg
build-fg: PACKER_FLAGS += -var 'headless=false'
build-fg: build
build-fg: ## Builds the haproxy OVA in the foreground

.PHONY: verify-packer
verify-packer: ## Verifies the packer config
	packer validate $(PACKER_FLAGS) packer.json

## --------------------------------------
## Clean
## --------------------------------------

.PHONY: clean
clean: ## Cleans the build directory
	rm -fr ./output

