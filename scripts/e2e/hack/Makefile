# Makefile

VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
REGISTRY ?=luoh/cluster-api-provider-vsphere-prow-ci

all: build upload clean
.PHONY : all

.PHONY : build
build: 
	docker build . --tag $(REGISTRY):$(VERSION)
	docker tag $(REGISTRY):$(VERSION) $(REGISTRY):debug
        
upload:
	docker login -u="$(DOCKER_USERNAME)" -p="$(DOCKER_PASSWORD)";
	docker push $(REGISTRY):$(VERSION)
	docker push $(REGISTRY):debug

clean:
	docker image rm -f $(REGISTRY):$(VERSION)
	docker image rm -f $(REGISTRY):debug
