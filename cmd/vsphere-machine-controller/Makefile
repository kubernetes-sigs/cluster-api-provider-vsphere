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

.PHONY: image push dev_image dev_push ci_image ci_push

VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
	   git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
REGISTRY_CI ?= luoh/cluster-api-provider-vsphere

GCR_BUCKET = k8s-cluster-api
PREFIX = gcr.io/$(GCR_BUCKET)
DEV_PREFIX ?= gcr.io/$(shell gcloud config get-value project)
NAME = vsphere-machine-controller

image:
	docker build -t "$(PREFIX)/$(NAME):$(VERSION)" -f ./Dockerfile ../..

push: image
	docker push "$(PREFIX)/$(NAME):$(VERSION)"
	$(MAKE) fix_gcs_permissions

fix_gcs_permissions:
	gsutil acl ch -u AllUsers:READ gs://artifacts.$(GCR_BUCKET).appspot.com
	gsutil -m acl ch -r -u AllUsers:READ gs://artifacts.$(GCR_BUCKET).appspot.com

dev_image: 
	docker build -t "$(DEV_PREFIX)/$(NAME):$(VERSION)-dev" -f ./Dockerfile ../..

dev_push: dev_image
	docker push "$(DEV_PREFIX)/$(NAME):$(VERSION)-dev"

ci_image: 
	docker build -t "$(REGISTRY_CI):$(VERSION)" -f ./Dockerfile ../..

ci_push: ci_image
# Log into the registry with a Docker username and password.
ifneq (,$(strip $(DOCKER_USERNAME))) # begin docker-username
ifneq (,$(strip $(DOCKER_PASSWORD))) # begin docker-password
	@echo "logging into docker registry with username and password"
	docker login -u="$(DOCKER_USERNAME)" -p="$(DOCKER_PASSWORD)"
endif # end docker-password
endif # end docker-username
	docker push "$(REGISTRY_CI):$(VERSION)"
