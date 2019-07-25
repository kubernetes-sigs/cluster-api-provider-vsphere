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
ARG BASE_IMAGE=gcr.io/cluster-api-provider-vsphere/extra/manifests-base:v0.4.0-beta.1-6-g9e51ae05
FROM ${BASE_IMAGE}
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"

# Run things out of the /build directory.
ENV BUILDDIR /build
WORKDIR /build

# Copy in the hack tooling.
COPY ./hack/envsubst.py ./hack/generate-yaml.sh ./hack/

# Copy in the CAPV and CAPI config directories.
COPY ./config ./config
COPY ./vendor/sigs.k8s.io/cluster-api/config ./vendor/sigs.k8s.io/cluster-api/config

# Copy in the examples directory.
COPY ./cmd/clusterctl/examples/vsphere/*.template ./cmd/clusterctl/examples/vsphere/

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

ENTRYPOINT [ "/build/hack/generate-yaml.sh" ]
