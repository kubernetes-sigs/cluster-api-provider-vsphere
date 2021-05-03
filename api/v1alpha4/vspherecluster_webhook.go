/*
Copyright 2021 The Kubernetes Authors.

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

package v1alpha4

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (c *VSphereCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha4-vspherecluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters,versions=v1alpha4,name=validation.vspherecluster.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *VSphereCluster) ValidateCreate() error {
	var allErrs field.ErrorList
	spec := c.Spec

	if spec.Thumbprint != "" && spec.Insecure != nil && *spec.Insecure {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "Insecure"), spec.Insecure, "cannot be set to true at the same time as .spec.Thumbprint"))
	}

	return aggregateObjErrors(c.GroupVersionKind().GroupKind(), c.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *VSphereCluster) ValidateUpdate(old runtime.Object) error {
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *VSphereCluster) ValidateDelete() error {
	return nil
}
