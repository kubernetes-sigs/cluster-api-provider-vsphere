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

package conversion

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ConverterBuilder collects functions that add things to a converter. It's to allow
// code to compile without explicitly referencing generated types.
type ConverterBuilder []func(converter *Converter) error

// AddToConverter applies all the stored functions to the converter. A non-nil error
// indicates that one function failed and the attempt was abandoned.
func (sb *ConverterBuilder) AddToConverter(s *Converter) error {
	for _, f := range *sb {
		if err := f(s); err != nil {
			return err
		}
	}
	return nil
}

// AddTypes adds to the Converter types that require conversion.
func (sb *ConverterBuilder) AddTypes(gv schema.GroupVersion, types ...runtime.Object) {
	*sb = append(*sb, func(s *Converter) error {
		return s.AddTypes(gv, types...)
	})
}

// AddConversion adds to the Converter functions to be used when converting objects from one version to another.
func (sb *ConverterBuilder) AddConversion(hub runtime.Object, version string, spoke runtime.Object, srcToDst, dstToSrc any) {
	*sb = append(*sb, func(s *Converter) error {
		return s.AddConversion(hub, version, spoke, srcToDst, dstToSrc)
	})
}

// NewConverterBuilder returns a ConverterBuilder.
func NewConverterBuilder(fs ...func(s *Converter) error) ConverterBuilder {
	cb := ConverterBuilder{}
	for _, f := range fs {
		cb = append(cb, f)
	}
	return cb
}
