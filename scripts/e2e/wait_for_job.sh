#!/bin/sh

# Copyright 2018 The Kubernetes Authors.
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

export KUBECONFIG=$(kind get kubeconfig-path)
TOTAL=600
INTERVAL=6
retry=$((${TOTAL}/ INTERVAL))
ret=0
until kubectl get jobs --no-headers | awk -F" " '{print $2}' | awk -F"/" '{s+=($1!=$2)} END {exit s}';
do
   sleep ${INTERVAL};
   retry=$((retry - 1))
   if [ $retry -lt 0 ];
   then
      ret=1
      echo "job timeout"
      break
   fi;
   kubectl get jobs --no-headers;
done;

job=$(kubectl get jobs --no-headers)
word="No resources found"
test "${job#*$word}" != "$job" && exit 1
echo "all jobs finished";



echo "--------- vsphere-provider-controller-manager log begin ----------"
manager_pod_name=$(kubectl get pods -a --no-headers -n vsphere-provider-system | grep vsphere | awk -F" " '{print $1}')
kubectl logs "${manager_pod_name}" -n vsphere-provider-system
echo "--------- vsphere-provider-controller-manager log end ----------"

job_pod_names=$(kubectl get pods -a --no-headers | grep cluster-api-provider-vsphere-ci | awk -F" " '{print $1}')
for job_pod_name in ${job_pod_names}
do
   echo "--------- ci job log begin ----------"
   kubectl logs "${job_pod_name}"
   echo "--------- ci job log end ----------"
done

exit ${ret}
