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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

var someProviderID = "vsphere://42305f0b-dad7-1d3d-5727-0eaffffffffc"

func TestVsphereMachine_Default(t *testing.T) {
	g := NewWithT(t)
	m := &infrav1.VSphereMachine{
		Spec: infrav1.VSphereMachineSpec{},
	}
	webhook := &VSphereMachineWebhook{}
	g.Expect(webhook.Default(context.Background(), m)).ToNot(HaveOccurred())

	g.Expect(m.Spec.Datacenter).To(Equal("*"))
}

func TestVSphereMachine_ValidateCreate(t *testing.T) {
	g := NewWithT(t)
	tests := []struct {
		name           string
		vsphereMachine *infrav1.VSphereMachine
		wantErr        bool
	}{
		{
			name:           "preferredAPIServerCIDR set on creation ",
			vsphereMachine: createVSphereMachine("foo.com", nil, "192.168.0.1/32", []string{}, infrav1.VirtualMachinePowerOpModeTrySoft, nil),
			wantErr:        true,
		},
		{
			name:           "IPs are not in CIDR format",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3"}, infrav1.VirtualMachinePowerOpModeTrySoft, nil),
			wantErr:        true,
		},
		{
			name:           "IPs are not valid IPs in CIDR format",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"<nil>/32", "192.168.0.644/33"}, infrav1.VirtualMachinePowerOpModeTrySoft, nil),
			wantErr:        true,
		},
		{
			name:           "guestSoftPowerOffTimeout should not be set with powerOffMode set to hard",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, infrav1.VirtualMachinePowerOpModeHard, &metav1.Duration{Duration: infrav1.GuestSoftPowerOffDefaultTimeout}),
			wantErr:        true,
		},
		{
			name:           "guestSoftPowerOffTimeout should not be set with powerOffMode set to soft",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, infrav1.VirtualMachinePowerOpModeSoft, &metav1.Duration{Duration: infrav1.GuestSoftPowerOffDefaultTimeout}),
			wantErr:        true,
		},
		{
			name:           "guestSoftPowerOffTimeout should not be negative",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, infrav1.VirtualMachinePowerOpModeHard, &metav1.Duration{Duration: -1234}),
			wantErr:        true,
		},
		{
			name:           "successful VSphereMachine creation",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, infrav1.VirtualMachinePowerOpModeTrySoft, nil),
			wantErr:        false,
		},
		{
			name:           "successful VSphereMachine creation with powerOffMode set to hard",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, infrav1.VirtualMachinePowerOpModeHard, nil),
			wantErr:        false,
		},
		{
			name:           "successful VSphereMachine creation with powerOffMode set to soft",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			wantErr:        false,
		},
		{
			name:           "successful VSphereMachine creation with powerOffMode set to trySoft and non-default timeout",
			vsphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, infrav1.VirtualMachinePowerOpModeTrySoft, &metav1.Duration{Duration: 1234}),
			wantErr:        false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			webhook := &VSphereMachineWebhook{}
			_, err := webhook.ValidateCreate(context.Background(), tc.vsphereMachine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestVSphereMachine_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name              string
		oldVSphereMachine *infrav1.VSphereMachine
		vsphereMachine    *infrav1.VSphereMachine
		wantErr           bool
	}{
		{
			name:              "ProviderID can be updated",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			wantErr:           false,
		},
		{
			name:              "updating ips can be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			wantErr:           false,
		},
		{
			name:              "updating non-existing IP with invalid ips can not be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", nil, infrav1.VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"<nil>/32", "192.168.0.10/33"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			wantErr:           true,
		},
		{
			name:              "updating existing IP with invalid ips can not be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"<nil>/32", "192.168.0.10/33"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			wantErr:           true,
		},
		{
			name:              "updating server cannot be done",
			oldVSphereMachine: createVSphereMachine("foo.com", nil, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			vsphereMachine:    createVSphereMachine("bar.com", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			wantErr:           true,
		},
		{
			name:              "powerOffMode cannot be updated when new powerOffMode is not valid",
			oldVSphereMachine: createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeTrySoft, nil),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeHard, &metav1.Duration{Duration: infrav1.GuestSoftPowerOffDefaultTimeout}),
			wantErr:           true,
		},
		{
			name:              "powerOffMode can be updated to hard",
			oldVSphereMachine: createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeTrySoft, &metav1.Duration{Duration: infrav1.GuestSoftPowerOffDefaultTimeout}),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeHard, nil),
			wantErr:           false,
		},
		{
			name:              "powerOffMode can be updated to soft",
			oldVSphereMachine: createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeTrySoft, &metav1.Duration{Duration: infrav1.GuestSoftPowerOffDefaultTimeout}),
			vsphereMachine:    createVSphereMachine("foo.com", &someProviderID, "", []string{"192.168.0.1/32"}, infrav1.VirtualMachinePowerOpModeSoft, nil),
			wantErr:           false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			webhook := &VSphereMachineWebhook{}
			_, err := webhook.ValidateUpdate(context.Background(), tc.oldVSphereMachine, tc.vsphereMachine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createVSphereMachine(server string, providerID *string, preferredAPIServerCIDR string, ips []string, powerOffMode infrav1.VirtualMachinePowerOpMode, guestSoftPowerOffTimeout *metav1.Duration) *infrav1.VSphereMachine {
	VSphereMachine := &infrav1.VSphereMachine{
		Spec: infrav1.VSphereMachineSpec{
			VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
				Server: server,
				Network: infrav1.NetworkSpec{
					PreferredAPIServerCIDR: preferredAPIServerCIDR,
					Devices:                []infrav1.NetworkDeviceSpec{},
				},
			},
			ProviderID:               providerID,
			PowerOffMode:             powerOffMode,
			GuestSoftPowerOffTimeout: guestSoftPowerOffTimeout,
		},
	}
	for _, ip := range ips {
		VSphereMachine.Spec.Network.Devices = append(VSphereMachine.Spec.Network.Devices, infrav1.NetworkDeviceSpec{
			IPAddrs: []string{ip},
		})
	}
	return VSphereMachine
}
