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

# Retrieves the git hash
VERSION ?= $(shell git describe --always --dirty)

# The Go packages
PKGS := ./api/... ./controllers/... ./pkg/cloud/vsphere/util/... .

# Build manager binary
.PHONY: manager
manager: check
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static" -w -s' -o bin/manager

.PHONY: clusterctl
clusterctl:
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static" -w -s' -o bin/clusterctl ./vendor/sigs.k8s.io/cluster-api/cmd/clusterctl

# Run go fmt against code
.PHONY: fmt
fmt: | generate-kubebuilder-code
	hack/check-format.sh

# Run go vet against code
.PHONY: vet
vet: | generate-kubebuilder-code
	hack/check-vet.sh

# Run go lint against code
.PHONY: lint
lint: | generate-kubebuilder-code
	hack/check-lint.sh

# Generate assets
.PHONY: generate
generate:
	$(MAKE) generate-manifests
	$(MAKE) generate-kubebuilder-code

# Runs go generate
.PHONY: generate-go
generate-go:
	go generate $(PKGS)

# Generates the CRD and RBAC manifests
.PHONY: generate-manifests
generate-manifests:
	hack/update-generated.sh crd rbac

# Generates the kubebuilder code
.PHONY: generate-kubebuilder-code
generate-kubebuilder-code:
	hack/update-generated.sh kubebuilder

.PHONY: vendor
vendor:
	hack/update-vendor.sh

################################################################################
##                          The default targets                               ##
################################################################################

# The default build target
.PHONY: build
build: clusterctl manager

# Check all the sources.
.PHONY: check
check: fmt lint vet

# The default test target
.PHONY: test
test: build
	go test $(PKGS) -coverprofile cover.out
