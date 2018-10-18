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

# Keep an existing GOPATH, make a private one if it is undefined
GOPATH_DEFAULT := $(PWD)/.go
export GOPATH ?= $(GOPATH_DEFAULT)
GOBIN_DEFAULT := $(GOPATH)/bin
export GOBIN ?= $(GOBIN_DEFAULT)
HAS_DEP := $(shell command -v dep;)

.PHONY: gendeepcopy

all: generate build images

$(GOBIN):
	echo "create gobin"
	mkdir -p $(GOBIN)

depend: $(GOBIN)
ifndef HAS_DEP
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
endif
	dep ensure

depend-update: work
	dep ensure -update

generate: gendeepcopy

gendeepcopy:
	go build -o $$GOPATH/bin/deepcopy-gen sigs.k8s.io/cluster-api-provider-vsphere/vendor/k8s.io/code-generator/cmd/deepcopy-gen
	deepcopy-gen \
	  -i ./pkg/cloud/vsphere/vsphereproviderconfig,./pkg/cloud/vsphere/vsphereproviderconfig/v1alpha1 \
	  -O zz_generated.deepcopy \
	  -h boilerplate.go.txt

build: depend
	CGO_ENABLED=0 go install -a -ldflags '-extldflags "-static"' sigs.k8s.io/cluster-api-provider-vsphere/cmd/vsphere-machine-controller

images: depend
	$(MAKE) -C cmd/vsphere-machine-controller image

push: depend
	$(MAKE) -C cmd/vsphere-machine-controller push

check: depend fmt vet

test: depend
	go test -race -cover ./cmd/... ./cloud/...

fmt:
	hack/verify-gofmt.sh

vet:
	go vet ./...
