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
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/govmomi/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

var _ = Describe("VSphereClusterIdentity Reconciler", func() {
	controllerNamespace := testEnv.Manager.GetControllerManagerContext().Namespace
	if controllerNamespace != manager.DefaultPodNamespace {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: controllerNamespace,
			},
		}
		if err := testEnv.Client.Create(ctx, ns); !apierrors.IsAlreadyExists(err) {
			Expect(err).ToNot(HaveOccurred())
		}
	}

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
				return clusterutilv1.IsOwnedByObject(s, i, infrav1.GroupVersion.WithKind("VSphereClusterIdentity").GroupKind())
			}, timeout).Should(BeTrue())
		})

		It("should set the ownerRef on a secret and set Ready condition, if owned by an external identity", func() {
			// create secret
			credentialSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    controllerNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "bitnami.com/v1alpha1",
							Kind:       "SealedSecret",
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
				return clusterutilv1.IsOwnedByObject(s, i, infrav1.GroupVersion.WithKind("VSphereClusterIdentity").GroupKind())
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
							Kind:       "VSphereCluster",
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

				if !ptr.Deref(i.Status.Ready, false) && conditions.GetReason(i, infrav1.VSphereClusterIdentityAvailableCondition) == infrav1.VSphereClusterIdentitySecretAlreadyInUseReason {
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

				if !ptr.Deref(i.Status.Ready, false) && conditions.GetReason(i, infrav1.VSphereClusterIdentityAvailableCondition) == infrav1.VSphereClusterIdentitySecretNotAvailableReason {
					return true
				}
				return false
			}, timeout).Should(BeTrue())
		})
	})
})

func TestClusterIdentityReconciler_ReconcileDelete_SecretOwnership(t *testing.T) {
	for _, tt := range []struct {
		name      string
		owner     *metav1.OwnerReference
		wantError bool
	}{
		{
			name:      "secret has no owner",
			wantError: true,
		},
		{
			name: "secret is owned by another VSphereClusterIdentity",
			owner: &metav1.OwnerReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "VSphereClusterIdentity",
				Name:       "another-identity",
				UID:        "another-identity-uid",
			},
			wantError: true,
		},
		{
			name: "secret is owned by a VSphereCluster",
			owner: &metav1.OwnerReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "VSphereCluster",
				Name:       "identity",
				UID:        "identity-uid",
			},
			wantError: true,
		},
		{
			name: "secret is owned by the VSphereClusterIdentity",
			owner: &metav1.OwnerReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "VSphereClusterIdentity",
				Name:       "identity",
				UID:        "identity-uid",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "credentials",
					Namespace:  fake.ControllerManagerNamespace,
					Finalizers: []string{infrav1.SecretIdentitySetFinalizer},
				},
			}
			if tt.owner != nil {
				secret.OwnerReferences = []metav1.OwnerReference{*tt.owner}
			}
			controllerManagerContext := fake.NewControllerManagerContext(secret)
			identity := &infrav1.VSphereClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "identity",
					UID:        "identity-uid",
					Finalizers: []string{infrav1.VSphereClusterIdentityFinalizer},
				},
				Spec: infrav1.VSphereClusterIdentitySpec{
					SecretName: secret.Name,
				},
			}
			r := clusterIdentityReconciler{
				ControllerManagerCtx: controllerManagerContext,
				Client:               controllerManagerContext.Client,
			}
			secretKey := client.ObjectKeyFromObject(secret)
			g.Expect(r.Client.Get(ctx, secretKey, secret)).To(Succeed())
			originalSecret := secret.DeepCopy()

			err := r.reconcileDelete(ctx, identity)
			if tt.wantError {
				g.Expect(err).To(MatchError(fmt.Sprintf("failed to delete Secret %s/%s: VSphereClusterIdentity %s does not own the secret", secret.Namespace, secret.Name, identity.Name)))
				g.Expect(r.Client.Get(ctx, secretKey, secret)).To(Succeed())
				g.Expect(secret).To(Equal(originalSecret))
				g.Expect(ctrlutil.ContainsFinalizer(identity, infrav1.VSphereClusterIdentityFinalizer)).To(BeTrue())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(apierrors.IsNotFound(r.Client.Get(ctx, secretKey, secret))).To(BeTrue())
				g.Expect(ctrlutil.ContainsFinalizer(identity, infrav1.VSphereClusterIdentityFinalizer)).To(BeFalse())
			}
		})
	}
}
