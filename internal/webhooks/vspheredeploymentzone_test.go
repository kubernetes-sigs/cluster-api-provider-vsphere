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

package webhooks

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

func TestVSphereDeploymentZone_Default(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name        string
		boolPtr     *bool
		expectedVal bool
	}{
		{
			name:        "when control plane is not set",
			boolPtr:     nil,
			expectedVal: true,
		},
		{
			name:        "when control plane is set",
			boolPtr:     pointer.Bool(false),
			expectedVal: false,
		},
	}

	for _, tt := range tests {
		// Need to reinit the test variable
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			vdz := infrav1.VSphereDeploymentZone{
				Spec: infrav1.VSphereDeploymentZoneSpec{
					ControlPlane: tt.boolPtr,
				},
			}
			webhook := VSphereDeploymentZoneWebhook{}
			g.Expect(webhook.Default(context.Background(), &vdz)).NotTo(HaveOccurred())
			g.Expect(vdz.Spec.ControlPlane).NotTo(BeNil())
			g.Expect(*vdz.Spec.ControlPlane).To(Equal(tt.expectedVal))
		})
	}
}
