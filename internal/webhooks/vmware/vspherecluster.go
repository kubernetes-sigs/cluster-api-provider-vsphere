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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/webhooks"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-vmware-infrastructure-cluster-x-k8s-io-v1beta1-vspherecluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=vmware.infrastructure.cluster.x-k8s.io,resources=vsphereclusters,versions=v1beta1,name=validation.vspherecluster.vmware.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

// VSphereClusterWebhook implements a validation and defaulting webhook for VSphereCluster.
type VSphereClusterWebhook struct{}

var _ webhook.CustomValidator = &VSphereClusterWebhook{}

func (webhook *VSphereClusterWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&vmwarev1.VSphereCluster{}).
		WithValidator(webhook).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *VSphereClusterWebhook) ValidateCreate(_ context.Context, newRaw runtime.Object) (admission.Warnings, error) {
	var allErrs field.ErrorList

	newTyped, ok := newRaw.(*vmwarev1.VSphereCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a VSphereCluster but got a %T", newRaw))
	}

	newSpec := newTyped.Spec

	if !feature.Gates.Enabled(feature.WorkerAntiAffinity) {
		// Cluster mode is not allowed without WorkerAntiAffinity being enabled.
		if newSpec.Placement != nil && newSpec.Placement.WorkerAntiAffinity != nil && newSpec.Placement.WorkerAntiAffinity.Mode == vmwarev1.VSphereClusterWorkerAntiAffinityModeMachineDeployment {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "placement", "workerAntiAffinity", "mode"), "cannot be set to Cluster with feature-gate WorkerAntiAffinity being disabled"))
		}
	}

	return nil, webhooks.AggregateObjErrors(newTyped.GroupVersionKind().GroupKind(), newTyped.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *VSphereClusterWebhook) ValidateUpdate(_ context.Context, _ runtime.Object, newRaw runtime.Object) (admission.Warnings, error) {
	var allErrs field.ErrorList

	newTyped, ok := newRaw.(*vmwarev1.VSphereCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a VSphereCluster but got a %T", newRaw))
	}

	newSpec := newTyped.Spec

	if !feature.Gates.Enabled(feature.WorkerAntiAffinity) {
		// Cluster mode is not allowed without WorkerAntiAffinity being enabled.
		if newSpec.Placement != nil && newSpec.Placement.WorkerAntiAffinity != nil && newSpec.Placement.WorkerAntiAffinity.Mode == vmwarev1.VSphereClusterWorkerAntiAffinityModeMachineDeployment {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "placement", "workerAntiAffinity", "mode"), "cannot be set to Cluster with feature-gate WorkerAntiAffinity being disabled"))
		}
	}

	return nil, webhooks.AggregateObjErrors(newTyped.GroupVersionKind().GroupKind(), newTyped.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *VSphereClusterWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
