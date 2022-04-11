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
##                             INSTALL MDLINT                                 ##
################################################################################
FROM node:12.6.0-slim as build
ARG MDLINT_CLI_VERSION=0.31.1
ENV MDLINT_CLI_VERSION=${MDLINT_CLI_VERSION}
RUN npm install -g --prefix=/md markdownlint-cli@${MDLINT_CLI_VERSION} && \
    ln -s /md/lib/node_modules/markdownlint-cli/markdownlint.js /md/lint

################################################################################
##                               RUN MDLINT                                   ##
################################################################################
FROM gcr.io/distroless/nodejs:latest
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"
COPY --from=build /md/ /md/
WORKDIR /build
CMD [ "/md/lint", "-i", "vendor", "." ]
ENTRYPOINT [ "/nodejs/bin/node" ]
