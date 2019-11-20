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

package util_test

import (
	"testing"

	"github.com/onsi/gomega"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func TestGetAPIEndpointForControlPlaneEndpoint(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	testCases := []struct {
		name                 string
		controlPlaneEndpoint string
		expectedAPIEndpoint  infrav1.APIEndpoint
		expectedError        error
	}{
		{
			name:                 "empty value",
			controlPlaneEndpoint: "",
			expectedError:        errors.Errorf("invalid ControlPlaneEndpoint: %q", ""),
		},
		{
			name:                 "IPv4 and port",
			controlPlaneEndpoint: "1.1.1.1:6443",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "1.1.1.1",
				Port: 6443,
			},
		},
		{
			name:                 "http IPv4 and port",
			controlPlaneEndpoint: "http://1.1.1.1:6443",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "1.1.1.1",
				Port: 6443,
			},
		},
		{
			name:                 "https IPv4 and port",
			controlPlaneEndpoint: "https://1.1.1.1:6443",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "1.1.1.1",
				Port: 6443,
			},
		},
		{
			name:                 "IPv4",
			controlPlaneEndpoint: "1.1.1.1",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "1.1.1.1",
			},
		},
		{
			name:                 "http IPv4",
			controlPlaneEndpoint: "http://1.1.1.1",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "1.1.1.1",
			},
		},
		{
			name:                 "https IPv4",
			controlPlaneEndpoint: "https://1.1.1.1",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "1.1.1.1",
			},
		},
		{
			name:                 "FQDN and port",
			controlPlaneEndpoint: "a.b.c.d:6443",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "a.b.c.d",
				Port: 6443,
			},
		},
		{
			name:                 "http FQDN and port",
			controlPlaneEndpoint: "http://a.b.c.d:6443",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "a.b.c.d",
				Port: 6443,
			},
		},
		{
			name:                 "https FQDN and port",
			controlPlaneEndpoint: "https://a.b.c.d:6443",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "a.b.c.d",
				Port: 6443,
			},
		},
		{
			name:                 "FQDN",
			controlPlaneEndpoint: "a.b.c.d",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "a.b.c.d",
			},
		},
		{
			name:                 "http FQDN",
			controlPlaneEndpoint: "http://a.b.c.d",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "a.b.c.d",
			},
		},
		{
			name:                 "https FQDN",
			controlPlaneEndpoint: "https://a.b.c.d",
			expectedAPIEndpoint: infrav1.APIEndpoint{
				Host: "a.b.c.d",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apiEndpoint, err := util.GetAPIEndpointForControlPlaneEndpoint(tc.controlPlaneEndpoint)
			if err != nil {
				if tc.expectedError == nil {
					g.Expect(err).ShouldNot(
						gomega.HaveOccurred(),
						"unexpected error")
				} else {
					g.Expect(err.Error()).Should(
						gomega.Equal(tc.expectedError.Error()),
						"unexpected error")
				}
				return
			}
			g.Expect(apiEndpoint).ShouldNot(gomega.BeNil())
			g.Expect(*apiEndpoint).Should(gomega.Equal(tc.expectedAPIEndpoint))
		})
	}
}
