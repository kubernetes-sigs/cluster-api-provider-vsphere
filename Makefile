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
IMG ?= gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:latest

# Retrieves the git hash
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
	   git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

# Registry to push images to in CI
REGISTRY_CI ?= gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:ci

all: test manager clusterctl

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: fmt vet
	go build -o bin/manager sigs.k8s.io/cluster-api-provider-vsphere/cmd/manager

# Build the clusterctl binary
clusterctl: fmt vet
	go build -o bin/clusterctl sigs.k8s.io/cluster-api-provider-vsphere/cmd/clusterctl

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
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# Create YAML file for deployment
create-yaml:
	cmd/clusterctl/examples/vsphere/generate-yaml.sh

# Build the docker image
docker-build: test
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/vsphere_manager_image_patch.yaml

# Push the docker image
docker-push:
	@echo "logging into gcr.io registry with key file"
	@docker login -u _json_key --password-stdin gcr.io <"$(GCR_KEY_FILE)"
	docker push ${IMG}

# Used for CI
ci_image:
	docker build -t "$(REGISTRY_CI):$(VERSION)" -f ./Dockerfile ../..

ci_push: ci_image
# Log into the registry with a Docker username and password.
	@echo "logging into gcr.io registry with key file"
	@docker login -u _json_key --password-stdin gcr.io <"$(GCR_KEY_FILE)"
	docker push "$(REGISTRY_CI):$(VERSION)"
