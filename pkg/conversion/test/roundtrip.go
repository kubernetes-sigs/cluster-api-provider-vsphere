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

// Package test provides test util for conversions.
package test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	conversionutil "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/randfill"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion"
	conversionmeta "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta"
)

// RoundTripTestInput contains input parameters
// for the RoundTripTest function.
type RoundTripTestInput struct {
	Scheme    *runtime.Scheme
	Converter *conversion.Converter

	CheckTypes RoundTripCheckTypesInput

	Hub   client.Object
	Spoke client.Object

	FuzzerFuncs []fuzzer.FuzzerFuncs
}

// RoundTripCheckTypesInput contains input parameters
// for checking the types before running round trip tests.
type RoundTripCheckTypesInput struct {
	// Skip allow to skip type checks entirely.
	// This provides an escape path to get test green when type checking cannot handle a difference between hub and spoke types.
	// Important! Use this flag only temporarily, because when this check is skipped there is no automatic check
	// of type modeling issue that might lead to data loss when creating patches for spoke types.
	Skip bool

	// Instruct the type checked about a field rename, e.g. "VirtualMachine.Status.NodeName": "Host",
	FieldNameMap map[string]string

	// Internal settings

	// fullMatchRequired is set to true to ensure that hub types nested under fields of type slice/array
	// fully match with the corresponding spoke type, thus preventing data loss.
	fullMatchRequired bool
}

// RoundTripTest returns a new testing function to be used in tests to make sure conversions between
// the Hub version of an object and an the corresponding Spoke version aren't lossy.
func RoundTripTest(input RoundTripTestInput) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()

		// Check types for differences that might lead to data loss when creating patches for spoke types,
		t.Run("Check types", func(t *testing.T) {
			if input.CheckTypes.Skip {
				t.Skip("skipping type check. Please check hub and spoke types manually to ensure no data loss happens when creating patches for spoke types")
			}
			g := gomega.NewWithT(t)

			hubT, err := objType(input.Hub)
			g.Expect(err).NotTo(gomega.HaveOccurred(), "failed to get type of the hub object")

			spokeT, err := objType(input.Spoke)
			g.Expect(err).NotTo(gomega.HaveOccurred(), "failed to get type of the spoke object")

			inspectTypes(t, hubT, spokeT, field.NewPath(hubT.Name()), input.CheckTypes)
		})

		// Perform round trip between types ensuring conversion are properly implemented.
		// Note: The test is checking only the hub-spoke-hub round trip because hub types have only a subset of spoke types,
		// and this would require specific fuzzer config for each type.
		t.Run("hub-spoke-hub", func(t *testing.T) {
			g := gomega.NewWithT(t)

			if _, isConvertible := input.Hub.(conversionmeta.Convertible); !isConvertible {
				g.Fail("Hub type must implement sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta/Convertible")
			}

			funcs := append(input.FuzzerFuncs, func(_ runtimeserializer.CodecFactory) []interface{} {
				return []interface{}{
					func(in *conversionmeta.SourceTypeMeta, _ randfill.Continue) {
						// Ensure SourceTypeMeta is not set by the fuzzer.
						in.APIVersion = ""
					},
				}
			})

			fuzzer := conversionutil.GetFuzzer(input.Scheme, funcs...)

			spokeGVK, err := input.Converter.GroupVersionKindFor(input.Spoke)
			if err != nil {
				t.Fatal(err.Error())
			}

			for range 10000 {
				// Create the hub and fuzz it
				hubBefore := input.Hub.DeepCopyObject()
				fuzzer.Fill(hubBefore)

				// First convert hub to spoke
				spoke := input.Spoke.DeepCopyObject()
				g.Expect(input.Converter.Convert(t.Context(), hubBefore, spoke)).To(gomega.Succeed(), "error calling Convert from hub to spoke")

				// Convert spoke back to hub and check if the resulting hub is equal to the hub before the round trip
				hubAfter := input.Hub.DeepCopyObject()
				g.Expect(input.Converter.Convert(t.Context(), spoke, hubAfter)).To(gomega.Succeed(), "error calling Convert from spoke to hub: %v")

				convertibleAfter, _ := hubAfter.(conversionmeta.Convertible)
				g.Expect(convertibleAfter.GetSource().APIVersion).To(gomega.Equal(spokeGVK.GroupVersion().String()), "Convert is expected to set Convertible.APIVersion")

				convertibleAfter.SetSource(conversionmeta.SourceTypeMeta{})

				if !apiequality.Semantic.DeepEqual(hubBefore, hubAfter) {
					diff := cmp.Diff(hubBefore, hubAfter)
					g.Expect(false).To(gomega.BeTrue(), diff)
				}
			}
		})
	}
}

var (
	timeT          = reflect.TypeFor[metav1.Time]()
	conditionT     = reflect.TypeFor[metav1.Condition]()
	labelSelectorT = reflect.TypeFor[metav1.LabelSelector]()
	quantityT      = reflect.TypeFor[resource.Quantity]()
)

func inspectTypes(t *testing.T, hubT reflect.Type, spokeT reflect.Type, path *field.Path, input RoundTripCheckTypesInput) {
	t.Helper()
	g := gomega.NewWithT(t)

	// If hub type is a pointer, inspect Elem types.
	// Note: Current logic assumes that when a field in hub is a pointer, also the corresponding spoke field is. This can be improved in the future.
	if hubT.Kind() == reflect.Ptr {
		g.Expect(spokeT.Kind()).To(gomega.Equal(reflect.Ptr), fmt.Sprintf("field %s is a pointer in hub, not in spoke", path.String()))
		inspectTypes(t, hubT.Elem(), spokeT.Elem(), path, input)
		return
	}

	t.Logf("Checking type for %s (%s)", path, hubT.String())

	// If hub type is one of metav1.(Time|Condition|LabelSelector) or resource.Quantity, do not check further.
	// Note: Current logic assumes that when a field in hub is one of the types above, also the corresponding spoke field is. This can be improved in the future.
	if hubT == timeT || hubT == conditionT || hubT == labelSelectorT || hubT == quantityT {
		g.Expect(spokeT).To(gomega.Equal(hubT), fmt.Sprintf("field %s has type %s in hub, %s in spoke", path.String(), hubT, spokeT))
		return
	}

	switch hubKind := hubT.Kind(); hubKind {
	case reflect.Map:
		// If hub type is a map, check both key and value types.
		// Note: Current logic assumes that when a field in hub is a map, also the corresponding spoke field is. This can be improved in the future.
		g.Expect(spokeT.Kind()).To(gomega.Equal(reflect.Map), fmt.Sprintf("field %s is a map in hub, not in spoke", path.String()))
		inspectTypes(t, hubT.Key(), spokeT.Key(), path.Key("$key"), input)
		inspectTypes(t, hubT.Elem(), spokeT.Elem(), path.Key("$elem"), input)
	case reflect.Array, reflect.Slice:
		// If hub type is a map, check items type.
		// Notably, starting from this type, a full match is required to prevent data loss.
		// Note: Current logic assumes that when a field in hub is a slice/array, also the corresponding spoke field is. This can be improved in the future.
		g.Expect(spokeT.Kind()).To(gomega.Equal(hubKind), fmt.Sprintf("field %s is a %s in hub, not in spoke", hubKind, path.String()))

		if !input.fullMatchRequired {
			t.Logf("Enforcing full match requirement for %s", path.Index(0))
			input.fullMatchRequired = true
		}
		inspectTypes(t, hubT.Elem(), spokeT.Elem(), path.Index(0), input)
	case reflect.Struct:
		// If hub type is a struct, checks fields in the hub type when they have a corresponding field in the spoke type.
		// Note: hub types are a superset of spoke types in different API versions.
		spokeFieldNames := sets.New[string]()
		for i := 0; i < hubT.NumField(); i++ {
			hubField := hubT.Field(i)
			hubFieldName := hubField.Name
			fieldPath := path.Child(hubFieldName)

			spokeFieldName := hubField.Name
			if mappedSpokeFieldName, ok := input.FieldNameMap[fieldPath.String()]; ok {
				spokeFieldName = mappedSpokeFieldName
			}
			spokeFieldNames.Insert(hubFieldName)

			// If the field is the TypeMeta, ObjectMeta or the Source field, do not check further.
			if path.Root().String() == path.String() {
				if hubFieldName == "TypeMeta" || hubFieldName == "ObjectMeta" || hubFieldName == "Source" {
					continue
				}
			}

			spokeField, found := spokeT.FieldByName(spokeFieldName)
			if !found {
				t.Logf("field %s not found in spoke type", fieldPath.String())
				continue
			}
			inspectTypes(t, hubField.Type, spokeField.Type, fieldPath, input)
		}

		// If a full match is required, also check the other way around: all the spoke fields have a corresponding field in hub.
		if input.fullMatchRequired {
			for i := 0; i < spokeT.NumField(); i++ {
				spokeField := spokeT.Field(i)
				spokeFieldName := spokeField.Name
				if spokeFieldNames.Has(spokeFieldName) {
					continue
				}
				g.Fail(fmt.Sprintf("Field %s not found in hub type", path.Child(spokeFieldName)))
			}
		}
	case reflect.Chan, reflect.Func, reflect.Interface:
		g.Fail(fmt.Sprintf("field %s has type %s which is not allowed in API types", path.String(), spokeT))
	default:
		// If is a scalar type, perform a check ensuring types in hub and spoke are equal.
		// Note: Current logic assumes that scalar types in hub are equal to spoke fields. This can be improved in the future.

		if hubT == spokeT {
			break
		}

		// Tolerate enums where only the type name is equal. e.g. hub.VirtualMachinePowerState is considered equal to v1alpha5.VirtualMachinePowerState.
		// TODO: implement checks to ensure that values for enum types in hub are a superset of corresponding values in spoke
		if hubT.Name() == spokeT.Name() {
			break
		}

		g.Expect(hubT).To(gomega.Equal(spokeT), fmt.Sprintf("field %s has type %s in hub, %s in spoke", path.String(), hubT, spokeT))
	}
}

func objType(obj runtime.Object) (reflect.Type, error) {
	if obj == nil {
		return nil, errors.New("all objects must be pointers to structs, got nil")
	}

	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Pointer {
		return nil, errors.Errorf("all objects must be pointers to structs, got %s", t.Kind())
	}
	t = t.Elem()
	if t.Kind() != reflect.Struct {
		return nil, errors.Errorf("all objects must be pointers to structs, got *%s", t.Kind())
	}
	return t, nil
}
