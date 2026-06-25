/*
Copyright 2026 The Kubernetes Authors.

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
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

func TestVSphereClusterTemplate_ValidateCreate(t *testing.T) {
	tests := []struct {
		name            string
		template        *vmwarev1.VSphereClusterTemplate
		networkProvider string
		featureGates    map[string]bool
		wantErr         bool
		errType         *apierrors.StatusError
		errMsg          string // expected error message or substring
	}{
		{
			name:            "successful creation without network and failure domain selector",
			template:        createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"NamespaceScopedZones": true, "MultiNetworks": true},
			wantErr:         false,
		},
		{
			name: "successful creation with nsxVPC and createSubnetSet is true and MultiNetworks enabled",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(true),
				},
			}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": true},
			wantErr:         false,
		},
		{
			name: "failed creation with nsxVPC and createSubnetSet is true but MultiNetworks disabled",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(true),
				},
			}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": false},
			wantErr:         true,
			errType:         &apierrors.StatusError{},
			errMsg:          "createSubnetSet can only be set when MultiNetworks feature gate is enabled",
		},
		{
			name: "failed creation with nsxVPC and createSubnetSet is false but MultiNetworks disabled",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(false),
				},
			}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": false},
			wantErr:         true,
			errType:         &apierrors.StatusError{},
			errMsg:          "createSubnetSet can only be set when MultiNetworks feature gate is enabled",
		},
		{
			name: "successful creation with nsxVPC and createSubnetSet is nil and MultiNetworks disabled",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{},
			}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": false},
			wantErr:         false,
		},
		{
			name: "failed creation with nsxVPC when network provider is vsphere-network",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(true),
				},
			}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.VDSNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": true},
			wantErr:         true,
			errType:         &apierrors.StatusError{},
			errMsg:          fmt.Sprintf("nsxVPC can only be set when network provider is %s", manager.NSXVPCNetworkProvider),
		},
		{
			name: "gate on: nsxVPC check is skipped for cluster templates",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(true),
				},
			}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.VDSNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": true, "ClusterNetworkProvider": true},
			wantErr:         false,
		},
		{
			name: "failed creation with nsxVPC when network provider is NSX",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(false),
				},
			}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": true},
			wantErr:         true,
			errType:         &apierrors.StatusError{},
			errMsg:          fmt.Sprintf("nsxVPC can only be set when network provider is %s", manager.NSXVPCNetworkProvider),
		},
		{
			name: "successful creation with valid control plane selector and feature gate enabled",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
				ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"type": "management"},
					},
				},
			}),
			featureGates: map[string]bool{"NamespaceScopedZones": true},
			wantErr:      false,
		},
		{
			name: "failed creation with control plane selector but feature gate disabled",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
				ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"type": "management"},
					},
				},
			}),
			featureGates: map[string]bool{"NamespaceScopedZones": false},
			wantErr:      true,
			errType:      &apierrors.StatusError{},
			errMsg:       "control plane zone selector can only be set when feature gate NamespaceScopedZones is enabled",
		},
		{
			name: "failed creation with invalid control plane selector syntax",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
				ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
					Selector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{Key: "k", Operator: "InvalidOp", Values: []string{"v"}},
						},
					},
				},
			}),
			featureGates: map[string]bool{"NamespaceScopedZones": true},
			wantErr:      true,
			errType:      &apierrors.StatusError{},
		},
		{
			name: "failed creation with empty control plane selector",
			template: createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
				ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
					Selector: &metav1.LabelSelector{}, // Empty selector
				},
			}),
			featureGates: map[string]bool{"NamespaceScopedZones": true},
			wantErr:      true,
			errType:      &apierrors.StatusError{},
			errMsg:       "selector must not be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			// Set up feature gates for this test case
			setupFeatureGates(t, tc.featureGates)

			webhook := &VSphereClusterTemplate{
				NetworkProvider: tc.networkProvider,
			}

			_, err := webhook.ValidateCreate(context.Background(), tc.template)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tc.errType != nil {
					g.Expect(err).To(BeAssignableToTypeOf(tc.errType))
				}
				if tc.errMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errMsg))
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestVSphereClusterTemplate_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name            string
		oldTemplate     *vmwarev1.VSphereClusterTemplate
		newTemplate     *vmwarev1.VSphereClusterTemplate
		networkProvider string
		featureGates    map[string]bool
		wantErr         bool
		errType         *apierrors.StatusError
		errMsg          string // expected error message or substring
	}{
		{
			name:            "successful update without a failure domain selector",
			oldTemplate:     createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			newTemplate:     createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"NamespaceScopedZones": true},
			wantErr:         false,
		},
		{
			name:        "failed update with control plane selector when feature gate disabled",
			oldTemplate: createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			newTemplate: createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
				ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"type": "management"},
					},
				},
			}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"NamespaceScopedZones": false},
			wantErr:         true,
			errType:         &apierrors.StatusError{},
			errMsg:          "control plane zone selector can only be set when feature gate NamespaceScopedZones is enabled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			// Set up feature gates for this test case
			setupFeatureGates(t, tc.featureGates)

			webhook := &VSphereClusterTemplate{
				NetworkProvider: tc.networkProvider,
			}

			_, err := webhook.ValidateUpdate(context.Background(), tc.oldTemplate, tc.newTemplate)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tc.errType != nil {
					g.Expect(err).To(BeAssignableToTypeOf(tc.errType))
				}
				if tc.errMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errMsg))
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestVSphereClusterTemplate_ValidateDelete(t *testing.T) {
	g := NewWithT(t)

	webhook := &VSphereClusterTemplate{}
	template := createVSphereClusterTemplate("test-template", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{})

	warnings, err := webhook.ValidateDelete(context.Background(), template)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(warnings).To(BeNil())
}

// Helper functions.
func createVSphereClusterTemplate(name string, network vmwarev1.Network, failureDomains vmwarev1.FailureDomainsSpec) *vmwarev1.VSphereClusterTemplate {
	return &vmwarev1.VSphereClusterTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: vmwarev1.VSphereClusterTemplateSpec{
			Template: vmwarev1.VSphereClusterTemplateResource{
				Spec: vmwarev1.VSphereClusterSpec{
					Network:        network,
					FailureDomains: failureDomains,
				},
			},
		},
	}
}
