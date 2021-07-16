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

package v1alpha4

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *VSphereClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/default-infrastructure-cluster-x-k8s-io-v1alpha4-vsphereclustertemplate,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=vsphereclustertemplates,versions=v1alpha4,name=default.vsphereclustertemplate.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha4-vsphereclustertemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=vsphereclustertemplates,versions=v1alpha4,name=validation.vsphereclustertemplate.infrastructure.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &VSphereClusterTemplate{}

func (r *VSphereClusterTemplate) Default() {
	defaultVSphereCluterSpec(&r.Spec.Template.Spec)
}

var _ webhook.Validator = &VSphereClusterTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *VSphereClusterTemplate) ValidateCreate() error {
	allErrs := validateVSphereClusterSpec(r.Spec.Template.Spec)
	if len(allErrs) == 0 {
		return nil
	}
	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *VSphereClusterTemplate) ValidateUpdate(oldRaw runtime.Object) error {
	old := oldRaw.(*VSphereClusterTemplate)
	if !reflect.DeepEqual(r.Spec.Template.Spec, old.Spec.Template.Spec) {
		return field.Forbidden(field.NewPath("spec", "template", "spec"), "VSphereClusterTemplate spec is immutable")
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *VSphereClusterTemplate) ValidateDelete() error {
	return nil
}
