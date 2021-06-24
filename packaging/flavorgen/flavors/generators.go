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
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha4"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha4"
	addonsv1alpha4 "sigs.k8s.io/cluster-api/exp/addons/api/v1alpha4"
	"sigs.k8s.io/yaml"
)

func newVSphereCluster(lb *infrav1.HAProxyLoadBalancer) infrav1.VSphereCluster {
	vsphereCluster := infrav1.VSphereCluster{
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
		},
	}
	if lb != nil {
		vsphereCluster.Spec.LoadBalancerRef = &corev1.ObjectReference{
			APIVersion: lb.GroupVersionKind().GroupVersion().String(),
			Kind:       lb.Kind,
			Name:       lb.Name,
		}
	} else {
		vsphereCluster.Spec.ControlPlaneEndpoint = infrav1.APIEndpoint{
			Host: env.ControlPlaneEndpointVar,
			Port: 6443,
		}
	}
	return vsphereCluster
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
		InitConfiguration: &kubeadmv1beta1.InitConfiguration{
			NodeRegistration: defaultNodeRegistrationOptions(),
		},
		JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
			NodeRegistration: defaultNodeRegistrationOptions(),
		},
		ClusterConfiguration: &kubeadmv1beta1.ClusterConfiguration{
			APIServer: kubeadmv1beta1.APIServer{
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
					JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
						NodeRegistration: defaultNodeRegistrationOptions(),
					},
					Users:              defaultUsers(),
					PreKubeadmCommands: defaultPreKubeadmCommands(),
				},
			},
		},
	}
}

func defaultNodeRegistrationOptions() kubeadmv1beta1.NodeRegistrationOptions {
	return kubeadmv1beta1.NodeRegistrationOptions{
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

func defaultControlPlaneComponent() kubeadmv1beta1.ControlPlaneComponent {
	return kubeadmv1beta1.ControlPlaneComponent{
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
					Name:  "kube-vip",
					Image: "plndr/kube-vip:0.3.2",
					Args: []string{
						"start",
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"NET_ADMIN",
								"SYS_TIME",
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/etc/kubernetes/admin.conf",
							Name:      "kubeconfig",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "vip_arp",
							Value: "true",
						},
						{
							Name:  "vip_leaderelection",
							Value: "true",
						},
						{
							Name:  "vip_address",
							Value: env.ControlPlaneEndpointVar,
						},
						{
							// this is hardcoded since we use eth0 as a network interface for all of our machines in this template
							Name:  "vip_interface",
							Value: "eth0",
						},
						{
							Name:  "vip_leaseduration",
							Value: "15",
						},
						{
							Name:  "vip_renewdeadline",
							Value: "10",
						},
						{
							Name:  "vip_retryperiod",
							Value: "2",
						},
					},
				},
			},
			HostNetwork: true,
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
func newClusterResourceSet(cluster clusterv1.Cluster) addonsv1alpha4.ClusterResourceSet {
	crs := addonsv1alpha4.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       util.TypeToKind(&addonsv1alpha4.ClusterResourceSet{}),
			APIVersion: addonsv1alpha4.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + env.ClusterResourceSetNameSuffix,
			Labels:    clusterLabels(),
			Namespace: cluster.Namespace,
		},
		Spec: addonsv1alpha4.ClusterResourceSetSpec{
			ClusterSelector: metav1.LabelSelector{MatchLabels: clusterLabels()},
			Resources:       []addonsv1alpha4.ResourceRef{},
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

func newHAProxyLoadBalancer() infrav1.HAProxyLoadBalancer {
	cloneSpec := defaultVirtualMachineCloneSpec()
	cloneSpec.Template = env.VSphereHaproxyTemplateVar
	return infrav1.HAProxyLoadBalancer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       util.TypeToKind(&infrav1.HAProxyLoadBalancer{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.ClusterNameVar,
			Labels:    clusterLabels(),
			Namespace: env.NamespaceVar,
		},
		Spec: infrav1.HAProxyLoadBalancerSpec{
			VirtualMachineConfiguration: cloneSpec,
			User: &infrav1.SSHUser{
				Name: "capv",
				AuthorizedKeys: []string{
					env.VSphereSSHAuthorizedKeysVar,
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
			InfrastructureTemplate: corev1.ObjectReference{
				APIVersion: infraTemplate.GroupVersionKind().GroupVersion().String(),
				Kind:       infraTemplate.Kind,
				Name:       infraTemplate.Name,
			},
			KubeadmConfigSpec: defaultKubeadmInitSpec(files),
		},
	}
}
