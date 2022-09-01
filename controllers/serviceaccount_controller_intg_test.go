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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	helpers "sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vmware"
)

var _ = Describe("ProviderServiceAccount controller integration tests", func() {
	var (
		intCtx *helpers.IntegrationTestContext
	)

	BeforeEach(func() {
		intCtx = helpers.NewIntegrationTestContextWithClusters(ctx, testEnv.Manager.GetClient())
		testSystemSvcAcctCM := "test-system-svc-acct-cm"
		cfgMap := getSystemServiceAccountsConfigMap(intCtx.VSphereCluster.Namespace, testSystemSvcAcctCM)
		Expect(intCtx.Client.Create(intCtx, cfgMap)).To(Succeed())
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
			createTestResource(intCtx, intCtx.Client, pSvcAccount)
			assertEventuallyExistsInNamespace(intCtx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName(), pSvcAccount)
		})
		AfterEach(func() {
			// Deleting the provider service account is not strictly required as the context itself
			// gets teared down but keeping it for clarity.
			deleteTestResource(intCtx, intCtx.Client, pSvcAccount)
		})

		Context("When serviceaccount secret is created", func() {
			BeforeEach(func() {
				// Note: Envtest doesn't run controller-manager, hence, the token controller. The token controller is required
				// to create a secret containing the bearer token, cert etc for a service account. We need to
				// simulate the job of the token controller by waiting for the service account creation and then updating it
				// with a prototype secret.
				assertServiceAccountAndUpdateSecret(intCtx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName())
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
				assertTargetSecret(intCtx, intCtx.GuestClient, pSvcAccount.Spec.TargetNamespace, testTargetSecret)
			})
		})

		Context("When serviceaccount secret is rotated", func() {
			BeforeEach(func() {
				targetNSObj = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: pSvcAccount.Spec.TargetNamespace,
					},
				}
				Expect(intCtx.GuestClient.Create(intCtx, targetNSObj)).To(Succeed())
				createTargetSecretWithInvalidToken(intCtx, intCtx.GuestClient, pSvcAccount.Spec.TargetNamespace)
				assertServiceAccountAndUpdateSecret(intCtx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName())
			})
			AfterEach(func() {
				deleteTestResource(intCtx, intCtx.GuestClient, targetNSObj)
			})
			It("Should reconcile", func() {
				By("Updating the target secret in the target namespace")
				assertTargetSecret(intCtx, intCtx.GuestClient, pSvcAccount.Spec.TargetNamespace, testTargetSecret)
			})
		})
	})

	Context("With non-existent Cluster object", func() {
		It("cannot reconcile the ProviderServiceAccount object", func() {
			By("Deleting the CAPI cluster object", func() {
				clusterName, ok := intCtx.VSphereCluster.GetLabels()[clusterv1.ClusterLabelName]
				Expect(ok).To(BeTrue())
				cluster := &clusterv1.Cluster{}
				key := client.ObjectKey{Namespace: intCtx.Namespace, Name: clusterName}
				Expect(intCtx.Client.Get(intCtx, key, cluster)).To(Succeed())
				Expect(intCtx.Client.Delete(intCtx, cluster)).To(Succeed())
			})

			By("Creating the ProviderServiceAccount", func() {
				pSvcAccount := getTestProviderServiceAccount(intCtx.Namespace, intCtx.VSphereCluster)
				createTestResource(intCtx, intCtx.Client, pSvcAccount)
				assertEventuallyExistsInNamespace(intCtx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName(), pSvcAccount)
			})

			By("ProviderServiceAccountsReady Condition is not set", func() {
				vsphereCluster := &vmwarev1.VSphereCluster{}
				key := client.ObjectKey{Namespace: intCtx.Namespace, Name: intCtx.VSphereCluster.GetName()}
				Expect(intCtx.Client.Get(intCtx, key, vsphereCluster)).To(Succeed())
				Expect(conditions.Has(vsphereCluster, vmwarev1.ProviderServiceAccountsReadyCondition)).To(BeFalse())
			})
		})
	})

	Context("With non-existent Cluster credentials secret", func() {
		It("cannot reconcile the ProviderServiceAccount object", func() {
			By("Deleting the CAPI kubeconfig secret object", func() {
				clusterName, ok := intCtx.VSphereCluster.GetLabels()[clusterv1.ClusterLabelName]
				Expect(ok).To(BeTrue())
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: intCtx.Namespace,
						Name:      fmt.Sprintf("%s-kubeconfig", clusterName),
					},
				}
				Expect(intCtx.Client.Delete(intCtx, secret)).To(Succeed())
			})

			By("Creating the ProviderServiceAccount", func() {
				pSvcAccount := getTestProviderServiceAccount(intCtx.Namespace, intCtx.VSphereCluster)
				createTestResource(intCtx, intCtx.Client, pSvcAccount)
				assertEventuallyExistsInNamespace(intCtx, intCtx.Client, intCtx.Namespace, pSvcAccount.GetName(), pSvcAccount)
			})

			By("ProviderServiceAccountsReady Condition is not set", func() {
				vsphereCluster := &vmwarev1.VSphereCluster{}
				key := client.ObjectKey{Namespace: intCtx.Namespace, Name: intCtx.VSphereCluster.GetName()}
				Expect(intCtx.Client.Get(intCtx, key, vsphereCluster)).To(Succeed())
				Expect(conditions.Has(vsphereCluster, vmwarev1.ProviderServiceAccountsReadyCondition)).To(BeFalse())
			})
		})
	})
})
