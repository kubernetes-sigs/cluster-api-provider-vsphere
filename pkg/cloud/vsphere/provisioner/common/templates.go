/*
Copyright 2017 The Kubernetes Authors.

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

package common

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/util/version"

	corev1 "k8s.io/api/core/v1"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/provisioner/common/esx"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/provisioner/common/vc"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type TemplateParams struct {
	Token             string
	MajorMinorVersion string
	Cluster           *clusterv1.Cluster
	Machine           *clusterv1.Machine
	DockerImages      []string
	Preloaded         bool
}

// Returns the startup script for the nodes.
func GetNodeStartupScript(params TemplateParams, deployOnVC bool) (string, error) {
	var buf bytes.Buffer
	tName := "fullScript"
	if isPreloaded(params) {
		tName = "preloadedScript"
	}

	if deployOnVC {
		if err := vcNodeStartupScriptTemplate.ExecuteTemplate(&buf, tName, params); err != nil {
			return "", err
		}
	} else {
		if err := esxNodeStartupScriptTemplate.ExecuteTemplate(&buf, tName, params); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func GetMasterStartupScript(params TemplateParams, deployOnVC bool) (string, error) {
	var buf bytes.Buffer
	tName := "fullScript"
	if isPreloaded(params) {
		tName = "preloadedScript"
	}

	if deployOnVC {
		if err := vcMasterStartupScriptTemplate.ExecuteTemplate(&buf, tName, params); err != nil {
			return "", err
		}
	} else {
		if err := esxMasterStartupScriptTemplate.ExecuteTemplate(&buf, tName, params); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func isPreloaded(params TemplateParams) bool {
	return params.Preloaded
}

func preloadScript(t *template.Template, k8sVersion string, dockerImages []string) (string, error) {
	var buf bytes.Buffer
	parsedVersion, err := version.ParseSemantic(k8sVersion)
	if err != nil {
		return buf.String(), err
	}
	params := TemplateParams{
		MajorMinorVersion: fmt.Sprintf("%d.%d", parsedVersion.Major(), parsedVersion.Minor()),
		Machine:           &clusterv1.Machine{},
		DockerImages:      dockerImages,
	}
	params.Machine.Spec.Versions.Kubelet = k8sVersion
	err = t.ExecuteTemplate(&buf, "generatePreloadedImage", params)
	return buf.String(), err
}

var (
	vcNodeStartupScriptTemplate   *template.Template
	vcMasterStartupScriptTemplate *template.Template
	vcCloudInitUserDataTemplate   *template.Template
	vcCloudProviderConfigTemplate *template.Template

	esxNodeStartupScriptTemplate   *template.Template
	esxMasterStartupScriptTemplate *template.Template
	esxCloudInitUserDataTemplate   *template.Template

	cloudInitMetaDataNetworkTemplate *template.Template
	cloudInitMetaDataTemplate        *template.Template
)

func init() {
	endpoint := func(apiEndpoint *clusterv1.APIEndpoint) string {
		return fmt.Sprintf("%s:%d", apiEndpoint.Host, apiEndpoint.Port)
	}

	labelMap := func(labels map[string]string) string {
		var builder strings.Builder
		for k, v := range labels {
			builder.WriteString(fmt.Sprintf("%s=%s,", k, v))
		}
		return strings.TrimRight(builder.String(), ",")
	}

	taintMap := func(taints []corev1.Taint) string {
		var builder strings.Builder
		for _, taint := range taints {
			builder.WriteString(fmt.Sprintf("%s=%s:%s,", taint.Key, taint.Value, taint.Effect))
		}
		return strings.TrimRight(builder.String(), ",")
	}

	base64Decode := func(content string) (string, error) {
		dec, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return "", err
		}
		return string(dec), nil
	}

	indent := func(spaces int, v string) string {
		padding := strings.Repeat(" ", spaces)
		return padding + strings.Replace(v, "\n", "\n"+padding, -1)
	}

	// Force a compliation error if getSubnet changes. This is the
	// signature the templates expect, so changes need to be
	// reflected in templates below.
	var _ func(clusterv1.NetworkRanges) string = vsphereutils.GetSubnet
	funcMap := map[string]interface{}{
		"endpoint":     endpoint,
		"getSubnet":    vsphereutils.GetSubnet,
		"labelMap":     labelMap,
		"taintMap":     taintMap,
		"base64Decode": base64Decode,
		"indent":       indent,
	}
	vcNodeStartupScriptTemplate = template.Must(template.New("vcNodeStartupScript").Funcs(funcMap).Parse(vc.NodeStartupScript))
	vcNodeStartupScriptTemplate = template.Must(vcNodeStartupScriptTemplate.Parse(genericTemplates))
	vcMasterStartupScriptTemplate = template.Must(template.New("vcMasterStartupScript").Funcs(funcMap).Parse(vc.MasterStartupScript))
	vcMasterStartupScriptTemplate = template.Must(vcMasterStartupScriptTemplate.Parse(genericTemplates))
	vcCloudInitUserDataTemplate = template.Must(template.New("vcCloudInitUserData").Funcs(funcMap).Parse(vc.CloudInitUserData))
	vcCloudProviderConfigTemplate = template.Must(template.New("vcCloudProviderConfig").Parse(vc.CloudProviderConfig))

	esxNodeStartupScriptTemplate = template.Must(template.New("esxNodeStartupScript").Funcs(funcMap).Parse(esx.NodeStartupScript))
	esxNodeStartupScriptTemplate = template.Must(esxNodeStartupScriptTemplate.Parse(genericTemplates))
	esxMasterStartupScriptTemplate = template.Must(template.New("esxMasterStartupScript").Funcs(funcMap).Parse(esx.MasterStartupScript))
	esxMasterStartupScriptTemplate = template.Must(esxMasterStartupScriptTemplate.Parse(genericTemplates))
	esxCloudInitUserDataTemplate = template.Must(template.New("esxCloudInitUserData").Funcs(funcMap).Parse(esx.CloudInitUserData))

	cloudInitMetaDataNetworkTemplate = template.Must(template.New("cloudInitMetaDataNetwork").Parse(networkSpec))
	cloudInitMetaDataTemplate = template.Must(template.New("cloudInitMetaData").Parse(cloudInitMetaData))
}

// Returns the startup script for the nodes.
func GetCloudInitMetaData(name string, params *vsphereconfigv1.VsphereMachineProviderConfig) (string, error) {
	var buf bytes.Buffer
	param := CloudInitMetadataNetworkTemplate{
		Networks: params.MachineSpec.Networks,
	}
	if err := cloudInitMetaDataNetworkTemplate.Execute(&buf, param); err != nil {
		return "", err
	}
	param2 := CloudInitMetadataTemplate{
		NetworkSpec: base64.StdEncoding.EncodeToString(buf.Bytes()),
		Hostname:    name,
	}
	buf.Reset()
	if err := cloudInitMetaDataTemplate.Execute(&buf, param2); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Returns the startup script for the nodes.
func GetCloudInitUserData(params CloudInitTemplate, deployOnVC bool) (string, error) {
	var buf bytes.Buffer

	if deployOnVC {
		if err := vcCloudInitUserDataTemplate.Execute(&buf, params); err != nil {
			return "", err
		}
	} else {
		if err := esxCloudInitUserDataTemplate.Execute(&buf, params); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

// Returns the startup script for the nodes.
func GetCloudProviderConfigConfig(params CloudProviderConfigTemplate, deployOnVC bool) (string, error) {
	if deployOnVC {
		var buf bytes.Buffer

		if err := vcCloudProviderConfigTemplate.Execute(&buf, params); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	return "", nil
}

type CloudProviderConfigTemplate struct {
	Datacenter   string
	Server       string
	Insecure     bool
	UserName     string
	Password     string
	ResourcePool string
	Datastore    string
	Network      string
}

type CloudInitTemplate struct {
	Script              string
	IsMaster            bool
	CloudProviderConfig string
	SSHPublicKey        string
	TrustedCerts        []string
	NTPServers          []string
}

type CloudInitMetadataNetworkTemplate struct {
	Networks []vsphereconfigv1.NetworkSpec
}
type CloudInitMetadataTemplate struct {
	NetworkSpec string
	Hostname    string
}

const cloudInitMetaData = `
{
  "network": "{{ .NetworkSpec }}",
  "network.encoding": "base64",
  "local-hostname": "{{ .Hostname }}"
}
`

const networkSpec = `
version: 1
config:
{{- range $index, $network := .Networks}}
  - type: physical
    name: eth{{ $index }}
    subnets:
    {{- if eq $network.IPConfig.NetworkType "static" }}
      - type: static
        address: {{ $network.IPConfig.IP }}
        {{- if $network.IPConfig.Gateway }}
        gateway: {{ $network.IPConfig.Gateway }}
        {{- end }}
        {{- if $network.IPConfig.Netmask }}
        netmask: {{ $network.IPConfig.Netmask }}
        {{- end }}
        {{- if $network.IPConfig.Dns }}
        dns_nameservers:
        {{- range $network.IPConfig.Dns }}
          - {{ . }}
        {{- end }}
        {{- end }}
    {{- else }}
      - type: dhcp
    {{- end }}
{{- end }}
`

const genericTemplates = `
{{ define "fullScript" -}}
  {{ template "startScript" . }}
  {{ template "install" . }}
  {{ template "configure" . }}
  {{ template "endScript" . }}
{{- end }}

{{ define "preloadedScript" -}}
  {{ template "startScript" . }}
  {{ template "configure" . }}
  {{ template "endScript" . }}
{{- end }}

{{ define "generatePreloadedImage" -}}
  {{ template "startScript" . }}
  {{ template "install" . }}

systemctl enable docker || true
systemctl start docker || true

  {{ range .DockerImages }}
docker pull {{ . }}
  {{ end  }}

  {{ template "endScript" . }}
{{- end }}

{{ define "startScript" -}}
#!/bin/bash

set -e
set -x

(
{{- end }}

{{define "endScript" -}}

echo done.
) 2>&1 | tee /var/log/startup.log

{{- end }}
`
