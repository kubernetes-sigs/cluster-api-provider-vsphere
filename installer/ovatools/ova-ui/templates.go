// +build linux

/*
Copyright 2018 The Kubernetes Authors.

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

package main

type OVAProps struct {
	Build  string
	CPUs   string
	Memory string
	Error  string
}

type OVAStatus struct {
	KubeStatus     string
	SCPPath        string
	SSHFingerprint string
	DNS            string
	IP             string
	Gateway        string
}

var TopPanelTemplate = `
Kubernetes Cluster API vSphere Appliance {{.Build}}

{{.CPUs}}
{{.Memory}}

{{with .Error -}}
[error: {{.}}](fg-red)
{{end}}
`

var BottomPanelTemplate = `
Kubernetes cluster status: {{.KubeStatus}}

{{with .SCPPath -}}
Download Kubeconfig on your machine:
scp {{.}} clusterapi.kubeconfig
{{end}}
SSH Key fingerprint:
{{.SSHFingerprint}}

Access the Documentation at:
https://github.com/kubernetes-sigs/cluster-api-provider-vsphere

Network Status:

DNS: {{.DNS}}
IP: {{.IP}}
Gateway: {{.Gateway}}

Settings with 'MATCH' indicate the system network
configuration matches the OVF network configuration.
`
