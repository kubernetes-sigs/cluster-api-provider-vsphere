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

# Image URL to use all building/pushing image targets
PRODUCTION_IMG ?= gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:0.2.0
CI_IMG ?= gcr.io/cnx-cluster-api/vsphere-cluster-api-provider
CLUSTERCTL_CI_IMG ?= gcr.io/cnx-cluster-api/clusterctl
DEV_IMG ?= # <== NOTE:  outside dev, change this!!!

# Retrieves the git hash
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
	   git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

KUSTOMIZE := $(shell command -v kustomize 2>/dev/null)
ifeq (,$(strip $(KUSTOMIZE)))
$(KUSTOMIZE):
	GO111MODULE=off go get sigs.k8s.io/kustomize
endif

all: build test

build: clusterctl manager

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: fmt vet
	go build -o bin/manager ./cmd/manager

# Build the clusterctl binary
clusterctl: fmt vet
	go build -o bin/clusterctl ./cmd/clusterctl

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
	$(MAKE) -C config/crds

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

CAPI_CONFIG_SRC=$(abspath $(shell go list -f '{{.Dir}}' sigs.k8s.io/cluster-api/cmd/clusterctl 2>/dev/null)/../../config)
CAPI_CONFIG_DST=./vendor/sigs.k8s.io/cluster-api/

vendor:
	go mod tidy -v
	go mod vendor -v
	go mod verify
	mkdir -p $(CAPI_CONFIG_DST)
	cp -rf --no-preserve=mode $(CAPI_CONFIG_SRC) $(CAPI_CONFIG_DST)
.PHONY: vendor

####################################
# DEVELOPMENT Build and Push targets
####################################

# Create YAML file for deployment
dev-yaml: | $(KUSTOMIZE)
	@$(KUSTOMIZE) version 2>&1 | grep -q 'KustomizeVersion:\(unknown\|v2\)' || { echo "kustomize v2+ required" 1>&2; exit 1; }
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${DEV_IMG}"'@' ./config/default/vsphere_manager_image_patch.yaml
	cmd/clusterctl/examples/vsphere/generate-yaml.sh

# Build the docker image
dev-build: #test
	docker build . -t ${DEV_IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${DEV_IMG}"'@' ./config/default/vsphere_manager_image_patch.yaml

# Push the docker image
dev-push:
	docker push ${DEV_IMG}


###################################
# PRODUCTION Build and Push targets
###################################

# Create YAML file for deployment
prod-yaml: | $(KUSTOMIZE)
	@$(KUSTOMIZE) version 2>&1 | grep -q 'KustomizeVersion:\(unknown\|v2\)' || { echo "kustomize v2+ required" 1>&2; exit 1; }
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${PRODUCTION_IMG}"'@' ./config/default/vsphere_manager_image_patch.yaml
	cmd/clusterctl/examples/vsphere/generate-yaml.sh

# Build the docker image
prod-build: test
	docker build . -t ${PRODUCTION_IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${PRODUCTION_IMG}"'@' ./config/default/vsphere_manager_image_patch.yaml

# Push the docker image
prod-push:
	@echo "logging into gcr.io registry with key file"
	@docker login -u _json_key --password-stdin gcr.io <"$(GCR_KEY_FILE)"
	docker push ${PRODUCTION_IMG}


###################################
# CI Build and Push targets
###################################

# Create YAML file for deployment into CI
ci-yaml: | $(KUSTOMIZE)
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"$(CI_IMG):$(VERSION)"'@' ./config/default/vsphere_manager_image_patch.yaml
	cmd/clusterctl/examples/vsphere/generate-yaml.sh

ci-image: generate fmt vet manifests
	docker build . -t "$(CI_IMG):$(VERSION)"
	docker build . -f cmd/clusterctl/Dockerfile -t "$(CLUSTERCTL_CI_IMG):$(VERSION)"
	@echo "updating kustomize image patch file for manager resource"
	sed -i.tmp -e 's@image: .*@image: '"$(CI_IMG):$(VERSION)"'@' ./config/default/vsphere_manager_image_patch.yaml

ci-push: ci-image
# Log into the registry with a service account file.  In CI, GCR_KEY_FILE contains the content and not the file name.
	@echo "logging into gcr.io registry with key file"
	@echo $$GCR_KEY_FILE | docker login -u _json_key --password-stdin gcr.io
	docker push "$(CI_IMG):$(VERSION)"
	docker push "$(CLUSTERCTL_CI_IMG):$(VERSION)"
	@echo docker logout gcr.io
