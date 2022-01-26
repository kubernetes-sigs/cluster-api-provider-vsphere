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

var someProviderID = "vsphere://42305f0b-dad7-1d3d-5727-0eaffffffffc"

//nolint
func TestVsphereMachine_Default(t *testing.T) {
	g := NewWithT(t)
	m := &VSphereMachine{
		Spec: VSphereMachineSpec{},
	}
	m.Default()

	g.Expect(m.Spec.Datacenter).To(Equal("*"))
}

//nolint
func TestVSphereMachine_ValidateCreate(t *testing.T) {

	g := NewWithT(t)
	tests := []struct {
		name           string
		vsphereMachine *VSphereMachine
		wantErr        bool
	}{
		{
			name:           "preferredAPIServerCIDR set on creation ",
			vsphereMachine: createVSphereMachine("foo.com", nil, "192.168.0.1/32", []string{}),
			wantErr:        true,
		},
		{
			name:           "IPs are not in CIDR format",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3"}),
			wantErr:        true,
		},
		{
			name:           "IPs are not valid IPs in CIDR format",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"<nil>/32", "192.168.0.644/33"}),
			wantErr:        true,
		},
		{
			name:           "successful VSphereMachine creation",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}),
			wantErr:        false,
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
func TestVSphereMachine_ValidateUpdate(t *testing.T) {

	g := NewWithT(t)

	tests := []struct {
		name              string
		oldVSphereMachine *VSphereMachine
		vsphereMachine    *VSphereMachine
		wantErr           bool
	}{
		{
			name:              "ProviderID can be updated",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}),
			wantErr:           false,
		},
		{
			name:              "updating ips can be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}),
			wantErr:           false,
		},
		{
			name:              "updating non-existing IP with invalid ips can not be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"<nil>/32", "192.168.0.10/33"}),
			wantErr:           true,
		},
		{
			name:              "updating existing IP with invalid ips can not be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"<nil>/32", "192.168.0.10/33"}),
			wantErr:           true,
		},
		{
			name:              "updating server cannot be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachine("bar.com", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}),
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

func createVSphereMachine(server string, providerID *string, preferredAPIServerCIDR string, ips []string) *VSphereMachine {
	VSphereMachine := &VSphereMachine{
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
	}
	for _, ip := range ips {
		VSphereMachine.Spec.Network.Devices = append(VSphereMachine.Spec.Network.Devices, NetworkDeviceSpec{
			IPAddrs: []string{ip},
		})
	}
	return VSphereMachine
}
