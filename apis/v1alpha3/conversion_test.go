/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	nextver "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

//nolint:paralleltest
func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(nextver.AddToScheme(scheme)).To(Succeed())

	t.Run("for VSphereCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &nextver.VSphereCluster{},
		Spoke:       &VSphereCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideVSphereClusterDeprecatedFieldsFuncs},
	}))
	t.Run("for VSphereMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &nextver.VSphereMachine{},
		Spoke:  &VSphereMachine{},
	}))
	t.Run("for VSphereMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &nextver.VSphereMachineTemplate{},
		Spoke:       &VSphereMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{CustomObjectMetaFuzzFunc},
	}))
	t.Run("for VSphereVM", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &nextver.VSphereVM{},
		Spoke:  &VSphereVM{},
	}))
}

func overrideVSphereClusterDeprecatedFieldsFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(vsphereClusterSpec *VSphereClusterSpec, c fuzz.Continue) {
			vsphereClusterSpec.CloudProviderConfiguration = CPIConfig{}
		},
	}
}

func CustomObjectMetaFuzzFunc(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		CustomObjectMetaFuzzer,
	}
}

//nolint
func CustomObjectMetaFuzzer(in *clusterv1.ObjectMeta, c fuzz.Continue) {
	c.FuzzNoCustom(in)

	// These fields have been removed in v1alpha4
	// data is going to be lost, so we're forcing zero values here.
	in.Name = ""
	in.GenerateName = ""
	in.Namespace = ""
	in.OwnerReferences = nil
}
