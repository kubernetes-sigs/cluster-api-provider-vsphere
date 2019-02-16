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

# the first argument can be empty or string:"ovf"

get_random_str() {
   if [ -z "$random_str" ]; then
      random_str=$(tr -dc 'a-z0-9' < /dev/urandom | fold -w 8 | head -n 1)
   fi
}

get_bootstrap_ssh_pub() {
   if [ -z "$bootstrap_ssh_pub" ]; then
      bootstrap_ssh_pub=$(cat ../scripts/e2e/hack/bootstrapper.pub)
   fi
}

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
   export_base64_value "BOOTSTRAP_KUBECONFIG" "${BOOTSTRAP_KUBECONFIG}"

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

build_upload_deploy_ovf() { 
   # build the ova
   ova_revision="v0.0.1-debug"
   get_random_str
   get_bootstrap_ssh_pub
   docker volume create ova-tmp
   docker run -t --rm --privileged -v /dev:/dev \
     -v ova-tmp:/go/src/sigs.k8s.io/cluster-api-provider-vsphere/installer/bin \
     gcr.io/cnx-cluster-api/cluster-api-provider-vsphere-installer:"$1" \
     ova-ci --ci-root-password "${random_str}" --ci-root-ssh-key "${bootstrap_ssh_pub}" \
     --build-ova-revision "${ova_revision}"

   mkdir -p ./bin
   CID=$(docker run -d -v ova-tmp:/ova-tmp busybox true)
   docker cp "${CID}":/ova-tmp/cluster-api-vsphere-"${ova_revision}".ova ./bin/
   docker stop "${CID}"
   docker rm "${CID}"
   docker volume rm ova-tmp

   if [ ! -e ./bin/cluster-api-vsphere-"${ova_revision}".ova ]; then
      echo "file not exist"
      exit 1
   fi
 
   # upload and deploy bootstrap VM
   name_prefix="clusterapi-bootstrap"
   bootstrap_vm_name="$name_prefix"-"$random_str"
   bootstrap_vm_folder="clusterapi"
   bootstrap_vm_rp="clusterapi"
   bootstrap_vm_network="sddc-cgw-network-3"
   library_name="clusterapi"
   datastore_name="WorkloadDatastore"
   
   library_id=$(govc library.create -ds="${datastore_name}" "${library_name}")
   govc library.ova "${library_name}" "./bin/cluster-api-vsphere-${ova_revision}.ova"
   govc vcenter.deploy -ds="${datastore_name}" -folder="${bootstrap_vm_folder}" -pool="${bootstrap_vm_rp}" "${library_name}" "cluster-api-vsphere-${ova_revision}.ova"  "$bootstrap_vm_name"
   govc library.rm "${library_id}"
  

   # wait for bootstrap_vm_ip
   bootstrap_vm=$(govc find / -type m -name "$bootstrap_vm_name")
   govc vm.network.change -vm "${bootstrap_vm_name}" -net "${bootstrap_vm_network}" ethernet-0
   govc vm.power -on "${bootstrap_vm}"
      
   bootstrap_vm_ip=$(govc vm.ip "${bootstrap_vm}")
   echo "bootstrap ip: ${bootstrap_vm_ip}"

   retry=100
   until run_cmd_on_bootstrap "${bootstrap_vm_ip}" "ls /etc/kubernetes/admin.conf";
   do
      sleep 6
      retry=$((retry - 1))
      if [ $retry -lt 0 ]
      then
         exit 1
      fi
   done;

   retry=100
   until run_cmd_on_bootstrap "${bootstrap_vm_ip}" "kubectl get nodes";
   do
      sleep 6
      retry=$((retry - 1))
      if [ $retry -lt 0 ]
      then
         exit 1
      fi
   done;
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
   if [ -z "${PULL_PULL_SHA}" ] ; then
      # for periodic job
      vsphere_controller_version="${PULL_JOB_ID}"
   else
      # for presubmit job
      vsphere_controller_version="${PULL_PULL_SHA}"
   fi
fi

export VERSION="${vsphere_controller_version}"
make ci-push
cd ./scripts/e2e/bootstrap_job && make && cd .. || exit 1
echo "build vSphere controller version: ${vsphere_controller_version}"

# set target cluster vm name prefix
get_random_str
target_vm_prefix="clusterapi-""$random_str"
export_base64_value "TARGET_VM_PREFIX" "$target_vm_prefix"

# get bootstrap VM
#install_govc
go get -u github.com/vmware/govmomi/govc
if [ -z "$1" ]; then
   # use vm snapshot by default
   get_bootstrap_vm "$context"
else
   # build ovf and deploy bootstrap ovf
   cd ../../installer/build/container || exit 1
   # push the installer container
   make push
   cd ../../ || exit 1
   build_upload_deploy_ovf "${vsphere_controller_version}"
   cd ../scripts/e2e || exit 1
fi

# apply secret at bootstrap cluster
kubeconfig=$(run_cmd_on_bootstrap "${bootstrap_vm_ip}" "cat /etc/kubernetes/admin.conf")
export BOOTSTRAP_KUBECONFIG="${kubeconfig}"
apply_secret_to_bootstrap "${vsphere_controller_version}"

# launch the job at bootstrap cluster
fill_file_with_value "bootstrap_job.template"
run_cmd_on_bootstrap "${bootstrap_vm_ip}" "cat > /tmp/bootstrap_job.yml" < bootstrap_job.yml
run_cmd_on_bootstrap "${bootstrap_vm_ip}" "kubectl create -f /tmp/bootstrap_job.yml"

# wait for job to finish
run_cmd_on_bootstrap "${bootstrap_vm_ip}" 'bash -s' < wait_for_job.sh
ret="$?"

# cleanup
if [ -z "$1" ]; then
   get_bootstrap_vm "$context"
else
   echo "trying to delete bootstrap vm"
   delete_vm "$bootstrap_vm_name"
fi
delete_vm "$target_vm_prefix"

exit "${ret}"
