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

# Retrieves the git hash
VERSION ?= $(shell git describe --always --dirty)

# Build manager binary
manager: check
	go build -o bin/manager ./cmd/manager

# Build the clusterctl binary
clusterctl: check
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static" -w -s' \
	-o bin/clusterctl."$${GOOS:-$$(go env GOHOSTOS)}"_"$${GOARCH:-$$(go env GOHOSTARCH)}" \
	./cmd/clusterctl
	@cp -f bin/clusterctl."$${GOOS:-$$(go env GOHOSTOS)}"_"$${GOARCH:-$$(go env GOHOSTARCH)}" bin/clusterctl

clusterctl-in-docker:
	docker run --rm -v $(CWD):/go/src/sigs.k8s.io/cluster-api-provider-vsphere \
	  -w /go/src/sigs.k8s.io/cluster-api-provider-vsphere \
	  -e CGO_ENABLED=0 -e GOOS="$${GOOS:-linux}" -e GOARCH="$${GOARCH:-amd64}" \
	  golang:1.12.6 sh -c "\
	  go build -ldflags '-extldflags \"-static\" -w -s' \
	  -o bin/clusterctl.\"$${GOOS:-linux}\"_\"$${GOARCH:-amd64}\" ./cmd/clusterctl && \
	  cp -f bin/clusterctl.\"$${GOOS:-linux}\"_\"$${GOARCH:-amd64}\" bin/clusterctl"
.PHONY: clusterctl-in-docker

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	hack/update-generated.sh crd rbac

# Run go fmt against code
fmt: | generate
	hack/check-format.sh

# Run go vet against code
vet: | generate
	hack/check-vet.sh

# Run go lint against code
lint: | generate
	hack/check-lint.sh

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

################################################################################
##                          The default targets                               ##
################################################################################

# The default build target
build: manager clusterctl
.PHONY: build

# Check all the sources.
check: fmt lint vet
.PHONY: check

# The default test target
test: build manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out
.PHONY: test
