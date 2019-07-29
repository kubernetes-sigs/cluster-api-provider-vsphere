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
	"reflect"
	"text/template"

	"github.com/pkg/errors"
	gcfg "gopkg.in/gcfg.v1"
)

// MarshalINI marshals the cloud provider configuration to INI-style
// configuration data.
func (c *Config) MarshalINI() ([]byte, error) {
	t, err := template.New("t").Funcs(template.FuncMap{
		"IsNotEmpty": IsNotEmpty,
	}).Parse(configFormat)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse config template")
	}
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, c); err != nil {
		return nil, errors.Wrap(err, "failed to execute config template")
	}
	return buf.Bytes(), nil
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

/*
Please see the package documentation for why the MarshalINI function that
uses the "gopkg.in/go-ini/ini.v1" package is commented out.

// MarshalINI marshals the cloud provider configuration as an INI-style
// configuration.
func (c *Config) MarshalINI() ([]byte, error) {
	cfg := ini.Empty()
	if err := ini.ReflectFrom(cfg, c); err != nil {
		return nil, errors.Wrap(err, "failed to marshal cloud provider config to ini")
	}
	buf := &bytes.Buffer{}
	if _, err := cfg.WriteTo(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
*/
