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
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

func TestVSphereMachineTemplate_ValidateCreate(t *testing.T) {
	g := NewWithT(t)
	tests := []struct {
		name           string
		vsphereMachine *infrav1.VSphereMachineTemplate
		wantErr        bool
	}{
		{
			name:           "preferredAPIServerCIDR set on creation ",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "192.168.0.1/32", []string{}),
			wantErr:        true,
		},
		{
			name:           "ProviderID set on creation",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "", &someProviderID, "", []string{}),
			wantErr:        true,
		},
		{
			name:           "IPs are not in CIDR format",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32", "192.168.0.3"}),
			wantErr:        true,
		},
		{
			name:           "successful VSphereMachine creation",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}),
			wantErr:        true,
		},
		{
			name:           "incomplete hardware version",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}),
			wantErr:        true,
		},
		{
			name:           "incorrect hardware version",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-0", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}),
			wantErr:        true,
		},
		{
			name:           "successful VSphereMachine creation with hardware version set",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			webhook := &VSphereMachineTemplateWebhook{}
			_, err := webhook.ValidateCreate(context.Background(), tc.vsphereMachine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestVSphereMachineTemplate_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)
	tests := []struct {
		name              string
		oldVSphereMachine *infrav1.VSphereMachineTemplate
		vsphereMachine    *infrav1.VSphereMachineTemplate
		req               *admission.Request
		wantErr           bool
	}{
		{
			name:              "ProviderID cannot be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", "", &someProviderID, "", []string{"192.168.0.1/32"}),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: pointer.Bool(false)}},
			wantErr:           true,
		},
		{
			name:              "ip addresses cannot be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", "", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: pointer.Bool(false)}},
			wantErr:           true,
		},
		{
			name:              "server cannot be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachineTemplate("baz.com", "", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: pointer.Bool(false)}},
			wantErr:           true,
		},
		{
			name:              "hardware version cannot be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachineTemplate("baz.com", "vmx-17", nil, "", []string{"192.168.0.1/32"}),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: pointer.Bool(false)}},
			wantErr:           true,
		},
		{
			name:              "with hardware version set and not updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{"192.168.0.1/32"}),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{"192.168.0.1/32"}),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: pointer.Bool(false)}},
			wantErr:           false, // explicitly calling out that this is a valid scenario.
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			webhook := &VSphereMachineTemplateWebhook{}
			ctx := context.Background()
			if tc.req != nil {
				ctx = admission.NewContextWithRequest(ctx, *tc.req)
			}
			_, err := webhook.ValidateUpdate(ctx, tc.oldVSphereMachine, tc.vsphereMachine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createVSphereMachineTemplate(server, hwVersion string, providerID *string, preferredAPIServerCIDR string, ips []string) *infrav1.VSphereMachineTemplate {
	vsphereMachineTemplate := &infrav1.VSphereMachineTemplate{
		Spec: infrav1.VSphereMachineTemplateSpec{
			Template: infrav1.VSphereMachineTemplateResource{
				Spec: infrav1.VSphereMachineSpec{
					ProviderID: providerID,
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
						Server: server,
						Network: infrav1.NetworkSpec{
							PreferredAPIServerCIDR: preferredAPIServerCIDR,
							Devices:                []infrav1.NetworkDeviceSpec{},
						},
						HardwareVersion: hwVersion,
					},
				},
			},
		},
	}
	for _, ip := range ips {
		vsphereMachineTemplate.Spec.Template.Spec.Network.Devices = append(vsphereMachineTemplate.Spec.Template.Spec.Network.Devices, infrav1.NetworkDeviceSpec{
			IPAddrs: []string{ip},
		})
	}
	return vsphereMachineTemplate
}
