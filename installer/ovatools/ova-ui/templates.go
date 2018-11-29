// +build linux

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
