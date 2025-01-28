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
	"k8s.io/utils/ptr"
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
			vsphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "192.168.0.1/32", []string{}, nil, nil),
			wantErr:        true,
		},
		{
			name:           "ProviderID set on creation",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "", &someProviderID, "", []string{}, nil, nil),
			wantErr:        true,
		},
		{
			name:           "IPs are not in CIDR format",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32", "192.168.0.3"}, nil, nil),
			wantErr:        true,
		},
		{
			name:           "successful VSphereMachine creation",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, nil, nil),
			wantErr:        true,
		},
		{
			name:           "incomplete hardware version",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, nil, nil),
			wantErr:        true,
		},
		{
			name:           "incorrect hardware version",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-0", nil, "", []string{"192.168.0.1/32", "192.168.0.3/32"}, nil, nil),
			wantErr:        true,
		},
		{
			name:           "empty pciDevice",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, []infrav1.PCIDeviceSpec{{VGPUProfile: ""}}, nil),
			wantErr:        true,
		},
		{
			name:           "incorrect pciDevice",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, []infrav1.PCIDeviceSpec{{VGPUProfile: "vgpu", DeviceID: ptr.To[int32](1)}}, nil),
			wantErr:        true,
		},
		{
			name:           "incorrect pciDevice",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, []infrav1.PCIDeviceSpec{{VGPUProfile: "vgpu", DeviceID: ptr.To[int32](1), VendorID: ptr.To[int32](1)}}, nil),
			wantErr:        true,
		},
		{
			name:           "incomplete pciDevice",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, []infrav1.PCIDeviceSpec{{DeviceID: ptr.To[int32](1)}}, nil),
			wantErr:        true,
		},
		{
			name:           "incomplete pciDevice",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, []infrav1.PCIDeviceSpec{{VendorID: ptr.To[int32](1)}}, nil),
			wantErr:        true,
		},
		{
			name:           "successful VSphereMachine creation with PCI device",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, []infrav1.PCIDeviceSpec{{DeviceID: ptr.To[int32](1), VendorID: ptr.To[int32](1)}}, nil),
		},
		{
			name:           "successful VSphereMachine creation with vgpu",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, []infrav1.PCIDeviceSpec{{VGPUProfile: "vgpu"}}, nil),
		},
		{
			name:           "successful VSphereMachine creation with hardware version set and namingStrategy not set",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, nil, nil),
		},
		{
			name:           "successful VSphereMachineTemplate creation with namingStrategy.Template not set",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, nil, &infrav1.VSphereVMNamingStrategy{Template: nil}),
		},
		{
			name:           "successful VSphereMachineTemplate creation with namingStrategy.template is set to the fallback value",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, nil, &infrav1.VSphereVMNamingStrategy{Template: ptr.To[string]("{{ .machine.name }}")}),
		},
		{
			name:           "successful VSphereMachineTemplate creation with namingStrategy.template is set the Windows example",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, nil, &infrav1.VSphereVMNamingStrategy{Template: ptr.To[string]("{{ if le (len .machine.name) 20 }}{{ .machine.name }}{{else}}{{ trimSuffix \"-\" (trunc 14 .machine.name) }}-{{ trunc -5 .machine.name }}{{end}}")}),
		},
		{
			name:           "failed VSphereMachineTemplate creation with namingStrategy.template is set to an invalid template",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, nil, &infrav1.VSphereVMNamingStrategy{Template: ptr.To[string]("{{ invalid")}),
			wantErr:        true,
		},
		{
			name:           "failed VSphereMachineTemplate creation with namingStrategy.template  is set to a valid template that renders an invalid name",
			vsphereMachine: createVSphereMachineTemplate("foo.com", "vmx-17", nil, "", []string{}, nil, &infrav1.VSphereVMNamingStrategy{Template: ptr.To[string]("-{{ .machine.name }}")}),
			wantErr:        true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(*testing.T) {
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
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32"}, nil, nil),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", "", &someProviderID, "", []string{"192.168.0.1/32"}, nil, nil),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantErr:           true,
		},
		{
			name:              "ip addresses cannot be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32"}, nil, nil),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", "", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}, nil, nil),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantErr:           true,
		},
		{
			name:              "server cannot be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "", nil, "", []string{"192.168.0.1/32"}, nil, nil),
			vsphereMachine:    createVSphereMachineTemplate("baz.com", "", &someProviderID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}, nil, nil),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantErr:           true,
		},
		{
			name:              "hardware version cannot be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{"192.168.0.1/32"}, nil, nil),
			vsphereMachine:    createVSphereMachineTemplate("baz.com", "vmx-17", nil, "", []string{"192.168.0.1/32"}, nil, nil),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantErr:           true,
		},
		{
			name:              "pci devices cannot be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{"192.168.0.1/32"}, []infrav1.PCIDeviceSpec{{VGPUProfile: "vgpu"}}, nil),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{"192.168.0.1/32"}, []infrav1.PCIDeviceSpec{{VGPUProfile: "new-vgpu"}}, nil),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantErr:           true,
		},
		{
			name:              "with hardware version set and not updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{"192.168.0.1/32"}, nil, nil),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{"192.168.0.1/32"}, nil, nil),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantErr:           false, // explicitly calling out that this is a valid scenario.
		},
		{
			name:              "naming strategy can not be updated",
			oldVSphereMachine: createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{"192.168.0.1/32"}, nil, nil),
			vsphereMachine:    createVSphereMachineTemplate("foo.com", "vmx-16", nil, "", []string{}, nil, &infrav1.VSphereVMNamingStrategy{Template: ptr.To[string]("{{ .machine.name }}")}),
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantErr:           true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(*testing.T) {
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

func createVSphereMachineTemplate(server, hwVersion string, providerID *string, preferredAPIServerCIDR string, ips []string, pciDevices []infrav1.PCIDeviceSpec, vmNamingStrategy *infrav1.VSphereVMNamingStrategy) *infrav1.VSphereMachineTemplate {
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
						PciDevices:      pciDevices,
					},
					NamingStrategy: vmNamingStrategy,
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
