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

# build from test-infra prow image
FROM gcr.io/k8s-testimages/kubekins-e2e:v20181120-dea0825e3-master

ADD cluster-api-provider-vsphere /go/src/sigs.k8s.io/cluster-api-provider-vsphere/

RUN chmod +x /go/src/sigs.k8s.io/cluster-api-provider-vsphere/scripts/e2e/e2e.sh
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-vsphere
CMD ["shell"]
ENTRYPOINT ["./scripts/e2e/e2e.sh"]
