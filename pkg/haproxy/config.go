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
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// Config contains the information required to communicate with an
// HAProxy dataplane API server.
type Config struct {
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

// LoadConfig returns the configuration for an HAProxy dataplane API client
// from the provided, raw configuration YAML.
func LoadConfig(data []byte) (Config, error) {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, errors.Wrap(err, "failed to unmarshal HAProxy API config")
	}
	return config, nil
}
