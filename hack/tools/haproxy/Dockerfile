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

FROM photon:3.0
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"

ARG DATAPLANEAPI_VERSION
ENV DATAPLANEAPI_VERSION 1.2.4

RUN yum update --assumeno && \
    yum install -y curl vim lsof haproxy shadow && \
    curl -Lo /usr/local/bin/dataplaneapi \
         https://github.com/haproxytech/dataplaneapi/releases/download/v${DATAPLANEAPI_VERSION}/dataplaneapi && \
    chmod 0755 /usr/local/bin/dataplaneapi && \
    useradd --system --home-dir=/var/lib/haproxy --user-group haproxy && \
    mkdir -p /var/lib/haproxy && \
    chown -R haproxy:haproxy /var/lib/haproxy

COPY example/haproxy.cfg example/ca.crt example/server.crt \
     example/server.key /etc/haproxy/
RUN chmod 0640 /etc/haproxy/haproxy.cfg /etc/haproxy/*.crt && \
    chmod 0440 /etc/haproxy/*.key

CMD [ "-f", "/etc/haproxy/haproxy.cfg" ]
ENTRYPOINT [ "/usr/sbin/haproxy" ]
