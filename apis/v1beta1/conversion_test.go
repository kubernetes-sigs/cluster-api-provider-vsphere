/*
Copyright 2025 The Kubernetes Authors.

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

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta2"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())

	t.Run("for VSphereCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereCluster{},
		Spoke:       &VSphereCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereClusterFuzzFuncs},
	}))
	t.Run("for VSphereClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereClusterTemplate{},
		Spoke:       &VSphereClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for VSphereClusterIdentity", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereClusterIdentity{},
		Spoke:       &VSphereClusterIdentity{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereClusterIdentityFuzzFuncs},
	}))
	t.Run("for VSphereDeploymentZone", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereDeploymentZone{},
		Spoke:       &VSphereDeploymentZone{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for VSphereFailureDomain", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereFailureDomain{},
		Spoke:       &VSphereFailureDomain{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for VSphereMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereMachine{},
		Spoke:       &VSphereMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for VSphereMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereMachineTemplate{},
		Spoke:       &VSphereMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
	t.Run("for VSphereVM", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereVM{},
		Spoke:       &VSphereVM{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{},
	}))
}

func VSphereClusterFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereClusterStatus,
		spokeVSphereClusterStatus,
	}
}

func hubVSphereClusterStatus(in *infrav1.VSphereClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &infrav1.VSphereClusterV1Beta1DeprecatedStatus{}) {
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

func VSphereClusterIdentityFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereClusterIdentityStatus,
		spokeVSphereClusterIdentityStatus,
	}
}

func hubVSphereClusterIdentityStatus(in *infrav1.VSphereClusterIdentityStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &infrav1.VSphereClusterIdentityV1Beta1DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeVSphereClusterIdentityStatus(in *VSphereClusterIdentityStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &VSphereClusterIdentityV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}
