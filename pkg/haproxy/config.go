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

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
)

const (
	defaultAPIServerPort = 6443

	// This template is based upon
	// https://github.com/kubernetes-sigs/kubespray/blob/7f74906d332942093ddbc1596497e9e2dd8eb7c2/roles/kubernetes/node/templates/loadbalancer/haproxy.cfg.j2
	haproxyConfigurationTemplate = `global
    log                      stdout format raw local0 info
    chroot                   /var/lib/haproxy
    stats                    timeout 30s
    user                     haproxy
    group                    haproxy
    stats                    socket /run/haproxy.sock user haproxy group haproxy mode 660 level admin
    master-worker
    maxconn                  4000
    ca-base /etc/ssl/certs
    crt-base /etc/ssl/private

    ssl-default-bind-ciphers ECDH+AESGCM:DH+AESGCM:ECDH+AES256:DH+AES256:ECDH+AES128:DH+AES:RSA+AESGCM:RSA+AES:!aNULL:!MD5:!DSS
    ssl-default-bind-options no-sslv3

defaults
    mode                     http
    log                      global
    option                   tcplog
    option                   dontlognull
    option                   http-server-close
    option                   redispatch
    retries                  5
    timeout http-request     5m
    timeout queue            5m
    timeout connect          30s
    timeout client           1m
    timeout server           1m
    timeout tunnel           1h
    timeout http-keep-alive  30s
    timeout check            30s
    maxconn                  4000


userlist controller
user {{.DPConfig.Username}} insecure-password {{.DPConfig.Password}}

frontend healthz
  bind *:8081
  mode http
  monitor-uri /healthz

frontend kube_api_frontend
  bind *:{{.Port | printf "%d"}} name lb
  mode tcp
  option tcplog
  default_backend kube_api_backend

  {{ $port := .Port | printf "%d" }}
backend kube_api_backend
  mode tcp
  balance leastconn
  default-server inter 10s downinter 10s rise 5 fall 3 slowstart 120s maxconn 1000 maxqueue 256 weight 100
  option httpchk GET /healthz
  http-check expect status 200
  {{range .Addresses}}
  server {{ .NodeName }} {{ .IP }}:{{ $port }} check check-ssl verify none
  {{end}}

program api
command dataplaneapi --scheme=https --haproxy-bin=/usr/sbin/haproxy --config-file=/etc/haproxy/haproxy.cfg --reload-cmd="/usr/bin/systemctl restart haproxy" --reload-delay=5 --tls-host=0.0.0.0 --tls-port=5556 --tls-ca=/etc/haproxy/ca.crt --tls-certificate=/etc/haproxy/server.crt --tls-key=/etc/haproxy/server.key --userlist=controller
no option start-on-reload
`
)

var haproxyLoadBalancerBootstrapTemplateFormat = `## template: jinja
#cloud-config

write_files:
- path: /etc/haproxy/haproxy.cfg
  owner: haproxy:haproxy
  permissions: "0640"
  content: |
{{ .HAProxyConfiguration | Indent 4 }}
- path: /etc/haproxy/ca.crt
  owner: haproxy:haproxy
  permissions: "0640"
  content: |
{{ .DPConfig.CertificateAuthorityData | BytesIndent 4 }}
- path: /etc/haproxy/ca.key
  owner: haproxy:haproxy
  permissions: "0440"
  content: |
{{ .CertificateAuthorityKey | BytesIndent 4 }}

runcmd:
- "hostname \"{{ .Hostname }}\""
- "hostnamectl set-hostname \"{{ .Hostname }}\""
- "echo \"::1         ipv6-localhost ipv6-loopback\" >/etc/hosts"
- "echo \"127.0.0.1   localhost {{ .Hostname }}\" >>/etc/hosts"
- "echo \"127.0.0.1   {{ .Hostname }}\" >>/etc/hosts"
- "echo \"{{ .Hostname }}\" >/etc/hostname"
- "new-cert.sh -1 /etc/haproxy/ca.crt -2 /etc/haproxy/ca.key -3 \"127.0.0.1,{{ .IPv4Address }}\" -4 \"localhost\" \"{{ .Hostname }}\" /etc/haproxy"

{{- if .SSHUser }}
users:
- name: {{ .SSHUser.Name }}
  sudo: ALL=(ALL) NOPASSWD:ALL
  {{- if .SSHUser.AuthorizedKeys }}
  ssh_authorized_keys:
  {{- range .SSHUser.AuthorizedKeys }}
  - "{{ . }}"
  {{- end }}
  {{- end }}
{{- end }}
`

// DataplaneConfig contains the information required to communicate with an
// HAProxy dataplane API server.
type DataplaneConfig struct {
	// Debug raises the logging emitted from the generated OpenAPI client
	// bindings.
	// +optional
	Debug bool `json:"debug,omitempty"`

	// InsecureSkipTLSVerify skips the validity check for the server's
	// certificate. This will make your HTTPS connections insecure.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`

	// Server is the address of the HAProxy dataplane API server. This value
	// should include the scheme, host, port, and API version, ex.:
	// https://hostname:port/v1.
	Server string `json:"server"`

	// ServerName is used to verify the hostname on the returned
	// certificates unless InsecureSkipTLSVerify is given. It is also included
	// in the client's handshake to support virtual hosting unless it is
	// an IP address.
	// Defaults to the host part parsed from Server.
	// +optional
	ServerName string `json:"serverName,omitempty"`

	// Username is the username for basic authentication.
	// Defaults to "client"
	// +optional
	Username string `json:"username,omitempty"`

	// Password is the password for basic authentication.
	// Defaults to "cert"
	// +optional
	Password string `json:"password,omitempty"`

	// Timeout is the amount of time before a client request times out.
	// Values should be parseable by time.ParseDuration.
	// Defaults to 10s.
	// +optional
	Timeout string `json:"timeout,omitempty"`

	// CertificateAuthorityData contains PEM-encoded certificate authority
	// certificates.
	CertificateAuthorityData []byte `json:"certificateAuthorityData,omitempty"`

	// ClientCertificateData contains PEM-encoded data from a client cert file
	// for TLS.
	ClientCertificateData []byte `json:"clientCertificateData,omitempty"`

	// ClientKeyData contains PEM-encoded data from a client key file for TLS.
	ClientKeyData []byte `json:"clientKeyData,omitempty"`
}

// RenderConfiguration represents data required to render HAProxyTemplates and
// CloudInit data
type RenderConfiguration struct {
	DPConfig *DataplaneConfig

	// CertificateAuthorityKey contains PEM-encoded certificate authority
	// certificates.
	CertificateAuthorityKey []byte

	// SSHUser is for breakglass access
	SSHUser *infrav1.SSHUser

	// Hostname is the hostname of the load balancer
	Hostname string

	// IPv4Address is the hostname of the load balancer
	IPv4Address string

	// HAProxyConfiguration is the string for haproxy.cfg for use only in CloudInit
	HAProxyConfiguration string

	// Addresses of the machines backing the control plane
	Addresses []corev1.EndpointAddress

	// The load balancer port. Is not currently configurable.
	Port uint32
}

// NewRenderConfiguration returns a new RenderConfiguration
func NewRenderConfiguration() RenderConfiguration {
	return RenderConfiguration{
		Port: defaultAPIServerPort,
	}
}

// WithBootstrapInfo adds information required to generate cloud-init
func (c RenderConfiguration) WithBootstrapInfo(haProxyLoadBalancer infrav1.HAProxyLoadBalancer, username, password string, signingCertificatePEM, signingCertificateKey []byte) RenderConfiguration {
	c.DPConfig = &DataplaneConfig{
		CertificateAuthorityData: signingCertificatePEM,
		Username:                 username,
		Password:                 password,
	}
	c.SSHUser = haProxyLoadBalancer.Spec.User
	c.Hostname = "{{ ds.meta_data.hostname }}"
	c.IPv4Address = "{{ ds.meta_data.local_ipv4 }}"
	c.CertificateAuthorityKey = signingCertificateKey
	return c
}

func (c RenderConfiguration) WithDataPlaneConfig(dpConfig DataplaneConfig) RenderConfiguration {
	c.DPConfig = &dpConfig
	return c
}

// WithAddresses adds API server endpoints to the RenderConfiguration
func (c RenderConfiguration) WithAddresses(addr []corev1.EndpointAddress) RenderConfiguration {
	c.Addresses = addr
	return c
}

// LoadConfig returns the configuration for an HAProxy dataplane API client
// from the provided, raw configuration YAML.
func LoadDataplaneConfig(data []byte) (DataplaneConfig, error) {
	var config DataplaneConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, errors.Wrap(err, "failed to unmarshal HAProxy API config")
	}
	return config, nil
}

// BootstrapDataForLoadBalancer generates the bootstrap data required
// to bootstrap a new HAProxy VM.
func (c *RenderConfiguration) BootstrapDataForLoadBalancer() ([]byte, error) {

	haProxyConfiguration, err := c.RenderHAProxyConfiguration()

	if err != nil {
		return nil, err
	}

	c.HAProxyConfiguration = haProxyConfiguration

	tpl := template.Must(template.
		New("bootstrapTemplate").
		Funcs(template.FuncMap{
			"Indent":      templateStringLinesIndent,
			"BytesIndent": templateByteLinesIndent,
		}).
		Parse(haproxyLoadBalancerBootstrapTemplateFormat))

	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, c); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// RenderHAProxyConfiguration generates a haproxy.cfg file
func (c *RenderConfiguration) RenderHAProxyConfiguration() (string, error) {
	tpl := template.Must(
		template.
			New("haproxyTemplate").
			Funcs(template.FuncMap{
				"Indent":      templateStringLinesIndent,
				"BytesIndent": templateByteLinesIndent,
			}).
			Parse(haproxyConfigurationTemplate))
	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, c); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func templateStringLinesIndent(i int, input string) string {
	split := strings.Split(input, "\n")
	ident := "\n" + strings.Repeat(" ", i)
	return strings.Repeat(" ", i) + strings.Join(split, ident)
}

func templateByteLinesIndent(i int, input []byte) string {
	return templateStringLinesIndent(i, string(input))
}
