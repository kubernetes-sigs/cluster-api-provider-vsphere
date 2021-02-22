/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	"testing"

	. "github.com/onsi/gomega"
)

//nolint
func TestVSphereCluster_ValidateCreate(t *testing.T) {

	g := NewWithT(t)
	tests := []struct {
		name           string
		vsphereCluster *VSphereCluster
		wantErr        bool
	}{
		{
			name:           "insecure true with empty thumbprint",
			vsphereCluster: createVSphereCluster("foo.com", true, ""),
			wantErr:        false,
		},
		{
			name:           "insecure false with non-empty thumbprint",
			vsphereCluster: createVSphereCluster("foo.com", false, "thumprint:foo"),
			wantErr:        false,
		},
		{
			name:           "insecure true with non-empty thumbprint",
			vsphereCluster: createVSphereCluster("foo.com", true, "thumprint:foo"),
			wantErr:        true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vsphereCluster.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createVSphereCluster(server string, insecure bool, thumbprint string) *VSphereCluster {
	vsphereCluster := &VSphereCluster{
		Spec: VSphereClusterSpec{
			Server:     server,
			Insecure:   &insecure,
			Thumbprint: thumbprint,
		},
	}
	return vsphereCluster
}
