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

# it requires the following enviroment variables:
# JUMPHOST
# GOVC_URL
# GOVC_USERNAME
# GOVC_PASSWORD
# VSPHERE_CONTROLLER_VERSION

# and it requires container has volumes
# /root/ssh/.jumphost/jumphost-key
# /root/ssh/.bootstrapper/bootstrapper-key

# the first argument should be the vsphere controller version

fill_file_with_value() {
  newfilename="$(echo "$1" | sed 's/template/yml/g')"
  rm -f "$newfilename" temp.sh  
  ( echo "cat <<EOF >$newfilename";
    cat "$1";
    echo "EOF";
  ) >temp.sh
  chmod +x temp.sh
  ./temp.sh
}

revert_bootstrap_vm() {
   bootstrap_vm=$(govc find / -type m -name clusterapi-bootstrap-$1)
   snapshot_name="cluster-api-provider-vsphere-ci-0.0.1"
   govc snapshot.revert -vm "${bootstrap_vm}" "${snapshot_name}"  
   bootstrap_vm_ip=$(govc vm.ip "${bootstrap_vm}")
}

run_cmd_on_bootstrap() {
   ssh -o ProxyCommand="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i /root/ssh/.jumphost/jumphost-key -W %h:%p luoh@$JUMPHOST" vmware@"$1" \
       -i "/root/ssh/.bootstrapper/bootstrapper-key" \
       -o "StrictHostKeyChecking=no" \
       -o "UserKnownHostsFile=/dev/null" "$2"
}

delete_vm() {
   vm=$(govc find / -type m -name "$1""-*")
   govc vm.power -off "${vm}"
   govc vm.destroy "${vm}"
}

get_bootstrap_vm() {
   export GOVC_INSECURE=1
   retry=10
   bootstrap_vm_ip=""
   until [ $bootstrap_vm_ip ]
   do
      sleep 6
      revert_bootstrap_vm "$1"
      retry=$((retry - 1))
      if [ $retry -lt 0 ]
      then
         break
      fi
   done

   if [ -z "$bootstrap_vm_ip" ] ; then
      echo "bootstrap vm ip is empty"
      exit 1
   fi
   echo "bootstrapper VM ip: ${bootstrap_vm_ip}"
}

export_base64_value() {
   base64_value=$(echo -n "$2" | base64 -w 0)
   export "$1"="$base64_value"
}

apply_secret_to_bootstrap() {
   provider_component=${PROVIDER_COMPONENT_SPEC:=provider-components-v2.0.yml}
   export_base64_value "PROVIDER_COMPONENT_SPEC" "${provider_component}"
   echo "test ${provider_component}"

   echo "test controller version $1"
   vsphere_controller_version="gcr.io/cnx-cluster-api/vsphere-cluster-api-provider:$1"
   export_base64_value "VSPHERE_CONTROLLER_VERSION" "${vsphere_controller_version}"
   echo "test ${vsphere_controller_version}"

   export_base64_value "VSPHERE_SERVER" "${GOVC_URL}"
   export_base64_value "VSPHERE_USERNAME" "${GOVC_USERNAME}"
   export_base64_value "VSPHERE_PASSWORD" "${GOVC_PASSWORD}"
   export_base64_value "TARGET_VM_SSH" "${TARGET_VM_SSH}"
   export_base64_value "TARGET_VM_SSH_PUB" "${TARGET_VM_SSH_PUB}"

   fill_file_with_value "bootstrap_secret.template"
   run_cmd_on_bootstrap "${bootstrap_vm_ip}" "cat > /tmp/bootstrap_secret.yml" < bootstrap_secret.yml
   run_cmd_on_bootstrap "${bootstrap_vm_ip}" "kubectl create -f /tmp/bootstrap_secret.yml"
}

start_docker() {
   service docker start
   # the service can be started but the docker socket not ready, wait for ready
   WAIT_N=0
   MAX_WAIT=5
   while true; do
      # docker ps -q should only work if the daemon is ready
      docker ps -q > /dev/null 2>&1 && break
      if [ ${WAIT_N} -lt ${MAX_WAIT} ]; then
         WAIT_N=$((WAIT_N+1))
         echo "Waiting for docker to be ready, sleeping for ${WAIT_N} seconds."
         sleep ${WAIT_N}
      else
         echo "Reached maximum attempts, not waiting any longer..."
         break
      fi
   done
}

clone_clusterapi_vsphere_repo() {
   mkdir -p /go/src/sigs.k8s.io/cluster-api-provider-vsphere
   git clone https://github.com/kubernetes-sigs/cluster-api-provider-vsphere.git \
             /go/src/sigs.k8s.io/cluster-api-provider-vsphere/
}

install_govc() {
   govc_bin="/tmp/govc/bin"
   mkdir -p "${govc_bin}"
   curl -sL https://github.com/vmware/govmomi/releases/download/v0.19.0/govc_linux_amd64.gz -o "${govc_bin}"/govc.gz
   gunzip "${govc_bin}"/govc.gz
   chmod +x "${govc_bin}"/govc
   export PATH=${govc_bin}:$PATH
}

# the main loop
vsphere_controller_version=""
context=""
if [ -z "${PROW_JOB_ID}" ] ; then
   context="debug"
   start_docker
   vsphere_controller_version=$(shell git describe --exact-match 2> /dev/null || \
      git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
else
   context="prow"
   vsphere_controller_version="${PULL_PULL_SHA}"
fi

export VERSION="${vsphere_controller_version}" && make ci-push
cd ./scripts/e2e/bootstrap_job && make && cd .. || exit 1

export_base64_value "CONTEXT" "$context"
echo "build vSphere controller version: ${vsphere_controller_version}"

# get bootstrap VM
install_govc
get_bootstrap_vm "$context"

# apply secret at bootstrap cluster
apply_secret_to_bootstrap "${vsphere_controller_version}"

# launch the job at bootstrap cluster
fill_file_with_value "bootstrap_job.template"
run_cmd_on_bootstrap "${bootstrap_vm_ip}" "cat > /tmp/bootstrap_job.yml" < bootstrap_job.yml
run_cmd_on_bootstrap "${bootstrap_vm_ip}" "kubectl create -f /tmp/bootstrap_job.yml"

# wait for job to finish
run_cmd_on_bootstrap "${bootstrap_vm_ip}" 'bash -s' < wait_for_job.sh
ret="$?"

# cleanup
get_bootstrap_vm "$context"
delete_vm "clusterapi-""$context"

exit "${ret}"
