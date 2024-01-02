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

package kubevip

import (
	_ "embed"

	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

var (
	// This two files are part of the workaround for https://github.com/kube-vip/kube-vip/issues/684

	//go:embed kube-vip-prepare.sh
	kubeVipPrepare string
)

func newKubeVIPFiles() []bootstrapv1.File {
	return []bootstrapv1.File{
		{
			Owner:       "root:root",
			Path:        "/etc/kubernetes/manifests/kube-vip.yaml",
			Content:     kubeVIPPod(),
			Permissions: "0644",
		},
		// This file is part of the workaround for https://github.com/kube-vip/kube-vip/issues/692
		{
			Owner:       "root:root",
			Path:        "/etc/kube-vip.hosts",
			Permissions: "0644",
			Content:     "127.0.0.1 localhost kubernetes",
		},
		// This two files are part of the workaround for https://github.com/kube-vip/kube-vip/issues/684
		{
			Owner:       "root:root",
			Path:        "/etc/kube-vip-prepare.sh",
			Permissions: "0700",
			Content:     kubeVipPrepare,
		},
	}
}
