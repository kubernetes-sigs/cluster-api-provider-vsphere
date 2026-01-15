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

// Package meta defines metadata for the hub version of supervisor's API objects.
package meta

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

// SourceTypeMeta defines type meta of the source of this object.
type SourceTypeMeta struct {
	// APIVersion defines the versioned schema of this representation of an object.
	// Servers should convert recognized schemas to the latest internal value, and
	// may reject unrecognized values.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
	// +optional
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,2,opt,name=apiVersion"`
}

var (
	fieldName = "Source"
	fieldType = reflect.TypeOf(SourceTypeMeta{})
)

// HasSource return true if the object has a Source field with type SourceTypeMeta.
func HasSource(obj any) bool {
	if obj == nil {
		return false
	}

	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Ptr {
		return false
	}
	t = t.Elem()

	if t.Kind() != reflect.Struct {
		return false
	}

	field, found := t.FieldByName(fieldName)
	if !found {
		return false
	}

	return field.Type == fieldType
}

// GetSource gets SourceTypeMeta value from the object Source field.
func GetSource(obj any) (*SourceTypeMeta, error) {
	field, err := getFieldVal(obj)
	if err != nil {
		return nil, err
	}

	val := field.Interface().(SourceTypeMeta)
	return &val, nil
}

// SetSource sets SourceTypeMeta value in the object Source field.
func SetSource(obj any, source SourceTypeMeta) error {
	field, err := getFieldVal(obj)
	if err != nil {
		return err
	}

	if !field.CanSet() {
		return fmt.Errorf("cannot set field %s (it might be unexported)", fieldName)
	}

	val := reflect.ValueOf(source)
	field.Set(val)
	return nil
}

func getFieldVal(obj any) (*reflect.Value, error) {
	if obj == nil {
		return nil, errors.New("all objects must be pointers to structs, got nil")
	}

	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr {
		return nil, errors.Errorf("all objects must be pointers to structs, got %s", v.Kind())
	}
	v = v.Elem()

	if v.Kind() != reflect.Struct {
		return nil, errors.Errorf("all objects must be pointers to structs, got *%s", v.Kind())
	}

	fieldVal := v.FieldByName(fieldName)
	if !fieldVal.IsValid() {
		return nil, errors.Errorf("field %s not found", fieldName)
	}

	if fieldVal.Type() != fieldType {
		return nil, errors.Errorf("field %s is type %v, not %s.%s", fieldName, fieldVal.Type(), fieldType.PkgPath(), fieldType.Name())
	}

	return &fieldVal, nil
}
