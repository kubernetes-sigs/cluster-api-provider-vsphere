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

// Package hub has test hub types.
package hub

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	conversionmeta "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta"
)

// A test type.
type A struct {
	Foo string

	Source conversionmeta.SourceTypeMeta
}

// GetObjectKind implements runtime.Object.
func (in A) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

// DeepCopyObject implements runtime.Object.
func (in A) DeepCopyObject() runtime.Object {
	panic("implement me")
}

// GetSource returns the Source for this object.
func (in *A) GetSource() conversionmeta.SourceTypeMeta {
	return in.Source
}

// SetSource sets Source for an API object.
func (in *A) SetSource(source conversionmeta.SourceTypeMeta) {
	in.Source = source
}

// AList test type.
type AList struct{}

// GetObjectKind implements runtime.Object.
func (in AList) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

// DeepCopyObject implements runtime.Object.
func (in AList) DeepCopyObject() runtime.Object {
	panic("implement me")
}
