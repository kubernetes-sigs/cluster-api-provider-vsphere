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

// Package topologymutation contains the handlers for the topologymutation webhook.
//
// The implementation of the handlers is specifically designed for Cluster API E2E tests use cases.
// When implementing custom RuntimeExtension, it is only required to expose HandlerFunc with the
// signature defined in sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1.
package topologymutation

import (
	"context"
	"fmt"
	"regexp"

	"github.com/pkg/errors"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/clusterclass"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/kubevip"
)

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;patch;update;create

// ExtensionHandlers provides a common struct shared across the topology mutation hooks handlers;
// this is convenient because in Cluster API's E2E tests all of them are using a decoder for working with typed
// API objects, which makes code easier to read and less error prone than using unstructured or working with raw json/yaml.
// NOTE: it is not mandatory to use a ExtensionHandlers in custom RuntimeExtension, what is important
// is to expose HandlerFunc with the signature defined in sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1.
type ExtensionHandlers struct {
	decoder runtime.Decoder
}

// NewExtensionHandlers returns a new ExtensionHandlers for the topology mutation hook handlers.
func NewExtensionHandlers() *ExtensionHandlers {
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)
	_ = vmwarev1.AddToScheme(scheme)
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &ExtensionHandlers{
		// Add the apiGroups being handled to the decoder
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			infrav1.GroupVersion,
			vmwarev1.GroupVersion,
			controlplanev1.GroupVersion,
			bootstrapv1.GroupVersion,
		),
	}
}

// GeneratePatches implements the HandlerFunc for the GeneratePatches hook.
// The hook adds to the response the patches we are using in Cluster API E2E tests.
// NOTE: custom RuntimeExtension must implement the body of this func according to the specific use case.
func (h *ExtensionHandlers) GeneratePatches(ctx context.Context, req *runtimehooksv1.GeneratePatchesRequest, resp *runtimehooksv1.GeneratePatchesResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("GeneratePatches is called")

	// By using WalkTemplates it is possible to implement patches using typed API objects, which makes code
	// easier to read and less error prone than using unstructured or working with raw json/yaml.
	// IMPORTANT: by unit testing this func/nested func properly, it is possible to prevent unexpected rollouts when patches are modified.
	topologymutation.WalkTemplates(ctx, h.decoder, req, resp,
		func(ctx context.Context, obj runtime.Object, variables map[string]apiextensionsv1.JSON, holderRef runtimehooksv1.HolderReference) error {
			log := ctrl.LoggerFrom(ctx)

			isControlPlane := holderRef.Kind == "KubeadmControlPlane"

			switch obj := obj.(type) {
			case *controlplanev1.KubeadmControlPlaneTemplate:
				if err := patchKubeadmControlPlaneTemplate(ctx, obj, variables); err != nil {
					log.Error(err, "Error patching KubeadmControlPlaneTemplate")
					return errors.Wrap(err, "error patching KubeadmControlPlaneTemplate")
				}
			case *bootstrapv1.KubeadmConfigTemplate:
				if err := patchKubeadmConfigTemplate(ctx, obj, variables); err != nil {
					log.Error(err, "Error patching KubeadmConfigTemplate")
					return errors.Wrap(err, "error patching KubeadmConfigTemplate")
				}
			case *infrav1.VSphereClusterTemplate:
				if err := patchGovmomiClusterTemplate(ctx, obj, variables); err != nil {
					log.Error(err, "Error patching VSphereClusterTemplate")
					return errors.Wrap(err, "error patching VSphereClusterTemplate")
				}
			case *infrav1.VSphereMachineTemplate:
				if err := patchGovmomiMachineTemplate(ctx, obj, variables, isControlPlane); err != nil {
					log.Error(err, "Error patching VSphereMachineTemplate")
					return errors.Wrap(err, "error patching VSphereMachineTemplate")
				}
			case *vmwarev1.VSphereClusterTemplate:
				if err := patchSupervisorClusterTemplate(ctx, obj, variables); err != nil {
					log.Error(err, "Error patching VSphereClusterTemplate")
					return errors.Wrap(err, "error patching VSphereClusterTemplate")
				}
			case *vmwarev1.VSphereMachineTemplate:
				if err := patchSupervisorMachineTemplate(ctx, obj, variables, isControlPlane); err != nil {
					log.Error(err, "Error patching VSphereMachineTemplate")
					return errors.Wrap(err, "error patching VSphereMachineTemplate")
				}
			}
			return nil
		},
		// Use a merge-patch instead of a JSON patch because WalkTemplates would create
		// an incompatible patch for vmwarev1.VSphereClusterTemplate because we provide
		// an empty template without a set `.spec` and due to omitempty
		// `.spec.template.spec.controlPlaneEndpoint` does not exist.
		topologymutation.PatchFormat{Format: runtimehooksv1.JSONMergePatchType},
		topologymutation.FailForUnknownTypes{},
	)
}

// patchKubeadmControlPlaneTemplate patches the KubeadmControlPlaneTemplate.
func patchKubeadmControlPlaneTemplate(_ context.Context, tpl *controlplanev1.KubeadmControlPlaneTemplate, templateVariables map[string]apiextensionsv1.JSON) error {
	// patch enableSSHIntoNodes
	if err := patchUsers(&tpl.Spec.Template.Spec.KubeadmConfigSpec, templateVariables); err != nil {
		return err
	}

	// patch kubeVipPodManifest
	kubeVipPodManifest, err := topologymutation.GetStringVariable(templateVariables, "kubeVipPodManifest")
	kubeVipPodManifestNotFound := topologymutation.IsNotFoundError(err)
	if err != nil && !kubeVipPodManifestNotFound {
		return err
	}
	// Skip patch if kubeVipPodManifest variable was not found / not set.
	if !kubeVipPodManifestNotFound {
		controlPlaneIPAddr, err := topologymutation.GetStringVariable(templateVariables, "controlPlaneIpAddr")
		if err != nil {
			return err
		}
		kubeVipPodManifestModified := regexp.MustCompile("(name: address\n +value:).*").ReplaceAllString(kubeVipPodManifest, fmt.Sprintf("$1 %s", controlPlaneIPAddr))

		for _, file := range kubevip.Files() {
			if file.Path == "/etc/kubernetes/manifests/kube-vip.yaml" {
				file.Content = kubeVipPodManifestModified
			}
			tpl.Spec.Template.Spec.KubeadmConfigSpec.Files = append(tpl.Spec.Template.Spec.KubeadmConfigSpec.Files, file)
		}
	}

	// patch preKubeadmScript
	preKubeadmScript, err := topologymutation.GetStringVariable(templateVariables, "preKubeadmScript")
	preKubeadmScriptNotFound := topologymutation.IsNotFoundError(err)
	if err != nil && !preKubeadmScriptNotFound {
		return err
	}
	// Skip patch if preKubeadmScript variable was not found / not set.
	if !preKubeadmScriptNotFound {
		version, err := topologymutation.GetStringVariable(templateVariables, "builtin.controlPlane.version")
		if err != nil {
			return err
		}

		versionRegex := regexp.MustCompile("(KUBERNETES_VERSION=.*)")
		tpl.Spec.Template.Spec.KubeadmConfigSpec.Files = append(tpl.Spec.Template.Spec.KubeadmConfigSpec.Files,
			bootstrapv1.File{
				Owner:       "root:root",
				Path:        "/etc/pre-kubeadm-commands/10-prekubeadmscript.sh",
				Permissions: "0755",
				Content:     versionRegex.ReplaceAllString(preKubeadmScript, fmt.Sprintf("KUBERNETES_VERSION=%s", version)),
			},
		)
	}

	return nil
}

// KubeadmConfigTemplate patches the KubeadmConfigTemplate.
func patchKubeadmConfigTemplate(_ context.Context, tpl *bootstrapv1.KubeadmConfigTemplate, templateVariables map[string]apiextensionsv1.JSON) error {
	// patch enableSSHIntoNodes
	if err := patchUsers(&tpl.Spec.Template.Spec, templateVariables); err != nil {
		return err
	}

	// always add a file so we don't have an empty array.
	tpl.Spec.Template.Spec.Files = append(tpl.Spec.Template.Spec.Files,
		bootstrapv1.File{
			Owner:       "root:root",
			Path:        "/etc/test-extension",
			Permissions: "0755",
			Content:     "injected from the test extension",
		},
	)

	// patch preKubeadmScript
	preKubeadmScript, err := topologymutation.GetStringVariable(templateVariables, "preKubeadmScript")
	preKubeadmScriptNotFound := topologymutation.IsNotFoundError(err)
	if err != nil && !preKubeadmScriptNotFound {
		return err
	}
	if !preKubeadmScriptNotFound {
		version, err := topologymutation.GetStringVariable(templateVariables, "builtin.machineDeployment.version")
		if err != nil {
			return err
		}

		versionRegex := regexp.MustCompile("(KUBERNETES_VERSION=.*)")
		tpl.Spec.Template.Spec.Files = append(tpl.Spec.Template.Spec.Files,
			bootstrapv1.File{
				Owner:       "root:root",
				Path:        "/etc/pre-kubeadm-commands/10-prekubeadmscript.sh",
				Permissions: "0755",
				Content:     versionRegex.ReplaceAllString(preKubeadmScript, fmt.Sprintf("KUBERNETES_VERSION=%s", version)),
			},
		)
	}

	return nil
}

func patchUsers(kubeadmConfigSpec *bootstrapv1.KubeadmConfigSpec, templateVariables map[string]apiextensionsv1.JSON) error {
	sshKey, err := topologymutation.GetStringVariable(templateVariables, "sshKey")
	if err != nil {
		// Skip patch if sshKey variable is not set
		if topologymutation.IsNotFoundError(err) {
			return nil
		}
		return err
	}

	kubeadmConfigSpec.Users = append(kubeadmConfigSpec.Users,
		bootstrapv1.User{
			Name:              "capv",
			SSHAuthorizedKeys: []string{sshKey},
			Sudo:              "ALL=(ALL) NOPASSWD:ALL",
		})
	return nil
}

// patchGovmomiClusterTemplate patches the govmomi VSphereClusterTemplate.
// NOTE: this patch is not required for any special reason, it is used for testing the patch machinery itself.
func patchGovmomiClusterTemplate(_ context.Context, vsphereCluster *infrav1.VSphereClusterTemplate, templateVariables map[string]apiextensionsv1.JSON) error {
	// patch infraClusterSubstitutions
	controlPlaneIPAddr, err := topologymutation.GetStringVariable(templateVariables, "controlPlaneIpAddr")
	if err != nil {
		return err
	}
	var controlPlanePort int32
	if err := topologymutation.GetObjectVariableInto(templateVariables, "controlPlanePort", &controlPlanePort); err != nil {
		return err
	}

	vsphereCluster.Spec.Template.Spec.ControlPlaneEndpoint.Host = controlPlaneIPAddr
	vsphereCluster.Spec.Template.Spec.ControlPlaneEndpoint.Port = controlPlanePort

	credsSecretName, err := topologymutation.GetStringVariable(templateVariables, "credsSecretName")
	if err != nil {
		return err
	}

	vsphereCluster.Spec.Template.Spec.IdentityRef = &infrav1.VSphereIdentityReference{
		Kind: infrav1.SecretKind,
		Name: credsSecretName,
	}

	infraServerURL, err := topologymutation.GetStringVariable(templateVariables, "infraServer.url")
	if err != nil {
		return err
	}

	vsphereCluster.Spec.Template.Spec.Server = infraServerURL

	infraServerThumbprint, err := topologymutation.GetStringVariable(templateVariables, "infraServer.thumbprint")
	if err != nil {
		return err
	}

	vsphereCluster.Spec.Template.Spec.Thumbprint = infraServerThumbprint

	return nil
}

// patchSupervisorClusterTemplate patches the supervisor VSphereClusterTemplate.
// NOTE: this patch is not required for any special reason, it is used for testing the patch machinery itself.
func patchSupervisorClusterTemplate(_ context.Context, vsphereCluster *vmwarev1.VSphereClusterTemplate, templateVariables map[string]apiextensionsv1.JSON) error {
	// patch infraClusterSubstitutions
	controlPlaneIPAddr, err := topologymutation.GetStringVariable(templateVariables, "controlPlaneIpAddr")
	if err != nil {
		return err
	}
	var controlPlanePort int32
	if err := topologymutation.GetObjectVariableInto(templateVariables, "controlPlanePort", &controlPlanePort); err != nil {
		return err
	}

	vsphereCluster.Spec.Template.Spec.ControlPlaneEndpoint.Host = controlPlaneIPAddr
	vsphereCluster.Spec.Template.Spec.ControlPlaneEndpoint.Port = controlPlanePort

	return nil
}

// patchGovmomiMachineTemplate patches the govmomi VSphereMachineTemplate.
// NOTE: this patch is not required for any special reason, it is used for testing the patch machinery itself.
func patchGovmomiMachineTemplate(_ context.Context, vsphereMachineTemplate *infrav1.VSphereMachineTemplate, templateVariables map[string]apiextensionsv1.JSON, isControlPlane bool) error {
	// patch vSphereTemplate

	var err error
	vsphereMachineTemplate.Spec.Template.Spec.Template, err = calculateImageName(templateVariables, isControlPlane)

	return err
}

// patchSupervisorMachineTemplate patches the supervisor VSphereMachineTemplate.
// NOTE: this patch is not required for any special reason, it is used for testing the patch machinery itself.
func patchSupervisorMachineTemplate(_ context.Context, vsphereMachineTemplate *vmwarev1.VSphereMachineTemplate, templateVariables map[string]apiextensionsv1.JSON, isControlPlane bool) error {
	// patch vSphereTemplate

	var err error
	vsphereMachineTemplate.Spec.Template.Spec.ImageName, err = calculateImageName(templateVariables, isControlPlane)

	return err
}

func calculateImageName(templateVariables map[string]apiextensionsv1.JSON, isControlPlane bool) (string, error) {
	// patch vSphereTemplate
	versionVariable := "builtin.controlPlane.version"
	if !isControlPlane {
		versionVariable = "builtin.machineDeployment.version"
	}

	version, err := topologymutation.GetStringVariable(templateVariables, versionVariable)
	if err != nil {
		return "", err
	}

	// Use known image.
	if version == "v1.28.0" || version == "v1.29.0" || version == "v1.30.0" {
		return fmt.Sprintf("ubuntu-2204-kube-%s", version), nil
	}

	// Fallback to ubuntu-2404-kube-v1.31.0 otherwise
	return "ubuntu-2404-kube-v1.31.0", nil
}

// ValidateTopology implements the HandlerFunc for the ValidateTopology hook.
// Cluster API E2E currently are just validating the hook gets called.
// NOTE: custom RuntimeExtension must implement the body of this func according to the specific use case.
func (h *ExtensionHandlers) ValidateTopology(ctx context.Context, _ *runtimehooksv1.ValidateTopologyRequest, resp *runtimehooksv1.ValidateTopologyResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("ValidateTopology called")

	resp.Status = runtimehooksv1.ResponseStatusSuccess
}

// DiscoverVariables implements the HandlerFunc for the DiscoverVariables hook.
func (h *ExtensionHandlers) DiscoverVariables(ctx context.Context, req *runtimehooksv1.DiscoverVariablesRequest, resp *runtimehooksv1.DiscoverVariablesResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DiscoverVariables called")

	vars := []clusterv1beta1.ClusterClassVariable{}

	for _, in := range clusterclass.GetClusterClassVariables(req.Settings["testMode"] == "govmomi") {
		out := clusterv1beta1.ClusterClassVariable{}
		if err := clusterv1beta1.Convert_v1beta2_ClusterClassVariable_To_v1beta1_ClusterClassVariable(&in, &out, nil); err != nil {
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = fmt.Sprintf("Failed to Convert ClusterClass variable %q to v1beta1", in.Name)
			return
		}
		vars = append(vars, out)
	}

	resp.Status = runtimehooksv1.ResponseStatusSuccess
	resp.Variables = vars
}
