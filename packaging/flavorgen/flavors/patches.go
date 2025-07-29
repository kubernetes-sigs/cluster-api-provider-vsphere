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
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
)

func createEmptyArraysPatch() clusterv1.ClusterClassPatch {
	return clusterv1.ClusterClassPatch{
		Name: "createEmptyArrays",
		Definitions: []clusterv1.PatchDefinition{
			{
				Selector: clusterv1.PatchSelector{
					APIVersion: controlplanev1.GroupVersion.String(),
					Kind:       util.TypeToKind(&controlplanev1.KubeadmControlPlaneTemplate{}),
					MatchResources: clusterv1.PatchSelectorMatch{
						ControlPlane: ptr.To(true),
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
		EnabledIf: "{{ if .sshKey }}true{{end}}",
		Definitions: []clusterv1.PatchDefinition{
			{
				Selector: clusterv1.PatchSelector{
					APIVersion: controlplanev1.GroupVersion.String(),
					Kind:       util.TypeToKind(&controlplanev1.KubeadmControlPlaneTemplate{}),
					MatchResources: clusterv1.PatchSelectorMatch{
						ControlPlane: ptr.To(true),
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
						InfrastructureCluster: ptr.To(true),
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
							Variable: "infraServer.url",
						},
					},
					{
						Op:   "add",
						Path: "/spec/template/spec/thumbprint",
						ValueFrom: &clusterv1.JSONPatchValue{
							Variable: "infraServer.thumbprint",
						},
					},
				},
			},
		},
	}
}

func vmWareInfraClusterPatch() clusterv1.ClusterClassPatch {
	return clusterv1.ClusterClassPatch{
		Name: "infraClusterSubstitutions",
		Definitions: []clusterv1.PatchDefinition{
			{
				Selector: clusterv1.PatchSelector{
					APIVersion: vmwarev1.GroupVersion.String(),
					Kind:       util.TypeToKind(&vmwarev1.VSphereClusterTemplate{}),
					MatchResources: clusterv1.PatchSelectorMatch{
						InfrastructureCluster: ptr.To(true),
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
				},
			},
		},
	}
}
