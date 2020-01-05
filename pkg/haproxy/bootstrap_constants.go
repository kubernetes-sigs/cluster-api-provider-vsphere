/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package haproxy

const haproxyLoadBalancerBootstrapTemplateFormat = `## template: jinja
#cloud-config

write_files:
- path: /etc/haproxy/haproxy.cfg
  owner: haproxy:haproxy
  permissions: "0640"
  content: |
    global
    log    /dev/log  local0
    log    /dev/log  local1 notice
    chroot /var/lib/haproxy
    stats  socket /run/haproxy.sock mode 660 level admin expose-fd listeners
    stats  timeout 30s
    user   haproxy
    group  haproxy
    stats  socket /run/haproxy.sock user haproxy group haproxy mode 660 level admin
    master-worker

    ca-base /etc/ssl/certs
    crt-base /etc/ssl/private

    ssl-default-bind-ciphers ECDH+AESGCM:DH+AESGCM:ECDH+AES256:DH+AES256:ECDH+AES128:DH+AES:RSA+AESGCM:RSA+AES:!aNULL:!MD5:!DSS
    ssl-default-bind-options no-sslv3

    defaults
    log     global
    mode    http
    option  httplog
    option  dontlognull
        timeout connect 5000
        timeout client  50000
        timeout server  50000

    userlist controller
    user {{.Username}} insecure-password {{.Password}}

    program api
    command dataplaneapi --scheme=https --haproxy-bin=/usr/sbin/haproxy --config-file=/etc/haproxy/haproxy.cfg --reload-cmd="/usr/bin/systemctl restart haproxy" --reload-delay=5 --tls-host=0.0.0.0 --tls-port=5556 --tls-ca=/etc/haproxy/ca.crt --tls-certificate=/etc/haproxy/server.crt --tls-key=/etc/haproxy/server.key --userlist=controller
    no option start-on-reload

- path: /etc/haproxy/ca.crt
  owner: haproxy:haproxy
  permissions: "0640"
  content: |
{{ .SigningAuthorityCertificate | Indent 4 }}
- path: /etc/haproxy/ca.key
  owner: haproxy:haproxy
  permissions: "0440"
  content: |
{{ .SigningAuthorityKey | Indent 4 }}

runcmd:
- "hostname \"{{ .DSMetaHostName }}\""
- "hostnamectl set-hostname \"{{ .DSMetaHostName }}\""
- "echo \"::1         ipv6-localhost ipv6-loopback\" >/etc/hosts"
- "echo \"127.0.0.1   localhost {{ .DSMetaHostName }}\" >>/etc/hosts"
- "echo \"127.0.0.1   {{ .DSMetaHostName }}\" >>/etc/hosts"
- "echo \"{{ .DSMetaHostName }}\" >/etc/hostname"
- "new-cert.sh -1 /etc/haproxy/ca.crt -2 /etc/haproxy/ca.key -3 \"127.0.0.1,{{ .DSMetaLocalIPv4 }}\" -4 \"localhost\" \"{{ .DSMetaHostName }}\" /etc/haproxy"

{{- if .User }}
users:
- name: {{ .User.Name }}
  sudo: ALL=(ALL) NOPASSWD:ALL
  {{- if .User.AuthorizedKeys }}
  ssh_authorized_keys:
  {{- range .User.AuthorizedKeys }}
  - "{{ . }}"
  {{- end }}
  {{- end }}
{{- end }}
`
