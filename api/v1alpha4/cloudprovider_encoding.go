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
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

// UnmarshalINIOptions defines the options used to influence how INI data is
// unmarshalled.
//
// +kubebuilder:object:generate=false
type UnmarshalINIOptions struct {
	// WarnAsFatal indicates that warnings that occur when unmarshalling INI
	// data should be treated as fatal errors.
	WarnAsFatal bool
}

// UnmarshalINIOptionFunc is used to set unmarshal options.
//
// +kubebuilder:object:generate=false
type UnmarshalINIOptionFunc func(*UnmarshalINIOptions)

// WarnAsFatal sets the option to treat warnings as fatal errors when
// unmarshalling INI data.
func WarnAsFatal(opts *UnmarshalINIOptions) {
	opts.WarnAsFatal = true
}

// IsEmpty returns true if an object is its empty value or if a struct, all of
// its fields are their empty values.
func IsEmpty(obj interface{}) bool {
	return isEmpty(reflect.ValueOf(obj))
}

// IsNotEmpty returns true when IsEmpty returns false.
func IsNotEmpty(obj interface{}) bool {
	return !IsEmpty(obj)
}

// isEmpty returns true if an object's fields are all set to their empty values.
func isEmpty(val reflect.Value) bool {
	switch val.Kind() {

	case reflect.Interface, reflect.Ptr:
		return val.IsNil() || isEmpty(val.Elem())

	case reflect.Struct:
		structIsEmpty := true
		for fieldIndex := 0; fieldIndex < val.NumField(); fieldIndex++ {
			if structIsEmpty = isEmpty(val.Field(fieldIndex)); !structIsEmpty {
				break
			}
		}
		return structIsEmpty

	case reflect.Array, reflect.String:
		return val.Len() == 0

	case reflect.Bool:
		return !val.Bool()

	case reflect.Map, reflect.Slice:
		return val.IsNil() || val.Len() == 0

	case reflect.Float32, reflect.Float64:
		return val.Float() == 0

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int() == 0

	default:
		panic(errors.Errorf("invalid kind: %s", val.Kind()))
	}
}

// MarshalCloudProviderArgs marshals the cloud provider arguments for passing
// into a pod spec
func (cpic *CPICloudConfig) MarshalCloudProviderArgs() []string {
	args := []string{
		"--v=2",
		"--cloud-provider=vsphere",
		"--cloud-config=/etc/cloud/vsphere.conf",
	}
	if cpic.ExtraArgs != nil {
		for k, v := range cpic.ExtraArgs {
			args = append(args, fmt.Sprintf("--%s=%s", k, v))
		}
	}
	return args
}
