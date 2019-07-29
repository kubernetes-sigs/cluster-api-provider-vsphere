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

package cloud

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"github.com/pkg/errors"
	gcfg "gopkg.in/gcfg.v1"
)

const gcfgTag = "gcfg"

// MarshalINI marshals the cloud provider configuration to INI-style
// configuration data.
func (c *Config) MarshalINI() ([]byte, error) {
	if c == nil {
		return nil, errors.New("config is nil")
	}

	buf := &bytes.Buffer{}

	// Get the reflected type and value of the Config object.
	configValue := reflect.ValueOf(*c)
	configType := reflect.TypeOf(*c)

	for fieldIndex := 0; fieldIndex < configValue.NumField(); fieldIndex++ {
		fieldValue := configValue.Field(fieldIndex)
		fieldType := configType.Field(fieldIndex)

		// Do not proceed if the field is empty.
		if isEmpty(fieldValue) {
			continue
		}

		// Get the name of the section by inspecting the field's gcfg tag.
		sectionName, sectionNameOk := fieldType.Tag.Lookup(gcfgTag)
		if !sectionNameOk {
			return nil, errors.Errorf("field %q is missing tag %q", fieldType.Name, gcfgTag)
		}

		switch fieldValue.Kind() {
		case reflect.Map:
			iter := fieldValue.MapRange()
			for iter.Next() {
				mapKey, mapVal := iter.Key(), iter.Value()
				sectionName := fmt.Sprintf(`%s "%v"`, sectionName, mapKey.String())
				c.marshalINISectionProperties(buf, mapVal, sectionName)
			}
		default:
			c.marshalINISectionProperties(buf, fieldValue, sectionName)
		}
	}

	return buf.Bytes(), nil
}

func (c *Config) marshalINISectionProperties(
	out io.Writer,
	sectionValue reflect.Value,
	sectionName string) error {

	sectionKind := sectionValue.Kind()
	if sectionKind == reflect.Interface || sectionKind == reflect.Ptr {
		return c.marshalINISectionProperties(out, sectionValue.Elem(), sectionName)
	}

	fmt.Fprintf(out, "[%s]\n", sectionName)

	sectionType := sectionValue.Type()
	for fieldIndex := 0; fieldIndex < sectionType.NumField(); fieldIndex++ {
		fieldType := sectionType.Field(fieldIndex)
		propertyName, propertyNameOk := fieldType.Tag.Lookup(gcfgTag)
		if !propertyNameOk {
			return errors.Errorf("field %q is missing tag %q", fieldType.Name, gcfgTag)
		}
		propertyValue := sectionValue.Field(fieldIndex)
		if isEmpty(propertyValue) {
			continue
		}
		propertyKind := propertyValue.Kind()
		if propertyKind == reflect.Interface || propertyKind == reflect.Ptr {
			propertyValue = propertyValue.Elem()
		}
		fmt.Fprintf(out, "%s = %v\n", propertyName, propertyValue.Interface())
	}
	return nil
}

// UnmarshalOptions defines the options used to influence how INI data is
// unmarshalled.
type UnmarshalOptions struct {
	// WarnAsFatal indicates that warnings that occur when unmarshalling INI
	// data should be treated as fatal errors.
	WarnAsFatal bool
}

// UnmarshalOptionFunc is used to set unmarshal options.
type UnmarshalOptionFunc func(*UnmarshalOptions)

// WarnAsFatal sets the option to treat warnings as fatal errors when
// unmarshalling INI data.
func WarnAsFatal(opts *UnmarshalOptions) {
	opts.WarnAsFatal = true
}

// UnmarshalINI unmarshals the cloud provider configuration from INI-style
// configuration data.
func (c *Config) UnmarshalINI(data []byte, optFuncs ...UnmarshalOptionFunc) error {
	opts := &UnmarshalOptions{}
	for _, setOpts := range optFuncs {
		setOpts(opts)
	}
	if err := gcfg.ReadStringInto(c, string(data)); err != nil {
		if opts.WarnAsFatal {
			return err
		}
		if err := gcfg.FatalOnly(err); err != nil {
			return err
		}
	}
	return nil
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
