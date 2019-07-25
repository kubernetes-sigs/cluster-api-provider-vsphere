# Makefile

VERSION ?= $(shell git describe --always --dirty)
REGISTRY ?=gcr.io/cluster-api-provider-vsphere/pr/ci

all: build
.PHONY : all

.PHONY : copyspec
copyspec:
	cp ../../../cmd/clusterctl/examples/vsphere/addons.yaml.template ./spec/addons.template

.PHONY : build
build: copyspec
	docker build . --tag $(REGISTRY):$(VERSION)

push: build
	@echo "logging into gcr.io registry with key file"
	# TODO hardcoded key file location is a temp workaround
	gcloud auth activate-service-account --key-file /root/.capv/keyfile.json
	docker login -u _json_key --password-stdin gcr.io </root/.capv/keyfile.json
	docker push $(REGISTRY):$(VERSION)
	@echo docker logout gcr.io
	gcloud auth revoke

clean:
	docker image rm -f $(REGISTRY):$(VERSION)
	rm ./spec/addons.template
