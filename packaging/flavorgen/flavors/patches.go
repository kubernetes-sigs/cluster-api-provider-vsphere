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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
)

func createFilesArrayPatch() clusterv1.ClusterClassPatch {
	return clusterv1.ClusterClassPatch{
		Name: "createFilesArray",
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
						Path: "/spec/template/spec/kubeadmConfigSpec/files",
						Value: &apiextensionsv1.JSON{
							Raw: []byte("[]"),
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
						Path: "/spec/template/spec/files",
						Value: &apiextensionsv1.JSON{
							Raw: []byte("[]"),
						},
					},
				},
			},
		},
	}
}

func enableSSHPatch() clusterv1.ClusterClassPatch {
	return clusterv1.ClusterClassPatch{
		Name:      "enableSSHIntoNodes",
		EnabledIf: pointer.String("{{ if .sshKey }}true{{end}}"),
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
	}
}

func infraClusterPatch() clusterv1.ClusterClassPatch {
	return clusterv1.ClusterClassPatch{
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
							Variable: pointer.String("infraServer.url"),
						},
					},
					{
						Op:   "add",
						Path: "/spec/template/spec/thumbprint",
						ValueFrom: &clusterv1.JSONPatchValue{
							Variable: pointer.String("infraServer.thumbprint"),
						},
					},
				},
			},
		},
	}
}

func kubeVipEnabledPatch() clusterv1.ClusterClassPatch {
	return clusterv1.ClusterClassPatch{
		Name: "kubeVipPodManifest",
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
						Path: "/spec/template/spec/kubeadmConfigSpec/files/-",
						ValueFrom: &clusterv1.JSONPatchValue{
							// This patch ensures that the ControlPlaneIP which is set as variable `controlPlaneIPAddr` is also set
							// in the kube-vip static pod manifest.
							Template: pointer.String(`owner: root:root
path:  "/etc/kubernetes/manifests/kube-vip.yaml"
content: {{ printf "%q" (regexReplaceAll "(name: address\n +value:).*" .kubeVipPodManifest (printf "$1 %s" .controlPlaneIpAddr)) }}`),
						},
					},
				},
			},
		},
	}
}
