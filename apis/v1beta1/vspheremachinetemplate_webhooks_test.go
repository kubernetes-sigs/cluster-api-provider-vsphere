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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
)

//nolint
func TestVSphereMachineTemplate_ValidateCreate(t *testing.T) {

	g := NewWithT(t)
	tests := []struct {
		name           string
		vsphereMachine *VSphereMachineTemplate
		wantErr        bool
	}{
		{
			name:           "preferredAPIServerCIDR set on creation ",
			vsphereMachine: createVSphereMachineTemplate("foo.com", nil, "192.168.0.1/32", []string{}),
			wantErr:        true,
		},
		{
			name:           "ProviderID set on creation",
			vsphereMachine: createVSphereMachineTemplate("foo.com", &someProviderID, "", []string{}),
			wantErr:        true,
		},
		{
			name:           "IPs are not in CIDR format",
			vsphereMachine: createVSphereMachineTemplate("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3"}),
			wantErr:        true,
		},
		{
			name:           "successful VSphereMachine creation",
			vsphereMachine: createVSphereMachineTemplate("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}),
			wantErr:        true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vsphereMachine.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

//nolint
func TestVSphereMachineTemplate_ValidateUpdate(t *testing.T) {

	g := NewWithT(t)

	tests := []struct {
		name              string
		oldVSphereMachine *VSphereMachineTemplate
		vsphereMachine    *VSphereMachineTemplate
		wantErr           bool
	}{
		{
			name:              "ProviderID cannot be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}),
			wantErr:           true,
		},
		{
			name:              "updating ips cannot be done",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}),
			wantErr:           true,
		},
		{
			name:              "updating server cannot be done",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachineTemplate("baz.com", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}),
			wantErr:           true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vsphereMachine.ValidateUpdate(tc.oldVSphereMachine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createVSphereMachineTemplate(server string, providerID *string, preferredAPIServerCIDR string, ips []string) *VSphereMachineTemplate {
	VSphereMachineTemplate := &VSphereMachineTemplate{
		Spec: VSphereMachineTemplateSpec{
			Template: VSphereMachineTemplateResource{
				Spec: VSphereMachineSpec{
					ProviderID: providerID,
					VirtualMachineCloneSpec: VirtualMachineCloneSpec{
						Server: server,
						Network: NetworkSpec{
							PreferredAPIServerCIDR: preferredAPIServerCIDR,
							Devices:                []NetworkDeviceSpec{},
						},
					},
				},
			},
		},
	}
	for _, ip := range ips {
		VSphereMachineTemplate.Spec.Template.Spec.Network.Devices = append(VSphereMachineTemplate.Spec.Template.Spec.Network.Devices, NetworkDeviceSpec{
			IPAddrs: []string{ip},
		})
	}
	return VSphereMachineTemplate
}
