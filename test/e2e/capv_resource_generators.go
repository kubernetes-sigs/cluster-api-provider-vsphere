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

package e2e

import (
	"flag"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"

	"sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
)

var (
	sshAuthKey string
)

func init() {
	flag.StringVar(&sshAuthKey, "e2e.sshAuthKey", os.Getenv("SSH_AUTH_KEY"), "the SSH public key that provides access to deployed VMs")
}

// ClusterGenerator may be used to generate a new CAPI and infrastructure
// resource for testing.
type ClusterGenerator struct{}

// Generate returns a new CAPI and infrastructure resource.
func (c ClusterGenerator) Generate(clusterNamespace, clusterName string) (*clusterv1.Cluster, *infrav1.VSphereCluster) {

	infraCluster := &infrav1.VSphereCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&infrav1.VSphereCluster{}),
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      clusterName,
		},
		Spec: infrav1.VSphereClusterSpec{
			Server: vsphereServer,
			CloudProviderConfiguration: infrav1.CPIConfig{
				Global: infrav1.CPIGlobalConfig{
					Insecure:        true,
					SecretName:      "cloud-provider-vsphere-credentials",
					SecretNamespace: "kube-system",
				},
				Network: infrav1.CPINetworkConfig{
					Name: vsphereNetwork,
				},
				ProviderConfig: infrav1.CPIProviderConfig{
					Cloud: &infrav1.CPICloudConfig{
						ControllerImage: "gcr.io/cloud-provider-vsphere/cpi/release/manager:v1.0.0",
					},
					Storage: &infrav1.CPIStorageConfig{
						AttacherImage:       "quay.io/k8scsi/csi-attacher:v1.1.1",
						ControllerImage:     "gcr.io/cloud-provider-vsphere/csi/release/driver:v1.0.1",
						LivenessProbeImage:  "quay.io/k8scsi/livenessprobe:v1.1.0",
						MetadataSyncerImage: "gcr.io/cloud-provider-vsphere/csi/release/syncer:v1.0.1",
						NodeDriverImage:     "gcr.io/cloud-provider-vsphere/csi/release/driver:v1.0.1",
						ProvisionerImage:    "quay.io/k8scsi/csi-provisioner:v1.2.1",
						RegistrarImage:      "quay.io/k8scsi/csi-node-driver-registrar:v1.1.0",
					},
				},
				VCenter: map[string]infrav1.CPIVCenterConfig{
					vsphereServer: {
						Datacenters: vsphereDatacenter,
					},
				},
				Workspace: infrav1.CPIWorkspaceConfig{
					Datacenter:   vsphereDatacenter,
					Datastore:    vsphereDatastore,
					Folder:       vsphereFolder,
					ResourcePool: vspherePool,
					Server:       vsphereServer,
				},
			},
		},
	}

	cluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&clusterv1.Cluster{}),
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      clusterName,
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Services: &clusterv1.NetworkRanges{CIDRBlocks: []string{"100.64.0.0/13"}},
				Pods:     &clusterv1.NetworkRanges{CIDRBlocks: []string{"100.96.0.0/11"}},
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       framework.TypeToKind(infraCluster),
				Namespace:  infraCluster.GetNamespace(),
				Name:       infraCluster.GetName(),
			},
		},
	}
	return cluster, infraCluster
}

var (
	sudoAll    = "ALL=(ALL) NOPASSWD:ALL"
	passwd     = "capv"
	lockPasswd = true
)

// ControlPlaneNodeGenerator may be used to generate control plane nodes.
type ControlPlaneNodeGenerator struct{}

// Generate returns the resources required to create a machine.
func (n ControlPlaneNodeGenerator) Generate(clusterNamespace, clusterName string) framework.Node {
	generatedName := fmt.Sprintf("%s-%s", clusterName, Hash7())

	infraMachine := &infrav1.VSphereMachine{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&infrav1.VSphereMachine{}),
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      generatedName,
			Labels: map[string]string{
				clusterv1.MachineControlPlaneLabelName: "true",
				clusterv1.ClusterLabelName:             clusterName,
			},
		},
		Spec: infrav1.VSphereMachineSpec{
			VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
				Datacenter: vsphereDatacenter,
				DiskGiB:    50,
				MemoryMiB:  2048,
				Network: infrav1.NetworkSpec{
					Devices: []infrav1.NetworkDeviceSpec{
						{
							NetworkName: vsphereNetwork,
							DHCP4:       true,
						},
					},
				},
				NumCPUs:  2,
				Template: vsphereMachineTemplate,
			},
		},
	}

	bootstrapConfig := &bootstrapv1.KubeadmConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&bootstrapv1.KubeadmConfig{}),
			APIVersion: bootstrapv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      generatedName,
		},
		Spec: bootstrapv1.KubeadmConfigSpec{
			ClusterConfiguration: &v1beta1.ClusterConfiguration{
				APIServer: v1beta1.APIServer{
					ControlPlaneComponent: v1beta1.ControlPlaneComponent{
						ExtraArgs: map[string]string{
							"cloud-provider": "external",
						},
					},
				},
				ControllerManager: v1beta1.ControlPlaneComponent{
					ExtraArgs: map[string]string{
						"cloud-provider": "external",
					},
				},
			},
			InitConfiguration: &v1beta1.InitConfiguration{
				NodeRegistration: v1beta1.NodeRegistrationOptions{
					CRISocket: "/var/run/containerd/containerd.sock",
					KubeletExtraArgs: map[string]string{
						"cloud-provider": "external",
					},
					Name: "{{ ds.meta_data.hostname }}",
				},
			},
			JoinConfiguration: &v1beta1.JoinConfiguration{
				NodeRegistration: v1beta1.NodeRegistrationOptions{
					CRISocket: "/var/run/containerd/containerd.sock",
					KubeletExtraArgs: map[string]string{
						"cloud-provider": "external",
					},
					Name: "{{ ds.meta_data.hostname }}",
				},
			},
			PreKubeadmCommands: []string{
				`hostname "{{ ds.meta_data.hostname }}"`,
				`echo "::1        ipv6-localhost ipv6-loopback" >/etc/hosts`,
				`echo "127.0.0.1  localhost" >>/etc/hosts`,
				`echo "127.0.0.1  {{ ds.meta_data.hostname }}" >>/etc/hosts`,
				`echo "{{ ds.meta_data.hostname }}" >/etc/hostname`,
			},
			Users: []bootstrapv1.User{
				{
					Name:              "capv",
					SSHAuthorizedKeys: []string{sshAuthKey},
					Sudo:              &sudoAll,
					Passwd:            &passwd,
					LockPassword:      &lockPasswd,
				},
			},
		},
	}

	machine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&clusterv1.Machine{}),
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      generatedName,
			Labels: map[string]string{
				clusterv1.MachineControlPlaneLabelName: "true",
				clusterv1.ClusterLabelName:             clusterName,
			},
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					APIVersion: bootstrapv1.GroupVersion.String(),
					Kind:       framework.TypeToKind(bootstrapConfig),
					Namespace:  bootstrapConfig.GetNamespace(),
					Name:       bootstrapConfig.GetName(),
				},
			},
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       framework.TypeToKind(infraMachine),
				Namespace:  infraMachine.GetNamespace(),
				Name:       infraMachine.GetName(),
			},
			Version:     &config.KubernetesVersion,
			ClusterName: clusterName,
		},
	}
	return framework.Node{
		Machine:         machine,
		InfraMachine:    infraMachine,
		BootstrapConfig: bootstrapConfig,
	}
}

// KubeadmControlPlaneGenerator may be used to generate the resources for a
// kubeadm-based control plane.
type KubeadmControlPlaneGenerator struct{}

// Generate returns the resources required to create a kubeadm control plane.
func (g KubeadmControlPlaneGenerator) Generate(clusterNamespace, clusterName string, replicas int32) (*controlplanev1.KubeadmControlPlane, *infrav1.VSphereMachineTemplate) {
	generatedName := fmt.Sprintf("%s-%s", clusterName, Hash7())

	infraMachineTemplate := &infrav1.VSphereMachineTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&infrav1.VSphereMachineTemplate{}),
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      generatedName,
		},
		Spec: infrav1.VSphereMachineTemplateSpec{
			Template: infrav1.VSphereMachineTemplateResource{
				Spec: infrav1.VSphereMachineSpec{
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
						Datacenter: vsphereDatacenter,
						DiskGiB:    50,
						MemoryMiB:  2048,
						Network: infrav1.NetworkSpec{
							Devices: []infrav1.NetworkDeviceSpec{
								{
									NetworkName: vsphereNetwork,
									DHCP4:       true,
								},
							},
						},
						NumCPUs:  2,
						Template: vsphereMachineTemplate,
					},
				},
			},
		},
	}

	kubeadmControlPlane := &controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&controlplanev1.KubeadmControlPlane{}),
			APIVersion: controlplanev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      fmt.Sprintf("%s-kcp", clusterName),
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Replicas: &replicas,
			Version:  config.KubernetesVersion,
			InfrastructureTemplate: corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       framework.TypeToKind(infraMachineTemplate),
				Namespace:  infraMachineTemplate.GetNamespace(),
				Name:       infraMachineTemplate.GetName(),
			},
			KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
				ClusterConfiguration: &v1beta1.ClusterConfiguration{
					APIServer: v1beta1.APIServer{
						ControlPlaneComponent: v1beta1.ControlPlaneComponent{
							ExtraArgs: map[string]string{
								"cloud-provider": "external",
							},
						},
					},
					ControllerManager: v1beta1.ControlPlaneComponent{
						ExtraArgs: map[string]string{
							"cloud-provider": "external",
						},
					},
				},
				InitConfiguration: &v1beta1.InitConfiguration{
					NodeRegistration: v1beta1.NodeRegistrationOptions{
						CRISocket: "/var/run/containerd/containerd.sock",
						KubeletExtraArgs: map[string]string{
							"cloud-provider": "external",
						},
						Name: "{{ ds.meta_data.hostname }}",
					},
				},
				JoinConfiguration: &v1beta1.JoinConfiguration{
					NodeRegistration: v1beta1.NodeRegistrationOptions{
						CRISocket: "/var/run/containerd/containerd.sock",
						KubeletExtraArgs: map[string]string{
							"cloud-provider": "external",
						},
						Name: "{{ ds.meta_data.hostname }}",
					},
				},
				PreKubeadmCommands: []string{
					`hostname "{{ ds.meta_data.hostname }}"`,
					`echo "::1        ipv6-localhost ipv6-loopback" >/etc/hosts`,
					`echo "127.0.0.1  localhost" >>/etc/hosts`,
					`echo "127.0.0.1  {{ ds.meta_data.hostname }}" >>/etc/hosts`,
					`echo "{{ ds.meta_data.hostname }}" >/etc/hostname`,
				},
				Users: []bootstrapv1.User{
					{
						Name:              "capv",
						SSHAuthorizedKeys: []string{sshAuthKey},
						Sudo:              &sudoAll,
						Passwd:            &passwd,
						LockPassword:      &lockPasswd,
					},
				},
			},
		},
	}

	return kubeadmControlPlane, infraMachineTemplate
}

// MachineDeploymentGenerator may be used to generate the resources
// required to create a machine deployment for testing.
type MachineDeploymentGenerator struct{}

// Generate returns the resources required to create a machine deployment.
func (n MachineDeploymentGenerator) Generate(clusterNamespace, clusterName string, replicas int32) framework.MachineDeployment {
	if replicas == 0 {
		return framework.MachineDeployment{}
	}
	generatedName := clusterName

	infraMachineTemplate := &infrav1.VSphereMachineTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&infrav1.VSphereMachineTemplate{}),
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      generatedName,
		},
		Spec: infrav1.VSphereMachineTemplateSpec{
			Template: infrav1.VSphereMachineTemplateResource{
				Spec: infrav1.VSphereMachineSpec{
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
						Datacenter: vsphereDatacenter,
						DiskGiB:    50,
						MemoryMiB:  2048,
						Network: infrav1.NetworkSpec{
							Devices: []infrav1.NetworkDeviceSpec{
								{
									NetworkName: vsphereNetwork,
									DHCP4:       true,
								},
							},
						},
						NumCPUs:  2,
						Template: vsphereMachineTemplate,
					},
				},
			},
		},
	}

	bootstrapConfigTemplate := &bootstrapv1.KubeadmConfigTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&bootstrapv1.KubeadmConfigTemplate{}),
			APIVersion: bootstrapv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      generatedName,
		},
		Spec: bootstrapv1.KubeadmConfigTemplateSpec{
			Template: bootstrapv1.KubeadmConfigTemplateResource{
				Spec: bootstrapv1.KubeadmConfigSpec{
					JoinConfiguration: &v1beta1.JoinConfiguration{
						NodeRegistration: v1beta1.NodeRegistrationOptions{
							CRISocket: "/var/run/containerd/containerd.sock",
							KubeletExtraArgs: map[string]string{
								"cloud-provider": "external",
							},
							Name: "{{ ds.meta_data.hostname }}",
						},
					},
					PreKubeadmCommands: []string{
						`hostname "{{ ds.meta_data.hostname }}"`,
						`echo "::1        ipv6-localhost ipv6-loopback" >/etc/hosts`,
						`echo "127.0.0.1  localhost" >>/etc/hosts`,
						`echo "127.0.0.1  {{ ds.meta_data.hostname }}" >>/etc/hosts`,
						`echo "{{ ds.meta_data.hostname }}" >/etc/hostname`,
					},
					Users: []bootstrapv1.User{
						{
							Name:              "capv",
							SSHAuthorizedKeys: []string{sshAuthKey},
							Sudo:              &sudoAll,
							Passwd:            &passwd,
							LockPassword:      &lockPasswd,
						},
					},
				},
			},
		},
	}

	machineDeployment := &clusterv1.MachineDeployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&clusterv1.MachineDeployment{}),
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      generatedName,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
		},
		Spec: clusterv1.MachineDeploymentSpec{
			ClusterName: clusterName,
			Replicas:    &replicas,
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					clusterv1.ClusterLabelName: clusterName,
				},
			},
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.ClusterLabelName: clusterName,
					},
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: clusterName,
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: bootstrapv1.GroupVersion.String(),
							Kind:       framework.TypeToKind(bootstrapConfigTemplate),
							Namespace:  bootstrapConfigTemplate.GetNamespace(),
							Name:       bootstrapConfigTemplate.GetName(),
						},
					},
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       framework.TypeToKind(infraMachineTemplate),
						Namespace:  infraMachineTemplate.GetNamespace(),
						Name:       infraMachineTemplate.GetName(),
					},
					Version: &config.KubernetesVersion,
				},
			},
		},
	}

	return framework.MachineDeployment{
		MachineDeployment:       machineDeployment,
		BootstrapConfigTemplate: bootstrapConfigTemplate,
		InfraMachineTemplate:    infraMachineTemplate,
	}
}

// LoadBalancerGenerator generates a load balancer resource.
type LoadBalancerGenerator interface {
	// Generate returns a load balancer resource.
	Generate(clusterNamespace, clusterName string) runtime.Object
}

// HAProxyLoadBalancerGenerator may be used to generate a new load balancer
// resource for testing.
type HAProxyLoadBalancerGenerator struct{}

// Generate returns the resources required to create a load balancer.
func (n HAProxyLoadBalancerGenerator) Generate(clusterNamespace, clusterName string) runtime.Object {
	return &infrav1.HAProxyLoadBalancer{
		TypeMeta: metav1.TypeMeta{
			Kind:       framework.TypeToKind(&infrav1.HAProxyLoadBalancer{}),
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      clusterName,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
		},
		Spec: infrav1.HAProxyLoadBalancerSpec{
			VirtualMachineConfiguration: infrav1.VirtualMachineCloneSpec{
				Datacenter:   vsphereDatacenter,
				Datastore:    vsphereDatastore,
				Folder:       vsphereFolder,
				ResourcePool: vspherePool,
				Server:       vsphereServer,
				DiskGiB:      50,
				MemoryMiB:    2048,
				Network: infrav1.NetworkSpec{
					Devices: []infrav1.NetworkDeviceSpec{
						{
							NetworkName: vsphereNetwork,
							DHCP4:       true,
						},
					},
				},
				NumCPUs:  2,
				Template: vsphereHAProxyTemplate,
			},
			User: &infrav1.SSHUser{
				Name: "capv",
				AuthorizedKeys: []string{
					sshAuthKey,
				},
			},
		},
	}
}
