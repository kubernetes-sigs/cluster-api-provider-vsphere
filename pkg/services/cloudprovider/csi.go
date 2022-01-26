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
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/crs/types"
)

// NOTE: the contents of this file are derived from https://github.com/kubernetes-sigs/vsphere-csi-driver/tree/master/manifests/1.14

const (
	DefaultCSIControllerImage     = "gcr.io/cloud-provider-vsphere/csi/release/driver:v2.1.0"
	DefaultCSINodeDriverImage     = "gcr.io/cloud-provider-vsphere/csi/release/driver:v2.1.0"
	DefaultCSIAttacherImage       = "quay.io/k8scsi/csi-attacher:v3.0.0"
	DefaultCSIProvisionerImage    = "quay.io/k8scsi/csi-provisioner:v2.0.0"
	DefaultCSIMetadataSyncerImage = "gcr.io/cloud-provider-vsphere/csi/release/syncer:v2.1.0"
	DefaultCSILivenessProbeImage  = "quay.io/k8scsi/livenessprobe:v2.1.0"
	DefaultCSIRegistrarImage      = "quay.io/k8scsi/csi-node-driver-registrar:v2.0.1"
	CSINamespace                  = metav1.NamespaceSystem
	CSIControllerName             = "vsphere-csi-controller"
	CSIFeatureStateConfigMapName  = "internal-feature-states.csi.vsphere.vmware.com"
)

func CSIControllerServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CSIControllerName,
			Namespace: CSINamespace,
		},
	}
}

func CSIControllerClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vsphere-csi-controller-role",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"csidrivers"},
				Verbs:     []string{"create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "pods", "secrets", "configmaps"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get", "list", "watch", "update", "create", "delete", "patch"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"volumeattachments"},
				Verbs:     []string{"get", "list", "watch", "update", "patch"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"volumeattachments/status"},
				Verbs:     []string{"patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses", "csinodes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"list", "watch", "create", "update", "patch"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "watch", "list", "delete", "update", "create"},
			},
			{
				APIGroups: []string{"snapshot.storage.k8s.io"},
				Resources: []string{"volumesnapshots"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{"snapshot.storage.k8s.io"},
				Resources: []string{"volumesnapshotcontents"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
}

func CSIControllerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vsphere-csi-controller-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      CSIControllerName,
				Namespace: CSINamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "vsphere-csi-controller-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func CSIDriver() *storagev1beta1.CSIDriver {
	return &storagev1beta1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: "csi.vsphere.vmware.com",
		},
		Spec: storagev1beta1.CSIDriverSpec{
			AttachRequired: boolPtr(true),
			PodInfoOnMount: boolPtr(false),
		},
	}
}

func VSphereCSINodeDaemonSet(storageConfig *types.CPIStorageConfig) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vsphere-csi-node",
			Namespace: CSINamespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "vsphere-csi-node",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":  "vsphere-csi-node",
						"role": "vsphere-csi",
					},
				},
				Spec: corev1.PodSpec{
					DNSPolicy: corev1.DNSDefault,
					Containers: []corev1.Container{
						NodeDriverRegistrarContainer(storageConfig.RegistrarImage),
						VSphereCSINodeContainer(storageConfig.NodeDriverImage),
						LivenessProbeForNodeContainer(storageConfig.LivenessProbeImage),
					},
					Tolerations: []corev1.Toleration{
						{
							Effect:   corev1.TaintEffectNoSchedule,
							Operator: corev1.TolerationOpExists,
						},
						{
							Effect:   corev1.TaintEffectNoExecute,
							Operator: corev1.TolerationOpExists,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vsphere-config-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "csi-vsphere-config",
								},
							},
						},
						{
							Name: "registration-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins_registry",
									Type: newHostPathType(string(corev1.HostPathDirectory)),
								},
							},
						},
						{
							Name: "plugin-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins/csi.vsphere.vmware.com/",
									Type: newHostPathType(string(corev1.HostPathDirectoryOrCreate)),
								},
							},
						},
						{
							Name: "pods-mount-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet",
									Type: newHostPathType(string(corev1.HostPathDirectory)),
								},
							},
						},
						{
							Name: "device-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/dev",
								},
							},
						},
					},
				},
			},
		},
	}
}

func NodeDriverRegistrarContainer(image string) corev1.Container {
	return corev1.Container{
		Name:  "node-driver-registrar",
		Image: image,
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-c",
						"rm -rf /registration/csi.vsphere.vmware.com-reg.sock /csi/csi.sock",
					},
				},
			},
		},
		Args: []string{
			"--v=5",
			"--csi-address=$(ADDRESS)",
			"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: "/csi/csi.sock",
			},
			{
				Name:  "DRIVER_REG_SOCK_PATH",
				Value: "/var/lib/kubelet/plugins/csi.vsphere.vmware.com/csi.sock",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: boolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "plugin-dir",
				MountPath: "/csi",
			},
			{
				Name:      "registration-dir",
				MountPath: "/registration",
			},
		},
	}
}

func VSphereCSINodeContainer(image string) corev1.Container {
	return corev1.Container{
		Name:  "vsphere-csi-node",
		Image: image,
		Env: []corev1.EnvVar{
			{
				Name:  "CSI_ENDPOINT",
				Value: "unix:///csi/csi.sock",
			},
			{
				Name:  "X_CSI_MODE",
				Value: "node",
			},
			{
				Name:  "X_CSI_SPEC_REQ_VALIDATION",
				Value: "false",
			},
			{
				Name:  "VSPHERE_CSI_CONFIG",
				Value: "/etc/cloud/csi-vsphere.conf",
			},
			{
				Name:  "LOGGER_LEVEL",
				Value: "PRODUCTION",
			},
			{
				Name:  "X_CSI_LOG_LEVEL",
				Value: "INFO",
			},
			{
				Name: "NODE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "healthz",
				ContainerPort: 9808,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.Parse("healthz"),
				},
			},
			InitialDelaySeconds: 10,
			TimeoutSeconds:      3,
			PeriodSeconds:       5,
			FailureThreshold:    3,
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: boolPtr(true),
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{corev1.Capability("SYS_ADMIN")},
			},
			AllowPrivilegeEscalation: boolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "vsphere-config-volume",
				MountPath: "/etc/cloud",
			},
			{
				Name:      "plugin-dir",
				MountPath: "/csi",
			},
			{
				Name:             "pods-mount-dir",
				MountPath:        "/var/lib/kubelet",
				MountPropagation: newMountPropagation(string(corev1.MountPropagationBidirectional)),
			},
			{
				Name:      "device-dir",
				MountPath: "/dev",
			},
		},
	}
}

func LivenessProbeForNodeContainer(image string) corev1.Container {
	return corev1.Container{
		Name:  "liveness-probe",
		Image: image,
		Args:  []string{"--csi-address=/csi/csi.sock"},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "plugin-dir",
				MountPath: "/csi",
			},
		},
	}
}

func CSIControllerDeployment(storageConfig *types.CPIStorageConfig) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CSIControllerName,
			Namespace: CSINamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: boolInt32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": CSIControllerName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":  CSIControllerName,
						"role": "vsphere-csi",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: CSIControllerName,
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/master": "",
					},
					Tolerations: []corev1.Toleration{

						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					DNSPolicy: corev1.DNSDefault,
					Containers: []corev1.Container{
						CSIAttacherContainer(storageConfig.AttacherImage),
						VSphereCSIControllerContainer(storageConfig.ControllerImage),
						LivenessProbeForCSIControllerContainer(storageConfig.LivenessProbeImage),
						VSphereSyncerContainer(storageConfig.MetadataSyncerImage),
						CSIProvisionerContainer(storageConfig.ProvisionerImage),
					},
					Volumes: []corev1.Volume{
						{
							Name: "vsphere-config-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "csi-vsphere-config",
								},
							},
						},
						{
							Name: "socket-dir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

func CSIAttacherContainer(image string) corev1.Container {
	return corev1.Container{
		Name:  "csi-attacher",
		Image: image,
		Args:  []string{"--v=4", "--timeout=300s", "--csi-address=$(ADDRESS)", "--leader-election"},
		Env: []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: "/csi/csi.sock",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/csi",
				Name:      "socket-dir",
			},
		},
	}
}

func VSphereCSIControllerContainer(image string) corev1.Container {
	return corev1.Container{
		Name:  CSIControllerName,
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "healthz",
				ContainerPort: 9808,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.Parse("healthz"),
				},
			},
			InitialDelaySeconds: 10,
			TimeoutSeconds:      3,
			PeriodSeconds:       5,
			FailureThreshold:    3,
		},
		Env: []corev1.EnvVar{
			{
				Name:  "CSI_ENDPOINT",
				Value: "unix:///var/lib/csi/sockets/pluginproxy/csi.sock",
			},
			{
				Name:  "X_CSI_MODE",
				Value: "controller",
			},
			{
				Name:  "VSPHERE_CSI_CONFIG",
				Value: "/etc/cloud/csi-vsphere.conf",
			},
			{
				Name:  "LOGGER_LEVEL",
				Value: "PRODUCTION",
			},
			{
				Name:  "X_CSI_LOG_LEVEL",
				Value: "INFO",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/etc/cloud",
				Name:      "vsphere-config-volume",
				ReadOnly:  true,
			},
			{
				MountPath: "/var/lib/csi/sockets/pluginproxy/",
				Name:      "socket-dir",
			},
		},
	}
}

func LivenessProbeForCSIControllerContainer(image string) corev1.Container {
	return corev1.Container{
		Name:  "liveness-probe",
		Image: image,
		Args:  []string{"--csi-address=$(ADDRESS)"},
		Env: []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: "/var/lib/csi/sockets/pluginproxy/csi.sock",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/var/lib/csi/sockets/pluginproxy/",
				Name:      "socket-dir",
			},
		},
	}
}

func VSphereSyncerContainer(image string) corev1.Container {
	return corev1.Container{
		Name:  "vsphere-syncer",
		Image: image,
		Args:  []string{"--leader-election"},
		Env: []corev1.EnvVar{
			{
				Name:  "X_CSI_FULL_SYNC_INTERVAL_MINUTES",
				Value: "30",
			},
			{
				Name:  "LOGGER_LEVEL",
				Value: "PRODUCTION",
			},
			{
				Name:  "VSPHERE_CSI_CONFIG",
				Value: "/etc/cloud/csi-vsphere.conf",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/etc/cloud",
				Name:      "vsphere-config-volume",
				ReadOnly:  true,
			},
		},
	}
}

func CSIProvisionerContainer(image string) corev1.Container {
	return corev1.Container{
		Name:  "csi-provisioner",
		Image: image,
		Args: []string{
			"--v=4",
			"--timeout=300s",
			"--csi-address=$(ADDRESS)",
			"--leader-election",
			"--default-fstype=ext4",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: "/csi/csi.sock",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/csi",
				Name:      "socket-dir",
			},
		},
	}
}

func CSICloudConfigSecret(data string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-vsphere-config",
			Namespace: CSINamespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"csi-vsphere.conf": data,
		},
	}
}

func CSIComponentConfigSecret(secretName string, data string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: CSINamespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"csi-vsphere.conf": data,
		},
	}
}

func CSIFeatureStatesConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      CSIFeatureStateConfigMapName,
			Namespace: CSINamespace,
		},
		Data: map[string]string{
			"csi-migration": "false",
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func boolInt32(i int32) *int32 {
	return &i
}

func newHostPathType(pathType string) *corev1.HostPathType {
	hostPathType := new(corev1.HostPathType)
	*hostPathType = corev1.HostPathType(pathType)
	return hostPathType
}

func newMountPropagation(propagation string) *corev1.MountPropagationMode {
	propagationMode := new(corev1.MountPropagationMode)
	*propagationMode = corev1.MountPropagationMode(propagation)
	return propagationMode
}
