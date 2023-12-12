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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	helpers "sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vmware"
)

var _ = Describe("ServiceAccountReconciler reconcileNormal", unitTestsReconcileNormal)

func unitTestsReconcileNormal() {
	var (
		controllerCtx  *helpers.UnitTestContextForController
		vsphereCluster *vmwarev1.VSphereCluster
		initObjects    []client.Object
		namespace      string
		reconciler     ServiceAccountReconciler
	)

	JustBeforeEach(func() {
		controllerCtx = helpers.NewUnitTestContextForController(ctx, namespace, vsphereCluster, false, initObjects, nil)
		// Note: The service account provider requires a reference to the vSphereCluster hence the need to create
		// a fake vSphereCluster in the test and pass it to during context setup.
		reconciler = ServiceAccountReconciler{
			Client: controllerCtx.ControllerManagerContext.Client,
		}
		_, err := reconciler.reconcileNormal(ctx, controllerCtx.GuestClusterContext)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		controllerCtx = nil
	})

	Context("When no provider service account is available", func() {
		namespace = capiutil.RandomString(6)
		It("Should reconcile", func() {
			By("Not creating any entities")
			assertNoEntities(ctx, controllerCtx.ControllerManagerContext.Client, namespace)
			assertProviderServiceAccountsCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
		})
	})

	Describe("When the ProviderServiceAccount is created", func() {
		BeforeEach(func() {
			namespace = capiutil.RandomString(6)
			obj := fake.NewVSphereCluster(namespace)
			vsphereCluster = &obj
			_ = os.Setenv("SERVICE_ACCOUNTS_CM_NAMESPACE", testSystemSvcAcctNs)
			_ = os.Setenv("SERVICE_ACCOUNTS_CM_NAME", testSystemSvcAcctCM)
			initObjects = []client.Object{
				getSystemServiceAccountsConfigMap(testSystemSvcAcctNs, testSystemSvcAcctCM),
				getTestProviderServiceAccount(namespace, vsphereCluster, false),
			}
		})
		It("should create a service account and a secret", func() {
			_, err := reconciler.reconcileNormal(ctx, controllerCtx.GuestClusterContext)
			Expect(err).NotTo(HaveOccurred())

			svcAccount := &corev1.ServiceAccount{}
			assertEventuallyExistsInNamespace(ctx, controllerCtx.ControllerManagerContext.Client, namespace, vsphereCluster.GetName(), svcAccount)

			secret := &corev1.Secret{}
			assertEventuallyExistsInNamespace(ctx, controllerCtx.ControllerManagerContext.Client, namespace, fmt.Sprintf("%s-secret", vsphereCluster.GetName()), secret)
		})
		Context("When serviceaccount secret is created", func() {
			It("Should reconcile", func() {
				assertTargetNamespace(ctx, controllerCtx.GuestClient, testTargetNS, false)
				updateServiceAccountSecretAndReconcileNormal(ctx, controllerCtx, reconciler, vsphereCluster)
				assertTargetNamespace(ctx, controllerCtx.GuestClient, testTargetNS, true)
				By("Creating the target secret in the target namespace")
				assertTargetSecret(ctx, controllerCtx.GuestClient, testTargetNS, testTargetSecret)
				assertProviderServiceAccountsCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
			})
		})
		Context("When serviceaccount secret is modified", func() {
			It("Should reconcile", func() {
				// This is to simulate an outdated token that will be replaced when the serviceaccount secret is created.
				createTargetSecretWithInvalidToken(ctx, controllerCtx.GuestClient, testTargetNS)
				updateServiceAccountSecretAndReconcileNormal(ctx, controllerCtx, reconciler, vsphereCluster)
				By("Updating the target secret in the target namespace")
				assertTargetSecret(ctx, controllerCtx.GuestClient, testTargetNS, testTargetSecret)
				assertProviderServiceAccountsCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
			})
		})
		Context("When invalid role exists", func() {
			BeforeEach(func() {
				initObjects = append(initObjects, getTestRoleWithGetPod(namespace, vsphereCluster.GetName()))
			})
			It("Should update role", func() {
				assertRoleWithGetPVC(ctx, controllerCtx.ControllerManagerContext.Client, namespace, vsphereCluster.GetName())
				assertProviderServiceAccountsCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
			})
		})
		Context("When invalid rolebinding exists", func() {
			BeforeEach(func() {
				initObjects = append(initObjects, getTestRoleBindingWithInvalidRoleRef(namespace, vsphereCluster.GetName()))
			})
			It("Should update rolebinding", func() {
				assertRoleBinding(ctx, controllerCtx.ControllerManagerContext.Client, namespace, vsphereCluster.GetName())
				assertProviderServiceAccountsCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
			})
		})
	})
}

// Updates the service account secret similar to how a token controller would act upon a service account
// and then re-invokes reconcileNormal.
func updateServiceAccountSecretAndReconcileNormal(ctx context.Context, controllerCtx *helpers.UnitTestContextForController, reconciler ServiceAccountReconciler, object client.Object) {
	assertServiceAccountAndUpdateSecret(ctx, controllerCtx.ControllerManagerContext.Client, object.GetNamespace(), object.GetName())
	_, err := reconciler.reconcileNormal(ctx, controllerCtx.GuestClusterContext)
	Expect(err).NotTo(HaveOccurred())
}
