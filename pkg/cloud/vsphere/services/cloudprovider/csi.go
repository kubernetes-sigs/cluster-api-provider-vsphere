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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2/cloudprovider"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

// NOTE: the contents of this file are derived from https://github.com/kubernetes-sigs/vsphere-csi-driver/tree/master/manifests/1.14

func CSIControllerServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vsphere-csi-controller",
			Namespace: "kube-system",
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
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"csidrivers"},
				Verbs:     []string{"create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get", "list", "watch", "update", "create", "delete"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"csinodes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"volumeattachments"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"list", "watch", "create", "update", "patch"},
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
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
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
				Name:      "vsphere-csi-controller",
				Namespace: "kube-system",
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

func VSphereCSINodeDaemonSet(storageConfig cloudprovider.StorageConfig) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vsphere-csi-node",
			Namespace: "kube-system",
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
					HostNetwork: true,
					Containers: []corev1.Container{
						NodeDriverRegistrarContainer(),
						VSphereCSINodeContainer(storageConfig.NodeDriverImage),
						LivenessProbeForNodeContainer(),
					},
					Tolerations: []corev1.Toleration{
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: corev1.TaintEffectNoSchedule,
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
									Type: newHostPathType(string(corev1.HostPathDirectoryOrCreate)),
								},
							},
						},
						{
							Name: "plugin-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins_registry/csi.vsphere.vmware.com",
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

func NodeDriverRegistrarContainer() corev1.Container {
	return corev1.Container{
		Name:  "node-driver-registrar",
		Image: "quay.io/k8scsi/csi-node-driver-registrar:v1.1.0",
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-c",
						"rm -rf /registration/csi.vsphere.vmware.com /var/lib/kubelet/plugins_registry/csi.vsphere.vmware.com /var/lib/kubelet/plugins_registry/csi.vsphere.vmware.com-reg.sock",
					},
				},
			},
		},
		Args: []string{
			"--v=4",
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
				Value: "/var/lib/kubelet/plugins_registry/csi.vsphere.vmware.com/csi.sock",
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
		Name:            "vsphere-csi-node",
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
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
				Name:  "X_CSI_VSPHERE_CLOUD_CONFIG",
				Value: "/etc/cloud/csi-vsphere.conf",
			},
		},
		Args: []string{"--v=4"},
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

func LivenessProbeForNodeContainer() corev1.Container {
	return corev1.Container{
		Name:  "liveness-probe",
		Image: "quay.io/k8scsi/livenessprobe:v1.1.0",
		Args:  []string{"--csi-address=$(ADDRESS)", "--health-port=9808"},
		Env: []corev1.EnvVar{
			{
				Name:  "ADDRESS",
				Value: "/csi/csi.sock",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "plugin-dir",
				MountPath: "/csi",
			},
		},
	}
}

func CSIControllerStatefulSet(storageConfig cloudprovider.StorageConfig) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vsphere-csi-controller",
			Namespace: "kube-system",
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "vsphere-csi-controller",
			Replicas:    boolInt32(1),
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "vsphere-csi-controller",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":  "vsphere-csi-controller",
						"role": "vsphere-csi",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "vsphere-csi-controller",
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
					HostNetwork: true,
					Containers: []corev1.Container{
						CSIAttacherContainer(storageConfig.AttacherImage),
						VSphereCSIControllerContainer(storageConfig.ControllerImage),
						LivenessProbeForCSIControllerContainer(),
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
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/csi/sockets/pluginproxy/csi.vsphere.vmware.com",
									Type: newHostPathType(string(corev1.HostPathDirectoryOrCreate)),
								},
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
		Args:  []string{"--v=4", "--timeout=60s", "--csi-address=$(ADDRESS)"},
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
		Name:  "vsphere-csi-controller",
		Image: image,
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{"/bin/sh", "-c", "rm -rf /var/lib/csi/sockets/pluginproxy/csi.vsphere.vmware.com"},
				},
			},
		},
		Args:            []string{"--v=4"},
		ImagePullPolicy: corev1.PullAlways,
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
				Name:  "X_CSI_VSPHERE_CLOUD_CONFIG",
				Value: "/etc/cloud/csi-vsphere.conf",
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

func LivenessProbeForCSIControllerContainer() corev1.Container {
	return corev1.Container{
		Name:  "liveness-probe",
		Image: "quay.io/k8scsi/livenessprobe:v1.1.0",
		Args:  []string{"--csi-address=$(ADDRESS)", "--health-port=9809"},
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
		Name:            "vsphere-syncer",
		Image:           image,
		Args:            []string{"--v=4"},
		ImagePullPolicy: corev1.PullAlways,
		Env: []corev1.EnvVar{
			{
				Name:  "X_CSI_FULL_SYNC_INTERVAL_MINUTES",
				Value: "30",
			},
			{
				Name:  "X_CSI_VSPHERE_CLOUD_CONFIG",
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
			"--timeout=60s",
			"--csi-address=$(ADDRESS)",
			"--feature-gates=Topology=true",
			"--strict-topology",
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
			Namespace: "kube-system",
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"csi-vsphere.conf": data,
		},
	}
}

// ConfigForCSI returns a cloudprovider.Config specific to the vSphere CSI driver until
// it supports using Secrets for vCenter credentials
func ConfigForCSI(ctx *context.ClusterContext) *cloudprovider.Config {
	config := &cloudprovider.Config{}

	config.Global.ClusterID = fmt.Sprintf("%s/%s", ctx.Cluster.Namespace, ctx.Cluster.Name)
	config.Global.Insecure = ctx.VSphereCluster.Spec.CloudProviderConfiguration.Global.Insecure
	config.Network.Name = ctx.VSphereCluster.Spec.CloudProviderConfiguration.Network.Name

	config.VCenter = map[string]cloudprovider.VCenterConfig{}
	for name, vcenter := range ctx.VSphereCluster.Spec.CloudProviderConfiguration.VCenter {
		config.VCenter[name] = cloudprovider.VCenterConfig{
			Username:    ctx.User(),
			Password:    ctx.Pass(),
			Datacenters: vcenter.Datacenters,
		}
	}

	return config
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
