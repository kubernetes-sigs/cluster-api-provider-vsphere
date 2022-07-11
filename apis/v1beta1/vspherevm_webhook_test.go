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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	biosUUID      = "vsphere://42305f0b-dad7-1d3d-5727-0eafffffbbbfc"
	windowsVMName = "cluster-md-containerd-b7fccbf59-2qj6q"
	linuxVMName   = "linux-control-plane-qkkbv"
)

//nolint
func TestVSphereVM_Default(t *testing.T) {
	g := NewWithT(t)

	WindowsVM := createVSphereVM(windowsVMName, "foo.com", "", "", []string{"192.168.0.1/32", "192.168.0.3/32"}, nil, Windows)
	LinuxVM := createVSphereVM(linuxVMName, "foo.com", "", "", []string{"192.168.0.1/32", "192.168.0.3/32"}, nil, Linux)
	NoOSVM := createVSphereVM(linuxVMName, "foo.com", "", "", []string{"192.168.0.1/32", "192.168.0.3/32"}, nil, "")

	WindowsVM.Default()
	LinuxVM.Default()
	NoOSVM.Default()

	g.Expect(LinuxVM.Spec.OS).To(Equal(Linux))
	g.Expect(WindowsVM.Spec.OS).To(Equal(Windows))
	g.Expect(NoOSVM.Spec.OS).To(Equal(Linux))

	// WindowsVM gets name updated to be 15 characters. Linux remains unchanged
	g.Expect(WindowsVM.Name).To(Equal("cluster-m-2qj6q"))
	g.Expect(LinuxVM.Name).To(Equal("linux-control-plane-qkkbv"))
}

//nolint
func TestVSphereVM_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name      string
		vSphereVM *VSphereVM
		wantErr   bool
	}{
		{
			name:      "preferredAPIServerCIDR set on creation ",
			vSphereVM: createVSphereVM("vsphere-vm-1", "foo.com", "", "192.168.0.1/32", []string{}, nil, Linux),
			wantErr:   true,
		},
		{
			name:      "IPs are not in CIDR format",
			vSphereVM: createVSphereVM("vsphere-vm-1", "foo.com", "", "", []string{"192.168.0.1/32", "192.168.0.3"}, nil, Linux),
			wantErr:   true,
		},
		{
			name:      "successful VSphereVM creation",
			vSphereVM: createVSphereVM("vsphere-vm-1", "foo.com", "", "", []string{"192.168.0.1/32", "192.168.0.3/32"}, nil, Linux),
			wantErr:   false,
		},
		{
			name:      "name too long for Windows VM",
			vSphereVM: createVSphereVM(windowsVMName, "foo.com", "", "", []string{"192.168.0.1/32", "192.168.0.3/32"}, nil, Windows),
			wantErr:   true,
		},
		{
			name:      "name too long for Linux VM",
			vSphereVM: createVSphereVM(linuxVMName, "foo.com", "", "", []string{"192.168.0.1/32", "192.168.0.3/32"}, nil, Linux),
			wantErr:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vSphereVM.ValidateCreate()
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

//nolint
func TestVSphereVM_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name         string
		oldVSphereVM *VSphereVM
		vSphereVM    *VSphereVM
		wantErr      bool
	}{
		{
			name:         "ProviderID can be updated",
			oldVSphereVM: createVSphereVM("vsphere-vm-1", "foo.com", "", "", []string{"192.168.0.1/32"}, nil, Linux),
			vSphereVM:    createVSphereVM("vsphere-vm-1", "foo.com", biosUUID, "", []string{"192.168.0.1/32"}, nil, Linux),
			wantErr:      false,
		},
		{
			name:         "updating ips can be done",
			oldVSphereVM: createVSphereVM("vsphere-vm-1", "foo.com", "", "", []string{"192.168.0.1/32"}, nil, Linux),
			vSphereVM:    createVSphereVM("vsphere-vm-1", "foo.com", biosUUID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}, nil, Linux),
			wantErr:      false,
		},
		{
			name:         "updating bootstrapRef can be done",
			oldVSphereVM: createVSphereVM("vsphere-vm-1", "foo.com", "", "", []string{"192.168.0.1/32"}, nil, Linux),
			vSphereVM:    createVSphereVM("vsphere-vm-1", "foo.com", biosUUID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}, &corev1.ObjectReference{}, Linux),
			wantErr:      false,
		},
		{
			name:         "updating server cannot be done",
			oldVSphereVM: createVSphereVM("vsphere-vm-1", "foo.com", "", "", []string{"192.168.0.1/32"}, nil, Linux),
			vSphereVM:    createVSphereVM("vsphere-vm-1", "bar.com", biosUUID, "", []string{"192.168.0.1/32", "192.168.0.10/32"}, nil, Linux),
			wantErr:      true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vSphereVM.ValidateUpdate(tc.oldVSphereVM)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createVSphereVM(name, server, biosUUID, preferredAPIServerCIDR string, ips []string, bootstrapRef *corev1.ObjectReference, os OS) *VSphereVM {
	VSphereVM := &VSphereVM{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: VSphereVMSpec{
			BiosUUID:     biosUUID,
			BootstrapRef: bootstrapRef,
			VirtualMachineCloneSpec: VirtualMachineCloneSpec{
				Server: server,
				Network: NetworkSpec{
					PreferredAPIServerCIDR: preferredAPIServerCIDR,
					Devices:                []NetworkDeviceSpec{},
				},
			},
		},
	}

	if os != "" {
		VSphereVM.Spec.OS = os
	}
	for _, ip := range ips {
		VSphereVM.Spec.Network.Devices = append(VSphereVM.Spec.Network.Devices, NetworkDeviceSpec{
			IPAddrs: []string{ip},
		})
	}
	return VSphereVM
}
