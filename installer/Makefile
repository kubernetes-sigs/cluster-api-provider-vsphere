# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SHELL := /bin/bash

all: build

GO ?= go
SED ?= sed
RM ?= rm
OS := $(shell uname | tr '[:upper:]' '[:lower:]')

BASE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

BIN ?= bin

export GOPATH ?= $(shell echo $(CURDIR) | sed -e 's,/src/.*,,')
GAS ?= $(GOPATH)/bin/gas$(BIN_ARCH)
GOLINT ?= $(GOPATH)/bin/golint$(BIN_ARCH)

.PHONY: gas ova

LDFLAGS := $(shell BUILD_NUMBER=${BUILD_NUMBER} TAG=${TAG} $(BASE_DIR)/scripts/version-linker-flags.sh)

OVFENV := $(BIN)/ovfenv
OVAUI := $(BIN)/ova-ui
TOOLBOX := $(BIN)/toolbox
RPCTOOL := $(BIN)/rpctool
gas: $(GAS)
ovfenv: $(OVFENV)
ova-ui: $(OVAUI)
toolbox: $(TOOLBOX)
rpctool: $(RPCTOOL)
tools: $(GOLINT) $(GAS)

SRC_DIRS := lib ovatools pkg toolbox

gas: $(GAS)
	@echo running go AST tool
	@$< $(addsuffix /...,$(SRC_DIRS)) 2>/dev/null

golint: $(GOLINT)
	@echo checking go lint...
	$< ./lib/... ./ovatools/... ./pkg/... ./toolbox

gofmt:
	@echo checking gofmt...
	@! gofmt -d -e -s $$(find . -mindepth 1 -maxdepth 1 -type d -not -name vendor) 2>&1 | egrep -v '^$$'

govet:
	@echo checking go vet...
	@$(GO) vet -all -lostcancel -tests $$(find . -mindepth 1 -maxdepth 1 -type d -not -name vendor)

clean:
	@echo removing binaries
	rm -rf $(BIN)

$(OVFENV):
	@echo building ovfenv linux...
	GOARCH=amd64 GOOS=linux $(TIME) $(GO) build $(RACE) -ldflags "$(LDFLAGS)" -o ./$@ ./ovatools/ovfenv
.PHONY: $(OVFENV)

$(OVAUI):
	@echo building ova-ui
	GOARCH=amd64 GOOS=linux $(TIME) $(GO) build  $(RACE) -ldflags "$(LDFLAGS)" -o ./$@ ./ovatools/ova-ui
.PHONY: $(OVAUI)

$(TOOLBOX):
	@echo building toolbox
	GOARCH=amd64 GOOS=linux $(TIME) $(GO) build $(RACE) -ldflags "$(LDFLAGS)" -o ./$@ ./toolbox
.PHONY: $(TOOLBOX)

$(RPCTOOL):
	@echo building rpctool
	GOARCH=amd64 GOOS=linux $(TIME) $(GO) build $(RACE) -ldflags "$(LDFLAGS)" -o ./$@ ./ovatools/rpctool
.PHONY: $(RPCTOOL)

# utility targets
$(GAS):
	@echo building $(GAS)...
	@$(GO) build $(RACE) -o $(GAS) ./vendor/github.com/GoASTScanner/gas

$(GOLINT):
	@echo building $(GOLINT)...
	@$(GO) build $(RACE) -o $(GOLINT) ./vendor/golang.org/x/lint/golint

build: $(OVFENV) $(OVAUI) $(TOOLBOX) $(RPCTOOL)
