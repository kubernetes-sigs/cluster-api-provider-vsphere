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

// Package conversion implements conversion utilities.
// Note: This package will be removed after the next CAPI bump and we'll use the util from there instead.
package conversion

import (
	"context"
	"maps"
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metafuzzer "k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/randfill"
)

const (
	// DataAnnotation is the annotation that conversion webhooks
	// use to retain the data in case of down-conversion from the hub.
	DataAnnotation = "cluster.x-k8s.io/conversion-data"
)

// GetFuzzer returns a new fuzzer to be used for testing.
func GetFuzzer(scheme *runtime.Scheme, funcs ...fuzzer.FuzzerFuncs) *randfill.Filler {
	funcs = append([]fuzzer.FuzzerFuncs{
		metafuzzer.Funcs,
		func(_ runtimeserializer.CodecFactory) []interface{} {
			return []interface{}{
				// Custom fuzzer for metav1.Time pointers which weren't
				// fuzzed and always resulted in `nil` values.
				// This implementation is somewhat similar to the one provided
				// in the metafuzzer.Funcs.
				func(input **metav1.Time, c randfill.Continue) {
					if c.Bool() {
						// Leave the Time sometimes nil to also get coverage for this case.
						return
					}
					if c.Bool() {
						// Set the Time sometimes empty to also get coverage for this case.
						*input = &metav1.Time{}
						return
					}
					var sec, nsec uint32
					c.Fill(&sec)
					c.Fill(&nsec)
					fuzzed := metav1.Unix(int64(sec), int64(nsec)).Rfc3339Copy()
					*input = &metav1.Time{Time: fuzzed.Time}
				},
				// Custom fuzzer for intstr.IntOrString which does not get fuzzed otherwise.
				func(in **intstr.IntOrString, c randfill.Continue) {
					if c.Bool() {
						// Leave the IntOrString sometimes nil to also get coverage for this case.
						return
					}
					if c.Bool() {
						// Set the IntOrString sometimes empty to also get coverage for this case.
						*in = &intstr.IntOrString{}
						return
					}
					*in = ptr.To(intstr.FromInt32(c.Int31n(50)))
				},
			}
		},
	}, funcs...)
	return fuzzer.FuzzerFor(
		fuzzer.MergeFuzzerFuncs(funcs...),
		rand.NewSource(rand.Int63()), //nolint:gosec
		runtimeserializer.NewCodecFactory(scheme),
	)
}

// SpokeConverterFuzzTestFuncInput contains input parameters
// for the SpokeConverterFuzzTestFunc function.
type SpokeConverterFuzzTestFuncInput[hubObject, spokeObject client.Object] struct {
	Scheme *runtime.Scheme

	HubAfterMutation func(hubObject)

	SpokeAfterMutation         func(spoke spokeObject)
	SkipSpokeAnnotationCleanup bool

	ConvertHubToSpokeFunc func(ctx context.Context, src hubObject, dst spokeObject) error
	ConvertSpokeToHubFunc func(ctx context.Context, src spokeObject, dst hubObject) error

	FuzzerFuncs []fuzzer.FuzzerFuncs
}

// SpokeConverterFuzzTestFunc returns a new testing function to be used in tests to make sure conversions between
// the Hub version of an object and an older version aren't lossy.
func SpokeConverterFuzzTestFunc[hubObject, spokeObject client.Object](input SpokeConverterFuzzTestFuncInput[hubObject, spokeObject]) func(*testing.T) {
	if input.Scheme == nil {
		input.Scheme = scheme.Scheme
	}

	return func(t *testing.T) {
		t.Helper()
		t.Run("spoke-hub-spoke", func(t *testing.T) {
			ctx := t.Context()
			g := gomega.NewWithT(t)
			fuzzer := GetFuzzer(input.Scheme, input.FuzzerFuncs...)

			for range 10000 {
				// Create the spoke and fuzz it
				spokeBefore := reflect.New(reflect.TypeOf(*new(spokeObject)).Elem()).Interface().(spokeObject)
				fuzzer.Fill(spokeBefore)

				// First convert spoke to hub
				hubCopy := reflect.New(reflect.TypeOf(*new(hubObject)).Elem()).Interface().(hubObject)
				g.Expect(input.ConvertSpokeToHubFunc(ctx, spokeBefore, hubCopy)).To(gomega.Succeed())

				// Convert hub back to spoke and check if the resulting spoke is equal to the spoke before the round trip
				spokeAfter := reflect.New(reflect.TypeOf(*new(spokeObject)).Elem()).Interface().(spokeObject)
				g.Expect(input.ConvertHubToSpokeFunc(ctx, hubCopy, spokeAfter)).To(gomega.Succeed())

				// Remove data annotation eventually added by ConvertFrom for avoiding data loss in hub-spoke-hub round trips
				// NOTE: There are use case when we want to skip this operation, e.g. if the spoke object does not have ObjectMeta (e.g. kubeadm types).
				if !input.SkipSpokeAnnotationCleanup {
					delete(spokeAfter.GetAnnotations(), DataAnnotation)
				}

				if input.SpokeAfterMutation != nil {
					input.SpokeAfterMutation(spokeAfter)
				}

				if !apiequality.Semantic.DeepEqual(spokeBefore, spokeAfter) {
					diff := cmp.Diff(spokeBefore, spokeAfter)
					g.Expect(false).To(gomega.BeTrue(), diff)
				}
			}
		})
		t.Run("hub-spoke-hub", func(t *testing.T) {
			ctx := t.Context()
			g := gomega.NewWithT(t)
			fuzzer := GetFuzzer(input.Scheme, input.FuzzerFuncs...)

			for range 10000 {
				// Create the hub and fuzz it
				hubBefore := reflect.New(reflect.TypeOf(*new(hubObject)).Elem()).Interface().(hubObject)
				fuzzer.Fill(hubBefore)

				// First convert hub to spoke
				dstCopy := reflect.New(reflect.TypeOf(*new(spokeObject)).Elem()).Interface().(spokeObject)
				// DeepCopy hubBefore because otherwise the mutations in MarshalDataUnsafeNoCopy would affect
				// hubBefore and accordingly the comparison between hubBefore and hubAfter below.
				g.Expect(input.ConvertHubToSpokeFunc(ctx, hubBefore.DeepCopyObject().(hubObject), dstCopy)).To(gomega.Succeed())

				// Sometimes the apiserver sends us objects without a spec (likely in the context of managedField conversion)
				// This test verifies that the ConvertTo code can handle this scenario (i.e. it doesn't return an error
				// and it doesn't panic)
				// Note: It's important that this test is run here, because ConvertSpokeToHubFunc below clears the restore annotation from dstCopy.
				dstCopyNoSpec := reflect.New(reflect.TypeOf(*new(spokeObject)).Elem()).Interface().(spokeObject)
				dstCopyNoSpec.SetLabels(maps.Clone(dstCopy.GetLabels()))
				dstCopyNoSpec.SetAnnotations(maps.Clone(dstCopy.GetAnnotations()))
				hubNoSpec := reflect.New(reflect.TypeOf(*new(hubObject)).Elem()).Interface().(hubObject)
				g.Expect(input.ConvertSpokeToHubFunc(ctx, dstCopyNoSpec, hubNoSpec)).To(gomega.Succeed())

				// Convert spoke back to hub and check if the resulting hub is equal to the hub before the round trip
				hubAfter := reflect.New(reflect.TypeOf(*new(hubObject)).Elem()).Interface().(hubObject)
				g.Expect(input.ConvertSpokeToHubFunc(ctx, dstCopy, hubAfter)).To(gomega.Succeed())

				if input.HubAfterMutation != nil {
					input.HubAfterMutation(hubAfter)
				}

				if !apiequality.Semantic.DeepEqual(hubBefore, hubAfter) {
					diff := cmp.Diff(hubBefore, hubAfter)
					g.Expect(false).To(gomega.BeTrue(), diff)
				}
			}
		})
	}
}
