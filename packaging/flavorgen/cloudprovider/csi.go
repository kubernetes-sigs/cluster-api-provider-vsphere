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

// Package cloudprovider contains tools to generate CSI and CPI manifests.
package cloudprovider

import (
	"embed"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: the CSI manifests are derived from https://github.com/kubernetes-sigs/vsphere-csi-driver/tree/master/manifests/vanilla

var (
	// CSIKustomizationTemplates contains the kustomization templates for CSI driver
	//go:embed csi/kustomization.yaml csi/namespace.yaml csi/vsphere-csi-driver.yaml
	CSIKustomizationTemplates embed.FS
)

const (
	CSINamespace = "vmware-system-csi"
)

func CSICloudConfigSecret(data string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vsphere-config-secret",
			Namespace: CSINamespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"csi-vsphere.conf": data,
		},
	}
}
