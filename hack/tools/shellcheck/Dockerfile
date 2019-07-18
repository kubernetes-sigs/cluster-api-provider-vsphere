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
##                          INSTALL SHELLCHECK                                ##
################################################################################
ARG SHELLCHECK_VERSION=v0.6.0
FROM koalaman/shellcheck:${SHELLCHECK_VERSION} as build

################################################################################
##                                 MAIN                                       ##
################################################################################
FROM debian:stretch-slim
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"
COPY --from=build /bin/shellcheck /bin/
COPY shellcheck.sh /bin/shellcheck.sh
WORKDIR /build
ENTRYPOINT [ "/bin/shellcheck.sh" ]
