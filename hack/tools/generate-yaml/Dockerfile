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
##                            DOWNLOAD KUSTOMIZE                              ##
################################################################################
FROM debian:stretch-slim as kustomize
RUN apt-get --assume-no update && apt-get -y install curl
ARG KUSTOMIZE_VERSION
ENV KUSTOMIZE_VERSION 2.0.0
RUN curl -Lo /kustomize \
    "https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64" && \
    chmod 0755 /kustomize


################################################################################
##                                MAIN STAGE                                  ##
################################################################################
FROM debian:stretch-slim
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"

# Install python
RUN apt-get --assume-no update && apt-get -y install curl python

# Copy kustomize from the kustomize build stage.
COPY --from=kustomize /kustomize /usr/local/bin/kustomize

# Run things out of the /build directory.
WORKDIR /build

# Copy in the hack tooling.
COPY ./hack/envsubst.py ./hack/generate-yaml.sh ./hack/

# Copy in the CAPV and CAPI config directories.
COPY ./config ./config
COPY ./vendor/sigs.k8s.io/cluster-api/config ./vendor/sigs.k8s.io/cluster-api/config

# Copy in the examples directory.
COPY ./cmd/clusterctl/examples/vsphere/*.template ./cmd/clusterctl/examples/vsphere/

# This build arg specifies the value of the -m flag for generate-yaml.sh, the
# CAPV manager image inserted into the generated provider components.
ARG CAPV_MANAGER_IMAGE
ENV CAPV_MANAGER_IMAGE gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:latest

# The YAML is always written to the /out directory. Mount the volumes there.
ENV OUT_DIR /out

ENTRYPOINT [ "./hack/generate-yaml.sh" ]
