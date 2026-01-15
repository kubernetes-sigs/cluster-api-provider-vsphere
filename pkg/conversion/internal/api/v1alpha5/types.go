/*
Copyright 2026 The Kubernetes Authors.

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

// Package v1alpha5 has test hub types.
package v1alpha5

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/internal/api/hub"
)

// A test type.
type A struct {
	Foo string
}

// GetObjectKind implements runtime.Object.
func (a A) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

// DeepCopyObject implements runtime.Object.
func (a A) DeepCopyObject() runtime.Object {
	panic("implement me")
}

// AList test type.
type AList struct{}

// GetObjectKind implements runtime.Object.
func (a AList) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

// DeepCopyObject implements runtime.Object.
func (a AList) DeepCopyObject() runtime.Object {
	panic("implement me")
}

// ConvertAFromHubToV1alpha5 converts A from hub to v1alpha5.
func ConvertAFromHubToV1alpha5(src *hub.A, dst *A) error {
	dst.Foo = src.Foo
	return nil
}

// ConvertAFromV1alpha5ToHub converts A from v1alpha5 to hub.
func ConvertAFromV1alpha5ToHub(src *A, dst *hub.A) error {
	dst.Foo = src.Foo
	return nil
}
