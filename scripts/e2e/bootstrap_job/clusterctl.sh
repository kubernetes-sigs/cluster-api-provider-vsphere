#!/bin/sh

# this script takes care of everything after bootstrap cluster created, it means
# bootstrap need be created beforehand.

# specs requires following enviroments variables:
# VSPHERE_SERVER
# VSPHERE_USERNAME
# VSPHERE_PASSWORD
# VSPHERE_CONTROLLER_VERSION
# TARGET_VM_SSH  (base64 encoded)
# TARGET_VM_SSH_PUB (base64 encoded)


# base64 encode SSH keys (k8s secret automatically decode it)
export TARGET_VM_SSH=$(echo -n "${TARGET_VM_SSH}" | base64 -w 0)
export TARGET_VM_SSH_PUB=$(echo -n "${TARGET_VM_SSH_PUB}" | base64 -w 0)

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

# download kubectl binary
retry=20
until [ "$(ping www.google.com -c 1)" ]
do
   sleep 6
   retry=$((retry - 1))
   if [ $retry -lt 0 ]
   then
      echo "can not access internet"
      exit 1
   fi
done
wget https://storage.googleapis.com/kubernetes-release/release/v1.10.2/bin/linux/amd64/kubectl \
     -O /usr/local/bin/kubectl
chmod +x /usr/local/bin/kubectl

# run clusterctl
echo "test ${PROVIDER_COMPONENT_SPEC}"
/tmp/clusterctl/clusterctl create cluster -e ~/.kube/config -c ./spec/cluster.yml \
    -m ./spec/machines.yml \
    -p ./spec/${PROVIDER_COMPONENT_SPEC} \
    --provider vsphere \
    -v 6

# cleanup the cluster
# TODO (clusterctl delete is not working)
