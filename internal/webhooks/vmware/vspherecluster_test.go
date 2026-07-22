/*
Copyright 2025 The Kubernetes Authors.

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
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

func TestVSphereCluster_ValidateCreate(t *testing.T) {
	tests := []struct {
		name            string
		vsphereCluster  *vmwarev1.VSphereCluster
		networkProvider string
		featureGates    map[string]bool
		wantErr         bool
		errType         *apierrors.StatusError
		errMsg          string // expected error message or substring
	}{
		{
			name:            "successful VSphereCluster creation without network and failure domain selector",
			vsphereCluster:  createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"NamespaceScopedZones": true, "MultiNetworks": true},
			wantErr:         false,
		},
		{
			name: "successful VSphereCluster creation with nsxVPC and createSubnetSet is true and MultiNetworks enabled",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(true),
				},
			}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": true},
			wantErr:         false,
		},
		{
			name: "failed VSphereCluster creation with nsxVPC and createSubnetSet is true but MultiNetworks disabled",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
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
			name: "failed VSphereCluster creation with nsxVPC and createSubnetSet is false but MultiNetworks disabled",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
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
			name: "successful VSphereCluster creation with nsxVPC and createSubnetSet is nil and MultiNetworks disabled",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{},
			}, vmwarev1.FailureDomainsSpec{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": false},
			wantErr:         false,
		},
		{
			name: "failed VSphereCluster creation with nsxVPC when network provider is vsphere-network",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
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
			name: "failed VSphereCluster creation with nsxVPC when network provider is NSX",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
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
			name: "gate on: successful creation with nsxVPC when spec.network.provider is VPC",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(true),
				},
				Provider: manager.NSXVPCNetworkProvider,
			}, vmwarev1.FailureDomainsSpec{}),
			featureGates: map[string]bool{"MultiNetworks": true, "ClusterNetworkProvider": true},
			wantErr:      false,
		},
		{
			name: "gate on: failed creation with nsxVPC when spec.network.provider is VSphereDistributed",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(true),
				},
				Provider: manager.VDSNetworkProvider,
			}, vmwarev1.FailureDomainsSpec{}),
			featureGates: map[string]bool{"MultiNetworks": true, "ClusterNetworkProvider": true},
			wantErr:      true,
			errType:      &apierrors.StatusError{},
			errMsg:       fmt.Sprintf("nsxVPC can only be set when network provider is %s", manager.NSXVPCNetworkProvider),
		},
		{
			name: "gate off: failed creation with spec.network.provider set",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
				Provider: manager.NSXVPCNetworkProvider,
			}, vmwarev1.FailureDomainsSpec{}),
			featureGates: map[string]bool{"ClusterNetworkProvider": false},
			wantErr:      true,
			errType:      &apierrors.StatusError{},
			errMsg:       "provider can only be set when ClusterNetworkProvider feature gate is enabled",
		},
		{
			name:           "gate on: failed creation with empty spec.network.provider",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			featureGates:   map[string]bool{"ClusterNetworkProvider": true},
			wantErr:        true,
			errType:        &apierrors.StatusError{},
			errMsg:         "spec.network.provider must be set",
		},
		{
			name: "ExternallyManaged rejected when ExternallyManagedProvider gate is off",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
				Provider: manager.ExternallyManagedNetworkProvider,
			}, vmwarev1.FailureDomainsSpec{}),
			featureGates: map[string]bool{"ClusterNetworkProvider": true, "ExternallyManagedProvider": false},
			wantErr:      true,
			errType:      &apierrors.StatusError{},
			errMsg:       "provider ExternallyManaged can only be set when feature gate ExternallyManagedProvider is enabled",
		},
		{
			name: "ExternallyManaged accepted when ExternallyManagedProvider and ClusterNetworkProvider gates are on",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
				Provider: manager.ExternallyManagedNetworkProvider,
			}, vmwarev1.FailureDomainsSpec{}),
			featureGates: map[string]bool{"ClusterNetworkProvider": true, "ExternallyManagedProvider": true},
			wantErr:      false,
		},
		{
			name: "successful VSphereCluster creation with valid control plane selector and feature gate enabled",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
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
			name: "failed VSphereCluster creation with control plane selector but feature gate disabled",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
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
			name: "failed VSphereCluster creation with invalid control plane selector syntax",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
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
			name: "failed VSphereCluster creation with empty control plane selector",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
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

			// Set up feature gates
			setupFeatureGates(t, tc.featureGates)

			webhook := &VSphereCluster{
				NetworkProvider: tc.networkProvider,
			}

			_, err := webhook.ValidateCreate(context.Background(), tc.vsphereCluster)
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

func TestVSphereCluster_ValidateUpdate(t *testing.T) {
	emptyProviderCluster := createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{})
	emptyProviderClusterDryRun := emptyProviderCluster.DeepCopy()
	emptyProviderClusterDryRun.SetAnnotations(map[string]string{clusterv1.TopologyDryRunAnnotation: ""})

	tests := []struct {
		name              string
		oldVSphereCluster *vmwarev1.VSphereCluster
		newVSphereCluster *vmwarev1.VSphereCluster
		networkProvider   string
		featureGates      map[string]bool
		req               *admission.Request
		wantErr           bool
		errType           *apierrors.StatusError
		errMsg            string // expected error message or substring
	}{
		{
			name:              "successful VSphereCluster update without network",
			oldVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			newVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			networkProvider:   manager.NSXVPCNetworkProvider,
			featureGates:      map[string]bool{"MultiNetworks": true},
			wantErr:           false,
		},
		{
			name:              "failed update with control plane selector when feature gate disabled",
			oldVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			newVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{
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
		{
			name:              "gate off: failed update with spec.network.provider set",
			oldVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			newVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{Provider: manager.NSXVPCNetworkProvider}, vmwarev1.FailureDomainsSpec{}),
			featureGates:      map[string]bool{"ClusterNetworkProvider": false},
			wantErr:           true,
			errType:           &apierrors.StatusError{},
			errMsg:            "provider can only be set when ClusterNetworkProvider feature gate is enabled",
		},
		{
			name:              "gate on: failed update with empty spec.network.provider",
			oldVSphereCluster: emptyProviderCluster,
			newVSphereCluster: emptyProviderCluster,
			featureGates:      map[string]bool{"ClusterNetworkProvider": true},
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}},
			wantErr:           true,
			errType:           &apierrors.StatusError{},
			errMsg:            "spec.network.provider must be set",
		},
		{
			name:              "gate on: allow empty provider on topology SSA dry-run",
			oldVSphereCluster: emptyProviderCluster,
			newVSphereCluster: emptyProviderClusterDryRun,
			featureGates:      map[string]bool{"ClusterNetworkProvider": true},
			req:               &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(true)}},
			wantErr:           false,
		},
		{
			name:              "gate on: successful update setting spec.network.provider",
			oldVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{}),
			newVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{Provider: manager.NSXVPCNetworkProvider}, vmwarev1.FailureDomainsSpec{}),
			featureGates:      map[string]bool{"ClusterNetworkProvider": true},
			wantErr:           false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			// Set up feature gates
			setupFeatureGates(t, tc.featureGates)

			webhook := &VSphereCluster{
				NetworkProvider: tc.networkProvider,
			}

			ctx := context.Background()
			if tc.req != nil {
				ctx = admission.NewContextWithRequest(ctx, *tc.req)
			}

			_, err := webhook.ValidateUpdate(ctx, tc.oldVSphereCluster, tc.newVSphereCluster)
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

func TestVSphereCluster_ValidateDelete(t *testing.T) {
	g := NewWithT(t)

	webhook := &VSphereCluster{}
	cluster := createVSphereCluster("test-cluster", vmwarev1.Network{}, vmwarev1.FailureDomainsSpec{})

	warnings, err := webhook.ValidateDelete(context.Background(), cluster)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(warnings).To(BeNil())
}

// Helper functions.
func createVSphereCluster(name string, network vmwarev1.Network, failureDomains vmwarev1.FailureDomainsSpec) *vmwarev1.VSphereCluster {
	return &vmwarev1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: vmwarev1.VSphereClusterSpec{
			Network:        network,
			FailureDomains: failureDomains,
		},
	}
}

func setupFeatureGates(t *testing.T, featureGates map[string]bool) {
	t.Helper()
	// Set up feature gates for the test duration
	for featureName, enabled := range featureGates {
		if featureName == "MultiNetworks" {
			featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.MultiNetworks, enabled)
		}
		if featureName == "NamespaceScopedZones" {
			featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.NamespaceScopedZones, enabled)
		}
		if featureName == "ClusterNetworkProvider" {
			featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.ClusterNetworkProvider, enabled)
		}
		if featureName == "ExternallyManagedProvider" {
			featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.ExternallyManagedProvider, enabled)
		}
	}
}
