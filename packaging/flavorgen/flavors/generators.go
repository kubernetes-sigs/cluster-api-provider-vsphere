/*
Copyright 2020 The Kubernetes Authors.

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

package flavors

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/yaml"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
)

func newVSphereCluster() infrav1.VSphereCluster {
	return infrav1.VSphereCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       util.TypeToKind(&infrav1.VSphereCluster{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.ClusterNameVar,
			Namespace: env.NamespaceVar,
		},
		Spec: infrav1.VSphereClusterSpec{
			Server:     env.VSphereServerVar,
			Thumbprint: env.VSphereThumbprint,
			IdentityRef: &infrav1.VSphereIdentityReference{
				Name: env.ClusterNameVar,
				Kind: infrav1.SecretKind,
			},
			ControlPlaneEndpoint: infrav1.APIEndpoint{
				Host: env.ControlPlaneEndpointVar,
				Port: 6443,
			},
		},
	}
}

func newCluster(vsphereCluster infrav1.VSphereCluster, controlPlane *controlplanev1.KubeadmControlPlane) clusterv1.Cluster {
	cluster := clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       util.TypeToKind(&clusterv1.Cluster{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.ClusterNameVar,
			Namespace: env.NamespaceVar,
			Labels:    clusterLabels(),
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{env.DefaultClusterCIDR},
				},
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: vsphereCluster.GroupVersionKind().GroupVersion().String(),
				Kind:       vsphereCluster.Kind,
				Name:       vsphereCluster.Name,
			},
		},
	}
	if controlPlane != nil {
		cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
			APIVersion: controlPlane.GroupVersionKind().GroupVersion().String(),
			Kind:       controlPlane.Kind,
			Name:       controlPlane.Name,
		}
	}
	return cluster
}

func clusterLabels() map[string]string {
	return map[string]string{"cluster.x-k8s.io/cluster-name": env.ClusterNameVar}
}

func newVSphereMachineTemplate() infrav1.VSphereMachineTemplate {
	return infrav1.VSphereMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.ClusterNameVar,
			Namespace: env.NamespaceVar,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       util.TypeToKind(&infrav1.VSphereMachineTemplate{}),
		},
		Spec: infrav1.VSphereMachineTemplateSpec{
			Template: infrav1.VSphereMachineTemplateResource{
				Spec: defaultVirtualMachineSpec(),
			},
		},
	}
}

func defaultVirtualMachineSpec() infrav1.VSphereMachineSpec {
	return infrav1.VSphereMachineSpec{
		VirtualMachineCloneSpec: defaultVirtualMachineCloneSpec(),
	}
}

func defaultVirtualMachineCloneSpec() infrav1.VirtualMachineCloneSpec {
	return infrav1.VirtualMachineCloneSpec{
		Datacenter: env.VSphereDataCenterVar,
		Network: infrav1.NetworkSpec{
			Devices: []infrav1.NetworkDeviceSpec{
				{
					NetworkName: env.VSphereNetworkVar,
					DHCP4:       true,
					DHCP6:       false,
				},
			},
		},
		CustomVMXKeys:     defaultCustomVMXKeys(),
		CloneMode:         infrav1.LinkedClone,
		NumCPUs:           env.DefaultNumCPUs,
		DiskGiB:           env.DefaultDiskGiB,
		MemoryMiB:         env.DefaultMemoryMiB,
		Template:          env.VSphereTemplateVar,
		Server:            env.VSphereServerVar,
		Thumbprint:        env.VSphereThumbprint,
		ResourcePool:      env.VSphereResourcePoolVar,
		Datastore:         env.VSphereDatastoreVar,
		StoragePolicyName: env.VSphereStoragePolicyVar,
		Folder:            env.VSphereFolderVar,
	}
}

func defaultKubeadmInitSpec(files []bootstrapv1.File) bootstrapv1.KubeadmConfigSpec {
	return bootstrapv1.KubeadmConfigSpec{
		InitConfiguration: &bootstrapv1.InitConfiguration{
			NodeRegistration: defaultNodeRegistrationOptions(),
		},
		JoinConfiguration: &bootstrapv1.JoinConfiguration{
			NodeRegistration: defaultNodeRegistrationOptions(),
		},
		ClusterConfiguration: &bootstrapv1.ClusterConfiguration{
			APIServer: bootstrapv1.APIServer{
				ControlPlaneComponent: defaultControlPlaneComponent(),
			},
			ControllerManager: defaultControlPlaneComponent(),
		},
		Users:                    defaultUsers(),
		PreKubeadmCommands:       defaultPreKubeadmCommands(),
		UseExperimentalRetryJoin: true,
		Files:                    files,
	}
}

func newKubeadmConfigTemplate() bootstrapv1.KubeadmConfigTemplate {
	return bootstrapv1.KubeadmConfigTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.ClusterNameVar + env.MachineDeploymentNameSuffix,
			Namespace: env.NamespaceVar,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: bootstrapv1.GroupVersion.String(),
			Kind:       util.TypeToKind(&bootstrapv1.KubeadmConfigTemplate{}),
		},
		Spec: bootstrapv1.KubeadmConfigTemplateSpec{
			Template: bootstrapv1.KubeadmConfigTemplateResource{
				Spec: bootstrapv1.KubeadmConfigSpec{
					JoinConfiguration: &bootstrapv1.JoinConfiguration{
						NodeRegistration: defaultNodeRegistrationOptions(),
					},
					Users:              defaultUsers(),
					PreKubeadmCommands: defaultPreKubeadmCommands(),
				},
			},
		},
	}
}

func defaultNodeRegistrationOptions() bootstrapv1.NodeRegistrationOptions {
	return bootstrapv1.NodeRegistrationOptions{
		Name:             "{{ ds.meta_data.hostname }}",
		CRISocket:        "/var/run/containerd/containerd.sock",
		KubeletExtraArgs: defaultExtraArgs(),
	}
}

func defaultUsers() []bootstrapv1.User {
	return []bootstrapv1.User{
		{
			Name: "capv",
			Sudo: pointer.StringPtr("ALL=(ALL) NOPASSWD:ALL"),
			SSHAuthorizedKeys: []string{
				env.VSphereSSHAuthorizedKeysVar,
			},
		},
	}
}

func defaultControlPlaneComponent() bootstrapv1.ControlPlaneComponent {
	return bootstrapv1.ControlPlaneComponent{
		ExtraArgs: defaultExtraArgs(),
	}
}

func defaultCustomVMXKeys() map[string]string {
	return map[string]string{}
}

func defaultExtraArgs() map[string]string {
	return map[string]string{
		"cloud-provider": "external",
	}
}

func defaultPreKubeadmCommands() []string {
	return []string{
		"hostname \"{{ ds.meta_data.hostname }}\"",
		"echo \"::1         ipv6-localhost ipv6-loopback\" >/etc/hosts",
		"echo \"127.0.0.1   localhost\" >>/etc/hosts",
		"echo \"127.0.0.1   {{ ds.meta_data.hostname }}\" >>/etc/hosts",
		"echo \"{{ ds.meta_data.hostname }}\" >/etc/hostname",
	}
}

func kubeVIPPod() string {
	hostPathType := corev1.HostPathFileOrCreate
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       util.TypeToKind(&corev1.Pod{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-vip",
			Namespace: "kube-system",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "kube-vip",
					Image:           "ghcr.io/kube-vip/kube-vip:v0.4.0",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"manager",
					},
					Env: []corev1.EnvVar{
						{
							// Enables kube-vip control-plane functionality
							Name:  "cp_enable",
							Value: "true",
						},
						{
							// Interface that the vip should bind to
							Name:  "vip_interface",
							Value: env.VipNetworkInterfaceVar,
						},
						{
							// VIP IP address
							// 'vip_address' was replaced by 'address'
							Name:  "address",
							Value: env.ControlPlaneEndpointVar,
						},
						{
							// VIP TCP port
							Name:  "port",
							Value: "6443",
						},
						{
							// Enables ARP brodcasts from Leader (requires L2 connectivity)
							Name:  "vip_arp",
							Value: "true",
						},
						{
							// Kubernetes algorithm to be used.
							Name:  "vip_leaderelection",
							Value: "true",
						},
						{
							// Seconds a lease is held for
							Name:  "vip_leaseduration",
							Value: "15",
						},
						{
							// Seconds a leader can attempt to renew the lease
							Name:  "vip_renewdeadline",
							Value: "10",
						},
						{
							// Number of times the leader will hold the lease for
							Name:  "vip_retryperiod",
							Value: "2",
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"NET_ADMIN",
								"SYS_TIME",
								"NET_RAW",
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/etc/kubernetes/admin.conf",
							Name:      "kubeconfig",
						},
					},
				},
			},
			HostNetwork: true,
			HostAliases: []corev1.HostAlias{
				{
					IP: "127.0.0.1",
					Hostnames: []string{
						"kubernetes",
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "kubeconfig",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/kubernetes/admin.conf",
							Type: &hostPathType,
						},
					},
				},
			},
		},
	}
	podBytes, err := yaml.Marshal(pod)
	if err != nil {
		panic(err)
	}
	return string(podBytes)
}

func newClusterResourceSet(cluster clusterv1.Cluster) addonsv1.ClusterResourceSet {
	crs := addonsv1.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       util.TypeToKind(&addonsv1.ClusterResourceSet{}),
			APIVersion: addonsv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + env.ClusterResourceSetNameSuffix,
			Labels:    clusterLabels(),
			Namespace: cluster.Namespace,
		},
		Spec: addonsv1.ClusterResourceSetSpec{
			ClusterSelector: metav1.LabelSelector{MatchLabels: clusterLabels()},
			Resources:       []addonsv1.ResourceRef{},
		},
	}

	return crs
}

func newIdentitySecret() corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: env.NamespaceVar,
			Name:      env.ClusterNameVar,
		},
		StringData: map[string]string{
			identity.UsernameKey: env.VSphereUsername,
			identity.PasswordKey: env.VSpherePassword,
		},
	}
}

func newMachineDeployment(cluster clusterv1.Cluster, machineTemplate infrav1.VSphereMachineTemplate, bootstrapTemplate bootstrapv1.KubeadmConfigTemplate) clusterv1.MachineDeployment {
	return clusterv1.MachineDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       util.TypeToKind(&clusterv1.MachineDeployment{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.ClusterNameVar + env.MachineDeploymentNameSuffix,
			Labels:    clusterLabels(),
			Namespace: env.NamespaceVar,
		},
		Spec: clusterv1.MachineDeploymentSpec{
			ClusterName: env.ClusterNameVar,
			Replicas:    pointer.Int32Ptr(int32(555)),
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels: clusterLabels(),
				},
				Spec: clusterv1.MachineSpec{
					Version:     pointer.StringPtr(env.KubernetesVersionVar),
					ClusterName: cluster.Name,
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: bootstrapTemplate.GroupVersionKind().GroupVersion().String(),
							Kind:       bootstrapTemplate.Kind,
							Name:       bootstrapTemplate.Name,
						},
					},
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: machineTemplate.GroupVersionKind().GroupVersion().String(),
						Kind:       machineTemplate.Kind,
						Name:       machineTemplate.Name,
					},
				},
			},
		},
	}
}

func newKubeVIPFiles() []bootstrapv1.File {
	return []bootstrapv1.File{
		{
			Owner:   "root:root",
			Path:    "/etc/kubernetes/manifests/kube-vip.yaml",
			Content: kubeVIPPod(),
		},
	}
}

func newKubeadmControlplane(replicas int, infraTemplate infrav1.VSphereMachineTemplate, files []bootstrapv1.File) controlplanev1.KubeadmControlPlane {
	return controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       util.TypeToKind(&controlplanev1.KubeadmControlPlane{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.ClusterNameVar,
			Namespace: env.NamespaceVar,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Replicas: pointer.Int32Ptr(int32(replicas)),
			Version:  env.KubernetesVersionVar,
			MachineTemplate: controlplanev1.KubeadmControlPlaneMachineTemplate{
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: infraTemplate.GroupVersionKind().GroupVersion().String(),
					Kind:       infraTemplate.Kind,
					Name:       infraTemplate.Name,
				},
			},
			KubeadmConfigSpec: defaultKubeadmInitSpec(files),
		},
	}
}
