/*
Copyright 2022 The Kubernetes Authors.

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
	"fmt"
	"os"
	"reflect"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	helpers "sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vmware"
)

var _ = Describe("ProviderServiceAccount controller integration tests", func() {
	var intCtx *helpers.IntegrationTestContext

	BeforeEach(func() {
		intCtx = helpers.NewIntegrationTestContextWithClusters(ctx, testEnv.Manager.GetClient())
		testSystemSvcAcctCM := "test-system-svc-acct-cm"
		cfgMap := getSystemServiceAccountsConfigMap(intCtx.VSphereCluster.Namespace, testSystemSvcAcctCM)
		Expect(intCtx.Client.Create(ctx, cfgMap)).To(Succeed())
		_ = os.Setenv("SERVICE_ACCOUNTS_CM_NAMESPACE", intCtx.VSphereCluster.Namespace)
		_ = os.Setenv("SERVICE_ACCOUNTS_CM_NAME", testSystemSvcAcctCM)
	})

	AfterEach(func() {
		intCtx.AfterEach()
	})

	Describe("When the ProviderServiceAccount is created", func() {
		var (
			pSvcAccount *vmwarev1.ProviderServiceAccount
			targetNSObj *corev1.Namespace
		)
		BeforeEach(func() {
			pSvcAccount = getTestProviderServiceAccount(intCtx.Namespace, intCtx.VSphereCluster)
			createTestResource(ctx, intCtx.Client, pSvcAccount)
			assertEventuallyExistsInNamespace(ctx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName(), pSvcAccount)
		})
		AfterEach(func() {
			// Deleting the provider service account is not strictly required as the context itself
			// gets teared down but keeping it for clarity.
			deleteTestResource(ctx, intCtx.Client, pSvcAccount)
		})

		Context("When serviceaccount secret is created", func() {
			BeforeEach(func() {
				// Note: Envtest doesn't run controller-manager, hence, the token controller. The token controller is required
				// to create a secret containing the bearer token, cert etc for a service account. We need to
				// simulate the job of the token controller by waiting for the service account creation and then updating it
				// with a prototype secret.
				assertServiceAccountAndUpdateSecret(ctx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName())
			})

			It("should create the role and role binding", func() {
				Eventually(func() error {
					role := &rbacv1.Role{}
					key := client.ObjectKeyFromObject(pSvcAccount)
					return intCtx.Client.Get(ctx, key, role)
				}).Should(Succeed())

				Eventually(func() error {
					roleBinding := &rbacv1.RoleBinding{}
					key := client.ObjectKeyFromObject(pSvcAccount)
					if err := intCtx.Client.Get(ctx, key, roleBinding); err != nil {
						return err
					}
					if roleBinding.RoleRef.Name != pSvcAccount.GetName() || len(roleBinding.Subjects) != 1 {
						return errors.Errorf("roleBinding %s/%s is incorrect", roleBinding.GetNamespace(), roleBinding.GetName())
					}
					return nil
				}).Should(Succeed())
			})

			It("Should reconcile", func() {
				By("Creating the target secret in the target namespace")
				assertTargetSecret(ctx, intCtx.GuestClient, pSvcAccount.Spec.TargetNamespace, testTargetSecret)
			})
		})

		Context("When serviceaccount secret is rotated", func() {
			BeforeEach(func() {
				targetNSObj = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: pSvcAccount.Spec.TargetNamespace,
					},
				}
				Expect(intCtx.GuestClient.Create(ctx, targetNSObj)).To(Succeed())
				createTargetSecretWithInvalidToken(ctx, intCtx.GuestClient, pSvcAccount.Spec.TargetNamespace)
				assertServiceAccountAndUpdateSecret(ctx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName())
			})
			AfterEach(func() {
				deleteTestResource(ctx, intCtx.GuestClient, targetNSObj)
			})
			It("Should reconcile", func() {
				By("Updating the target secret in the target namespace")
				assertTargetSecret(ctx, intCtx.GuestClient, pSvcAccount.Spec.TargetNamespace, testTargetSecret)
			})
		})
	})

	Context("With non-existent Cluster object", func() {
		It("cannot reconcile the ProviderServiceAccount object", func() {
			By("Deleting the CAPI cluster object", func() {
				clusterName, ok := intCtx.VSphereCluster.GetLabels()[clusterv1.ClusterNameLabel]
				Expect(ok).To(BeTrue())
				cluster := &clusterv1.Cluster{}
				key := client.ObjectKey{Namespace: intCtx.Namespace, Name: clusterName}
				Expect(intCtx.Client.Get(ctx, key, cluster)).To(Succeed())
				Expect(intCtx.Client.Delete(ctx, cluster)).To(Succeed())
			})

			By("Creating the ProviderServiceAccount", func() {
				pSvcAccount := getTestProviderServiceAccount(intCtx.Namespace, intCtx.VSphereCluster)
				createTestResource(ctx, intCtx.Client, pSvcAccount)
				assertEventuallyExistsInNamespace(ctx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName(), pSvcAccount)
			})

			By("ProviderServiceAccountsReady Condition is not set", func() {
				vsphereCluster := &vmwarev1.VSphereCluster{}
				key := client.ObjectKey{Namespace: intCtx.Namespace, Name: intCtx.VSphereCluster.GetName()}
				Expect(intCtx.Client.Get(ctx, key, vsphereCluster)).To(Succeed())
				Expect(conditions.Has(vsphereCluster, vmwarev1.ProviderServiceAccountsReadyCondition)).To(BeFalse())
			})
		})
	})

	Context("With non-existent Cluster credentials secret", func() {
		It("cannot reconcile the ProviderServiceAccount object", func() {
			By("Deleting the CAPI kubeconfig secret object", func() {
				clusterName, ok := intCtx.VSphereCluster.GetLabels()[clusterv1.ClusterNameLabel]
				Expect(ok).To(BeTrue())
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: intCtx.Namespace,
						Name:      fmt.Sprintf("%s-kubeconfig", clusterName),
					},
				}
				Expect(intCtx.Client.Delete(ctx, secret)).To(Succeed())
			})

			By("Creating the ProviderServiceAccount", func() {
				pSvcAccount := getTestProviderServiceAccount(intCtx.Namespace, intCtx.VSphereCluster)
				createTestResource(ctx, intCtx.Client, pSvcAccount)
				assertEventuallyExistsInNamespace(ctx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName(), pSvcAccount)
			})

			By("ProviderServiceAccountsReady Condition is not set", func() {
				vsphereCluster := &vmwarev1.VSphereCluster{}
				key := client.ObjectKey{Namespace: intCtx.Namespace, Name: intCtx.VSphereCluster.GetName()}
				Expect(intCtx.Client.Get(ctx, key, vsphereCluster)).To(Succeed())
				Expect(conditions.Has(vsphereCluster, vmwarev1.ProviderServiceAccountsReadyCondition)).To(BeFalse())
			})
		})
	})

	Context("Upgrading from vSphere 7", func() {
		var pSvcAccount *vmwarev1.ProviderServiceAccount
		var role *rbacv1.Role
		var roleBinding *rbacv1.RoleBinding
		BeforeEach(func() {
			pSvcAccount = getTestProviderServiceAccount(intCtx.Namespace, intCtx.VSphereCluster)
			pSvcAccount.Spec.TargetNamespace = "default"
			// Pause the ProviderServiceAccount so we can create dependent but legacy resources
			pSvcAccount.ObjectMeta.Annotations = map[string]string{
				"cluster.x-k8s.io/paused": "true",
			}
			createTestResource(ctx, intCtx.Client, pSvcAccount)
			oldOwnerUID := uuid.New().String()

			role = &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pSvcAccount.GetName(),
					Namespace: pSvcAccount.GetNamespace(),
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "incorrect.api.com/v1beta1",
							Kind:       "ProviderServiceAccount",
							Name:       pSvcAccount.GetName(),
							UID:        types.UID(oldOwnerUID),
							Controller: pointer.Bool(true),
						},
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"get"},
						APIGroups: []string{""},
						Resources: []string{"oldpersistentvolumeclaims"},
					},
				},
			}
			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pSvcAccount.GetName(),
					Namespace: pSvcAccount.GetNamespace(),
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "incorrect.api.com/v1beta1",
							Kind:       "ProviderServiceAccount",
							Name:       pSvcAccount.GetName(),
							Controller: pointer.Bool(true),
							UID:        types.UID(oldOwnerUID),
						},
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Role",
					Name:     pSvcAccount.GetName() + "-incorrect",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "User",
						Name:      pSvcAccount.GetName(),
						Namespace: pSvcAccount.GetNamespace(),
					},
				},
			}

			createTestResource(ctx, intCtx.Client, role)
			createTestResource(ctx, intCtx.Client, roleBinding)
			assertEventuallyExistsInNamespace(ctx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName(), pSvcAccount)
			svcAccountPatcher, err := patch.NewHelper(pSvcAccount, intCtx.Client)
			Expect(err).ToNot(HaveOccurred())
			// Unpause the ProviderServiceAccount so we can reconcile
			pSvcAccount.SetAnnotations(map[string]string{})
			Expect(svcAccountPatcher.Patch(ctx, pSvcAccount)).To(Succeed())
		})
		AfterEach(func() {
			deleteTestResource(ctx, intCtx.Client, pSvcAccount)
			deleteTestResource(ctx, intCtx.Client, role)
			deleteTestResource(ctx, intCtx.Client, roleBinding)
		})

		It("should fully reconciles dependent resources", func() {
			correctOwnership := metav1.OwnerReference{
				APIVersion: pSvcAccount.APIVersion,
				Kind:       pSvcAccount.Kind,
				Name:       pSvcAccount.GetName(),
				UID:        pSvcAccount.UID,
				Controller: pointer.Bool(true),
			}
			By("Taking ownership of the role and reconciling the rules", func() {
				Eventually(func() error {
					role := &rbacv1.Role{}
					key := client.ObjectKeyFromObject(pSvcAccount)
					if err := intCtx.Client.Get(ctx, key, role); err != nil {
						return err
					}
					if err := verifyControllerOwnership(correctOwnership, role); err != nil {
						return err
					}
					correctRules := []rbacv1.PolicyRule{
						{
							Verbs:     []string{"get"},
							APIGroups: []string{""},
							Resources: []string{"persistentvolumeclaims"},
						},
					}
					if !reflect.DeepEqual(role.Rules, correctRules) {
						return errors.Errorf("role %s/%s is incorrect", role.GetNamespace(), role.GetName())
					}
					return nil
				}, "25s").Should(Succeed())
			})
			By("Taking ownership of the rolebinding and reconciling the subjects", func() {
				Eventually(func() error {
					role := &rbacv1.RoleBinding{}
					key := client.ObjectKeyFromObject(pSvcAccount)
					if err := intCtx.Client.Get(ctx, key, role); err != nil {
						return err
					}
					if err := verifyControllerOwnership(correctOwnership, role); err != nil {
						return err
					}
					correctRoleRef := rbacv1.RoleRef{
						Name:     pSvcAccount.Name,
						Kind:     "Role",
						APIGroup: rbacv1.GroupName,
					}
					correctSubjects := []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							APIGroup:  "",
							Name:      pSvcAccount.Name,
							Namespace: pSvcAccount.Namespace,
						},
					}
					if !reflect.DeepEqual(role.RoleRef, correctRoleRef) {
						return errors.Errorf("role reference %v is incorrect, got %v", correctRoleRef, role.RoleRef)
					}
					if !reflect.DeepEqual(role.Subjects, correctSubjects) {
						return errors.Errorf("subjects %v are incorrect, got %v", role.Subjects, role.RoleRef)
					}
					return nil
				}, "25s").Should(Succeed())
			})
		})
	})
})

func verifyControllerOwnership(expected metav1.OwnerReference, obj client.Object) error {
	controller := metav1.GetControllerOf(obj)
	if controller == nil {
		return errors.Errorf("%s/%s %s is not owned by %s/%s %s", obj.GetNamespace(), obj.GetName(), obj.GetObjectKind().GroupVersionKind().String(), expected.APIVersion, expected.Kind, expected.Name)
	}
	if controller.UID != expected.UID || controller.Name != expected.Name || controller.Kind != expected.Kind || controller.APIVersion != expected.APIVersion {
		return errors.Errorf("object %s/%s %s is not a controller of %s %s/%s, got %s/%s %s",
			expected.APIVersion, expected.Kind, expected.Name,
			obj.GetObjectKind().GroupVersionKind().String(), obj.GetNamespace(), obj.GetName(),
			controller.APIVersion, controller.Kind, controller.Name)
	}
	return nil
}
