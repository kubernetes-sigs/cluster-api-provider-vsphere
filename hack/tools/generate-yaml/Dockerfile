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

################################################################################
##                                MAIN STAGE                                  ##
################################################################################
ARG BASE_IMAGE=gcr.io/cluster-api-provider-vsphere/extra/manifests-base:v0.4.2-beta.0-15-g5b0bb299
FROM ${BASE_IMAGE}
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"

# install govc, used to check vSphere version
RUN curl -L https://github.com/vmware/govmomi/releases/download/v0.21.0/govc_linux_amd64.gz | gunzip > /usr/local/bin/govc && chmod +x /usr/local/bin/govc

# Run things out of the /build directory.
ENV BUILDDIR /build
WORKDIR /build

# Copy in the CAPV config directory.
COPY ./config ./config

# Copy in the examples directory.
COPY ./examples ./examples

# Ensure all the directories in the /build directory are writeable.
RUN find . -type d -exec chmod 0777 \{\} \;

# This build arg specifies the value of the -m flag for generate-yaml.sh, the
# CAPV manager image inserted into the generated provider components.
ARG CAPV_MANAGER_IMAGE=gcr.io/cluster-api-provider-vsphere/ci/manager:latest
ENV CAPV_MANAGER_IMAGE=${CAPV_MANAGER_IMAGE}

# Change the working directory to /.
ENV WORKDIR /out
WORKDIR /out

# Indicate that this is being execute in a container.
ENV DOCKER_ENABLED 1

ENTRYPOINT [ "/build/examples/generate.sh" ]
