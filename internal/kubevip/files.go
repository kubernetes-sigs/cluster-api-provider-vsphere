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

// Package kubevip provides the files required to run kube-vip in a cluster.
package kubevip

import (
	_ "embed"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/yaml"
)

var (
	// This file is part of the workaround for https://github.com/kube-vip/kube-vip/issues/684

	//go:embed kube-vip-prepare.sh
	kubeVipPrepare string

	// kubeVipPodRaw yaml is generated via:
	//   docker run --network host --rm ghcr.io/kube-vip/kube-vip:${TAG} manifest pod --controlplane --address '${CONTROL_PLANE_ENDPOINT_IP}' --interface '${VIP_NETWORK_INTERFACE:=""}' --arp --leaderElection --leaseDuration 15 --leaseRenewDuration 10 --leaseRetry 2 --services --servicesElection > packaging/flavorgen/flavors/kubevip/kube-vip.yaml
	//go:embed kube-vip.yaml
	kubeVipPodRaw string
)

// Files returns the files required for a control plane node to run kube-vip.
func Files() []bootstrapv1.File {
	return []bootstrapv1.File{
		{
			Owner:       "root:root",
			Path:        "/etc/kubernetes/manifests/kube-vip.yaml",
			Content:     PodYAML(),
			Permissions: "0644",
		},
		// This file is part of the workaround for https://github.com/kube-vip/kube-vip/issues/692
		{
			Owner:       "root:root",
			Path:        "/etc/kube-vip.hosts",
			Permissions: "0644",
			Content:     "127.0.0.1 localhost kubernetes",
		},
		// This file is part of the workaround for https://github.com/kube-vip/kube-vip/issues/684
		{
			Owner:       "root:root",
			Path:        "/etc/pre-kubeadm-commands/50-kube-vip-prepare.sh",
			Permissions: "0700",
			Content:     kubeVipPrepare,
		},
	}
}

// PodYAML returns the static pod manifest required to run kube-vip.
func PodYAML() string {
	pod := &corev1.Pod{}

	if err := yaml.Unmarshal([]byte(kubeVipPodRaw), pod); err != nil {
		panic(err)
	}

	if len(pod.Spec.Containers) != 1 {
		panic(fmt.Sprintf("Expected the kube-vip static pod manifest to have one container but got %d", len(pod.Spec.Containers)))
	}

	// Set IfNotPresent to prevent unnecessary image pulls
	pod.Spec.Containers[0].ImagePullPolicy = corev1.PullIfNotPresent

	// Apply workaround for https://github.com/kube-vip/kube-vip/issues/692
	// which is not using HostAliases, but a prebuilt /etc/hosts file instead.
	pod.Spec.HostAliases = nil
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "etchosts",
			MountPath: "/etc/hosts",
		},
	)
	pod.Spec.Volumes = append(pod.Spec.Volumes,
		corev1.Volume{
			Name: "etchosts",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/kube-vip.hosts",
					Type: ptr.To(corev1.HostPathFile),
				},
			},
		},
	)

	out, err := yaml.Marshal(pod)
	if err != nil {
		panic(err)
	}

	return string(out)
}
