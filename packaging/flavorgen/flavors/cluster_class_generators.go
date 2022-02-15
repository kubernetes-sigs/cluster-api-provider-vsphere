/*
Copyright 2022 The Kubernetes Authors.

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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/yaml"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
)

func newClusterClass() clusterv1.ClusterClass {
	return clusterv1.ClusterClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       util.TypeToKind(&clusterv1.ClusterClass{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: env.ClusterClassNameVar,
		},
		Spec: clusterv1.ClusterClassSpec{
			Infrastructure: clusterv1.LocalObjectTemplate{
				Ref: &corev1.ObjectReference{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       util.TypeToKind(&infrav1.VSphereClusterTemplate{}),
					Namespace:  env.NamespaceVar,
					Name:       fmt.Sprintf("%s-infra-cluster", env.ClusterClassNameVar),
				},
			},
			ControlPlane: clusterv1.ControlPlaneClass{
				LocalObjectTemplate: clusterv1.LocalObjectTemplate{
					Ref: &corev1.ObjectReference{
						Kind:       util.TypeToKind(&controlplanev1.KubeadmControlPlaneTemplate{}),
						Namespace:  env.NamespaceVar,
						Name:       fmt.Sprintf("%s-kcp", env.ClusterClassNameVar),
						APIVersion: controlplanev1.GroupVersion.String(),
					},
				},
				MachineInfrastructure: &clusterv1.LocalObjectTemplate{
					Ref: &corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       util.TypeToKind(&infrav1.VSphereMachineTemplate{}),
						Namespace:  env.NamespaceVar,
						Name:       fmt.Sprintf("%s-template", env.ClusterClassNameVar),
					},
				},
			},
			Workers: clusterv1.WorkersClass{
				MachineDeployments: []clusterv1.MachineDeploymentClass{
					{
						Class: fmt.Sprintf("%s-worker", env.ClusterClassNameVar),
						Template: clusterv1.MachineDeploymentClassTemplate{
							Bootstrap: clusterv1.LocalObjectTemplate{
								Ref: &corev1.ObjectReference{
									APIVersion: bootstrapv1.GroupVersion.String(),
									Kind:       util.TypeToKind(&bootstrapv1.KubeadmConfigTemplate{}),
									Namespace:  env.NamespaceVar,
									Name:       fmt.Sprintf("%s-worker-template", env.ClusterClassNameVar),
								},
							},
							Infrastructure: clusterv1.LocalObjectTemplate{
								Ref: &corev1.ObjectReference{
									Kind:       util.TypeToKind(&infrav1.VSphereMachineTemplate{}),
									Namespace:  env.NamespaceVar,
									Name:       fmt.Sprintf("%s-worker-template", env.ClusterClassNameVar),
									APIVersion: infrav1.GroupVersion.String(),
								},
							},
						},
					},
				},
			},
			Variables: getClusterClassVariables(),
			Patches:   getClusterClassPatches(),
		},
	}
}

func newVSphereClusterTemplate() infrav1.VSphereClusterTemplate {
	return infrav1.VSphereClusterTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       util.TypeToKind(&infrav1.VSphereClusterTemplate{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-infra-cluster", env.ClusterClassNameVar),
			Namespace: env.NamespaceVar,
		},
		Spec: infrav1.VSphereClusterTemplateSpec{},
	}
}

func newKubeadmControlPlaneTemplate(templateName string) controlplanev1.KubeadmControlPlaneTemplate {
	files := []bootstrapv1.File{
		{
			Owner: "root:root",
			Path:  "/etc/kubernetes/manifests/kube-vip.yaml",
		},
	}
	return controlplanev1.KubeadmControlPlaneTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       util.TypeToKind(&controlplanev1.KubeadmControlPlaneTemplate{}),
			APIVersion: controlplanev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      templateName,
			Namespace: env.NamespaceVar,
		},
		Spec: controlplanev1.KubeadmControlPlaneTemplateSpec{
			Template: controlplanev1.KubeadmControlPlaneTemplateResource{
				Spec: controlplanev1.KubeadmControlPlaneTemplateResourceSpec{
					KubeadmConfigSpec: defaultKubeadmInitSpec(files),
				},
			},
		},
	}
}

func getClusterClassPatches() []clusterv1.ClusterClassPatch {
	return []clusterv1.ClusterClassPatch{
		{
			Name:      "enableSSHIntoNodes",
			EnabledIf: pointer.StringPtr("{{ if .sshKey }}true{{end}}"),
			Definitions: []clusterv1.PatchDefinition{
				{
					Selector: clusterv1.PatchSelector{
						APIVersion: controlplanev1.GroupVersion.String(),
						Kind:       util.TypeToKind(&controlplanev1.KubeadmControlPlaneTemplate{}),
						MatchResources: clusterv1.PatchSelectorMatch{
							ControlPlane: true,
						},
					},
					JSONPatches: []clusterv1.JSONPatch{
						{
							Op:    "add",
							Path:  "/spec/template/spec/kubeadmConfigSpec/users",
							Value: nil,
							ValueFrom: &clusterv1.JSONPatchValue{
								Template: getEnableSSHIntoNodesTemplate(),
							},
						},
					},
				},
				{
					Selector: clusterv1.PatchSelector{
						APIVersion: bootstrapv1.GroupVersion.String(),
						Kind:       util.TypeToKind(&bootstrapv1.KubeadmConfigTemplate{}),
						MatchResources: clusterv1.PatchSelectorMatch{
							MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
								Names: []string{fmt.Sprintf("%s-worker", env.ClusterClassNameVar)},
							},
						},
					},
					JSONPatches: []clusterv1.JSONPatch{
						{
							Op:   "add",
							Path: "/spec/template/spec/users",
							ValueFrom: &clusterv1.JSONPatchValue{
								Template: getEnableSSHIntoNodesTemplate(),
							},
						},
					},
				},
			},
		},
		{
			Name: "infraClusterSubstitutions",
			Definitions: []clusterv1.PatchDefinition{
				{
					Selector: clusterv1.PatchSelector{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       util.TypeToKind(&infrav1.VSphereClusterTemplate{}),
						MatchResources: clusterv1.PatchSelectorMatch{
							InfrastructureCluster: true,
						},
					},
					JSONPatches: []clusterv1.JSONPatch{
						{
							Op:   "add",
							Path: "/spec/template/spec/controlPlaneEndpoint",
							ValueFrom: &clusterv1.JSONPatchValue{
								Template: getControlPlaneEndpointTemplate(),
							},
						},
						{
							Op:   "add",
							Path: "/spec/template/spec/identityRef",
							ValueFrom: &clusterv1.JSONPatchValue{
								Template: getCredSecretNameTemplate(),
							},
						},
						{
							Op:   "add",
							Path: "/spec/template/spec/server",
							ValueFrom: &clusterv1.JSONPatchValue{
								Variable: pointer.StringPtr("infraServer.url"),
							},
						},
						{
							Op:   "add",
							Path: "/spec/template/spec/thumbprint",
							ValueFrom: &clusterv1.JSONPatchValue{
								Variable: pointer.StringPtr("infraServer.thumbprint"),
							},
						},
					},
				},
			},
		},
		{
			Name: "kubeVipEnabled",
			Definitions: []clusterv1.PatchDefinition{
				{
					Selector: clusterv1.PatchSelector{
						APIVersion: controlplanev1.GroupVersion.String(),
						Kind:       util.TypeToKind(&controlplanev1.KubeadmControlPlaneTemplate{}),
						MatchResources: clusterv1.PatchSelectorMatch{
							ControlPlane: true,
						},
					},
					JSONPatches: []clusterv1.JSONPatch{
						{
							Op:   "add",
							Path: "/spec/template/spec/kubeadmConfigSpec/files/0/content",
							ValueFrom: &clusterv1.JSONPatchValue{
								Variable: pointer.String("kubeVipPodManifest"),
							},
						},
					},
				},
			},
		},
	}
}

func getCredSecretNameTemplate() *string {
	template := map[string]interface{}{
		"name": "{{ .credsSecretName }}",
		"kind": "Secret",
	}
	templateStr, _ := yaml.Marshal(template)
	return pointer.StringPtr(string(templateStr))
}

func getControlPlaneEndpointTemplate() *string {
	template := map[string]interface{}{
		"host": "{{ .controlPlaneIpAddr }}",
		"port": 6443,
	}
	templateStr, _ := yaml.Marshal(template)
	return pointer.StringPtr(string(templateStr))
}

func getEnableSSHIntoNodesTemplate() *string {
	template := []map[string]interface{}{
		{
			"name": "capv",
			"sshAuthorizedKeys": []string{
				"{{ .sshKey }}",
			},
			"sudo": "ALL=(ALL) NOPASSWD:ALL",
		},
	}
	templateStr, _ := yaml.Marshal(template)
	return pointer.StringPtr(string(templateStr))
}

func getClusterClassVariables() []clusterv1.ClusterClassVariable {
	return []clusterv1.ClusterClassVariable{
		{
			Name:     "sshKey",
			Required: false,
			Schema: clusterv1.VariableSchema{
				OpenAPIV3Schema: clusterv1.JSONSchemaProps{
					Description: "Public key to SSH onto the cluster nodes.",
					Type:        "string",
				},
			},
		},
		{
			Name:     "infraServer",
			Required: true,
			Schema: clusterv1.VariableSchema{
				OpenAPIV3Schema: clusterv1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]clusterv1.JSONSchemaProps{
						"url":        {Type: "string"},
						"thumbprint": {Type: "string"},
					},
				},
			},
		},
		{
			Name:     "controlPlaneIpAddr",
			Required: true,
			Schema: clusterv1.VariableSchema{
				OpenAPIV3Schema: clusterv1.JSONSchemaProps{
					Type:        "string",
					Description: "Floating VIP for the control plane.",
				},
			},
		},
		{
			Name:     "credsSecretName",
			Required: true,
			Schema: clusterv1.VariableSchema{
				OpenAPIV3Schema: clusterv1.JSONSchemaProps{
					Type:        "string",
					Description: "Secret containing the credentials for the infra cluster.",
				},
			},
		},
		{
			Name:     "kubeVipPodManifest",
			Required: true,
			Schema: clusterv1.VariableSchema{
				OpenAPIV3Schema: clusterv1.JSONSchemaProps{
					Type:        "string",
					Description: "kube-vip manifest for the control plane.",
				},
			},
		},
	}
}
