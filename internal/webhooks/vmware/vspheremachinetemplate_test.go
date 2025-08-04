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
	"k8s.io/utils/ptr"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
	pkgnetwork "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
)

func TestVSphereMachineTemplate_Validate(t *testing.T) {
	tests := []struct {
		name           string
		namingStrategy *vmwarev1.VirtualMachineNamingStrategy
		wantErr        bool
	}{
		{
			name:           "Should succeed if namingStrategy not set",
			namingStrategy: nil,
			wantErr:        false,
		},
		{
			name: "Should succeed if namingStrategy.template not set",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: nil,
			},
			wantErr: false,
		},
		{
			name: "Should succeed if namingStrategy.template is set to the fallback value",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: ptr.To[string]("{{ .machine.name }}"),
			},
			wantErr: false,
		},
		{
			name: "Should succeed if namingStrategy.template is set to the Windows example",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: ptr.To[string]("{{ if le (len .machine.name) 20 }}{{ .machine.name }}{{else}}{{ trimSuffix \"-\" (trunc 14 .machine.name) }}-{{ trunc -5 .machine.name }}{{end}}"),
			},
			wantErr: false,
		},
		{
			name: "Should fail if namingStrategy.template is set to an invalid template",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: ptr.To[string]("{{ invalid"),
			},
			wantErr: true,
		},
		{
			name: "Should fail if namingStrategy.template is set to a valid template that renders an invalid name",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: ptr.To[string]("-{{ .machine.name }}"), // Leading - is not valid for names.
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			vSphereMachineTemplate := &vmwarev1.VSphereMachineTemplate{
				Spec: vmwarev1.VSphereMachineTemplateSpec{
					Template: vmwarev1.VSphereMachineTemplateResource{
						Spec: vmwarev1.VSphereMachineSpec{
							NamingStrategy: tc.namingStrategy,
						},
					},
				},
			}

			webhook := &VSphereMachineTemplate{}
			_, err := webhook.validate(context.Background(), nil, vSphereMachineTemplate)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestVSphereMachineTemplate_ValidateInterfaces(t *testing.T) {
	featuregatetesting.SetFeatureGateDuringTest(t, feature.MutableGates, feature.MultiNetworks, true)
	tests := []struct {
		name            string
		featureGate     *bool
		networkProvider string
		network         vmwarev1.VSphereMachineNetworkSpec
		wantErr         bool
		wantErrMsg      string
	}{
		{
			name:            "interfaces set but feature gate disabled",
			featureGate:     ptrToBool(false),
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Primary: vmwarev1.InterfaceSpec{
						Network: vmwarev1.InterfaceNetworkRefeference{
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
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Primary: vmwarev1.InterfaceSpec{
						Network: vmwarev1.InterfaceNetworkRefeference{
							Kind:       pkgnetwork.NetworkGVKNetOperator.Kind,
							APIVersion: pkgnetwork.NetworkGVKNetOperator.GroupVersion().String(),
							Name:       "primary-wrong",
						},
					},
				},
			},
			wantErr:    true,
			wantErrMsg: "only support",
		},
		{
			name:            "secondary interface with wrong type for NSX-VPC",
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: "eth1",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkRefeference{
								Kind:       pkgnetwork.NetworkGVKNetOperator.Kind,
								APIVersion: pkgnetwork.NetworkGVKNetOperator.GroupVersion().String(),
								Name:       "secondary-wrong",
							},
						},
					}},
				},
			},
			wantErr:    true,
			wantErrMsg: "only support",
		},
		{
			name:            "primary interface set for VDS provider",
			networkProvider: manager.VDSNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Primary: vmwarev1.InterfaceSpec{
						Network: vmwarev1.InterfaceNetworkRefeference{
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
			networkProvider: manager.VDSNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: "eth1",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkRefeference{
								Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnetSet.Kind,
								APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnetSet.GroupVersion().String(),
								Name:       "secondary-wrong",
							},
						},
					}},
				},
			},
			wantErr:    true,
			wantErrMsg: "only support",
		},
		{
			name:            "duplicate interface names",
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: pkgnetwork.PrimaryInterfaceName,
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkRefeference{
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
			networkProvider: manager.NSXVPCNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Primary: vmwarev1.InterfaceSpec{
						Network: vmwarev1.InterfaceNetworkRefeference{
							Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnetSet.Kind,
							APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnetSet.GroupVersion().String(),
							Name:       "primary-subnetset",
						},
					},
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: "eth1",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkRefeference{
								Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnetSet.Kind,
								APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnetSet.GroupVersion().String(),
								Name:       "secondary-subnetset",
							},
						},
					}},
				},
			},
			wantErr: false,
		},
		{
			name:            "valid VDS secondary interface",
			networkProvider: manager.VDSNetworkProvider,
			network: vmwarev1.VSphereMachineNetworkSpec{
				Interfaces: vmwarev1.InterfacesSpec{
					Secondary: []vmwarev1.SecondaryInterfaceSpec{{
						Name: "eth1",
						InterfaceSpec: vmwarev1.InterfaceSpec{
							Network: vmwarev1.InterfaceNetworkRefeference{
								Kind:       pkgnetwork.NetworkGVKNetOperator.Kind,
								APIVersion: pkgnetwork.NetworkGVKNetOperator.GroupVersion().String(),
								Name:       "secondary-netop",
							},
						},
					}},
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			if tc.featureGate != nil {
				featuregatetesting.SetFeatureGateDuringTest(t, feature.MutableGates, feature.MultiNetworks, *tc.featureGate)
			}
			webhook := &VSphereMachineTemplate{NetworkProvider: tc.networkProvider}
			obj := &vmwarev1.VSphereMachineTemplate{
				Spec: vmwarev1.VSphereMachineTemplateSpec{
					Template: vmwarev1.VSphereMachineTemplateResource{
						Spec: vmwarev1.VSphereMachineSpec{
							Network: tc.network,
						},
					},
				},
			}
			_, err := webhook.validate(context.Background(), nil, obj)
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
