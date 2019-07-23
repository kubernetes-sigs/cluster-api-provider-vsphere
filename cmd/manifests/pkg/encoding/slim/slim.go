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

package slim

import (
	"encoding/json"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// EncodeAsRawExtension encodes a runtime.Object as a *runtime.RawExtension
// and strips the encoded data of any keys with empty values.
func EncodeAsRawExtension(obj runtime.Object) (*runtime.RawExtension, error) {
	raw, err := MarshalJSON(obj)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{Raw: raw}, nil
}

// MarshalYAML marshals the provided object to YAML and strips the encoded data
// of any keys with empty values.
func MarshalYAML(obj runtime.Object) ([]byte, error) {
	objJSON, err := MarshalJSON(obj)
	if err != nil {
		return nil, err
	}
	return yaml.JSONToYAML(objJSON)
}

// MarshalJSON marshals the provided object to JSON and strips the encoded data
// of any keys with empty values.
func MarshalJSON(obj runtime.Object) ([]byte, error) {
	objJSON, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	objMap := map[string]interface{}{}
	if json.Unmarshal(objJSON, &objMap); err != nil {
		return nil, err
	}
	Strip(objMap)
	objJSONStripped, err := json.Marshal(objMap)
	if err != nil {
		return nil, err
	}
	return objJSONStripped, nil
}

// Strip recursively removes empty values from a map and its contents.
func Strip(data map[string]interface{}) {
	strip(reflect.ValueOf(data))
}

var emptyValue reflect.Value

func strip(val reflect.Value) reflect.Value {
	switch val.Kind() {

	case reflect.Interface, reflect.Ptr:
		// Interfaces or pointers are either nil and can be removed or
		// this function should operate on the underlying value.
		if val.IsNil() {
			return emptyValue
		}
		return strip(val.Elem())

	case reflect.Map:
		// If the map is nil or empty, indicate it can be deleted.
		if val.IsNil() || val.Len() == 0 {
			return emptyValue
		}

		// Otherwise iterate over the map and check to see if its
		// contents can be deleted.
		iter := val.MapRange()
		for iter.Next() {
			k, v := iter.Key(), iter.Value()
			val.SetMapIndex(k, strip(v))
		}

		// Since the map could possibly be empty after its contents have
		// been processed, the map's fate rests on its new length.
		if val.Len() == 0 {
			return emptyValue
		}

	case reflect.Slice:
		// If the slice is nil or empty, indicate it can be deleted.
		if val.IsNil() || val.Len() == 0 {
			return emptyValue
		}

		// Create a copy of the slice as the contents of the original
		// must be iterated and examined to see if one or more elements
		// should be removed.
		copyOfSlice := reflect.MakeSlice(val.Type(), 0, val.Cap())
		for i := 0; i < val.Len(); i++ {
			if sliceElem := strip(val.Index(i)); sliceElem != emptyValue {
				copyOfSlice = reflect.Append(copyOfSlice, sliceElem)
			}
		}

		// If the copied slice has no values then it should be deleted.
		// Otherwise return the copied slice to take the place of the old one.
		if copyOfSlice.Len() == 0 {
			return emptyValue
		}
		return copyOfSlice

	case reflect.Float64:
		// Golang encodes floating point, integer, and Number values as
		// 64-bit floating point values when marshaling to JSON.
		if val.Float() == 0 {
			return emptyValue
		}

	case reflect.String:
		// Remove empty strings.
		if val.Len() == 0 {
			return emptyValue
		}
	}

	return val
}
