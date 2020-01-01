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
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	hapi "sigs.k8s.io/cluster-api-provider-vsphere/contrib/haproxy/openapi"
)

// Config contains the information required to communicate with an HAProxy
// dataplane API server.
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
	// +optional
	CertificateAuthorityData []byte `json:"certificateAuthorityData,omitempty"`

	// ClientCertificateData contains PEM-encoded data from a client cert file
	// for TLS.
	// +optional
	ClientCertificateData []byte `json:"clientCertificateData,omitempty"`

	// ClientKeyData contains PEM-encoded data from a client key file for TLS.
	// +optional
	ClientKeyData []byte `json:"clientKeyData,omitempty"`

	// SigningKeyData contains a PEM-encoded certificate used to sign new
	// client certificates.
	// +optional
	SigningKeyData []byte `json:"signingKeyData,omitempty"`

	// SigningCertificateData contains PEM-encoded data from the key file used
	// to sign new client certificates.
	// +optional
	SigningCertificateData []byte `json:"signingCertificateData,omitempty"`
}

// HAPIClientFromConfig returns an HAProxy dataplane API client from
// the provided configuration object.
func HAPIClientFromConfig(config *hapi.Configuration) (*hapi.APIClient, error) {
	return hapi.NewAPIClient(config), nil
}

// HAPIClientFromConfigData returns an HAProxy dataplane API client from
// the provided, raw configuration YAML/JSON.
func HAPIClientFromConfigData(data []byte) (*hapi.APIClient, error) {
	config, err := LoadConfig(data)
	if err != nil {
		return nil, err
	}
	return HAPIClientFromConfig(config)
}

// LoadConfig returns the configuration for an HAProxy dataplane API client
// from the provided, raw configuration YAML.
func LoadConfig(data []byte) (*hapi.Configuration, error) {
	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal HAProxy API config")
	}

	// Load the CA certs.
	var trustedRoots *x509.CertPool
	if len(config.CertificateAuthorityData) > 0 {
		trustedRoots = x509.NewCertPool()
		if !trustedRoots.AppendCertsFromPEM(config.CertificateAuthorityData) {
			return nil, errors.New("failed to parse certificate authority data from HAProxy API config")
		}
	}

	// Load the client cert.
	clientCrt, err := tls.X509KeyPair(config.ClientCertificateData, config.ClientKeyData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse client certificate/key from HAProxy API config")
	}

	// Get the timeout.
	szTimeout := config.Timeout
	if szTimeout == "" {
		szTimeout = "10s"
	}
	timeout, err := time.ParseDuration(szTimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse timeout %q from HAProxy API config", szTimeout)
	}

	// Create a cookie jar.
	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cookie jar for HAProxy API config")
	}

	// Parse the server URL.
	serverURL, err := url.Parse(config.Server)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse server URL %q from HAProxy API config", config.Server)
	}

	// Build the default header map to include the basic auth credentials.
	username := config.Username
	if username == "" {
		username = "client"
	}
	password := config.Password
	if password == "" {
		password = "cert"
	}
	credentials := fmt.Sprintf("%s:%s", username, password)
	credentials64 := base64.StdEncoding.EncodeToString([]byte(credentials))
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Basic %s", credentials64),
	}

	return &hapi.Configuration{
		BasePath:      serverURL.String(),
		DefaultHeader: headers,
		UserAgent:     "CAPV HAProxy Load Balancer Client",
		Debug:         config.Debug,
		HTTPClient: &http.Client{
			Jar:     cookieJar,
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					// Enable client-side cert auth
					Certificates: []tls.Certificate{clientCrt},
					// The CAs used to verify the server cert
					RootCAs: trustedRoots,
					// May be used to override the name of the peer the client
					// is verifying
					ServerName: config.ServerName,
				},
			},
		},
	}, nil
}
