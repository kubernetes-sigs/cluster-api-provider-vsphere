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

package v1alpha4

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
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
		Scheme: scheme,
		Hub:    &nextver.VSphereCluster{},
		Spoke:  &VSphereCluster{},
	}))
	t.Run("for VSphereMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &nextver.VSphereMachine{},
		Spoke:       &VSphereMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideVirtualMachineCloneSpecDeprecatedFieldsFuncs},
	}))
	t.Run("for VSphereMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &nextver.VSphereMachineTemplate{},
		Spoke:       &VSphereMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideVirtualMachineCloneSpecDeprecatedFieldsFuncs},
	}))
	t.Run("for VSphereVM", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &nextver.VSphereVM{},
		Spoke:       &VSphereVM{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideVirtualMachineCloneSpecDeprecatedFieldsFuncs},
	}))
}

func overrideVirtualMachineCloneSpecDeprecatedFieldsFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(virtualMachineCloneSpec *nextver.VirtualMachineCloneSpec, c fuzz.Continue) {
			c.FuzzNoCustom(virtualMachineCloneSpec)
			virtualMachineCloneSpec.TagIDs = nil
			//nolint:staticcheck // deprecated field
			virtualMachineCloneSpec.AdditionalDisksGiB = nil
			virtualMachineCloneSpec.Disks = nil
		},
	}
}
