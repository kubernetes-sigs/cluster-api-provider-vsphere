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
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	featuregatetesting "k8s.io/component-base/featuregate/testing"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
)

func TestVSphereClusterTemplate_ValidateCreate(t *testing.T) {
	tests := []struct {
		name         string
		template     *vmwarev1.VSphereClusterTemplate
		featureGates map[string]bool
		wantErr      bool
		errType      *apierrors.StatusError
		errMsg       string // expected error message or substring
	}{
		{
			name:         "successful creation without a failure domain selector",
			template:     createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{}),
			featureGates: map[string]bool{"NamespaceScopedZones": true},
			wantErr:      false,
		},
		{
			name: "successful creation with valid control plane selector and feature gate enabled",
			template: createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{
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
			template: createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{
				ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"type": "management"},
					},
				},
			}),
			featureGates: map[string]bool{"NamespaceScopedZones": false},
			wantErr:      true,
			errType:      &apierrors.StatusError{},
			errMsg:       "control plane zone selector is not supported on this cluster",
		},
		{
			name: "failed creation with invalid control plane selector syntax",
			template: createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{
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
			template: createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{
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
			setupTemplateFeatureGates(t, tc.featureGates)

			webhook := &VSphereClusterTemplate{}

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
		name         string
		oldTemplate  *vmwarev1.VSphereClusterTemplate
		newTemplate  *vmwarev1.VSphereClusterTemplate
		featureGates map[string]bool
		wantErr      bool
		errType      *apierrors.StatusError
		errMsg       string // expected error message or substring
	}{
		{
			name:         "successful update without a failure domain selector",
			oldTemplate:  createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{}),
			newTemplate:  createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{}),
			featureGates: map[string]bool{"NamespaceScopedZones": true},
			wantErr:      false,
		},
		{
			name:        "failed update with control plane selector when feature gate disabled",
			oldTemplate: createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{}),
			newTemplate: createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{
				ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"type": "management"},
					},
				},
			}),
			featureGates: map[string]bool{"NamespaceScopedZones": false},
			wantErr:      true,
			errType:      &apierrors.StatusError{},
			errMsg:       "control plane zone selector is not supported on this cluster",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			// Set up feature gates for this test case
			setupTemplateFeatureGates(t, tc.featureGates)

			webhook := &VSphereClusterTemplate{}

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
	template := createVSphereClusterTemplate("test-template", vmwarev1.FailureDomainsSpec{})

	warnings, err := webhook.ValidateDelete(context.Background(), template)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(warnings).To(BeNil())
}

// Helper functions.
func createVSphereClusterTemplate(name string, failureDomains vmwarev1.FailureDomainsSpec) *vmwarev1.VSphereClusterTemplate {
	return &vmwarev1.VSphereClusterTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: vmwarev1.VSphereClusterTemplateSpec{
			Template: vmwarev1.VSphereClusterTemplateResource{
				Spec: vmwarev1.VSphereClusterSpec{
					FailureDomains: failureDomains,
				},
			},
		},
	}
}

func setupTemplateFeatureGates(t *testing.T, featureGates map[string]bool) {
	t.Helper()
	// Set up feature gates for the test duration
	for featureName, enabled := range featureGates {
		if featureName == "NamespaceScopedZones" {
			featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.NamespaceScopedZones, enabled)
		}
	}
}
