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

package webhooks

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

var (
	testClusterName  = "test-cluster-name"
	targetNamespace  = "test-target-namespace"
	targetSecretName = "test-target-secret-name"
)

func TestProviderServiceAccount_ValidateCreate(t *testing.T) {
	g := NewWithT(t)
	tests := []struct {
		name                   string
		providerServiceAccount *vmwarev1.ProviderServiceAccount
		wantErr                bool
	}{
		{
			name:                   "successful ProviderServiceAccount creation with Ref specified ",
			providerServiceAccount: createProviderServiceAccount(true, false),
			wantErr:                false,
		},

		{
			name:                   "successful ProviderServiceAccount creation with ClusterName specified ",
			providerServiceAccount: createProviderServiceAccount(false, true),
			wantErr:                false,
		},

		{
			name:                   "ProviderServiceAccount creation should specify at least Ref or ClusterName ",
			providerServiceAccount: createProviderServiceAccount(false, false),
			wantErr:                true,
		},
		{
			name:                   "successful ProviderServiceAccount creation both Ref and ClusterName specified ",
			providerServiceAccount: createProviderServiceAccount(true, true),
			wantErr:                false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			webhook := &ProviderServiceAccountWebhook{}
			_, err := webhook.ValidateCreate(context.Background(), tc.providerServiceAccount)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestProviderServiceAccount_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name                      string
		oldProviderServiceAccount *vmwarev1.ProviderServiceAccount
		providerServiceAccount    *vmwarev1.ProviderServiceAccount
		wantErr                   bool
	}{
		{
			name:                      "ClusterName can be added during ProviderServiceAccount update",
			oldProviderServiceAccount: createProviderServiceAccount(true, false),
			providerServiceAccount:    createProviderServiceAccount(true, true),
			wantErr:                   false,
		},
		{
			name:                      "Ref can be removed during ProviderServiceAccount update",
			oldProviderServiceAccount: createProviderServiceAccount(true, true),
			providerServiceAccount:    createProviderServiceAccount(false, true),
			wantErr:                   false,
		},
		{
			name:                      "ProviderServiceAccount update should specify at least Ref or ClusterName ",
			oldProviderServiceAccount: createProviderServiceAccount(true, false),
			providerServiceAccount:    createProviderServiceAccount(false, false),
			wantErr:                   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			webhook := &ProviderServiceAccountWebhook{}
			_, err := webhook.ValidateUpdate(context.Background(), tc.oldProviderServiceAccount, tc.providerServiceAccount)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createProviderServiceAccount(withRef, withClusterName bool) *vmwarev1.ProviderServiceAccount {
	providerServiceAccount := &vmwarev1.ProviderServiceAccount{
		Spec: vmwarev1.ProviderServiceAccountSpec{
			Rules:            []rbacv1.PolicyRule{},
			TargetNamespace:  targetNamespace,
			TargetSecretName: targetSecretName,
		},
	}

	if withRef {
		providerServiceAccount.Spec.Ref = &corev1.ObjectReference{} //nolint:staticcheck
	}

	if withClusterName {
		providerServiceAccount.Spec.ClusterName = &testClusterName
	}

	return providerServiceAccount
}
