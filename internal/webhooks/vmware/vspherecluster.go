/*
Copyright 2025 The Kubernetes Authors.

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/util/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/webhooks/vmware/conversion"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-vmware-infrastructure-cluster-x-k8s-io-v1beta2-vspherecluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=vmware.infrastructure.cluster.x-k8s.io,resources=vsphereclusters,versions=v1beta2,name=validation.vspherecluster.vmware.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1

// VSphereCluster implements a validation and defaulting webhook for VSphereCluster.
type VSphereCluster struct {
	// NetworkProvider is the network provider used by Supervisor based clusters
	NetworkProvider string
}

var _ admission.Validator[*vmwarev1.VSphereCluster] = &VSphereCluster{}

func (webhook *VSphereCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &vmwarev1.VSphereCluster{}).
		WithValidator(webhook).
		WithConverter(conversion.VSphereCluster).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *VSphereCluster) ValidateCreate(ctx context.Context, obj *vmwarev1.VSphereCluster) (admission.Warnings, error) {
	return webhook.validate(ctx, obj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *VSphereCluster) ValidateUpdate(ctx context.Context, _, newTyped *vmwarev1.VSphereCluster) (admission.Warnings, error) {
	return webhook.validate(ctx, newTyped)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *VSphereCluster) ValidateDelete(_ context.Context, _ *vmwarev1.VSphereCluster) (admission.Warnings, error) {
	return nil, nil
}

// validateClusterNetwork validates the network configuration of the VSphereCluster.
func (webhook *VSphereCluster) validateClusterNetwork(cluster *vmwarev1.VSphereCluster, skipEmptyProvider bool) field.ErrorList {
	var allErrs field.ErrorList

	if !feature.Gates.Enabled(feature.MultiNetworks) && cluster.Spec.Network.NSXVPC.CreateSubnetSet != nil {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "network", "nsxVPC", "createSubnetSet"),
			"createSubnetSet can only be set when MultiNetworks feature gate is enabled",
		))
	}

	// When the ClusterNetworkProvider gate is enabled, the provider to validate against is the
	// cluster's own spec.network.provider; otherwise it is the static flag value.
	provider := webhook.NetworkProvider
	if feature.Gates.Enabled(feature.ClusterNetworkProvider) {
		provider = cluster.Spec.Network.Provider

		if provider == "" {
			// Topology SSA dry-runs the original object (still without provider) before the
			// Runtime Extension's write is applied. Allow empty only for those dry-run requests.
			if skipEmptyProvider {
				return allErrs
			}
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec", "network", "provider"),
				"spec.network.provider must be set",
			))
			return allErrs
		}

		if provider == manager.ExternallyManagedNetworkProvider && !feature.Gates.Enabled(feature.ExternallyManagedProvider) {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec", "network", "provider"),
				"provider ExternallyManaged can only be set when feature gate ExternallyManagedProvider is enabled",
			))
		}
	} else if cluster.Spec.Network.Provider != "" {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "network", "provider"),
			"provider can only be set when ClusterNetworkProvider feature gate is enabled",
		))
		return allErrs
	}

	if cluster.Spec.Network.NSXVPC.IsDefined() && provider != manager.NSXVPCNetworkProvider {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "network", "nsxVPC"),
			fmt.Sprintf("nsxVPC can only be set when network provider is %s", manager.NSXVPCNetworkProvider),
		))
	}

	return allErrs
}

// validate aggregates all shared validations for the VSphereCluster.
func (webhook *VSphereCluster) validate(ctx context.Context, cluster *vmwarev1.VSphereCluster) (admission.Warnings, error) {
	skipEmptyProvider := false
	if req, err := admission.RequestFromContext(ctx); err == nil {
		skipEmptyProvider = topology.IsDryRunRequest(req, cluster)
	}

	allErrs := webhook.validateClusterNetwork(cluster, skipEmptyProvider)
	allErrs = append(allErrs, validateFailureDomainsControlPlaneSelector(
		cluster.Spec.FailureDomains.ControlPlane.Selector,
		field.NewPath("spec", "failureDomains", "controlPlane", "selector"),
	)...)

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(cluster.GroupVersionKind().GroupKind(), cluster.Name, allErrs)
	}

	return nil, nil
}

// validateFailureDomainsControlPlaneSelector validates the control plane failure domain selector.
func validateFailureDomainsControlPlaneSelector(selector *metav1.LabelSelector, fldPath *field.Path) field.ErrorList {
	if selector == nil {
		return nil
	}

	var allErrs field.ErrorList

	// Validate Feature Gate is enabled.
	if !feature.Gates.Enabled(feature.NamespaceScopedZones) {
		allErrs = append(allErrs, field.Forbidden(
			fldPath,
			"control plane zone selector can only be set when feature gate NamespaceScopedZones is enabled",
		))
		return allErrs
	}

	// Validate the selector syntax is valid.
	parsedSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		allErrs = append(
			allErrs,
			field.Invalid(fldPath, selector, err.Error()),
		)
		return allErrs
	}

	// Validate the selector is not empty.
	if parsedSelector.Empty() {
		allErrs = append(
			allErrs,
			field.Invalid(fldPath, selector, "selector must not be empty"),
		)
	}

	return allErrs
}
