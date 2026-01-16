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
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

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
	return []interface{}{}
}

func VSphereClusterTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}

func VSphereClusterIdentityFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}

func VSphereDeploymentZoneFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}

func VSphereFailureDomainFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}

func VSphereMachineFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}

func VSphereMachineTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}

func ProviderServiceAccountFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}
