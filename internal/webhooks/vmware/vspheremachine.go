/*
Copyright 2024 The Kubernetes Authors.

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

// Package vmware is the package for webhooks of vmware resources.
package vmware

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/webhooks"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
	pkgnetwork "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-vmware-infrastructure-cluster-x-k8s-io-v1beta2-vspheremachine,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=vmware.infrastructure.cluster.x-k8s.io,resources=vspheremachines,versions=v1beta2,name=validation.vspheremachine.vmware.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1

// VSphereMachine implements a validation and defaulting webhook for VSphereMachine.
type VSphereMachine struct {
	// NetworkProvider is the network provider used by Supervisor based clusters
	NetworkProvider string
	// Client is used to validate references; it defaults to mgr.GetClient() from SetupWebhookWithManager.
	Client client.Client
}

var _ admission.Validator[*vmwarev1.VSphereMachine] = &VSphereMachine{}

func (webhook *VSphereMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if webhook.Client == nil {
		webhook.Client = mgr.GetClient()
	}
	return ctrl.NewWebhookManagedBy(mgr, &vmwarev1.VSphereMachine{}).
		WithValidator(webhook).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *VSphereMachine) ValidateCreate(ctx context.Context, objTyped *vmwarev1.VSphereMachine) (admission.Warnings, error) {
	allErrs := validateNetwork(webhook.NetworkProvider, objTyped.Spec.Network, field.NewPath("spec", "network"))
	if feature.Gates.Enabled(feature.InfrastructurePolicies) {
		allErrs = append(allErrs, validateInfrastructurePolicies(ctx, webhook.Client, objTyped.Namespace, objTyped.Spec.InfrastructurePolicies,
			field.NewPath("spec", "infrastructurePolicies"))...)
	}

	return nil, webhooks.AggregateObjErrors(objTyped.GroupVersionKind().GroupKind(), objTyped.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *VSphereMachine) ValidateUpdate(_ context.Context, oldTyped, newTyped *vmwarev1.VSphereMachine) (admission.Warnings, error) {
	var allErrs field.ErrorList

	newSpec, oldSpec := newTyped.Spec, oldTyped.Spec

	// In VM operator, following fields are immutable, so CAPV should not allow to update them.
	// - ImageName
	// - ClassName
	// - StorageClass
	// - MinHardwareVersion
	if newSpec.ImageName != oldSpec.ImageName {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "imageName"), "cannot be modified"))
	}

	if newSpec.ClassName != oldSpec.ClassName {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "className"), "cannot be modified"))
	}

	if newSpec.StorageClass != oldSpec.StorageClass {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "storageClass"), "cannot be modified"))
	}

	if newSpec.MinHardwareVersion != oldSpec.MinHardwareVersion {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "minHardwareVersion"), "cannot be modified"))
	}

	if feature.Gates.Enabled(feature.InfrastructurePolicies) {
		if !reflect.DeepEqual(newSpec.InfrastructurePolicies, oldSpec.InfrastructurePolicies) {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "infrastructurePolicies"), "cannot be modified"))
		}
	}

	if !reflect.DeepEqual(newSpec.Network.Interfaces, oldSpec.Network.Interfaces) {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "network", "interfaces"), "cannot be modified"))
	}

	allErrs = append(allErrs, validateNetwork(webhook.NetworkProvider, newSpec.Network, field.NewPath("spec", "network"))...)

	return nil, webhooks.AggregateObjErrors(newTyped.GroupVersionKind().GroupKind(), newTyped.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *VSphereMachine) ValidateDelete(_ context.Context, _ *vmwarev1.VSphereMachine) (admission.Warnings, error) {
	return nil, nil
}

// supportedInfrastructurePolicyKinds is the set of policy Kinds that may be
// referenced via spec.infrastructurePolicies. Add new entries here when
// additional policy types are supported.
var supportedInfrastructurePolicyKinds = map[string]struct{}{
	"ComputePolicy": {},
}

func resolveInfrastructurePolicyRef(ref vmwarev1.InfrastructurePolicyRef) (schema.GroupVersionKind, error) {
	if ref.APIVersion == "" {
		return schema.GroupVersionKind{}, fmt.Errorf("apiVersion is required")
	}
	if ref.Kind == "" {
		return schema.GroupVersionKind{}, fmt.Errorf("kind is required")
	}
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid apiVersion %q: %w", ref.APIVersion, err)
	}
	return gv.WithKind(ref.Kind), nil
}

func validateInfrastructurePolicies(ctx context.Context, c client.Client, namespace string, policies []vmwarev1.InfrastructurePolicyRef, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	for i, ref := range policies {
		idxPath := fldPath.Index(i)

		// Structural field checks — no API server required.
		if ref.Name == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), ""))
			continue
		}
		if ref.Kind == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("kind"), ""))
			continue
		}
		if _, ok := supportedInfrastructurePolicyKinds[ref.Kind]; !ok {
			supported := make([]string, 0, len(supportedInfrastructurePolicyKinds))
			for k := range supportedInfrastructurePolicyKinds {
				supported = append(supported, k)
			}
			sort.Strings(supported)
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("kind"), ref.Kind, supported))
			continue
		}
		if ref.APIVersion == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("apiVersion"), ""))
			continue
		}
		gvk, err := resolveInfrastructurePolicyRef(ref)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(idxPath, ref, err.Error()))
			continue
		}

		// Existence check — requires a live client; skipped in unit tests that
		// pass a nil client to isolate structural validation.
		if c == nil {
			continue
		}
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		key := client.ObjectKey{Namespace: namespace, Name: ref.Name}
		if err := c.Get(ctx, key, obj); err != nil {
			if apierrors.IsNotFound(err) {
				allErrs = append(allErrs, field.Invalid(idxPath, ref, fmt.Sprintf("referenced infrastructure policy does not exist (%s %s/%s)", gvk.Kind, namespace, ref.Name)))
				continue
			}
			if meta.IsNoMatchError(err) {
				allErrs = append(allErrs, field.Invalid(idxPath, gvk, fmt.Sprintf("no matching resource type in the cluster: %v", err)))
				continue
			}
			allErrs = append(allErrs, field.InternalError(idxPath, fmt.Errorf("failed to get infrastructure policy %s: %w", gvk.String(), err)))
		}
	}
	return allErrs
}

func validateNetwork(networkProvider string, network vmwarev1.VSphereMachineNetworkSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if network.Interfaces.IsDefined() {
		if !feature.Gates.Enabled(feature.MultiNetworks) {
			allErrs = append(allErrs, field.Forbidden(
				fldPath.Child("interfaces"),
				"interfaces can only be set when feature gate MultiNetworks is enabled"))
		} else {
			// Validate network type is supported
			switch networkProvider {
			case manager.NSXVPCNetworkProvider:
				primary := network.Interfaces.Primary
				if primary.IsDefined() {
					primaryNetGVK := primary.NetworkRef.GroupVersionKind()
					if primaryNetGVK != pkgnetwork.NetworkGVKNSXTVPCSubnetSet {
						allErrs = append(allErrs, field.Invalid(
							fldPath.Child("interfaces", "primary", "network"),
							primaryNetGVK,
							fmt.Sprintf("only supports %s", pkgnetwork.NetworkGVKNSXTVPCSubnetSet)))
					}
				}
				for i, secondaryInterface := range network.Interfaces.Secondary {
					secondaryNetGVK := secondaryInterface.NetworkRef.GroupVersionKind()
					if secondaryNetGVK != pkgnetwork.NetworkGVKNSXTVPCSubnetSet && secondaryNetGVK != pkgnetwork.NetworkGVKNSXTVPCSubnet {
						allErrs = append(allErrs, field.Invalid(
							fldPath.Child("interfaces", "secondary").Index(i).Child("network"),
							secondaryNetGVK,
							fmt.Sprintf("only supports %s or %s", pkgnetwork.NetworkGVKNSXTVPCSubnetSet, pkgnetwork.NetworkGVKNSXTVPCSubnet)))
					}
				}
			case manager.VDSNetworkProvider:
				if network.Interfaces.Primary.IsDefined() {
					allErrs = append(allErrs, field.Forbidden(
						fldPath.Child("interfaces", "primary"),
						"primary interface can not be set when network provider is vsphere-network"))
				}
				for i, secondaryInterface := range network.Interfaces.Secondary {
					secondaryNetGVK := secondaryInterface.NetworkRef.GroupVersionKind()
					if secondaryNetGVK != pkgnetwork.NetworkGVKNetOperator {
						allErrs = append(allErrs, field.Invalid(
							fldPath.Child("interfaces", "secondary").Index(i).Child("network"),
							secondaryNetGVK,
							fmt.Sprintf("only supports %s", pkgnetwork.NetworkGVKNetOperator)))
					}
				}
			default:
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("interfaces"), fmt.Sprintf("interfaces can not be set when network provider is %s", networkProvider)))
			}

			// Validate interface names are unique
			interfaceNames := map[string]struct{}{pkgnetwork.PrimaryInterfaceName: {}}
			for i, secondaryInterface := range network.Interfaces.Secondary {
				if _, ok := interfaceNames[secondaryInterface.Name]; ok {
					allErrs = append(allErrs, field.Invalid(
						fldPath.Child("interfaces", "secondary").Index(i).Child("name"),
						secondaryInterface.Name,
						"interface name is already in use"))
				} else {
					interfaceNames[secondaryInterface.Name] = struct{}{}
				}
			}
		}
	}
	return allErrs
}
