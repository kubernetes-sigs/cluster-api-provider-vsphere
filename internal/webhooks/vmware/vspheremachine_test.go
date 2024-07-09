/*
Copyright 2024 The Kubernetes Authors.

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

package vmware

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

func TestVSphereMachine_ValidateUpdate(t *testing.T) {
	fakeProviderID := "fake-000000"
	tests := []struct {
		name              string
		oldVSphereMachine *vmwarev1.VSphereMachine
		vsphereMachine    *vmwarev1.VSphereMachine
		wantErr           bool
	}{
		{
			name:              "updating ProviderID can be done",
			oldVSphereMachine: createVSphereMachine(nil, "tkgs-old-imagename", "best-effort-xsmall", "wcpglobalstorageprofile", "vmx-15"),
			vsphereMachine:    createVSphereMachine(&fakeProviderID, "tkgs-old-imagename", "best-effort-xsmall", "wcpglobalstorageprofile", "vmx-15"),
			wantErr:           false,
		},
		{
			name:              "updating ImageName cannot be done",
			oldVSphereMachine: createVSphereMachine(nil, "tkgs-old-imagename", "best-effort-xsmall", "wcpglobalstorageprofile", "vmx-15"),
			vsphereMachine:    createVSphereMachine(nil, "tkgs-new-imagename", "best-effort-xsmall", "wcpglobalstorageprofile", "vmx-15"),
			wantErr:           true,
		},
		{
			name:              "updating ClassName cannot be done",
			oldVSphereMachine: createVSphereMachine(nil, "tkgs-imagename", "old-best-effort-xsmall", "wcpglobalstorageprofile", "vmx-15"),
			vsphereMachine:    createVSphereMachine(nil, "tkgs-imagename", "new-best-effort-xsmall", "wcpglobalstorageprofile", "vmx-15"),
			wantErr:           true,
		},
		{
			name:              "updating StorageClass cannot be done",
			oldVSphereMachine: createVSphereMachine(nil, "tkgs-imagename", "best-effort-xsmall", "old-wcpglobalstorageprofile", "vmx-15"),
			vsphereMachine:    createVSphereMachine(nil, "tkgs-imagename", "best-effort-xsmall", "new-wcpglobalstorageprofile", "vmx-15"),
			wantErr:           true,
		},
		{
			name:              "updating MinHardwareVersion cannot be done",
			oldVSphereMachine: createVSphereMachine(nil, "tkgs-imagename", "best-effort-xsmall", "wcpglobalstorageprofile", "vmx-15"),
			vsphereMachine:    createVSphereMachine(nil, "tkgs-imagename", "best-effort-xsmall", "wcpglobalstorageprofile", "vmx-16"),
			wantErr:           true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

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

func createVSphereMachine(providerID *string, imageName, className, storageClass, minHardwareVersion string) *vmwarev1.VSphereMachine {
	vSphereMachine := &vmwarev1.VSphereMachine{
		Spec: vmwarev1.VSphereMachineSpec{
			ProviderID:         providerID,
			ImageName:          imageName,
			ClassName:          className,
			StorageClass:       storageClass,
			MinHardwareVersion: minHardwareVersion,
		},
	}

	return vSphereMachine
}
