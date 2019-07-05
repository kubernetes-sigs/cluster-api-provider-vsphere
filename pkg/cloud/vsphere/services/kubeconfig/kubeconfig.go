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

package kubeconfig

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphere/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/certificates"
)

// New returns a new kubeconfig for the given cluster.
func New(clusterName, controlPlaneEndpoint string, caKeyPair v1alpha1.KeyPair) (string, error) {
	cert, err := certificates.DecodeCertPEM(caKeyPair.Cert)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode CA Cert")
	} else if cert == nil {
		return "", errors.New("certificate not found in config")
	}

	key, err := certificates.DecodePrivateKeyPEM(caKeyPair.Key)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode private key")
	} else if key == nil {
		return "", errors.New("key not found in status")
	}

	controlPlaneEndpointURL, err := url.Parse(controlPlaneEndpoint)
	if err != nil {
		if controlPlaneEndpointURL, err = url.Parse("https://" + controlPlaneEndpoint); err != nil {
			return "", errors.Wrapf(err, "error parsing control plane endpoint: %s", controlPlaneEndpoint)
		}
	}

	var server string
	if controlPlaneEndpointURL.Path != "" {
		server = controlPlaneEndpointURL.String()
	} else {
		server = fmt.Sprintf("https://%s", controlPlaneEndpointURL.Host)
	}

	cfg, err := certificates.NewKubeconfig(clusterName, server, cert, key)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate a kubeconfig")
	}

	yaml, err := clientcmd.Write(*cfg)
	if err != nil {
		return "", errors.Wrap(err, "failed to serialize config to yaml")
	}

	return string(yaml), nil
}
