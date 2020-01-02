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

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	hapi "sigs.k8s.io/cluster-api-provider-vsphere/contrib/haproxy/openapi"
)

// ClientFromHAPIConfigData returns the API client config from some HAPI config
// data.
func ClientFromHAPIConfigData(data []byte) (*hapi.APIClient, error) {
	hapiConfig, err := LoadConfig(data)
	if err != nil {
		return nil, err
	}
	return ClientFromHAPIConfig(hapiConfig)
}

// ClientFromHAPIConfig returns the API client from a HAPI config object.
func ClientFromHAPIConfig(config infrav1.HAProxyAPIConfig) (*hapi.APIClient, error) {
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

	return hapi.NewAPIClient(&hapi.Configuration{
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
	}), nil
}

// LoadConfig returns the configuration for an HAProxy dataplane API client
// from the provided, raw configuration YAML.
func LoadConfig(data []byte) (infrav1.HAProxyAPIConfig, error) {
	config := infrav1.HAProxyAPIConfig{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, errors.Wrap(err, "failed to unmarshal HAProxy API config")
	}
	return config, nil
}
