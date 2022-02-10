/*
Copyright 2021 The Kubernetes Authors.

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

package util

import (
	"testing"

	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

type testCase struct {
	name         string
	input        interface{}
	expectedResp bool
	expectErr    bool
}

func TestIsSupervisorType(t *testing.T) {
	g := NewGomegaWithT(t)

	cases := []testCase{
		{
			name:         "VSphereCluster",
			input:        &infrav1.VSphereCluster{},
			expectedResp: false,
			expectErr:    false,
		},
		{
			name:         "VSphereMachine",
			input:        &infrav1.VSphereMachine{},
			expectedResp: false,
			expectErr:    false,
		},
		{
			name:         "vmwarev1.VSphereCluster",
			input:        &vmwarev1.VSphereCluster{},
			expectedResp: true,
			expectErr:    false,
		},
		{
			name:         "vmwarev1.VSphereMachine",
			input:        &vmwarev1.VSphereMachine{},
			expectedResp: true,
			expectErr:    false,
		},
		{
			name:         "bad type",
			input:        "string",
			expectedResp: false,
			expectErr:    true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actualResp, actualErr := IsSupervisorType(tc.input)
			if tc.expectErr {
				g.Expect(actualErr).To(HaveOccurred())
			} else {
				g.Expect(actualErr).NotTo(HaveOccurred())
			}

			g.Expect(actualResp).To(Equal(tc.expectedResp))
		})
	}
}
