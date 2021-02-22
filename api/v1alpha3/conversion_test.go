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

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	nextver "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(nextver.AddToScheme(scheme)).To(Succeed())

	t.Run("for VSphereCluster", utilconversion.FuzzTestFunc(scheme, &nextver.VSphereCluster{}, &VSphereCluster{}))
	t.Run("for VSphereCluster", utilconversion.FuzzTestFunc(scheme, &nextver.VSphereClusterList{}, &VSphereClusterList{}))
	t.Run("for VSphereMachine", utilconversion.FuzzTestFunc(scheme, &nextver.VSphereMachine{}, &VSphereMachine{}))
	t.Run("for VSphereMachineList", utilconversion.FuzzTestFunc(scheme, &nextver.VSphereMachineList{}, &VSphereMachineList{}))
	t.Run("for VSphereMachineTemplate", utilconversion.FuzzTestFunc(scheme, &nextver.VSphereMachineTemplate{}, &VSphereMachineTemplate{}))
	t.Run("for VSphereMachineTemplateList", utilconversion.FuzzTestFunc(scheme, &nextver.VSphereMachineTemplateList{}, &VSphereMachineTemplateList{}))
	t.Run("for VSphereVM", utilconversion.FuzzTestFunc(scheme, &nextver.VSphereVM{}, &VSphereVM{}))
	t.Run("for VSphereVMList", utilconversion.FuzzTestFunc(scheme, &nextver.VSphereVMList{}, &VSphereVMList{}))
}
