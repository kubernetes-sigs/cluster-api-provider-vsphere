//go:build e2e
// +build e2e

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

package kubescape

import (
	"context"
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KubescapeSpecInput is the input for KubescapeSpec.
type KubescapeSpecInput struct {
	BootstrapClusterProxy ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	FailThreshold         string
	Container             string
	SkipCleanup           bool
}

// KubescapeSpec implements a test that runs the kubescape security scanner.
// See https://github.com/armosec/kubescape for details about kubescape.
func KubescapeSpec(ctx context.Context, inputGetter func() KubescapeSpecInput) {
	var (
		specName      = "kubescape-scan"
		input         KubescapeSpecInput
		failThreshold int
	)

	input = inputGetter()
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)
	failThreshold, err := strconv.Atoi(input.FailThreshold)
	Expect(err).NotTo(HaveOccurred(), "Invalid argument. input.FailThreshold can't be parsed to int when calling %s spec", specName)
	Expect(failThreshold).To(BeNumerically(">=", 0), "Invalid argument. input.FailThreshold can't be less than 0 when calling %s spec", specName)
	Expect(failThreshold).To(BeNumerically("<=", 100), "Invalid argument. input.FailThreshold can't be more than 100 when calling %s spec", specName)
	Expect(input.Container).NotTo(BeEmpty(), "Invalid argument. input.Container can't be empty when calling %s spec", specName)

	By("creating a Kubernetes client to the workload cluster")
	clusterProxy := BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("running a security scan job")
	const (
		saName                 = "kubescape-discovery"
		roleName               = saName + "-role"
		roleBindingName        = roleName + "binding"
		clusterRoleName        = saName + "-clusterrole"
		clusterRoleBindingName = clusterRoleName + "binding"
	)

	Log("Creating a service account")
	saClient := clientset.CoreV1().ServiceAccounts(corev1.NamespaceDefault)
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: corev1.NamespaceDefault,
			Labels:    map[string]string{"app": "kubescape"},
		},
	}
	_, err = saClient.Create(ctx, serviceAccount, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	Log("Creating a role")
	rolesClient := clientset.RbacV1().Roles(corev1.NamespaceDefault)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: corev1.NamespaceDefault},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{"get", "list", "describe"},
			},
		},
	}
	_, err = rolesClient.Create(ctx, role, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	Log("Creating a role binding")
	rolebindingsClient := clientset.RbacV1().RoleBindings(corev1.NamespaceDefault)
	rolebinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: roleBindingName, Namespace: corev1.NamespaceDefault},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: roleName},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: saName}},
	}
	_, err = rolebindingsClient.Create(ctx, rolebinding, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	Log("Creating a cluster role")
	clusterRolesClient := clientset.RbacV1().ClusterRoles()
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{"get", "list", "describe"},
			},
		},
	}
	_, err = clusterRolesClient.Create(ctx, clusterRole, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	Log("Creating a cluster role binding")
	clusterRolebindingsClient := clientset.RbacV1().ClusterRoleBindings()
	clusterRolebinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterRoleName},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: saName, Namespace: corev1.NamespaceDefault}},
	}
	_, err = clusterRolebindingsClient.Create(ctx, clusterRolebinding, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	Log("Creating a security scan job")
	jobsClient := clientset.BatchV1().Jobs(corev1.NamespaceDefault)
	args := []string{"scan", "framework", "nsa", "--enable-host-scan", "--exclude-namespaces", "kube-system,kube-public"}
	if failThreshold < 100 {
		args = append(args, "--fail-threshold", strconv.Itoa(failThreshold))
	}
	scanJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: specName, Namespace: corev1.NamespaceDefault},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  specName,
							Image: input.Container,
							Args:  args,
						},
					},
					NodeSelector:       map[string]string{corev1.LabelOSStable: "linux"},
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: saName,
				},
			},
		},
	}
	_, err = jobsClient.Create(ctx, scanJob, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	scanJobInput := WaitForJobCompleteInput{
		Getter:    jobsClientAdapter{client: jobsClient},
		Job:       scanJob,
		Clientset: clientset,
	}
	WaitForJobComplete(ctx, scanJobInput, e2eConfig.GetIntervals(specName, "wait-job")...)

	fmt.Fprint(GinkgoWriter, getJobPodLogs(ctx, scanJobInput))

	if !input.SkipCleanup {
		Log("Cleaning up resources")
		if err := jobsClient.Delete(ctx, specName, metav1.DeleteOptions{}); err != nil {
			Logf("Failed to delete job %s: %v", specName, err)
		}
		if err := clusterRolebindingsClient.Delete(ctx, clusterRoleBindingName, metav1.DeleteOptions{}); err != nil {
			Logf("Failed to delete cluster role binding %s: %v", clusterRoleBindingName, err)
		}
		if err := clusterRolesClient.Delete(ctx, clusterRoleName, metav1.DeleteOptions{}); err != nil {
			Logf("Failed to delete cluster role %s: %v", clusterRoleName, err)
		}
		if err := rolebindingsClient.Delete(ctx, roleBindingName, metav1.DeleteOptions{}); err != nil {
			Logf("Failed to delete role binding %s: %v", roleBindingName, err)
		}
		if err := rolesClient.Delete(ctx, roleName, metav1.DeleteOptions{}); err != nil {
			Logf("Failed to delete role %s: %v", roleName, err)
		}
		if err := saClient.Delete(ctx, saName, metav1.DeleteOptions{}); err != nil {
			Logf("Failed to delete service account %s: %v", saName, err)
		}
	}
}
