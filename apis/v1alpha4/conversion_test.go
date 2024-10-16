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

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())

	t.Run("for VSphereCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &infrav1.VSphereCluster{},
		Spoke:  &VSphereCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{
			overrideVSphereClusterSpecFieldsFuncs,
			overrideVSphereClusterStatusFieldsFuncs,
		},
	}))
	t.Run("for VSphereMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereMachine{},
		Spoke:       &VSphereMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{CustomNewFieldFuzzFunc},
	}))
	t.Run("for VSphereMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereMachineTemplate{},
		Spoke:       &VSphereMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{CustomNewFieldFuzzFunc},
	}))
	t.Run("for VSphereVM", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereVM{},
		Spoke:       &VSphereVM{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{CustomNewFieldFuzzFunc},
	}))
}

func overrideVSphereClusterSpecFieldsFuncs(runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(in *infrav1.VSphereClusterSpec, c fuzz.Continue) {
			c.FuzzNoCustom(in)
			in.ClusterModules = nil
			in.FailureDomainSelector = nil
			in.DisableClusterModule = false
		},
	}
}

func overrideVSphereClusterStatusFieldsFuncs(runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(in *infrav1.VSphereClusterStatus, c fuzz.Continue) {
			c.FuzzNoCustom(in)
			in.VCenterVersion = ""
		},
	}
}

func CustomNewFieldFuzzFunc(runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		CustomSpecNewFieldFuzzer,
		CustomStatusNewFieldFuzzer,
	}
}

func CustomSpecNewFieldFuzzer(in *infrav1.VirtualMachineCloneSpec, c fuzz.Continue) {
	c.FuzzNoCustom(in)

	in.PciDevices = nil
	in.AdditionalDisksGiB = nil
	in.OS = ""
	in.HardwareVersion = ""
}

func CustomStatusNewFieldFuzzer(in *infrav1.VSphereVMStatus, c fuzz.Continue) {
	c.FuzzNoCustom(in)

	in.Host = ""
	in.ModuleUUID = nil
	in.VMRef = ""
}
