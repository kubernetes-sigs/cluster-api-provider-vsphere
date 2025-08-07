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
	featuregatetesting "k8s.io/component-base/featuregate/testing"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
	pkgnetwork "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
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

			webhook := &VSphereMachine{}
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

func TestVSphereMachine_ValidateCreate_MultiNetwork(t *testing.T) {
	tests := []struct {
		name            string
		featureGate     bool
		networkProvider string
		network         vmwarev1.VSphereMachineNetworkSpec
		wantErr         bool
		wantErrMsg      string
	}{
		{
			name:            "interfaces set but feature gate disabled",
			featureGate:     false,
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Primary: vmwarev1.InterfaceSpec{
						Network: vmwarev1.InterfaceNetworkReference{
							Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnetSet.Kind,
							APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnetSet.GroupVersion().String(),
							Name:       "primary-subnetset",
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "interfaces can only be set when feature gate MultiNetworks is enabled",
		},
		{
			name:            "primary interface with wrong type for NSX-VPC",
			featureGate:     true,
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Primary: vmwarev1.InterfaceSpec{
						Network: vmwarev1.InterfaceNetworkReference{
							Kind:       "WrongKind",
							APIVersion: "wrong/v1",
							Name:       "primary-wrong",
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "only supports crd.nsx.vmware.com/v1alpha1, Kind=SubnetSet",
		},
		{
			name:            "secondary interface with wrong type for NSX-VPC",
			featureGate:     true,
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: "eth1",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkReference{
								Kind:       "WrongKind",
								APIVersion: "wrong/v1",
								Name:       "secondary-wrong",
							},
						},
					}},
				},
			},
			wantErr:    true,
			wantErrMsg: "only supports crd.nsx.vmware.com/v1alpha1, Kind=SubnetSet or crd.nsx.vmware.com/v1alpha1, Kind=Subnet",
		},
		{
			name:            "primary interface set for VDS provider",
			featureGate:     true,
			networkProvider: manager.VDSNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Primary: vmwarev1.InterfaceSpec{
						Network: vmwarev1.InterfaceNetworkReference{
							Kind:       pkgnetwork.NetworkGVKNetOperator.Kind,
							APIVersion: pkgnetwork.NetworkGVKNetOperator.GroupVersion().String(),
							Name:       "primary-netop",
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "primary interface can not be set when network provider is vsphere-network",
		},
		{
			name:            "secondary interface with wrong type for VDS provider",
			featureGate:     true,
			networkProvider: manager.VDSNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: "eth1",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkReference{
								Kind:       "WrongKind",
								APIVersion: "wrong/v1",
								Name:       "secondary-wrong",
							},
						},
					}},
				},
			},
			wantErr:    true,
			wantErrMsg: "only supports netoperator.vmware.com/v1alpha1, Kind=Network",
		},
		{
			name:            "duplicate interface names",
			featureGate:     true,
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: pkgnetwork.PrimaryInterfaceName,
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkReference{
								Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnet.Kind,
								APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnet.GroupVersion().String(),
								Name:       "secondary-dup",
							},
						},
					}},
				},
			},
			wantErr:    true,
			wantErrMsg: "interface name is already in use",
		},
		{
			name:            "valid NSX-VPC interfaces",
			featureGate:     true,
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Primary: vmwarev1.InterfaceSpec{
						Network: vmwarev1.InterfaceNetworkReference{
							Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnetSet.Kind,
							APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnetSet.GroupVersion().String(),
							Name:       "primary-subnetset",
						},
					},
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: "eth1",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkReference{
								Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnet.Kind,
								APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnet.GroupVersion().String(),
								Name:       "secondary-subnet1",
							},
						},
					}, {
						Name: "eth2",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkReference{
								Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnet.Kind,
								APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnet.GroupVersion().String(),
								Name:       "secondary-subnet2",
							},
						},
					}},
				},
			},
			wantErr: false,
		},
		{
			name:            "valid VDS secondary interface",
			featureGate:     true,
			networkProvider: manager.VDSNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: "eth1",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkReference{
								Kind:       pkgnetwork.NetworkGVKNetOperator.Kind,
								APIVersion: pkgnetwork.NetworkGVKNetOperator.GroupVersion().String(),
								Name:       "secondary-netop1",
							},
						},
					}, {
						Name: "eth2",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkReference{
								Kind:       pkgnetwork.NetworkGVKNetOperator.Kind,
								APIVersion: pkgnetwork.NetworkGVKNetOperator.GroupVersion().String(),
								Name:       "secondary-netop2",
							},
						},
					}},
				},
			},
			wantErr: false,
		},
		{
			name:            "two secondary interfaces with duplicate names",
			featureGate:     true,
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Secondary: []vmwarev1.SecondaryInterfaceSpec{
						{
							Name: "eth1",
							InterfaceSpec: vmwarev1.InterfaceSpec{
								Network: vmwarev1.InterfaceNetworkReference{
									Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnet.Kind,
									APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnet.GroupVersion().String(),
									Name:       "secondary-dup1",
								},
							},
						},
						{
							Name: "eth1",
							InterfaceSpec: vmwarev1.InterfaceSpec{
								Network: vmwarev1.InterfaceNetworkReference{
									Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnet.Kind,
									APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnet.GroupVersion().String(),
									Name:       "secondary-dup2",
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "interface name is already in use",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.MultiNetworks, tc.featureGate)
			webhook := &VSphereMachine{NetworkProvider: tc.networkProvider}
			obj := &vmwarev1.VSphereMachine{Spec: vmwarev1.VSphereMachineSpec{Network: tc.network}}
			_, err := webhook.ValidateCreate(context.Background(), obj)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tc.wantErrMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.wantErrMsg))
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestVSphereMachine_ValidateUpdate_MultiNetwork(t *testing.T) {
	featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.MultiNetworks, true)
	g := NewWithT(t)

	// Old VSphereMachine with one interface
	oldVSphereMachine := &vmwarev1.VSphereMachine{
		Spec: vmwarev1.VSphereMachineSpec{
			Network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Primary: vmwarev1.InterfaceSpec{
						Network: vmwarev1.InterfaceNetworkReference{
							Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnetSet.Kind,
							APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnetSet.GroupVersion().String(),
							Name:       "primary-subnetset",
						},
					},
					Secondary: []vmwarev1.SecondaryInterfaceSpec{},
				},
			},
		},
	}

	// New VSphereMachine with a changed interface (add a secondary interface)
	newVSphereMachine := oldVSphereMachine.DeepCopy()
	newVSphereMachine.Spec.Network.Interfaces.Secondary = append(newVSphereMachine.Spec.Network.Interfaces.Secondary, vmwarev1.SecondaryInterfaceSpec{
		Name: "eth1",
		InterfaceSpec: vmwarev1.InterfaceSpec{
			Network: vmwarev1.InterfaceNetworkReference{
				Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnet.Kind,
				APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnet.GroupVersion().String(),
				Name:       "secondary-subnet",
			},
		},
	})

	webhook := &VSphereMachine{NetworkProvider: manager.NSXVPCNetworkProvider}
	_, err := webhook.ValidateUpdate(context.Background(), oldVSphereMachine, newVSphereMachine)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cannot be modified"))
}
