#/bin/bash
# Copyright (C) 2016 VMware, Inc. All rights reserved.

# This script is used to prepare gobuild slave using
# template linux-centos64-kernel-3.18.21 to install docker

set -e

root=$1

mkdir -p ${root}/docker
ln -s ${root}/docker /var/lib/
echo "Adding build artifactory as yum repo"
yum-config-manager --add-repo https://build-artifactory.eng.vmware.com/artifactory/rpm-packages/

# Installing docker
yum install -y docker-io --nogpgcheck
groupadd docker
usermod -aG docker mts
service docker restart
chown mts:docker /var/run/docker.sock
chown -R mts:docker /var/lib/docker
echo "Installing docker complete."

# Testing docker set up
docker run hello-world
echo "Testing docker running hello-world complete."

docker pull blueshift-docker-local.artifactory.eng.vmware.com/utils/docker:17.10.0-ce-dind
