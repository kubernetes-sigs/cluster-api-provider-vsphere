/*
Copyright 2019 The Kubernetes Authors.

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

package cloudprovider

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NOTE: the contents of this file are derived from https://github.com/kubernetes/cloud-provider-vsphere/tree/master/manifests/controller-manager

const (
	DefaultCPIControllerImage = "gcr.io/cloud-provider-vsphere/cpi/release/manager:v1.18.1"
)

// CloudControllerManagerServiceAccount returns the ServiceAccount used for the cloud-controller-manager.
func CloudControllerManagerServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloud-controller-manager",
			Namespace: "kube-system",
		},
	}
}

// CloudControllerManagerService returns a Service for the cloud-controller-manager.
func CloudControllerManagerService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloud-controller-manager",
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": "cloud-controller-manager",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(43001),
				},
			},
			Selector: map[string]string{
				"component": "cloud-controller-manager",
			},
		},
	}
}

// CloudControllerManagerConfigMap returns a ConfigMap containing data for the cloud config file.
func CloudControllerManagerConfigMap(cloudConfig string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vsphere-cloud-config",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"vsphere.conf": cloudConfig,
		},
	}
}

// CloudControllerManagerDaemonSet returns the DaemonSet which runs the cloud-controller-manager.
func CloudControllerManagerDaemonSet(image string, args []string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vsphere-cloud-controller-manager",
			Namespace: "kube-system",
			Labels: map[string]string{
				"k8s-app": "vsphere-cloud-controller-manager",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "vsphere-cloud-controller-manager",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app": "vsphere-cloud-controller-manager",
					},
				},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/master": "",
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser: int64ptr(0),
					},
					Tolerations: []corev1.Toleration{
						{
							Key:    "node.cloudprovider.kubernetes.io/uninitialized",
							Value:  "true",
							Effect: corev1.TaintEffectNoSchedule,
						},
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: corev1.TaintEffectNoSchedule,
						},
						{
							Key:    "node.kubernetes.io/not-ready",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
					ServiceAccountName: "cloud-controller-manager",
					Containers: []corev1.Container{
						{
							Name:  "vsphere-cloud-controller-manager",
							Image: image,
							Args:  args,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vsphere-config-volume",
									MountPath: "/etc/cloud",
									ReadOnly:  true,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("200m"),
								},
							},
						},
					},
					HostNetwork: true,
					Volumes: []corev1.Volume{
						{
							Name: "vsphere-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "vsphere-cloud-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// CloudControllerManagerClusterRole returns the ClusterRole systemLcloud-controller-manager
// used by the cloud-controller-manager.
func CloudControllerManagerClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "system:cloud-controller-manager",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes/status"},
				Verbs:     []string{"patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"serviceaccounts"},
				Verbs:     []string{"create", "get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"create", "get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "watch", "list", "delete", "update", "create"},
			},
		},
	}
}

// CloudControllerManagerRoleBinding binds the extension-apiserver-authentication-reader
// to the cloud-controller-manager.
func CloudControllerManagerRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "servicecatalog.k8s.io:apiserver-authentication-reader",
			Namespace: "kube-system",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "extension-apiserver-authentication-reader",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  "",
				Kind:      "ServiceAccount",
				Name:      "cloud-controller-manager",
				Namespace: "kube-system",
			},
			{
				APIGroup: "",
				Kind:     "User",
				Name:     "cloud-controller-manager",
			},
		},
	}
}

// CloudControllerManagerClusterRoleBinding binds the system:cloud-controller-manager
// cluster role to the cloud-controller-manager.
func CloudControllerManagerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "system:cloud-controller-manager",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:cloud-controller-manager",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "cloud-controller-manager",
				Namespace: "kube-system",
			},
			{
				Kind: "User",
				Name: "cloud-controller-manager",
			},
		},
	}
}

func int64ptr(i int) *int64 {
	ptr := int64(i)
	return &ptr
}
