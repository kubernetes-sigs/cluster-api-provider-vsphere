/*
Copyright 2023 The Kubernetes Authors.

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

// Package kubevip exposes functions to add kubevip to templates.
package kubevip

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
)

var (
	hostPathTypeFile = corev1.HostPathFile

	kubeVipPodSpec = &corev1.Pod{
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
					Image:           "ghcr.io/kube-vip/kube-vip:v0.6.3",
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
						{
							// Enables kube-vip to watch Services of type LoadBalancer
							Name:  "svc_enable",
							Value: "true",
						},
						{
							// Enables a leadership Election for each Service, allowing them to be distributed
							Name:  "svc_election",
							Value: "true",
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"NET_ADMIN",
								"NET_RAW",
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/etc/kubernetes/admin.conf",
							Name:      "kubeconfig",
						},
						// This mount is part of the workaround for https://github.com/kube-vip/kube-vip/issues/692
						{
							MountPath: "/etc/hosts",
							Name:      "etchosts",
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
							Type: &hostPathTypeFile,
						},
					},
				},
				// This mount is part of the workaround for https://github.com/kube-vip/kube-vip/issues/692
				{
					Name: "etchosts",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/kube-vip.hosts",
							Type: &hostPathTypeFile,
						},
					},
				},
			},
		},
	}
)

func PatchControlPlane(cp *controlplanev1.KubeadmControlPlane) {
	cp.Spec.KubeadmConfigSpec.Files = append(cp.Spec.KubeadmConfigSpec.Files, newKubeVIPFiles()...)

	// This two commands are part of the workaround for https://github.com/kube-vip/kube-vip/issues/684
	cp.Spec.KubeadmConfigSpec.PreKubeadmCommands = append(
		cp.Spec.KubeadmConfigSpec.PreKubeadmCommands,
		"/etc/kube-vip-prepare.sh",
	)
}

// kubeVIPPodYaml converts the KubeVip pod spec to a `printable` yaml
// this is needed for the file contents of KubeadmConfig.
func kubeVIPPodYaml() string {
	podYaml := util.GenerateObjectYAML(kubeVipPodSpec, []util.Replacement{})
	return podYaml
}

func kubeVIPPod() string {
	podBytes, err := yaml.Marshal(kubeVipPodSpec)
	if err != nil {
		panic(err)
	}
	return string(podBytes)
}
