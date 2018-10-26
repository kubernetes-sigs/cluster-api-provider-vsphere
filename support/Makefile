BUILD_NUMBER ?= $(shell date +%Y%m%d%M%S)
DIND_BUILDER ?= blueshift-docker-local.artifactory.eng.vmware.com/utils/docker:17.10.0-ce-dind

NAME = $(shell grep ^NAME ../cmd/vsphere-machine-controller/Makefile |awk '{print $$3}')
TAG = $(shell grep ^TAG ../cmd/vsphere-machine-controller/Makefile |awk '{print $$3}')

BUILDROOT := $(shell cd ../.. && pwd)
SRCROOT := $(shell cd .. && pwd)

# dind command
export run_dind = docker exec builder

all: publish-local

start-dind:
	docker run --name builder --net=host -v $(BUILDROOT):/dind$(BUILDROOT) --privileged -d $(DIND_BUILDER) --storage-driver=aufs || docker start builder

prep: start-dind
	mkdir -p $(PUBLISH_DIR)

images: prep
	$(run_dind) sh -c "cd /dind/$(SRCROOT)/cmd/vsphere-machine-controller && docker build -t $(NAME):$(TAG).$(BUILD_NUMBER) -f ./Dockerfile ../.."

publish-local: images
	$(run_dind) sh -c "docker save $(NAME):$(TAG).$(BUILD_NUMBER) | gzip > /dind/$(PUBLISH_DIR)/cluster-api-provider-vsphere-image.tar"
	@echo image built and published successfuly
