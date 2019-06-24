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
FROM debian:stretch-slim as build
RUN apt-get --assume-no update && apt-get -y install curl
ARG KUSTOMIZE_VERSION
ENV KUSTOMIZE_VERSION 2.0.0
RUN curl -Lo /kustomize \
    "https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64" && \
    chmod 0755 /kustomize

################################################################################
##                                RUN KUSTOMIZE                               ##
################################################################################
FROM gcr.io/distroless/static:latest
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"
COPY --from=build /kustomize /kustomize
WORKDIR /build
ENTRYPOINT [ "/kustomize" ]
