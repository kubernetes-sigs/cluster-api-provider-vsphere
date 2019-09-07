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

VERSION ?= $(shell git describe --always --dirty)
REGISTRY ?=gcr.io/cluster-api-provider-vsphere/pr/ci

all: build
.PHONY : all

.PHONY: genspec
genspec:
	../generate-e2e-spec.sh

.PHONY : build
build: genspec
	docker build . --tag $(REGISTRY):$(VERSION)

push: build
	@echo "logging into gcr.io registry with key file"
	# TODO hardcoded key file location is a temp workaround
	gcloud auth activate-service-account --key-file $(GCR_KEY_FILE)
	docker login -u _json_key --password-stdin gcr.io <"$(GCR_KEY_FILE)"
	docker push $(REGISTRY):$(VERSION)
	@echo docker logout gcr.io
	gcloud auth revoke

clean:
	docker image rm -f $(REGISTRY):$(VERSION)
	rm ./spec/addons.template
