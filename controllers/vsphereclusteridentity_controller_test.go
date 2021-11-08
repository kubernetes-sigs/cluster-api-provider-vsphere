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

package controllers

import (
	goctx "context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

var _ = Describe("VSphereClusterIdentity Reconciler", func() {
	ctx := goctx.Background()
	controllerNamespace := testEnv.Manager.GetContext().Namespace

	Context("Reconcile Normal", func() {
		It("should set the ownerRef on a secret and set Ready condition", func() {
			// create secret
			credentialSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    controllerNamespace,
				},
			}
			Expect(testEnv.Create(ctx, credentialSecret)).To(Succeed())

			// create identity
			identity := &infrav1.VSphereClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "identity-",
				},
				Spec: infrav1.VSphereClusterIdentitySpec{
					SecretName: credentialSecret.Name,
				},
			}
			Expect(testEnv.Create(ctx, identity)).To(Succeed())

			// wait for identity to set owner ref
			skey := client.ObjectKey{
				Namespace: credentialSecret.Namespace,
				Name:      credentialSecret.Name,
			}

			Eventually(func() bool {
				i := &infrav1.VSphereClusterIdentity{}
				if err := testEnv.Get(ctx, client.ObjectKey{Name: identity.Name}, i); err != nil {
					return false
				}

				s := &corev1.Secret{}
				if err := testEnv.Get(ctx, skey, s); err != nil {
					return false
				}
				return clusterutilv1.IsOwnedByObject(s, i)
			}, timeout).Should(BeTrue())
		})

		It("should error if secret has another owner reference", func() {
			// create secret
			credentialSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    controllerNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1.GroupVersion.String(),
							Kind:       "some-kind",
							Name:       "some-name",
							UID:        "some-uid",
						},
					},
				},
			}
			Expect(testEnv.Create(ctx, credentialSecret)).To(Succeed())

			// create identity
			identity := &infrav1.VSphereClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "identity-",
				},
				Spec: infrav1.VSphereClusterIdentitySpec{
					SecretName: credentialSecret.Name,
				},
			}
			Expect(testEnv.Create(ctx, identity)).To(Succeed())

			Eventually(func() bool {
				i := &infrav1.VSphereClusterIdentity{}
				if err := testEnv.Get(ctx, client.ObjectKey{Name: identity.Name}, i); err != nil {
					return false
				}

				if i.Status.Ready == false && conditions.GetReason(i, infrav1.CredentialsAvailableCondidtion) == infrav1.SecretAlreadyInUseReason {
					return true
				}
				return false
			}, timeout).Should(BeTrue())
		})

		It("should error if secret is not found", func() {
			identity := &infrav1.VSphereClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "identity-",
				},
				Spec: infrav1.VSphereClusterIdentitySpec{
					SecretName: "non-existent-secret",
				},
			}
			Expect(testEnv.Create(ctx, identity)).To(Succeed())

			Eventually(func() bool {
				i := &infrav1.VSphereClusterIdentity{}
				if err := testEnv.Get(ctx, client.ObjectKey{Name: identity.Name}, i); err != nil {
					return false
				}

				if i.Status.Ready == false && conditions.GetReason(i, infrav1.CredentialsAvailableCondidtion) == infrav1.SecretNotAvailableReason {
					return true
				}
				return false
			}, timeout).Should(BeTrue())
		})
	})
})
