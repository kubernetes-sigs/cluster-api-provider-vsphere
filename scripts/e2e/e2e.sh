#!/bin/bash

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

# and it requires container has volumes
# /root/ssh/.jumphost/jumphost-key
# /root/ssh/.bootstrapper/bootstrapper-key

set -o errexit   # exits immediately on any unexpected error (does not bypass traps)
set -o nounset   # will error if variables are used without first being defined
set -o pipefail  # any non-zero exit code in a piped command causes the pipeline to fail with that code

# VM_CREATED is set to "1" when the VM has been successfully created.
VM_CREATED=

# CONTEXT is set to "prow" when the env var PROW_JOB_ID is set, otherwise set
# to "debug"
CONTEXT=

# CLUSTER_NAME is the name of the cluster creted during the e2e process.
# it will also be the common prefix for all the VMs created
export CLUSTER_NAME="e2e-cluster"

PROW_JOB_ID=${PROW_JOB_ID:-}
PULL_PULL_SHA=${PULL_PULL_SHA:-}

fill_file_with_value() {
  newfilename="${1//template/yml}"
  rm -f "$newfilename" temp.sh
  ( echo "cat <<EOF >$newfilename";
    cat "$1";
    echo "EOF";
  ) >temp.sh
  chmod +x temp.sh
  ./temp.sh
}

revert_bootstrap_vm() {
   bootstrap_vm=$(govc find / -type m -name clusterapi-bootstrap-"$1")
   snapshot_name="cluster-api-provider-vsphere-ci-0.0.2"
   govc snapshot.revert -vm "${bootstrap_vm}" "${snapshot_name}"
   bootstrap_vm_ip=$(govc vm.ip "${bootstrap_vm}")
}

run_cmd_on_bootstrap() {
   ssh -o ProxyCommand="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i /root/ssh/.jumphost/jumphost-key -W %h:%p luoh@$JUMPHOST" root@"$1" \
       -i "/root/ssh/.bootstrapper/bootstrapper-key" \
       -o "StrictHostKeyChecking=no" \
       -o "UserKnownHostsFile=/dev/null" "$2"
}

delete_vm() {
   vm=$(govc find / -type m -name "$1""-*")
   if [ "$vm" = "" ]; then
      vm=$(govc find / -type m -name "$1")
   fi
   govc vm.destroy "${vm}"
   unset VM_CREATED
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
   VM_CREATED=1
   echo "bootstrapper VM ip: ${bootstrap_vm_ip}"
}

export_base64_value() {
   base64_value=$(printf '%s' "${2}" | { base64 -w0 || base64; } 2>/dev/null)
   export "$1"="$base64_value"
}

apply_secret_to_bootstrap() {
   provider_component=${PROVIDER_COMPONENT_SPEC:=provider-components.yaml}
   export_base64_value "PROVIDER_COMPONENT_SPEC" "${provider_component}"
   echo "test ${provider_component}"

   export_base64_value "TARGET_VM_SSH" "${TARGET_VM_SSH}"
   export_base64_value "TARGET_VM_SSH_PUB" "${TARGET_VM_SSH_PUB}"
   export_base64_value "BOOTSTRAP_KUBECONFIG" "${BOOTSTRAP_KUBECONFIG}"

   fill_file_with_value "bootstrap_secret.template"
   run_cmd_on_bootstrap "${bootstrap_vm_ip}" "cat > /tmp/bootstrap_secret.yml" < bootstrap_secret.yml
   run_cmd_on_bootstrap "${bootstrap_vm_ip}" "kubectl --kubeconfig ${kubeconfig_path} create -f /tmp/bootstrap_secret.yml"
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

on_exit() {
  [ "${VM_CREATED}" ] || return 0
  get_bootstrap_vm "${CONTEXT}"
  delete_vm "${CLUSTER_NAME}"
}

# the main loop
trap on_exit EXIT
if [ -z "${PROW_JOB_ID}" ] ; then
   CONTEXT="debug"
   start_docker
else
   CONTEXT="prow"
fi

VERSION=$(git describe --dirty --always 2>/dev/null)
export VERSION
export CAPI_VERSION=v0.2.6
echo "build vSphere controller version: ${VERSION}"
echo "using clusterctl version: ${CAPI_VERSION}"

# install_govc
go get -u github.com/vmware/govmomi/govc

# Push new container images
# TODO the `-k` flag here is a workaround until we can set GCR_KEY_FILE properly
hack/release.sh -t pr -p

# export variables needed to generate spec files
vsphere_controller_version="gcr.io/cluster-api-provider-vsphere/pr/manager:${VERSION}"
export VSPHERE_CONTROLLER_VERSION="${vsphere_controller_version}"
echo "test controller version: ${VSPHERE_CONTROLLER_VERSION}"

cd ./scripts/e2e/bootstrap_job && make push && cd .. || exit 1

# get bootstrap VM
get_bootstrap_vm "${CONTEXT}"

# bootstrap with kind
export bootstrap_vm_ip="${bootstrap_vm_ip}"
fill_file_with_value "kind_config.template"
run_cmd_on_bootstrap "${bootstrap_vm_ip}" "cat > /tmp/kind_config.yml" < kind_config.yml

run_cmd_on_bootstrap "${bootstrap_vm_ip}" 'bash -s' < create_kind_cluster.sh
kubeconfig_path=$(run_cmd_on_bootstrap "${bootstrap_vm_ip}" "kind get kubeconfig-path")
run_cmd_on_bootstrap "${bootstrap_vm_ip}" "sed -i s/localhost/${bootstrap_vm_ip}/g ${kubeconfig_path}"
kubeconfig=$(run_cmd_on_bootstrap "${bootstrap_vm_ip}" "cat ${kubeconfig_path}")
export BOOTSTRAP_KUBECONFIG="${kubeconfig}"
apply_secret_to_bootstrap "${VERSION}"

# launch the job at bootstrap cluster
fill_file_with_value "bootstrap_job.template"
run_cmd_on_bootstrap "${bootstrap_vm_ip}" "cat > /tmp/bootstrap_job.yml" < bootstrap_job.yml
run_cmd_on_bootstrap "${bootstrap_vm_ip}" "kubectl --kubeconfig ${kubeconfig_path} create -f /tmp/bootstrap_job.yml"

# wait for job to finish
run_cmd_on_bootstrap "${bootstrap_vm_ip}" 'bash -s' < wait_for_job.sh
ret="$?"

# cleanup done automatically by trap

exit "${ret}"
