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
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"

	"sigs.k8s.io/cluster-api-provider-vsphere/internal/kubevip"
)

// PatchControlPlane adds kube-vip to a KubeadmControlPlane object.
func PatchControlPlane(cp *controlplanev1.KubeadmControlPlane) {
	cp.Spec.KubeadmConfigSpec.Files = append(cp.Spec.KubeadmConfigSpec.Files, kubevip.Files()...)
}
