BUILD_NUMBER ?= $(shell date +%Y%m%d%M%S)
DIND_BUILDER ?= blueshift-docker-local.artifactory.eng.vmware.com/utils/docker:17.10.0-ce-dind

# TODO: make these name and tag come from the main repo.
# Right now it is manual in the main repo as well, thus it doesnt matter
NAME = vmware/vio/vsphere-cluster-api-provider
TAG = v0.1.0

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
	$(run_dind) sh -c "cd /dind/$(SRCROOT) && docker build -t $(NAME):$(TAG).$(BUILD_NUMBER) ."

publish-local: images
	$(run_dind) sh -c "docker save $(NAME):$(TAG).$(BUILD_NUMBER) | gzip > /dind/$(PUBLISH_DIR)/cluster-api-provider-vsphere-image.tar"
	@echo image built and published successfuly
	cp cluster-api-install.yaml $(PUBLISH_DIR)
	sed -i -e 's@image: vmware.*@image: '"$(NAME):$(TAG).$(BUILD_NUMBER)"'@' $(PUBLISH_DIR)/cluster-api-install.yaml
