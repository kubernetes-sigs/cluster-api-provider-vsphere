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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
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

	Hub   client.Object
	Spoke client.Object

	FuzzerFuncs []fuzzer.FuzzerFuncs
}

// RoundTripTest returns a new testing function to be used in tests to make sure conversions between
// the Hub version of an object and an the corresponding Spoke version aren't lossy.
func RoundTripTest(input RoundTripTestInput) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()
		t.Run("hub-spoke-hub", func(t *testing.T) {
			g := gomega.NewWithT(t)

			if _, isConvertible := input.Hub.(conversionmeta.Convertible); !isConvertible {
				t.Fatal("Hub type must implement sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta/Convertible")
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
