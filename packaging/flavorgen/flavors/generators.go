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
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	addonsv1alpha3 "sigs.k8s.io/cluster-api/exp/addons/api/v1alpha3"
	"sigs.k8s.io/yaml"
)

const (
	clusterNameVar               = "${ CLUSTER_NAME }"
	controlPlaneMachineCountVar  = "${ CONTROL_PLANE_MACHINE_COUNT }"
	defaultCloudProviderImage    = "gcr.io/cloud-provider-vsphere/cpi/release/manager:v1.0.0"
	defaultClusterCIDR           = "192.168.0.0/16"
	defaultDiskGiB               = 25
	defaultMemoryMiB             = 8192
	defaultNumCPUs               = 2
	kubernetesVersionVar         = "${ KUBERNETES_VERSION }"
	machineDeploymentNameSuffix  = "-md-0"
	namespaceVar                 = "${ NAMESPACE }"
	vSphereDataCenterVar         = "${ VSPHERE_DATACENTER }"
	vSphereDatastoreVar          = "${ VSPHERE_DATASTORE }"
	vSphereFolderVar             = "${ VSPHERE_FOLDER }"
	vSphereHaproxyTemplateVar    = "${ VSPHERE_HAPROXY_TEMPLATE }"
	vSphereNetworkVar            = "${ VSPHERE_NETWORK }"
	vSphereResourcePoolVar       = "${ VSPHERE_RESOURCE_POOL }"
	vSphereServerVar             = "${ VSPHERE_SERVER }"
	vSphereSSHAuthorizedKeysVar  = "${ VSPHERE_SSH_AUTHORIZED_KEY }"
	vSphereTemplateVar           = "${ VSPHERE_TEMPLATE }"
	workerMachineCountVar        = "${ WORKER_MACHINE_COUNT }"
	controlPlaneEndpointVar      = "${ CONTROL_PLANE_ENDPOINT_IP }"
	vipNetworkInterfaceVar       = "${ VIP_NETWORK_INTERFACE }"
	clusterResourceSetNameSuffix = "-crs-0"
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
	} else {
		vsphereCluster.Spec.ControlPlaneEndpoint = infrav1.APIEndpoint{
			Host: controlPlaneEndpointVar,
			Port: 6443,
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

func kubeVIPPod() string {
	hostPathType := v1.HostPathFileOrCreate
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       typeToKind(&v1.Pod{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-vip",
			Namespace: "kube-system",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "kube-vip",
					Image: "plndr/kube-vip:0.1.6",
					Args: []string{
						"start",
					},
					ImagePullPolicy: v1.PullIfNotPresent,
					SecurityContext: &v1.SecurityContext{
						Capabilities: &v1.Capabilities{
							Add: []v1.Capability{
								"NET_ADMIN",
								"SYS_TIME",
							},
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							MountPath: "/etc/kubernetes/admin.conf",
							Name:      "kubeconfig",
						},
					},
					Env: []v1.EnvVar{
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
							Value: controlPlaneEndpointVar,
						},
						{
							Name:  "lb_backendport",
							Value: "6443",
						},
						{
							Name:  "vip_addpeerstolb",
							Value: "true",
						},
						{
							Name:  "lb_name",
							Value: "kcpEndpoint",
						},
						{
							Name:  "lb_bindtovip",
							Value: "true",
						},
						{
							Name:  "vip_interface",
							Value: vipNetworkInterfaceVar,
						},
						{
							Name:  "lb_type",
							Value: "tcp",
						},
					},
				},
			},
			HostNetwork: true,
			Volumes: []v1.Volume{
				{
					Name: "kubeconfig",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
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
func newClusterResourceSet(cluster clusterv1.Cluster, cloudConfigSecret *v1.Secret) addonsv1alpha3.ClusterResourceSet {

	crs := addonsv1alpha3.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       typeToKind(&addonsv1alpha3.ClusterResourceSet{}),
			APIVersion: addonsv1alpha3.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + clusterResourceSetNameSuffix,
			Labels:    clusterLabels(),
			Namespace: cluster.Namespace,
		},
		Spec: addonsv1alpha3.ClusterResourceSetSpec{
			Resources: []addonsv1alpha3.ResourceRef{},
		},
	}
	crs.Spec.Resources = append(crs.Spec.Resources, addonsv1alpha3.ResourceRef{
		Name: cloudConfigSecret.Name,
		Kind: "Secret",
	})

	return crs
}
func appendResourceSecretToCRS(crs *addonsv1alpha3.ClusterResourceSet, generatedSecret *v1.Secret) {

	crs.Spec.Resources = append(crs.Spec.Resources, addonsv1alpha3.ResourceRef{
		Name: generatedSecret.Name,
		Kind: "Secret",
	})
}

func newClusterResourceSetBinding(cluster *clusterv1.Cluster, cloudConfigSecret *v1.Secret, crs *addonsv1alpha3.ClusterResourceSet) addonsv1alpha3.ClusterResourceSetBinding {

	binding := addonsv1alpha3.ClusterResourceSetBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterResourceSetBinding",
			APIVersion: addonsv1alpha3.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              cluster.Name,
			Namespace:         cluster.Namespace,
			Generation:        cluster.Generation,
			CreationTimestamp: cluster.CreationTimestamp,
			OwnerReferences:   cluster.OwnerReferences,
		},
	}

	binding.SetOwnerReferences([]metav1.OwnerReference{
		// binding are owned by the ClusterResourceSet / ownership set by the ClusterResourceSet controller
		{
			APIVersion: crs.APIVersion,
			Kind:       crs.Kind,
			Name:       crs.Name,
			UID:        crs.UID,
		},
	})
	// binding are owned by the Cluster / ownership set by the ClusterResourceSet controller
	binding.SetOwnerReferences(append(binding.OwnerReferences, metav1.OwnerReference{
		APIVersion: cluster.APIVersion,
		Kind:       cluster.Kind,
		Name:       cluster.Name,
		UID:        cluster.UID,
	}))
	resourceSetBinding := addonsv1alpha3.ResourceSetBinding{
		ClusterResourceSetName: crs.Name,
		Resources:              []addonsv1alpha3.ResourceBinding{},
	}
	resourceSetBinding.Resources = append(resourceSetBinding.Resources, addonsv1alpha3.ResourceBinding{
		ResourceRef: addonsv1alpha3.ResourceRef{
			Name: cloudConfigSecret.Name,
			Kind: cloudConfigSecret.Kind,
		},
		LastAppliedTime: &cloudConfigSecret.CreationTimestamp,
	})
	binding.SetCreationTimestamp(cloudConfigSecret.CreationTimestamp)
	binding.SetGenerateName(cloudConfigSecret.GetGenerateName())
	binding.SetName(cloudConfigSecret.GetName())
	binding.SetNamespace(cloudConfigSecret.GetNamespace())

	binding.Spec.Bindings = append(binding.Spec.Bindings, &resourceSetBinding)

	return binding

}

func appendResourceSetToBinding(binding *addonsv1alpha3.ClusterResourceSetBinding, resourceSetSecret *v1.Secret, crs *addonsv1alpha3.ClusterResourceSet) {

	resourceSetBinding := addonsv1alpha3.ResourceSetBinding{
		ClusterResourceSetName: crs.Name,
		Resources:              []addonsv1alpha3.ResourceBinding{},
	}
	resourceSetBinding.Resources = append(resourceSetBinding.Resources, addonsv1alpha3.ResourceBinding{
		ResourceRef: addonsv1alpha3.ResourceRef{
			Name: resourceSetSecret.Name,
			Kind: "Secret",
		},
		LastAppliedTime: &metav1.Time{Time: time.Now().UTC()}, // useless
	})
	binding.Spec.Bindings = append(binding.Spec.Bindings, &resourceSetBinding)

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
			KubeadmConfigSpec: defaultKubeadmInitSpec(files),
		},
	}
}
