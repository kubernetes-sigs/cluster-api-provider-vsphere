#!/bin/sh

# this script takes care of everything after bootstrap cluster created, it means
# bootstrap need be created beforehand.

# specs requires following enviroments variables:
# VSPHERE_SERVER
# VSPHERE_USERNAME
# VSPHERE_PASSWORD
# VSPHERE_CONTROLLER_VERSION
# TARGET_VM_PREFIX
# TARGET_VM_SSH  (base64 encoded)
# TARGET_VM_SSH_PUB (base64 encoded)
# BOOTSTRAP_KUBECONFIG

cleanup() {
   kubectl delete -f ./spec/machineset.yaml
   kubectl delete -f ./spec/machine.yaml
   kubectl delete -f ./spec/cluster.yaml
}

# param: 
# kubeconfig
# number of nodes
wait_for_node() {
   num_of_nodes="$2"
   TOTAL=600
   INTERVAL=6
   retry=$((${TOTAL}/ INTERVAL))
   ret=0

   # wait for the number of nodes expected
   until [ $(kubectl --kubeconfig=$1 get nodes --no-headers | awk '{print NR}' | tail -1) -eq "$num_of_nodes" ]
   do 
      sleep ${INTERVAL};
      retry=$((retry - 1))
      if [ $retry -lt 0 ];
      then
         ret=1
         echo "timeout"
         break
      fi;
      kubectl --kubeconfig=$1 get nodes --no-headers; 
   done;

   # wait for all nodes are ready
   retry=$((${TOTAL}/ INTERVAL)) 
   until kubectl --kubeconfig="$1" get nodes --no-headers | awk -F" " '{s+=($2!="Ready")} END {exit s}';
   do
      sleep ${INTERVAL};
      retry=$((retry - 1))
      if [ $retry -lt 0 ];
      then
         ret=1
         echo "timeout"
         break
      fi;
      kubectl --kubeconfig=$1 get nodes --no-headers;
   done;

   return $ret
}

run_job() {
   case $1 in
        "clusterctl-machine")
            /tmp/clusterctl/clusterctl create cluster -e ~/.kube/config -c ./spec/cluster.yml \
              -m ./spec/machines.yml \
              -p ./spec/provider-components.yml \
              --provider vsphere \
              -v 6 || { echo 'clusterctl machine deployment failed' ; cleanup; exit 1; }
            ;;
        "clusterctl-machineset")
            /tmp/clusterctl/clusterctl create cluster -e ~/.kube/config -c ./spec/cluster.yml \
              -m ./spec/machines.yml \
              -p ./spec/provider-components.yml \
              --provider vsphere \
              -v 6  || { echo 'clusterctl machine deployment failed' ; cleanup; exit 1; }

            kubectl create -f ./spec/machineset.yaml

            wait_for_node kubeconfig 3 || { echo 'nodes are not ready' ; cleanup; exit 1; }
            ;;
        "clusterapi")
            ;;
        "machine")
            ;;
        "machineset")
            ;;
        *)
   esac
   ret="$?"
}

# clusterctl requires ssh key file and kubeconfig file
mkdir -p ~/.ssh
mkdir -p ~/.kube
echo -n "${TARGET_VM_SSH}" > ~/.ssh/vsphere_tmp
echo -n "${TARGET_VM_SSH_PUB}" > ~/.ssh/vsphere_tmp.pub
echo "${BOOTSTRAP_KUBECONFIG}" > ~/.kube/config
chmod 600 ~/.ssh/vsphere_tmp

# base64 encode SSH keys (k8s secret automatically decode it)
export TARGET_VM_SSH=$(echo -n "${TARGET_VM_SSH}" | base64 -w 0)
export TARGET_VM_SSH_PUB=$(echo -n "${TARGET_VM_SSH_PUB}" | base64 -w 0)
echo "${TARGET_VM_SSH_PUB}"

# prepare spec
for filename in spec/*.template; do
  newfilename="$(echo "$filename" | sed 's/template/yml/g')"
  rm -f "$newfilename" temp.sh  
  ( echo "cat <<EOF >$newfilename";
    cat "$filename";
    echo "EOF";
  ) >temp.sh
  chmod +x temp.sh
  ./temp.sh
done
rm temp.sh

run_job "clusterctl-machineset"
cleanup
exit $ret