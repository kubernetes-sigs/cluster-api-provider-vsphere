#!/usr/bin/bash
# Copyright 2018 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
set -euf -o pipefail

# Enable systemd services
systemctl enable toolbox.service
systemctl enable set_sshd.service
systemctl enable getty@tty2.service
systemctl enable ova-firstboot.service
systemctl enable ovf-network.service ova-firewall.service
systemctl enable docker.service
systemctl enable kubelet.service kubeadm.service kubernetes-watchdog.timer

systemctl set-default multi-user.target

# Clean up temporary directories
rm -rf /tmp/* /var/tmp/*
tdnf clean all

# Disable IPv6 redirection and router advertisements in kernel settings
settings="net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.default.accept_redirects = 0
net.ipv6.conf.all.accept_ra = 0
net.ipv6.conf.default.accept_ra = 0"
echo "$settings" > "/etc/sysctl.d/40-ipv6.conf"

# Hardening SSH configuration
afsetting=$(grep "AllowAgentForwarding" /etc/ssh/sshd_config)
if [ -z "$afsetting" ]; then
    echo "AllowAgentForwarding no" >> "/etc/ssh/sshd_config"
else
    sed -i "s/.*AllowAgentForwarding.*/AllowAgentForwarding\ no/g" /etc/ssh/sshd_config
fi

tcpfsetting=$(grep "AllowTcpForwarding" /etc/ssh/sshd_config)
if [ -z "$tcpfsetting" ]; then
    echo "AllowTcpForwarding no" >> "/etc/ssh/sshd_config"
else
    sed -i "s/.*AllowTcpForwarding.*/AllowTcpForwarding\ no/g" /etc/ssh/sshd_config
fi
