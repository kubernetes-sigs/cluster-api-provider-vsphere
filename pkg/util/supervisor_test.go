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

package util

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
)

type controllerReferenceTestCase struct {
	name                   string
	controlled             client.Object
	newOwner               client.Object
	expectedNumberOfOwners int
	expectErr              bool
}

func TestSetControllerReferenceWithOverride(t *testing.T) {
	g := NewGomegaWithT(t)

	cases := []controllerReferenceTestCase{
		{
			name: "no existing owners",
			controlled: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "no-owner-secret",
				},
			},
			newOwner: &vmwarev1.ProviderServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "vmware.infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "ProviderServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "owner",
				},
			},
			expectedNumberOfOwners: 1,
			expectErr:              false,
		},
		{
			name: "1 existing non controller owner",
			controlled: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "no-owner-secret",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "ProviderServiceAccount",
							Name: "non-controller-owner",
						},
					},
				},
			},
			newOwner: &vmwarev1.ProviderServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "vmware.infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "ProviderServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "owner",
				},
			},
			expectedNumberOfOwners: 2,
			expectErr:              false,
		},
		{
			name: "1 existing controller owner",
			controlled: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "no-owner-secret",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "ProviderServiceAccount",
							Name:       "non-controller-owner",
							Controller: pointer.Bool(true),
						},
					},
				},
			},
			newOwner: &vmwarev1.ProviderServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "vmware.infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "ProviderServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "owner",
				},
			},
			expectedNumberOfOwners: 1,
			expectErr:              false,
		},
		{
			name: "cluster-scoped owner of namespaced owner",
			controlled: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-owner-secret",
				},
			},
			newOwner: &vmwarev1.ProviderServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "owner",
				},
			},
			expectedNumberOfOwners: 0,
			expectErr:              true,
		},
		{
			name: "no change of owner",
			controlled: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "no-owner-secret",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "ProviderServiceAccount",
							Name:       "owner",
							Controller: pointer.Bool(true),
						},
					},
				},
			},
			newOwner: &vmwarev1.ProviderServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "vmware.infrastructure.cluster.x-k8s.io/v1beta1",
					Kind:       "ProviderServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "owner",
				},
			},
			expectedNumberOfOwners: 1,
			expectErr:              false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			controllerManagerContext := fake.NewControllerManagerContext(tc.controlled)
			actualErr := SetControllerReferenceWithOverride(tc.newOwner, tc.controlled, controllerManagerContext.Scheme)
			if tc.expectErr {
				g.Expect(actualErr).To(HaveOccurred())
			} else {
				g.Expect(actualErr).NotTo(HaveOccurred())
				controller := metav1.GetControllerOf(tc.controlled)
				newOwnerRef := &metav1.OwnerReference{
					APIVersion: tc.newOwner.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:       tc.newOwner.GetObjectKind().GroupVersionKind().Kind,
					Name:       tc.newOwner.GetName(),
				}
				g.Expect(referSameObject(*controller, *newOwnerRef)).To(BeTrue(), "Expect controller to be: %v, got: %v", newOwnerRef, controller)
			}
			g.Expect(tc.controlled.GetOwnerReferences()).To(HaveLen(tc.expectedNumberOfOwners))
		})
	}
}
