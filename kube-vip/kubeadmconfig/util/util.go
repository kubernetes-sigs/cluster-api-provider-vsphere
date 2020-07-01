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

package util

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
)

const (
	defaultNetworkInterface = "eth0"
)

// GetKubeadmControlPlane returns the kubeadmControlPlane that owns a given kubeadmConfig
func GetKubeadmControlPlane(ctx context.Context, client ctrlclient.Client, kubeadmConfig *bootstrapv1.KubeadmConfig) (*controlplanev1.KubeadmControlPlane, error) {
	kcp := &controlplanev1.KubeadmControlPlane{}
	for _, ownerRef := range kubeadmConfig.OwnerReferences {
		if ownerRef.Kind == "KubeadmControlPlane" {
			kcptKey := ctrlclient.ObjectKey{Name: ownerRef.Name, Namespace: kubeadmConfig.Namespace}
			if err := client.Get(ctx, kcptKey, kcp); err != nil {
				return nil, err
			}
			return kcp, nil
		}
	}
	return nil, nil
}

// GetKubeVIPPod returns the kube-vip pod and its index from the kubeadmConfig list of files
func GetKubeVIPPod(kubeadmConfig *bootstrapv1.KubeadmConfig) (*v1.Pod, int, error) {
	vipPod := &v1.Pod{}
	for i, file := range kubeadmConfig.Spec.Files {
		if file.Path == "/etc/kubernetes/manifests/kube-vip.yaml" {
			if err := yaml.Unmarshal([]byte(file.Content), vipPod); err != nil {
				return nil, 0, err
			}
			return vipPod, i, nil
		}
	}
	return nil, 0, nil
}

// KubeVIPEnvs returns the list of environment variables for the kube-vip container
func KubeVIPEnvs(containers []v1.Container, networkInterface string) ([]v1.EnvVar, int) {
	var envs []v1.EnvVar
	index := -1
	for i, container := range containers {
		if container.Name == "kube-vip" {
			envs = container.Env
			index = i
		}
	}

	var found bool
	for _, env := range envs {
		if env.Name == "vip_interface" {
			found = true
			break
		}
	}

	if !found {
		envs = append(envs, v1.EnvVar{
			Name:  "vip_interface",
			Value: networkInterface,
		})
	}

	return envs, index
}

// NetworkInterface returns the network interface to be used by kube-vip
func NetworkInterface(ctx context.Context, client ctrlclient.Client, kcp *controlplanev1.KubeadmControlPlane) (string, error) {
	vsphereMachineTemplate := &infrav1.VSphereMachineTemplate{}
	infraKey := ctrlclient.ObjectKey{Name: kcp.Spec.InfrastructureTemplate.Name, Namespace: kcp.Namespace}
	if err := client.Get(ctx, infraKey, vsphereMachineTemplate); err != nil {
		return "", err
	}
	devices := vsphereMachineTemplate.Spec.Template.Spec.Network.Devices
	if len(devices) > 0 && devices[0].DeviceName != "" {
		return devices[0].DeviceName, nil
	}
	return defaultNetworkInterface, nil
}

// MutateKubeadmConfig mutates a given kubeadmConfig with the updated kube-vip pod
func MutateKubeadmConfig(kubeadmConfig *bootstrapv1.KubeadmConfig, index int, kubeVIP *v1.Pod) ([]byte, error) {
	vipPodBytes, err := yaml.Marshal(kubeVIP)
	if err != nil {
		return []byte{}, err
	}
	kubeadmConfig.Spec.Files[index].Content = string(vipPodBytes)

	kubeadmconfigBytes, err := json.Marshal(kubeadmConfig)
	if err != nil {
		return []byte{}, err
	}
	return kubeadmconfigBytes, nil
}
