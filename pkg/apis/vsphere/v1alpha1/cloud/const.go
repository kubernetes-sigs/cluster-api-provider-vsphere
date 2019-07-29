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

package cloud

const configFormat = `
{{- if IsNotEmpty .Global }}
{{- with .Global }}
[Global]
{{- if .Username }}
user = "{{ .Username }}"
{{- end }}
{{- if .Password }}
password = "{{ .Password }}"
{{- end }}
{{- if .Port }}
port = "{{ .Port }}"
{{- end }}
{{- if .SecretName }}
secret-name = "{{ .SecretName }}"
{{- end }}
{{- if .SecretNamespace }}
secret-namespace = "{{ .SecretNamespace }}"
{{- end }}
{{- if .Insecure }}
insecure-flag = "{{ .Insecure }}"
{{- end }}
{{- if .Datacenters }}
datacenters = "{{ .Datacenters }}"
{{- end }}
{{- if .CAFile }}
ca-file = "{{ .CAFile }}"
{{- end }}
{{- if .Thumbprint }}
thumbprint = "{{ .Thumbprint }}"
{{- end }}
{{- if .RoundTripperCount }}
soap-roundtripper-count = {{ .RoundTripperCount }}
{{- end }}
{{- if .ServiceAccount }}
service-account = {{ .ServiceAccount }}
{{- end }}
{{- if .SecretsDirectory }}
secrets-directory = {{ .SecretsDirectory }}
{{- end }}
{{- if .APIDisable }}
api-disable = {{ .APIDisable }}
{{- end }}
{{- if .APIBindPort }}
api-binding = "{{ .APIBindPort }}"
{{- end }}
{{- end }} {{/* with .Global */}}
{{- end }} {{/* if IsNotEmpty .Global */}}

{{- range $Server, $VCenter := .VCenter }}
[VirtualCenter "{{ $Server }}"]
{{- with $VCenter }}
{{- if .Username }}
user = "{{ .Username }}"
{{- end }}
{{- if .Password }}
password = "{{ .Password }}"
{{- end }}
{{- if .Port }}
port = "{{ .Port }}"
{{- end }}
{{- if .Datacenters }}
datacenters = "{{ .Datacenters }}"
{{- end }}
{{- if .RoundTripperCount }}
soap-roundtripper-count = {{ .RoundTripperCount }}
{{- end }}
{{- if .Thumbprint }}
thumbprint = "{{ .Thumbprint }}"
{{- end }}
{{- end }} {{/* with $VCenter */}}
{{- end }} {{/* range $Server, $VCenter := .VCenter */}}

{{- if IsNotEmpty .Workspace }}
{{- with .Workspace }}
[Workspace]
{{- if .Server }}
server = "{{ .Server }}"
{{- end }}
{{- if .Datacenter }}
datacenter = "{{ .Datacenter }}"
{{- end }}
{{- if .Folder }}
folder = "{{ .Folder }}"
{{- end }}
{{- if .Datastore }}
default-datastore = "{{ .Datastore }}"
{{- end }}
{{- if .ResourcePool }}
resourcepool-path = "{{ .ResourcePool }}"
{{- end }}
{{- end }} {{/* with .Workspace */}}
{{- end }} {{/* if IsNotEmpty .Workspace */}}

{{- if IsNotEmpty .Disk }}
{{- with .Disk }}
[Disk]
{{- if .SCSIControllerType }}
scsicontrollertype = "{{ .SCSIControllerType }}"
{{- end }}
{{- end }} {{/* with .Disk */}}
{{- end }} {{/* if IsNotEmpty .Disk */}}

{{- if IsNotEmpty .Network }}
{{- with .Network }}
[Network]
{{- if .Name }}
public-network = "{{ .Name }}"
{{- end }}
{{- end }} {{/* with .Network */}}
{{- end }} {{/* if IsNotEmpty .Network */}}

{{- if IsNotEmpty .Labels }}
{{- with .Labels }}
[Labels]
{{- if .Zone }}
zone = "{{ .Zone }}"
{{- end }}
{{- if .Region }}
region = "{{ .Region }}"
{{- end }}
{{- end }} {{/* with .Labels */}}
{{- end }} {{/* if IsNotEmpty .Labels */}}
`
