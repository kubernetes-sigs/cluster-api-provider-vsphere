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
	"strings"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/builder"
)

// This object is used for unit tests setup only
// Integration tests will be run using the existing envTest setup.
var ServiceAccountProviderTestsuite = builder.NewTestSuiteForController(NewServiceAccountReconciler)

const (
	testNS                     = "test-pvcsi-system"
	testProviderSvcAccountName = "test-pvcsi"

	testTargetNS             = "test-pvcsi-system"
	testTargetSecret         = "test-pvcsi-secret" // nolint:gosec
	testSvcAccountName       = testProviderSvcAccountName
	testSvcAccountSecretName = testProviderSvcAccountName + "-token-abcdef"
	testRoleName             = testProviderSvcAccountName
	testRoleBindingName      = testProviderSvcAccountName
	testSystemSvcAcctNs      = "test-system-svc-acct-namespace"
	testSystemSvcAcctCM      = "test-system-svc-acct-cm"

	testSecretToken = "ZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNklp" // nolint:gosec
)

var truePointer = true

func createTestResource(ctx goctx.Context, ctrlClient client.Client, obj client.Object) {
	Expect(ctrlClient.Create(ctx, obj)).To(Succeed())
}

func deleteTestResource(ctx goctx.Context, ctrlClient client.Client, obj client.Object) {
	Expect(ctrlClient.Delete(ctx, obj)).To(Succeed())
}

func createTargetSecretWithInvalidToken(ctx goctx.Context, guestClient client.Client, namespace string) {
	secret := getTestTargetSecretWithInvalidToken(namespace)
	Expect(guestClient.Create(ctx, secret)).To(Succeed())
}

func assertEventuallyExistsInNamespace(ctx goctx.Context, c client.Client, namespace, name string, obj client.Object) {
	EventuallyWithOffset(2, func() error {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		return c.Get(ctx, key, obj)
	}).Should(Succeed())
}

func assertNoEntities(ctx goctx.Context, ctrlClient client.Client, namespace string) {
	Consistently(func() int {
		var serviceAccountList corev1.ServiceAccountList
		err := ctrlClient.List(ctx, &serviceAccountList, client.InNamespace(namespace))
		Expect(err).ShouldNot(HaveOccurred())
		return len(serviceAccountList.Items)
	}, time.Second*3).Should(Equal(0))

	Consistently(func() int {
		var roleList rbacv1.RoleList
		err := ctrlClient.List(ctx, &roleList, client.InNamespace(namespace))
		Expect(err).ShouldNot(HaveOccurred())
		return len(roleList.Items)
	}, time.Second*3).Should(Equal(0))

	Consistently(func() int {
		var roleBindingList rbacv1.RoleBindingList
		err := ctrlClient.List(ctx, &roleBindingList, client.InNamespace(namespace))
		Expect(err).ShouldNot(HaveOccurred())
		return len(roleBindingList.Items)
	}, time.Second*3).Should(Equal(0))
}

func assertServiceAccountAndUpdateSecret(ctx goctx.Context, ctrlClient client.Client, namespace, name string) {
	svcAccount := &corev1.ServiceAccount{}
	assertEventuallyExistsInNamespace(ctx, ctrlClient, namespace, name, svcAccount)
	// Update the service account with a prototype secret
	secret := getTestSvcAccountSecret(namespace, testSvcAccountSecretName)
	Expect(ctrlClient.Create(ctx, secret)).To(Succeed())
	svcAccount.Secrets = []corev1.ObjectReference{
		{
			Name: testSvcAccountSecretName,
		},
	}
	Expect(ctrlClient.Update(ctx, svcAccount)).To(Succeed())
}

func assertTargetSecret(ctx goctx.Context, guestClient client.Client, namespace, name string) { // nolint
	secret := &corev1.Secret{}
	assertEventuallyExistsInNamespace(ctx, guestClient, namespace, name, secret)
	EventuallyWithOffset(2, func() []byte {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		Expect(guestClient.Get(ctx, key, secret)).Should(Succeed())
		return secret.Data["token"]
	}).Should(Equal([]byte(testSecretToken)))
}

func assertTargetNamespace(ctx *builder.UnitTestContextForController, guestClient client.Client, namespaceName string, isExist bool) {
	namespace := &corev1.Namespace{}
	err := guestClient.Get(ctx, client.ObjectKey{Name: namespaceName}, namespace)
	if isExist {
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}
}

func assertRoleWithGetPVC(ctx *builder.UnitTestContextForController, ctrlClient client.Client, namespace, name string) {
	var roleList rbacv1.RoleList
	opts := &client.ListOptions{
		Namespace: namespace,
	}
	err := ctrlClient.List(ctx, &roleList, opts)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(len(roleList.Items)).To(Equal(1))
	Expect(roleList.Items[0].Name).To(Equal(name))
	Expect(roleList.Items[0].Rules).To(Equal([]rbacv1.PolicyRule{
		{
			Verbs:     []string{"get"},
			APIGroups: []string{""},
			Resources: []string{"persistentvolumeclaims"},
		},
	}))
}

func assertRoleBinding(_ *builder.UnitTestContextForController, ctrlClient client.Client, namespace, name string) {
	var roleBindingList rbacv1.RoleBindingList
	opts := &client.ListOptions{
		Namespace: namespace,
	}
	err := ctrlClient.List(goctx.TODO(), &roleBindingList, opts)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(len(roleBindingList.Items)).To(Equal(1))
	Expect(roleBindingList.Items[0].Name).To(Equal(name))
	Expect(roleBindingList.Items[0].RoleRef).To(Equal(rbacv1.RoleRef{
		Name:     testRoleName,
		Kind:     "Role",
		APIGroup: rbacv1.GroupName,
	}))
}

// nolint
func assertProviderServiceAccountsCondition(vCluster *vmwarev1.VSphereCluster, status corev1.ConditionStatus,
	message string, reason string, severity clusterv1.ConditionSeverity) {
	c := conditions.Get(vCluster, vmwarev1.ProviderServiceAccountsReadyCondition)
	Expect(c).NotTo(BeNil())
	Expect(c.Status).To(Equal(status))
	Expect(c.Reason).To(Equal(reason))
	Expect(c.Severity).To(Equal(severity))
	if message == "" {
		Expect(c.Message).To(BeEmpty())
	} else {
		Expect(strings.Contains(c.Message, message)).To(BeTrue(), "expect condition message contains: %s, actual: %s", message, c.Message)
	}
}

func getTestTargetSecretWithInvalidToken(namespace string) *corev1.Secret {
	secret := getTestTargetSecretWithValidToken(namespace)
	secret.Data["token"] = []byte("invalid-token")
	return secret
}

func getTestTargetSecretWithValidToken(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testTargetSecret,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"token": []byte(testSecretToken),
		},
	}
}

func getTestProviderServiceAccount(namespace, name string, vSphereCluster *vmwarev1.VSphereCluster) *vmwarev1.ProviderServiceAccount {
	pSvcAccount := &vmwarev1.ProviderServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: vmwarev1.ProviderServiceAccountSpec{
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get"},
					APIGroups: []string{""},
					Resources: []string{"persistentvolumeclaims"},
				},
			},
			TargetNamespace:  testTargetNS,
			TargetSecretName: testTargetSecret,
		},
	}

	if vSphereCluster != nil {
		pSvcAccount.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: vmwarev1.GroupVersion.String(),
				Kind:       "VSphereCluster",
				Name:       vSphereCluster.Name,
				UID:        vSphereCluster.UID,
				Controller: &truePointer,
			},
		}
		pSvcAccount.Spec.Ref = &corev1.ObjectReference{
			Name: vSphereCluster.Name,
		}
	}
	return pSvcAccount
}

func getSystemServiceAccountsConfigMap(namespace, name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: map[string]string{
			"system-account-1": "true",
			"system-account-2": "true",
		},
	}
}

func getTestSvcAccountSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"token": []byte(testSecretToken),
		},
	}
}

func getTestRoleWithGetPod(namespace, name string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
	}
}

func getTestRoleBindingWithInvalidRoleRef(namespace, name string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: name,
		},
		RoleRef: rbacv1.RoleRef{
			Name:     "invalid-role-ref",
			Kind:     "Role",
			APIGroup: rbacv1.GroupName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				APIGroup:  "",
				Name:      testProviderSvcAccountName,
				Namespace: namespace},
		},
	}
}
