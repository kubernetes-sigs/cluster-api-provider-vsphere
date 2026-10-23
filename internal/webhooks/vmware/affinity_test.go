/*
Copyright The Kubernetes Authors.

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

package vmware

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apivalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	vmwarev1beta1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
)

func TestAffinitySerialization(t *testing.T) {
	for _, version := range []struct {
		name    string
		newSpec func() interface{}
	}{
		{"v1beta1", func() interface{} { return &vmwarev1beta1.VSphereMachineSpec{} }},
		{"v1beta2", func() interface{} { return &vmwarev1.VSphereMachineSpec{} }},
	} {
		t.Run(version.name, func(t *testing.T) {
			for _, input := range []string{
				`{}`,
				`{"affinity":{}}`,
				`{"affinity":{"vmAntiAffinity":{}}}`,
				`{"affinity":{"vmAntiAffinity":{"preferredDuringSchedulingPreferredDuringExecution":[]}}}`,
				`{"affinity":{"vmAntiAffinity":{"preferredDuringSchedulingPreferredDuringExecution":[{"topologyKey":"kubernetes.io/hostname"}]}}}`,
				`{"affinity":{"vmAntiAffinity":{"requiredDuringSchedulingPreferredDuringExecution":[]}}}`,
				`{"affinity":{"vmAntiAffinity":{"requiredDuringSchedulingRequiredDuringExecution":[]}}}`,
				`{"affinity":{"vmAntiAffinity":{"requiredDuringSchedulingRequiredDuringExecution":[],"requiredDuringSchedulingPreferredDuringExecution":[],"preferredDuringSchedulingPreferredDuringExecution":[]}}}`,
				`{"affinity":{"vmAntiAffinity":{"requiredDuringSchedulingRequiredDuringExecution":[{"labelSelector":{},"topologyKey":"kubernetes.io/hostname"}]}}}`,
				`{"affinity":{"vmAntiAffinity":{"requiredDuringSchedulingPreferredDuringExecution":[{"labelSelector":{},"topologyKey":"kubernetes.io/hostname"}]}}}`,
				`{"affinity":{"vmAntiAffinity":{"preferredDuringSchedulingPreferredDuringExecution":[{"labelSelector":{},"topologyKey":"kubernetes.io/hostname"}]}}}`,
			} {
				t.Run(input, func(t *testing.T) {
					spec := version.newSpec()
					if err := json.Unmarshal([]byte(input), spec); err != nil {
						t.Fatal(err)
					}
					data, err := json.Marshal(spec)
					if err != nil {
						t.Fatal(err)
					}
					var got, original map[string]interface{}
					if err := json.Unmarshal(data, &got); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal([]byte(input), &original); err != nil {
						t.Fatal(err)
					}
					var want interface{}
					if affinity, ok := original["affinity"].(map[string]interface{}); ok {
						if anti, ok := affinity["vmAntiAffinity"].(map[string]interface{}); ok {
							for _, terms := range anti {
								if terms != nil {
									want = affinity
								}
							}
						}
					}
					if !reflect.DeepEqual(got["affinity"], want) {
						t.Fatalf("affinity after JSON round trip = %#v, want %#v", got["affinity"], want)
					}
					unstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(spec)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(unstructured["affinity"], want) {
						t.Fatalf("unstructured affinity = %#v, want %#v", unstructured["affinity"], want)
					}
				})
			}
		})
	}
}

// Validate the generated schemas, since MinItems is enforced by the API server.
func TestAffinitySchemaValidation(t *testing.T) {
	machines, err := os.ReadFile("../../../config/supervisor/crd/bases/vmware.infrastructure.cluster.x-k8s.io_vspheremachines.yaml")
	if err != nil {
		t.Fatal(err)
	}
	templates, err := os.ReadFile("../../../config/supervisor/crd/bases/vmware.infrastructure.cluster.x-k8s.io_vspheremachinetemplates.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range []struct {
		name string
		data []byte
	}{
		{name: "vspheremachines", data: machines},
		{name: "vspheremachinetemplates", data: templates},
	} {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := yaml.UnmarshalStrict(resource.data, crd); err != nil {
			t.Fatal(err)
		}
		for _, version := range crd.Spec.Versions {
			t.Run(resource.name+"/"+version.Name, func(t *testing.T) {
				schema := version.Schema.OpenAPIV3Schema.Properties["spec"]
				if resource.name == "vspheremachinetemplates" {
					schema = schema.Properties["template"].Properties["spec"]
				}
				specValidator := newAffinitySchemaValidator(t, schema)
				spec := map[string]interface{}{"imageName": "test-image", "className": "test-class"}
				if errs := apivalidation.ValidateCustomResource(field.NewPath("spec"), spec, specValidator); len(errs) > 0 {
					t.Fatalf("omitted affinity must be accepted: %v", errs)
				}
				for _, affinity := range []interface{}{
					map[string]interface{}{},
					map[string]interface{}{"vmAntiAffinity": map[string]interface{}{}},
				} {
					spec["affinity"] = affinity
					if errs := apivalidation.ValidateCustomResource(field.NewPath("spec"), spec, specValidator); len(errs) > 0 {
						t.Fatalf("explicit empty affinity wrapper must be accepted: %v", errs)
					}
				}
				schema = schema.Properties["affinity"].Properties["vmAntiAffinity"]
				validator := newAffinitySchemaValidator(t, schema)
				for _, name := range []string{
					"preferredDuringSchedulingPreferredDuringExecution",
					"requiredDuringSchedulingPreferredDuringExecution",
					"requiredDuringSchedulingRequiredDuringExecution",
				} {
					t.Run(name, func(t *testing.T) {
						if _, ok := schema.Properties[name]; !ok {
							t.Fatalf("missing schema for %s", name)
						}
						empty := map[string]interface{}{name: []interface{}{}}
						if errs := apivalidation.ValidateCustomResource(field.NewPath("vmAntiAffinity"), empty, validator); len(errs) > 0 {
							t.Fatalf("explicit empty list must be accepted: %v", errs)
						}
						populated := map[string]interface{}{name: []interface{}{map[string]interface{}{
							"labelSelector": map[string]interface{}{},
							"topologyKey":   "kubernetes.io/hostname",
						}}}
						if errs := apivalidation.ValidateCustomResource(field.NewPath("vmAntiAffinity"), populated, validator); len(errs) > 0 {
							t.Fatalf("nonempty list must be accepted: %v", errs)
						}
						for _, missingField := range []string{"labelSelector", "topologyKey"} {
							for _, explicitNull := range []bool{false, true} {
								term := map[string]interface{}{
									"labelSelector": map[string]interface{}{},
									"topologyKey":   "kubernetes.io/hostname",
								}
								if explicitNull {
									term[missingField] = nil
								} else {
									delete(term, missingField)
								}
								invalid := map[string]interface{}{name: []interface{}{term}}
								if errs := apivalidation.ValidateCustomResource(field.NewPath("vmAntiAffinity"), invalid, validator); len(errs) == 0 {
									t.Fatalf("missing or null %s must be rejected (explicit null: %t)", missingField, explicitNull)
								}
							}
						}
						for _, topologyKey := range []string{"", "topology.kubernetes.io/zone", "custom-key"} {
							invalid := map[string]interface{}{name: []interface{}{map[string]interface{}{
								"labelSelector": map[string]interface{}{},
								"topologyKey":   topologyKey,
							}}}
							if errs := apivalidation.ValidateCustomResource(field.NewPath("vmAntiAffinity"), invalid, validator); len(errs) == 0 {
								t.Fatalf("unsupported topologyKey %q must be rejected", topologyKey)
							}
						}
					})
				}
			})
		}
	}
}

func newAffinitySchemaValidator(t *testing.T, schema apiextensionsv1.JSONSchemaProps) apivalidation.SchemaValidator {
	t.Helper()
	internalSchema := &apiextensions.JSONSchemaProps{}
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&schema, internalSchema, nil); err != nil {
		t.Fatal(err)
	}
	validator, _, err := apivalidation.NewSchemaValidator(internalSchema)
	if err != nil {
		t.Fatal(err)
	}
	return validator
}
