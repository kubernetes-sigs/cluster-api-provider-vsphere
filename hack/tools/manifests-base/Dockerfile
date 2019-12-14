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
##                            DOWNLOAD DEPENDENCIES                           ##
################################################################################
FROM debian:stretch-slim as kustomize
LABEL "maintainer" "Travis Rhoden <trhoden@vmware.com>"

RUN echo "deb http://deb.debian.org/debian stretch-backports main" \
         >/etc/apt/sources.list.d/stretch-backports.list && \
    apt-get --assume-no  update && \
    apt-get --assume-yes install curl python && \
    apt-get --assume-yes --target-release stretch-backports install git && \
    rm -fr /var/lib/apt/lists/*

################################################################################
##                            DOWNLOAD KUSTOMIZE                              ##
################################################################################
ARG KUSTOMIZE_VERSION
ENV KUSTOMIZE_VERSION 3.5.1
RUN curl -L \
    "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" | \
    tar xz -C /usr/local/bin
