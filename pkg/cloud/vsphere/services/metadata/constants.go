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

package metadata

const format = `
instance-id: "{{ .Hostname }}"
local-hostname: "{{ .Hostname }}"
network:
  version: 2
  ethernets:
    {{- range $net := .Devices }}
    "{{ $net.NetworkName }}":
      match:
        macaddress: "{{ $net.MACAddr }}"
      wakeonlan: true
      dhcp4: {{ $net.DHCP4 }}
      dhcp6: {{ $net.DHCP6 }}
      {{- if $net.IPAddrs }}
      addresses:
      {{- range $net.IPAddrs }}
      - "{{ . }}"
      {{- end }}
      {{- end }}
      {{- if $net.Gateway4 }}
      gateway4: "{{ $net.Gateway4 }}"
      {{- end }}
      {{- if $net.Gateway6 }}
      gateway6: "{{ $net.Gateway6 }}"
      {{- end }}
      {{- if .MTU }}
      mtu: {{ .MTU }}
      {{- end }}
      {{- if .Routes }}
      routes:
      {{- range .Routes }}
      - to: "{{ .To }}"
        via: "{{ .Via }}"
        metric: {{ .Metric }}
      {{- end }}
      {{- end }}
      {{- if nameservers $net }}
      nameservers:
        {{- if $net.Nameservers }}
        addresses:
        {{- range $net.Nameservers }}
        - "{{ . }}"
        {{- end }}
        {{- end }}
        {{- if $net.SearchDomains }}
        search:
        {{- range $net.SearchDomains }}
        - "{{ . }}"
        {{- end }}
        {{- end }}
      {{- end }}
    {{- end }}
  {{- if .Routes }}
  routes:
  {{- range .Routes }}
  - to: "{{ .To }}"
    via: "{{ .Via }}"
    metric: {{ .Metric }}
  {{- end }}
  {{- end }}
`
