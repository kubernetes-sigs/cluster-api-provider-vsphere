# Makefile

VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
REGISTRY ?=gcr.io/cnx-cluster-api/cluster-api-provider-vsphere-ci

all: build push clean
.PHONY : all

.PHONY : copyspec
copyspec:
	cp ../../../cmd/clusterctl/examples/vsphere/addons.yaml.template ./spec/addons.template

.PHONY : build
build: copyspec
	docker build . --tag $(REGISTRY):$(VERSION)
        
push: build
	@echo "logging into gcr.io registry with key file"
	@echo $$GCR_KEY_FILE | docker login -u _json_key --password-stdin gcr.io
	docker push $(REGISTRY):$(VERSION)
	@echo docker logout gcr.io

clean:
	docker image rm -f $(REGISTRY):$(VERSION)
	rm ./spec/addons.template
