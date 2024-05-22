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

package webhooks

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-vmware-infrastructure-cluster-x-k8s-io-v1beta1-providerserviceaccount,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=vmware.infrastructure.cluster.x-k8s.io,resources=providerserviceaccounts,versions=v1beta1,name=validation.providerserviceaccount.vmware.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-vmware-infrastructure-cluster-x-k8s-io-v1beta1-providerserviceaccount,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=vmware.infrastructure.cluster.x-k8s.io,resources=providerserviceaccounts,versions=v1beta1,name=default.providerserviceaccount.vmware.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

// ProviderServiceAccountWebhook implements a validation and defaulting webhook for ProviderServiceAccount.
type ProviderServiceAccountWebhook struct{}

var _ webhook.CustomValidator = &ProviderServiceAccountWebhook{}
var _ webhook.CustomDefaulter = &ProviderServiceAccountWebhook{}

func (webhook *ProviderServiceAccountWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vmwarev1.ProviderServiceAccount{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (webhook *ProviderServiceAccountWebhook) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *ProviderServiceAccountWebhook) ValidateCreate(_ context.Context, raw runtime.Object) (admission.Warnings, error) {
	var allErrs field.ErrorList

	obj, ok := raw.(*vmwarev1.ProviderServiceAccount)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a ProviderServiceAccount but got a %T", raw))
	}

	spec := obj.Spec
	if spec.Ref == nil && (spec.ClusterName == nil || *spec.ClusterName == "") { //nolint:staticcheck
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec"), "should specify Ref or ClusterName"))
		if spec.ClusterName != nil && *spec.ClusterName == "" {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "ClusterName"), "ClusterName should not be empty"))
		}
	}

	return nil, aggregateObjErrors(obj.GroupVersionKind().GroupKind(), obj.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *ProviderServiceAccountWebhook) ValidateUpdate(_ context.Context, _ runtime.Object, newRaw runtime.Object) (admission.Warnings, error) {
	var allErrs field.ErrorList

	newTyped, ok := newRaw.(*vmwarev1.ProviderServiceAccount)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a ProviderServiceAccount but got a %T", newRaw))
	}

	spec := newTyped.Spec
	if spec.Ref == nil && (spec.ClusterName == nil || *spec.ClusterName == "") { //nolint:staticcheck
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec"), "should specify Ref or ClusterName"))
		if spec.ClusterName != nil && *spec.ClusterName == "" {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "ClusterName"), "ClusterName should not be empty"))
		}
	}

	return nil, aggregateObjErrors(newTyped.GroupVersionKind().GroupKind(), newTyped.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *ProviderServiceAccountWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
