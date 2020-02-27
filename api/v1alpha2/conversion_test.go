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

package v1alpha2

import (
	"testing"

	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1alpha3.AddToScheme(scheme)).To(Succeed())

	t.Run("for VSphereCluster", utilconversion.FuzzTestFunc(scheme, &v1alpha3.VSphereCluster{}, &VSphereCluster{}))
	t.Run("for VSphereMachine", utilconversion.FuzzTestFunc(scheme, &v1alpha3.VSphereMachine{}, &VSphereMachine{}))
	t.Run("for VSphereMachineTemplate", utilconversion.FuzzTestFunc(scheme, &v1alpha3.VSphereMachineTemplate{}, &VSphereMachineTemplate{}))
}
