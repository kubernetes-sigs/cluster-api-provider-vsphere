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

package v1beta1

import (
	"reflect"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/randfill"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta2"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(vmwarev1.AddToScheme(scheme)).To(Succeed())

	t.Run("for VSphereCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &vmwarev1.VSphereCluster{},
		Spoke:       &VSphereCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereClusterFuzzFuncs},
	}))
	t.Run("for VSphereClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &vmwarev1.VSphereClusterTemplate{},
		Spoke:       &VSphereClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereClusterTemplateFuzzFuncs},
	}))
	t.Run("for VSphereMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &vmwarev1.VSphereMachine{},
		Spoke:       &VSphereMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereMachineFuzzFuncs},
	}))
	t.Run("for VSphereMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &vmwarev1.VSphereMachineTemplate{},
		Spoke:       &VSphereMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereMachineTemplateFuzzFuncs},
	}))
	t.Run("for ProviderServiceAccount", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &vmwarev1.ProviderServiceAccount{},
		Spoke:       &ProviderServiceAccount{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{ProviderServiceAccountFuzzFuncs},
	}))
}

func VSphereClusterFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereClusterStatus,
		spokeVSphereClusterStatus,
	}
}

func hubVSphereClusterStatus(in *vmwarev1.VSphereClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &vmwarev1.VSphereClusterV1Beta1DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeVSphereClusterStatus(in *VSphereClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &VSphereClusterV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}

func VSphereClusterTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}

func VSphereMachineFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereMachineStatus,
		spokeVSphereMachineStatus,
	}
}

func hubVSphereMachineStatus(in *vmwarev1.VSphereMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &vmwarev1.VSphereMachineV1Beta1DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeVSphereMachineStatus(in *VSphereMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &VSphereMachineV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}

func VSphereMachineTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}

func ProviderServiceAccountFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}
