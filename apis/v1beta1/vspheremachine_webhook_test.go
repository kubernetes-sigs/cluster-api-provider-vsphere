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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var someProviderID = "vsphere://42305f0b-dad7-1d3d-5727-0eaffffffffc"

func TestVsphereMachine_Default(t *testing.T) {
	g := NewWithT(t)
	m := &VSphereMachine{
		Spec: VSphereMachineSpec{},
	}
	m.Default()

	g.Expect(m.Spec.Datacenter).To(Equal("*"))
}

//nolint:all
func TestVSphereMachine_ValidateCreate(t *testing.T) {

	g := NewWithT(t)
	tests := []struct {
		name           string
		vsphereMachine *VSphereMachine
		wantErr        bool
	}{
		{
			name:           "preferredAPIServerCIDR set on creation ",
			vsphereMachine: createVSphereMachine("foo.com", nil, "192.168.0.1/32", []string{}, VirtualMachinePowerOpModeTrySoft, nil),
			wantErr:        true,
		},
		{
			name:           "IPs are not in CIDR format",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3"}, VirtualMachinePowerOpModeTrySoft, nil),
			wantErr:        true,
		},
		{
			name:           "IPs are not valid IPs in CIDR format",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"<nil>/32", "192.168.0.644/33"}, VirtualMachinePowerOpModeTrySoft, nil),
			wantErr:        true,
		},
		{
			name:           "guestSoftPowerOffTimeout should not be set with powerOffMode set to hard",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, VirtualMachinePowerOpModeHard, &metav1.Duration{Duration: GuestSoftPowerOffDefaultTimeout}),
			wantErr:        true,
		},
		{
			name:           "guestSoftPowerOffTimeout should not be set with powerOffMode set to soft",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, VirtualMachinePowerOpModeSoft, &metav1.Duration{Duration: GuestSoftPowerOffDefaultTimeout}),
			wantErr:        true,
		},
		{
			name:           "guestSoftPowerOffTimeout should not be negative",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, VirtualMachinePowerOpModeHard, &metav1.Duration{Duration: -1234}),
			wantErr:        true,
		},
		{
			name:           "successful VSphereMachine creation",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, VirtualMachinePowerOpModeTrySoft, nil),
			wantErr:        false,
		},
		{
			name:           "successful VSphereMachine creation with powerOffMode set to hard",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, VirtualMachinePowerOpModeHard, nil),
			wantErr:        false,
		},
		{
			name:           "successful VSphereMachine creation with powerOffMode set to soft",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, VirtualMachinePowerOpModeSoft, nil),
			wantErr:        false,
		},
		{
			name:           "successful VSphereMachine creation with powerOffMode set to trySoft and non-default timeout",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, VirtualMachinePowerOpModeTrySoft, &metav1.Duration{Duration: 1234}),
			wantErr:        false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.vsphereMachine.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

//nolint:all
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
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeSoft, nil),
			wantErr:           false,
		},
		{
			name:              "updating ips can be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}, VirtualMachinePowerOpModeSoft, nil),
			wantErr:           false,
		},
		{
			name:              "updating non-existing IP with invalid ips can not be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", nil, VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"<nil>/32", "192.168.0.10/33"}, VirtualMachinePowerOpModeSoft, nil),
			wantErr:           true,
		},
		{
			name:              "updating existing IP with invalid ips can not be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"<nil>/32", "192.168.0.10/33"}, VirtualMachinePowerOpModeSoft, nil),
			wantErr:           true,
		},
		{
			name:              "updating server cannot be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("bar.com", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}, VirtualMachinePowerOpModeSoft, nil),
			wantErr:           true,
		},
		{
			name:              "powerOffMode cannot be updated when new powerOffMode is not valid",
			oldVSphereMachine: createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeTrySoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeHard, &metav1.Duration{Duration: GuestSoftPowerOffDefaultTimeout}),
			wantErr:           true,
		},
		{
			name:              "powerOffMode can be updated to hard",
			oldVSphereMachine: createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeTrySoft, &metav1.Duration{Duration: GuestSoftPowerOffDefaultTimeout}),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeHard, nil),
			wantErr:           false,
		},
		{
			name:              "powerOffMode can be updated to soft",
			oldVSphereMachine: createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeTrySoft, &metav1.Duration{Duration: GuestSoftPowerOffDefaultTimeout}),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, VirtualMachinePowerOpModeSoft, nil),
			wantErr:           false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.vsphereMachine.ValidateUpdate(tc.oldVSphereMachine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createVSphereMachine(server string, providerID *string, preferredAPIServerCIDR string, ips []string, powerOffMode VirtualMachinePowerOpMode, guestSoftPowerOffTimeout *metav1.Duration) *VSphereMachine {
	VSphereMachine := &VSphereMachine{
		Spec: VSphereMachineSpec{
			VirtualMachineCloneSpec: VirtualMachineCloneSpec{
				Server: server,
				Network: NetworkSpec{
					PreferredAPIServerCIDR: preferredAPIServerCIDR,
					Devices:                []NetworkDeviceSpec{},
				},
			},
			ProviderID:               providerID,
			PowerOffMode:             powerOffMode,
			GuestSoftPowerOffTimeout: guestSoftPowerOffTimeout,
		},
	}
	for _, ip := range ips {
		VSphereMachine.Spec.Network.Devices = append(VSphereMachine.Spec.Network.Devices, NetworkDeviceSpec{
			IPAddrs: []string{ip},
		})
	}
	return VSphereMachine
}
