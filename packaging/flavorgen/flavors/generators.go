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
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	cloudprovidersvc "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
)

const (
	clusterNameVar              = "${ CLUSTER_NAME }"
	controlPlaneMachineCountVar = "${ CONTROL_PLANE_MACHINE_COUNT }"
	defaultCloudProviderImage   = "gcr.io/cloud-provider-vsphere/cpi/release/manager:v1.0.0"
	defaultClusterCIDR          = "192.168.0.0/16"
	defaultDiskGiB              = 25
	defaultMemoryMiB            = 8192
	defaultNumCPUs              = 2
	kubernetesVersionVar        = "${ KUBERNETES_VERSION }"
	machineDeploymentNameSuffix = "-md-0"
	namespaceVar                = "${ NAMESPACE }"
	vSphereDataCenterVar        = "${ VSPHERE_DATACENTER }"
	vSphereDatastoreVar         = "${ VSPHERE_DATASTORE }"
	vSphereFolderVar            = "${ VSPHERE_FOLDER }"
	vSphereHaproxyTemplateVar   = "${ VSPHERE_HAPROXY_TEMPLATE }"
	vSphereNetworkVar           = "${ VSPHERE_NETWORK }"
	vSphereResourcePoolVar      = "${ VSPHERE_RESOURCE_POOL }"
	vSphereServerVar            = "${ VSPHERE_SERVER }"
	vSphereSSHAuthorizedKeysVar = "${ VSPHERE_SSH_AUTHORIZED_KEY }"
	vSphereTemplateVar          = "${ VSPHERE_TEMPLATE }"
	workerMachineCountVar       = "${ WORKER_MACHINE_COUNT }"
)

type replacement struct {
	kind      string
	name      string
	value     interface{}
	fieldPath []string
}

var (
	replacements = []replacement{
		{
			kind:      "KubeadmControlPlane",
			name:      "${ CLUSTER_NAME }",
			value:     controlPlaneMachineCountVar,
			fieldPath: []string{"spec", "replicas"},
		},
		{
			kind:      "MachineDeployment",
			name:      "${ CLUSTER_NAME }-md-0",
			value:     workerMachineCountVar,
			fieldPath: []string{"spec", "replicas"},
		},
		{
			kind:      "MachineDeployment",
			name:      "${ CLUSTER_NAME }-md-0",
			value:     map[string]interface{}{},
			fieldPath: []string{"spec", "selector", "matchLabels"},
		},
	}

	stringVars = []string{
		regexVar(clusterNameVar),
		regexVar(clusterNameVar + machineDeploymentNameSuffix),
		regexVar(namespaceVar),
		regexVar(kubernetesVersionVar),
		regexVar(vSphereFolderVar),
		regexVar(vSphereHaproxyTemplateVar),
		regexVar(vSphereResourcePoolVar),
		regexVar(vSphereSSHAuthorizedKeysVar),
		regexVar(vSphereDataCenterVar),
		regexVar(vSphereDatastoreVar),
		regexVar(vSphereNetworkVar),
		regexVar(vSphereServerVar),
		regexVar(vSphereTemplateVar),
		regexVar(vSphereHaproxyTemplateVar),
	}
)

func regexVar(str string) string {
	return "((?m:\\" + str + "$))"
}

func newVSphereCluster(lb *infrav1.HAProxyLoadBalancer) infrav1.VSphereCluster {
	vsphereCluster := infrav1.VSphereCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       typeToKind(&infrav1.VSphereCluster{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterNameVar,
			Namespace: namespaceVar,
		},
		Spec: infrav1.VSphereClusterSpec{
			Server: vSphereServerVar,
			CloudProviderConfiguration: infrav1.CPIConfig{
				Global: infrav1.CPIGlobalConfig{
					SecretName:      "cloud-provider-vsphere-credentials",
					SecretNamespace: metav1.NamespaceSystem,
					Insecure:        true,
				},
				VCenter: map[string]infrav1.CPIVCenterConfig{
					vSphereServerVar: {Datacenters: vSphereDataCenterVar},
				},
				Network: infrav1.CPINetworkConfig{
					Name: vSphereNetworkVar,
				},
				Workspace: infrav1.CPIWorkspaceConfig{
					Server:       vSphereServerVar,
					Datacenter:   vSphereDataCenterVar,
					Datastore:    vSphereDatastoreVar,
					ResourcePool: vSphereResourcePoolVar,
					Folder:       vSphereFolderVar,
				},
				ProviderConfig: infrav1.CPIProviderConfig{
					Cloud: &infrav1.CPICloudConfig{
						ControllerImage: defaultCloudProviderImage,
					},
					Storage: &infrav1.CPIStorageConfig{
						ControllerImage:     cloudprovidersvc.DefaultCSIControllerImage,
						NodeDriverImage:     cloudprovidersvc.DefaultCSINodeDriverImage,
						AttacherImage:       cloudprovidersvc.DefaultCSIAttacherImage,
						ProvisionerImage:    cloudprovidersvc.DefaultCSIProvisionerImage,
						MetadataSyncerImage: cloudprovidersvc.DefaultCSIMetadataSyncerImage,
						LivenessProbeImage:  cloudprovidersvc.DefaultCSILivenessProbeImage,
						RegistrarImage:      cloudprovidersvc.DefaultCSIRegistrarImage,
					},
				},
			},
		},
	}
	if lb != nil {
		vsphereCluster.Spec.LoadBalancerRef = &corev1.ObjectReference{
			APIVersion: lb.GroupVersionKind().GroupVersion().String(),
			Kind:       lb.Kind,
			Name:       lb.Name,
		}
	}
	return vsphereCluster
}

func newCluster(vsphereCluster infrav1.VSphereCluster, controlPlane *controlplanev1.KubeadmControlPlane) clusterv1.Cluster {
	cluster := clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       typeToKind(&clusterv1.Cluster{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterNameVar,
			Namespace: namespaceVar,
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{defaultClusterCIDR},
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
	return map[string]string{"cluster.x-k8s.io/cluster-name": clusterNameVar}
}

func newVSphereMachineTemplate() infrav1.VSphereMachineTemplate {
	return infrav1.VSphereMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterNameVar,
			Namespace: namespaceVar,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       typeToKind(&infrav1.VSphereMachineTemplate{}),
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
		Datacenter: vSphereDataCenterVar,
		Network: infrav1.NetworkSpec{
			Devices: []infrav1.NetworkDeviceSpec{
				{
					NetworkName: vSphereNetworkVar,
					DHCP4:       true,
					DHCP6:       false,
				},
			},
		},
		CloneMode:    infrav1.LinkedClone,
		NumCPUs:      defaultNumCPUs,
		DiskGiB:      defaultDiskGiB,
		MemoryMiB:    defaultMemoryMiB,
		Template:     vSphereTemplateVar,
		Server:       vSphereServerVar,
		ResourcePool: vSphereResourcePoolVar,
		Datastore:    vSphereDatastoreVar,
		Folder:       vSphereFolderVar,
	}
}

func defaultKubeadmInitSpec() bootstrapv1.KubeadmConfigSpec {
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
	}
}

func newKubeadmConfigTemplate() bootstrapv1.KubeadmConfigTemplate {
	return bootstrapv1.KubeadmConfigTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterNameVar + machineDeploymentNameSuffix,
			Namespace: namespaceVar,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: bootstrapv1.GroupVersion.String(),
			Kind:       typeToKind(&bootstrapv1.KubeadmConfigTemplate{}),
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
				vSphereSSHAuthorizedKeysVar,
			},
		},
	}
}

func defaultControlPlaneComponent() kubeadmv1beta1.ControlPlaneComponent {
	return kubeadmv1beta1.ControlPlaneComponent{
		ExtraArgs: defaultExtraArgs(),
	}
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

func newMachineDeployment(cluster clusterv1.Cluster, machineTemplate infrav1.VSphereMachineTemplate, bootstrapTemplate bootstrapv1.KubeadmConfigTemplate) clusterv1.MachineDeployment {
	return clusterv1.MachineDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       typeToKind(&clusterv1.MachineDeployment{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterNameVar + machineDeploymentNameSuffix,
			Labels:    clusterLabels(),
			Namespace: namespaceVar,
		},
		Spec: clusterv1.MachineDeploymentSpec{
			ClusterName: clusterNameVar,
			Replicas:    pointer.Int32Ptr(int32(555)),
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels: clusterLabels(),
				},
				Spec: clusterv1.MachineSpec{
					Version:     pointer.StringPtr(kubernetesVersionVar),
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
	cloneSpec.Template = vSphereHaproxyTemplateVar
	return infrav1.HAProxyLoadBalancer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       typeToKind(&infrav1.HAProxyLoadBalancer{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterNameVar,
			Labels:    clusterLabels(),
			Namespace: namespaceVar,
		},
		Spec: infrav1.HAProxyLoadBalancerSpec{
			VirtualMachineConfiguration: cloneSpec,
			User: &infrav1.SSHUser{
				Name: "capv",
				AuthorizedKeys: []string{
					vSphereSSHAuthorizedKeysVar,
				},
			},
		},
	}
}

func newKubeadmControlplane(replicas int, infraTemplate infrav1.VSphereMachineTemplate) controlplanev1.KubeadmControlPlane {
	return controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       typeToKind(&controlplanev1.KubeadmControlPlane{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterNameVar,
			Namespace: namespaceVar,
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Replicas: pointer.Int32Ptr(int32(replicas)),
			Version:  kubernetesVersionVar,
			InfrastructureTemplate: corev1.ObjectReference{
				APIVersion: infraTemplate.GroupVersionKind().GroupVersion().String(),
				Kind:       infraTemplate.Kind,
				Name:       infraTemplate.Name,
			},
			KubeadmConfigSpec: defaultKubeadmInitSpec(),
		},
	}
}
