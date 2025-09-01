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
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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
							Kind:       pkgnetwork.NetworkGVKNetOperator.Kind,
							APIVersion: pkgnetwork.NetworkGVKNetOperator.GroupVersion().String(),
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
								Kind:       pkgnetwork.NetworkGVKNetOperator.Kind,
								APIVersion: pkgnetwork.NetworkGVKNetOperator.GroupVersion().String(),
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
								Kind:       pkgnetwork.NetworkGVKNSXTVPCSubnetSet.Kind,
								APIVersion: pkgnetwork.NetworkGVKNSXTVPCSubnetSet.GroupVersion().String(),
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
			featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.MultiNetworks, tc.featureGate)
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

func TestVSphereMachineTemplate_ValidateUpdate_Immutability(t *testing.T) {
	oldTemplate := vmwarev1.VSphereMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: "default",
		},
		Spec: vmwarev1.VSphereMachineTemplateSpec{
			Template: vmwarev1.VSphereMachineTemplateResource{
				Spec: vmwarev1.VSphereMachineSpec{
					ImageName:    "ubuntu-20.04",
					ClassName:    "best-effort-small",
					StorageClass: "fast-storage",
				},
			},
		},
	}

	newTemplate := oldTemplate.DeepCopy()
	newTemplate.Spec.Template.Spec.ImageName = "ubuntu-22.04"

	newTemplateSkipImmutabilityAnnotationSet := newTemplate.DeepCopy()
	newTemplateSkipImmutabilityAnnotationSet.SetAnnotations(map[string]string{clusterv1.TopologyDryRunAnnotation: ""})

	tests := []struct {
		name        string
		newTemplate *vmwarev1.VSphereMachineTemplate
		oldTemplate *vmwarev1.VSphereMachineTemplate
		req         *admission.Request
		wantError   bool
		wantErrMsg  string
	}{
		{
			name:        "return no error if no modification",
			newTemplate: &oldTemplate,
			oldTemplate: &oldTemplate,
			req:         &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantError:   false,
		},
		{
			name:        "don't allow modification of spec.template.spec",
			newTemplate: newTemplate,
			oldTemplate: &oldTemplate,
			req:         &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantError:   true,
			wantErrMsg:  "VSphereMachineTemplate spec.template.spec field is immutable",
		},
		{
			name:        "don't allow modification even with skip immutability annotation when not dry run",
			newTemplate: newTemplateSkipImmutabilityAnnotationSet,
			oldTemplate: &oldTemplate,
			req:         &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantError:   true,
			wantErrMsg:  "VSphereMachineTemplate spec.template.spec field is immutable",
		},
		{
			name:        "don't allow modification when dry run but no skip immutability annotation",
			newTemplate: newTemplate,
			oldTemplate: &oldTemplate,
			req:         &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(true)}},
			wantError:   true,
			wantErrMsg:  "VSphereMachineTemplate spec.template.spec field is immutable",
		},
		{
			name:        "skip immutability check when dry run and skip immutability annotation set",
			newTemplate: newTemplateSkipImmutabilityAnnotationSet,
			oldTemplate: &oldTemplate,
			req:         &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(true)}},
			wantError:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &VSphereMachineTemplate{}
			ctx := context.Background()
			if tc.req != nil {
				ctx = admission.NewContextWithRequest(ctx, *tc.req)
			}
			warnings, err := webhook.ValidateUpdate(ctx, tc.oldTemplate, tc.newTemplate)
			if tc.wantError {
				g.Expect(err).To(HaveOccurred())
				if tc.wantErrMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.wantErrMsg))
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(warnings).To(BeEmpty())
		})
	}
}
