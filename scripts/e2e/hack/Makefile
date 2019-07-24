# Makefile

VERSION ?= $(shell git describe --always --dirty)
REGISTRY ?=gcr.io/cluster-api-provider-vsphere/pr/ci-debug

all: build push clean
.PHONY : all

.PHONY : build
build:
	cd ../../../../ && docker build . -f ./cluster-api-provider-vsphere/scripts/e2e/hack/Dockerfile --tag $(REGISTRY):$(VERSION)
	docker tag $(REGISTRY):$(VERSION) $(REGISTRY):debug

push: build
	@echo "logging into gcr.io registry with key file"
	@echo $$GCR_KEY_FILE | docker login -u _json_key --password-stdin gcr.io
	docker push $(REGISTRY):$(VERSION)
	docker push $(REGISTRY):debug
	@echo docker logout gcr.io

clean:
	docker image rm -f $(REGISTRY):$(VERSION)
	docker image rm -f $(REGISTRY):debug
