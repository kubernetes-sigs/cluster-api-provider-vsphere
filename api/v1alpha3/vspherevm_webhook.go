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

package v1alpha3

import (
	"fmt"
	"net"
	"reflect"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *VSphereVM) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha3-vspherevm,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=vspherevms,versions=v1alpha3,name=validation.vspherevm.infrastructure.x-k8s.io,sideEffects=None
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha3-vspherevm,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=vspherevms,versions=v1alpha3,name=default.vspherevm.infrastructure.x-k8s.io,sideEffects=None

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *VSphereVM) Default() {
	// Windows hostnames must be < 16 characters in length
	if r.Spec.OS == Windows && len(r.Name) > 15 {
		name, err := base36TruncatedHash(r.Name, 15)

		if err == nil {
			r.Name = name
		}
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *VSphereVM) ValidateCreate() error {
	var allErrs field.ErrorList
	spec := r.Spec

	if spec.Network.PreferredAPIServerCIDR != "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "PreferredAPIServerCIDR"), spec.Network.PreferredAPIServerCIDR, "cannot be set, as it will be removed and is no longer used"))
	}

	for i, device := range spec.Network.Devices {
		for j, ip := range device.IPAddrs {
			if _, _, err := net.ParseCIDR(ip); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "network", fmt.Sprintf("devices[%d]", i), fmt.Sprintf("ipAddrs[%d]", j)), ip, "ip addresses should be in the CIDR format"))
			}
		}
	}

	if r.Spec.OS == Windows && len(r.Name) > 15 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("name"), r.Name, "name has to be less than 16 characters for Windows VM"))
	}
	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *VSphereVM) ValidateUpdate(old runtime.Object) error { //nolint
	newVSphereVM, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
	if err != nil {
		return apierrors.NewInternalError(errors.Wrap(err, "failed to convert new VSphereVM to unstructured object"))
	}
	oldVSphereVM, err := runtime.DefaultUnstructuredConverter.ToUnstructured(old)
	if err != nil {
		return apierrors.NewInternalError(errors.Wrap(err, "failed to convert old VSphereVM to unstructured object"))
	}

	var allErrs field.ErrorList

	newVSphereVMSpec := newVSphereVM["spec"].(map[string]interface{})
	oldVSphereVMSpec := oldVSphereVM["spec"].(map[string]interface{})

	// allow changes to biosUUID
	delete(oldVSphereVMSpec, "biosUUID")
	delete(newVSphereVMSpec, "biosUUID")

	// allow changes to bootstrapRef
	delete(oldVSphereVMSpec, "bootstrapRef")
	delete(newVSphereVMSpec, "bootstrapRef")

	newVSphereVMNetwork := newVSphereVMSpec["network"].(map[string]interface{})
	oldVSphereVMNetwork := oldVSphereVMSpec["network"].(map[string]interface{})

	// allow changes to the network devices
	delete(oldVSphereVMNetwork, "devices")
	delete(newVSphereVMNetwork, "devices")

	if !reflect.DeepEqual(oldVSphereVMSpec, newVSphereVMSpec) {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec"), "cannot be modified"))
	}

	return aggregateObjErrors(r.GroupVersionKind().GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *VSphereVM) ValidateDelete() error {
	return nil
}

const base36set = "0123456789abcdefghijklmnopqrstuvwxyz"

// From: https://github.com/kubernetes-sigs/cluster-api-provider-aws/blob/master/pkg/hash/base36.go
// Base36TruncatedHash returns a consistent hash using blake2b
// and truncating the byte values to alphanumeric only
// of a fixed length specified by the consumer.
func base36TruncatedHash(str string, len int) (string, error) {
	hasher, err := blake2b.New(len, nil)
	if err != nil {
		return "", errors.Wrap(err, "unable to create hash function")
	}

	if _, err := hasher.Write([]byte(str)); err != nil {
		return "", errors.Wrap(err, "unable to write hash")
	}
	return base36Truncate(hasher.Sum(nil)), nil
}

// base36Truncate returns a string that is base36 compliant
// It is not an encoding since it returns a same-length string
// for any byte value
func base36Truncate(bytes []byte) string {
	var chars string
	for _, bite := range bytes {
		idx := int(bite) % 36
		chars += string(base36set[idx])
	}

	return chars
}
