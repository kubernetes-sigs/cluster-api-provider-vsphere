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
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/kubevip"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
)

const (
	AdditionalIgnitionConfig = `storage:
  files:
  - path: /opt/set-hostname
    filesystem: root
    mode: 0744
    contents:
      inline: |
        #!/bin/sh
        set -x
        echo "$${COREOS_CUSTOM_HOSTNAME}" > /etc/hostname
        hostname "$${COREOS_CUSTOM_HOSTNAME}"
        echo "::1         ipv6-localhost ipv6-loopback" >/etc/hosts
        echo "127.0.0.1   localhost" >>/etc/hosts
        echo "127.0.0.1   $${COREOS_CUSTOM_HOSTNAME}" >>/etc/hosts
systemd:
  units:
  - name: coreos-metadata.service
    contents: |
      [Unit]
      Description=VMware metadata agent
      After=nss-lookup.target
      After=network-online.target
      Wants=network-online.target
      [Service]
      Type=oneshot
      Restart=on-failure
      RemainAfterExit=yes
      Environment=OUTPUT=/run/metadata/coreos
      ExecStart=/usr/bin/mkdir --parent /run/metadata
      ExecStart=/usr/bin/bash -cv 'echo "COREOS_CUSTOM_HOSTNAME=$("$(find /usr/bin /usr/share/oem -name vmtoolsd -type f -executable 2>/dev/null | head -n 1)" --cmd "info-get guestinfo.metadata" | base64 -d | grep local-hostname | awk {\'print $2\'} | tr -d \'"\')" > $${OUTPUT}'
  - name: set-hostname.service
    enabled: true
    contents: |
      [Unit]
      Description=Set the hostname for this machine
      Requires=coreos-metadata.service
      After=coreos-metadata.service
      [Service]
      Type=oneshot
      EnvironmentFile=/run/metadata/coreos
      ExecStart=/opt/set-hostname
      [Install]
      WantedBy=multi-user.target
  - name: kubeadm.service
    enabled: true
    dropins:
    - name: 10-flatcar.conf
      contents: |
        [Unit]
        # kubeadm must run after coreos-metadata populated /run/metadata directory.
        Requires=coreos-metadata.service
        After=coreos-metadata.service
        # kubeadm must run after containerd - see https://github.com/kubernetes-sigs/image-builder/issues/939.
        After=containerd.service
        [Service]
        # Make metadata environment variables available for pre-kubeadm commands.
        EnvironmentFile=/run/metadata/*
        # Log to file
        StandardOutput=append:/var/log/kubeadm-service.log
        StandardError=inherit`
)

func newClusterTopologyCluster(supervisorMode bool) (clusterv1.Cluster, error) {
	variables, err := clusterTopologyVariables(supervisorMode)
	if err != nil {
		return clusterv1.Cluster{}, errors.Wrap(err, "failed to create ClusterTopologyCluster template")
	}
	return clusterv1.Cluster{
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
			Topology: &clusterv1.Topology{
				Class:   env.ClusterClassNameVar,
				Version: env.KubernetesVersionVar,
				ControlPlane: clusterv1.ControlPlaneTopology{
					Replicas: ptr.To[int32](1),
				},
				Workers: &clusterv1.WorkersTopology{
					MachineDeployments: []clusterv1.MachineDeploymentTopology{
						{
							Class:    fmt.Sprintf("%s-worker", env.ClusterClassNameVar),
							Name:     "md-0",
							Replicas: ptr.To[int32](3),
						},
					},
				},
				Variables: variables,
			},
		},
	}, nil
}

func clusterTopologyVariables(supervisorMode bool) ([]clusterv1.ClusterVariable, error) {
	sshKey, err := json.Marshal(env.VSphereSSHAuthorizedKeysVar)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to json-encode variable VSphereSSHAuthorizedKeysVar: %q", env.VSphereSSHAuthorizedKeysVar)
	}
	controlPlaneIP, err := json.Marshal(env.ControlPlaneEndpointHostVar)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to json-encode variable ControlPlaneEndpointHostVar: %q", env.ControlPlaneEndpointHostVar)
	}
	controlPlanePort, err := json.Marshal(env.ControlPlaneEndpointPortVar)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to json-encode variable ControlPlaneEndpointPortVar: %q", env.ControlPlaneEndpointPortVar)
	}
	kubeVipVariable, err := kubevip.TopologyVariable()
	if err != nil {
		return nil, err
	}

	variables := []clusterv1.ClusterVariable{
		{
			Name: "sshKey",
			Value: apiextensionsv1.JSON{
				Raw: sshKey,
			},
		},
		*kubeVipVariable,
		{
			Name: "controlPlaneIpAddr",
			Value: apiextensionsv1.JSON{
				Raw: controlPlaneIP,
			},
		},
		{
			Name: "controlPlanePort",
			Value: apiextensionsv1.JSON{
				Raw: controlPlanePort,
			},
		},
	}

	if !supervisorMode {
		infraServerValue, err := getInfraServerValue()
		if err != nil {
			return nil, err
		}
		secretName, err := json.Marshal(env.ClusterNameVar)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to json-encode variable ClusterNameVar: %q", env.ClusterNameVar)
		}

		varForNoneSupervisorMode := []clusterv1.ClusterVariable{
			{
				Name: "infraServer",
				Value: apiextensionsv1.JSON{
					Raw: infraServerValue,
				},
			},
			{
				Name: "credsSecretName",
				Value: apiextensionsv1.JSON{
					Raw: secretName,
				},
			},
		}

		variables = append(variables, varForNoneSupervisorMode...)
	}
	return variables, nil
}

func getInfraServerValue() ([]byte, error) {
	byteArr, err := json.Marshal(map[string]string{
		"url":        env.VSphereServerVar,
		"thumbprint": env.VSphereThumbprint,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to json-encode, VSphereServerVar: %s, VSphereThumbprint: %s",
			env.VSphereServerVar, env.VSphereThumbprint)
	}
	return byteArr, nil
}

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
				Host: env.ControlPlaneEndpointHostVar,
				Port: 6443,
			},
		},
	}
}

func newVMWareCluster() vmwarev1.VSphereCluster {
	return vmwarev1.VSphereCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: vmwarev1.GroupVersion.String(),
			Kind:       util.TypeToKind(&vmwarev1.VSphereCluster{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.ClusterNameVar,
			Namespace: env.NamespaceVar,
		},
		Spec: vmwarev1.VSphereClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: env.ControlPlaneEndpointHostVar,
				Port: 6443,
			},
		},
	}
}

func newCluster(vsphereCluster client.Object, controlPlane *controlplanev1.KubeadmControlPlane) clusterv1.Cluster {
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
				APIVersion: vsphereCluster.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       vsphereCluster.GetObjectKind().GroupVersionKind().Kind,
				Name:       vsphereCluster.GetName(),
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

func newVSphereMachineTemplate(templateName string) infrav1.VSphereMachineTemplate {
	return infrav1.VSphereMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      templateName,
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
		PowerOffMode:            infrav1.VirtualMachinePowerOpModeTrySoft,
	}
}

func newVMWareMachineTemplate(templateName string) vmwarev1.VSphereMachineTemplate {
	return vmwarev1.VSphereMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      templateName,
			Namespace: env.NamespaceVar,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: vmwarev1.GroupVersion.String(),
			Kind:       util.TypeToKind(&vmwarev1.VSphereMachineTemplate{}),
		},
		Spec: vmwarev1.VSphereMachineTemplateSpec{
			Template: vmwarev1.VSphereMachineTemplateResource{
				Spec: vmwarev1.VSphereMachineSpec{
					ImageName:    env.VSphereMachineClassImageVar,
					ClassName:    env.VSphereMachineClassVar,
					StorageClass: env.VSphereMachineStorageClassVar,
					PowerOffMode: env.VSphereMachinePowerOffModeVar,
				},
			},
		},
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
		OS:                infrav1.Linux,
	}
}

func newNodeIPAMVSphereMachineTemplate(templateName string) infrav1.VSphereMachineTemplate {
	return infrav1.VSphereMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      templateName,
			Namespace: env.NamespaceVar,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       util.TypeToKind(&infrav1.VSphereMachineTemplate{}),
		},
		Spec: infrav1.VSphereMachineTemplateSpec{
			Template: infrav1.VSphereMachineTemplateResource{
				Spec: nodeIPAMVirtualMachineSpec(),
			},
		},
	}
}

func nodeIPAMVirtualMachineSpec() infrav1.VSphereMachineSpec {
	return infrav1.VSphereMachineSpec{
		VirtualMachineCloneSpec: nodeIPAMVirtualMachineCloneSpec(),
		PowerOffMode:            infrav1.VirtualMachinePowerOpModeTrySoft,
	}
}

func nodeIPAMVirtualMachineCloneSpec() infrav1.VirtualMachineCloneSpec {
	return infrav1.VirtualMachineCloneSpec{
		Datacenter: env.VSphereDataCenterVar,
		Network: infrav1.NetworkSpec{
			Devices: []infrav1.NetworkDeviceSpec{
				{
					NetworkName: env.VSphereNetworkVar,
					DHCP4:       false,
					DHCP6:       false,
					AddressesFromPools: []corev1.TypedLocalObjectReference{
						{
							APIGroup: ptr.To(env.NodeIPAMPoolAPIGroup),
							Kind:     env.NodeIPAMPoolKind,
							Name:     env.NodeIPAMPoolName,
						},
					},
					Nameservers: []string{
						env.Nameserver,
					},
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
		OS:                infrav1.Linux,
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
		Users:              defaultUsers(),
		PreKubeadmCommands: defaultPreKubeadmCommands(),
		Files:              files,
	}
}

func ignitionKubeadmInitSpec(files []bootstrapv1.File) bootstrapv1.KubeadmConfigSpec {
	nro := defaultNodeRegistrationOptions()
	nro.Name = "$${COREOS_CUSTOM_HOSTNAME}"

	return bootstrapv1.KubeadmConfigSpec{
		Format: bootstrapv1.Ignition,
		Ignition: &bootstrapv1.IgnitionSpec{
			ContainerLinuxConfig: &bootstrapv1.ContainerLinuxConfig{
				AdditionalConfig: AdditionalIgnitionConfig,
			},
		},
		InitConfiguration: &bootstrapv1.InitConfiguration{
			NodeRegistration: nro,
		},
		JoinConfiguration: &bootstrapv1.JoinConfiguration{
			NodeRegistration: nro,
		},
		ClusterConfiguration: &bootstrapv1.ClusterConfiguration{
			APIServer: bootstrapv1.APIServer{
				ControlPlaneComponent: defaultControlPlaneComponent(),
			},
			ControllerManager: defaultControlPlaneComponent(),
		},
		Users:              flatcarUsers(),
		PreKubeadmCommands: flatcarPreKubeadmCommands(),
		// UseExperimentalRetryJoin isn't supported with Ignition bootstrap.
		UseExperimentalRetryJoin: false,
		Files:                    files,
	}
}

func newKubeadmConfigTemplate(templateName string, addUsers bool) bootstrapv1.KubeadmConfigTemplate {
	template := bootstrapv1.KubeadmConfigTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      templateName,
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
					PreKubeadmCommands: defaultPreKubeadmCommands(),
				},
			},
		},
	}
	if addUsers {
		template.Spec.Template.Spec.Users = defaultUsers()
	}
	return template
}

func newIgnitionKubeadmConfigTemplate() bootstrapv1.KubeadmConfigTemplate {
	nro := defaultNodeRegistrationOptions()
	nro.Name = "$${COREOS_CUSTOM_HOSTNAME}"

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
					Format: bootstrapv1.Ignition,
					Ignition: &bootstrapv1.IgnitionSpec{
						ContainerLinuxConfig: &bootstrapv1.ContainerLinuxConfig{
							AdditionalConfig: AdditionalIgnitionConfig,
						},
					},
					JoinConfiguration: &bootstrapv1.JoinConfiguration{
						NodeRegistration: nro,
					},
					Users:              flatcarUsers(),
					PreKubeadmCommands: flatcarPreKubeadmCommands(),
				},
			},
		},
	}
}

func defaultNodeRegistrationOptions() bootstrapv1.NodeRegistrationOptions {
	return bootstrapv1.NodeRegistrationOptions{
		Name:             "{{ local_hostname }}",
		CRISocket:        "/var/run/containerd/containerd.sock",
		KubeletExtraArgs: defaultExtraArgs(),
	}
}

func defaultUsers() []bootstrapv1.User {
	return []bootstrapv1.User{
		{
			Name: "capv",
			Sudo: ptr.To("ALL=(ALL) NOPASSWD:ALL"),
			SSHAuthorizedKeys: []string{
				env.VSphereSSHAuthorizedKeysVar,
			},
		},
	}
}

func flatcarUsers() []bootstrapv1.User {
	return []bootstrapv1.User{
		{
			Name: "core",
			Sudo: ptr.To("ALL=(ALL) NOPASSWD:ALL"),
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
		"hostnamectl set-hostname \"{{ ds.meta_data.hostname }}\"",
		"echo \"::1         ipv6-localhost ipv6-loopback localhost6 localhost6.localdomain6\" >/etc/hosts",
		"echo \"127.0.0.1   {{ ds.meta_data.hostname }} {{ local_hostname }} localhost localhost.localdomain localhost4 localhost4.localdomain4\" >>/etc/hosts",
		// Ensure the directory exists so the find does not fail if no files got created.
		"mkdir -p /etc/pre-kubeadm-commands",
		"for script in $(find /etc/pre-kubeadm-commands/ -name '*.sh' -type f | sort); do echo \"Running script $script\"; \"$script\"; done",
	}
}

func flatcarPreKubeadmCommands() []string {
	return []string{
		"envsubst < /etc/kubeadm.yml > /etc/kubeadm.yml.tmp",
		"mv /etc/kubeadm.yml.tmp /etc/kubeadm.yml",
		// Ensure the directory exists so the find does not fail if no files got created.
		"mkdir -p /etc/pre-kubeadm-commands",
		"for script in $(find /etc/pre-kubeadm-commands/ -name '*.sh' -type f | sort); do echo \"Running script $script\"; \"$script\"; done",
	}
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
			Kind:       util.TypeToKind(&corev1.Secret{}),
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

func newMachineDeployment(cluster clusterv1.Cluster, machineTemplate client.Object, bootstrapTemplate bootstrapv1.KubeadmConfigTemplate) clusterv1.MachineDeployment {
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
			Replicas:    ptr.To[int32](555),
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels: clusterLabels(),
				},
				Spec: clusterv1.MachineSpec{
					Version:     ptr.To(env.KubernetesVersionVar),
					ClusterName: cluster.Name,
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: bootstrapTemplate.GroupVersionKind().GroupVersion().String(),
							Kind:       bootstrapTemplate.Kind,
							Name:       bootstrapTemplate.Name,
						},
					},
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: machineTemplate.GetObjectKind().GroupVersionKind().GroupVersion().String(),
						Kind:       machineTemplate.GetObjectKind().GroupVersionKind().Kind,
						Name:       machineTemplate.GetName(),
					},
				},
			},
		},
	}
}

func newKubeadmControlplane(infraTemplate client.Object, files []bootstrapv1.File) controlplanev1.KubeadmControlPlane {
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
			Version: env.KubernetesVersionVar,
			MachineTemplate: controlplanev1.KubeadmControlPlaneMachineTemplate{
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: infraTemplate.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:       infraTemplate.GetObjectKind().GroupVersionKind().Kind,
					Name:       infraTemplate.GetName(),
				},
			},
			KubeadmConfigSpec: defaultKubeadmInitSpec(files),
		},
	}
}

func newIgnitionKubeadmControlplane(infraTemplate infrav1.VSphereMachineTemplate, files []bootstrapv1.File) controlplanev1.KubeadmControlPlane {
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
			Version: env.KubernetesVersionVar,
			MachineTemplate: controlplanev1.KubeadmControlPlaneMachineTemplate{
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: infraTemplate.GroupVersionKind().GroupVersion().String(),
					Kind:       infraTemplate.Kind,
					Name:       infraTemplate.Name,
				},
			},
			KubeadmConfigSpec: ignitionKubeadmInitSpec(files),
		},
	}
}
