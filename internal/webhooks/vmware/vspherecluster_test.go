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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
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
			name:            "successful VSphereCluster creation without network",
			vsphereCluster:  createVSphereCluster("test-cluster", vmwarev1.Network{}),
			networkProvider: manager.NSXVPCNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": true},
			wantErr:         false,
		},
		{
			name: "successful VSphereCluster creation with nsxVPC and createSubnetSet is true and MultiNetworks enabled",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(true),
				},
			}),
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
			}),
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
			}),
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
			}),
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
			}),
			networkProvider: manager.VDSNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": true},
			wantErr:         true,
			errType:         &apierrors.StatusError{},
			errMsg:          "nsxVPC can only be set when network provider is NSX-VPC",
		},
		{
			name: "failed VSphereCluster creation with nsxVPC when network provider is NSX",
			vsphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{
				NSXVPC: vmwarev1.NSXVPC{
					CreateSubnetSet: ptr.To(false),
				},
			}),
			networkProvider: manager.NSXNetworkProvider,
			featureGates:    map[string]bool{"MultiNetworks": true},
			wantErr:         true,
			errType:         &apierrors.StatusError{},
			errMsg:          "nsxVPC can only be set when network provider is NSX-VPC",
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
	tests := []struct {
		name              string
		oldVSphereCluster *vmwarev1.VSphereCluster
		newVSphereCluster *vmwarev1.VSphereCluster
		networkProvider   string
		featureGates      map[string]bool
		wantErr           bool
		errType           *apierrors.StatusError
		errMsg            string // expected error message or substring
	}{
		{
			name:              "successful VSphereCluster update without network",
			oldVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}),
			newVSphereCluster: createVSphereCluster("test-cluster", vmwarev1.Network{}),
			networkProvider:   manager.NSXVPCNetworkProvider,
			featureGates:      map[string]bool{"MultiNetworks": true},
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

			_, err := webhook.ValidateUpdate(context.Background(), tc.oldVSphereCluster, tc.newVSphereCluster)
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
	cluster := createVSphereCluster("test-cluster", vmwarev1.Network{})

	warnings, err := webhook.ValidateDelete(context.Background(), cluster)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(warnings).To(BeNil())
}

func TestVSphereCluster_ValidateCreate_InvalidObjectType(t *testing.T) {
	g := NewWithT(t)

	webhook := &VSphereCluster{}
	// Create an invalid object that implements runtime.Object but is not a VSphereCluster
	invalidObj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-configmap",
		},
	}

	_, err := webhook.ValidateCreate(context.Background(), invalidObj)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(BeAssignableToTypeOf(&apierrors.StatusError{}))
	g.Expect(err.Error()).To(ContainSubstring("expected a VSphereCluster but got a"))
}

func TestVSphereCluster_ValidateUpdate_InvalidObjectType(t *testing.T) {
	g := NewWithT(t)

	webhook := &VSphereCluster{}
	// Create an invalid object that implements runtime.Object but is not a VSphereCluster
	invalidObj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-configmap",
		},
	}

	_, err := webhook.ValidateUpdate(context.Background(), nil, invalidObj)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(BeAssignableToTypeOf(&apierrors.StatusError{}))
	g.Expect(err.Error()).To(ContainSubstring("expected a VSphereCluster but got a"))
}

// Helper functions.
func createVSphereCluster(name string, network vmwarev1.Network) *vmwarev1.VSphereCluster {
	return &vmwarev1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: vmwarev1.VSphereClusterSpec{
			Network: network,
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
	}
}
