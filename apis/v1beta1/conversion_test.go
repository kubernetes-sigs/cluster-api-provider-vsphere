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
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereClusterTemplateFuzzFuncs},
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
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereDeploymentZoneFuzzFuncs},
	}))
	t.Run("for VSphereFailureDomain", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereFailureDomain{},
		Spoke:       &VSphereFailureDomain{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereFailureDomainFuzzFuncs},
	}))
	t.Run("for VSphereMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereMachine{},
		Spoke:       &VSphereMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereMachineFuzzFuncs},
	}))
	t.Run("for VSphereMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereMachineTemplate{},
		Spoke:       &VSphereMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereMachineTemplateFuzzFuncs},
	}))
	t.Run("for VSphereVM", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.VSphereVM{},
		Spoke:       &VSphereVM{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{VSphereVMFuzzFuncs},
	}))
}

func VSphereClusterFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereClusterStatus,
		hubVSphereFailureDomain,
		spokeVSphereClusterSpec,
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

	if len(in.FailureDomains) > 0 {
		in.FailureDomains = nil // Remove all pre-existing potentially invalid FailureDomains
		for i := range c.Int31n(20) {
			in.FailureDomains = append(in.FailureDomains,
				clusterv1.FailureDomain{
					Name:         fmt.Sprintf("%d-%s", i, c.String(255)), // Ensure valid unique non-empty names.
					ControlPlane: ptr.To(c.Bool()),
				},
			)
		}
		// The Cluster controller always ensures alphabetic sorting when writing this field.
		slices.SortFunc(in.FailureDomains, func(a, b clusterv1.FailureDomain) int {
			if a.Name < b.Name {
				return -1
			}
			return 1
		})
	}
}

func hubVSphereFailureDomain(in *clusterv1.FailureDomain, c randfill.Continue) {
	c.FillNoCustom(in)

	in.ControlPlane = ptr.To(c.Bool())
}

func spokeVSphereClusterSpec(in *VSphereClusterSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.IdentityRef != nil && reflect.DeepEqual(in.IdentityRef, &VSphereIdentityReference{}) {
		in.IdentityRef = nil
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
	return []interface{}{
		hubVSphereClusterTemplateResource,
		spokeVSphereClusterSpec,
	}
}

func hubVSphereClusterTemplateResource(in *infrav1.VSphereClusterTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)

	in.ObjectMeta = clusterv1.ObjectMeta{} // Field does not exist in v1beta1.
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

func VSphereDeploymentZoneFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereDeploymentZoneStatus,
		spokeVSphereDeploymentZoneStatus,
	}
}

func hubVSphereDeploymentZoneStatus(in *infrav1.VSphereDeploymentZoneStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &infrav1.VSphereDeploymentZoneV1Beta1DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeVSphereDeploymentZoneStatus(in *VSphereDeploymentZoneStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &VSphereDeploymentZoneV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}
}

func VSphereFailureDomainFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeVSphereFailureDomainSpec,
	}
}

func spokeVSphereFailureDomainSpec(in *VSphereFailureDomainSpec, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.Topology.Hosts != nil {
		if reflect.DeepEqual(in.Topology.Hosts, &FailureDomainHosts{}) {
			in.Topology.Hosts = nil
		}
	}

	if in.Topology.ComputeCluster != nil && *in.Topology.ComputeCluster == "" {
		in.Topology.ComputeCluster = nil
	}
}

func VSphereMachineFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereMachineStatus,
		spokeVSphereMachineSpec,
	}
}

func hubVSphereMachineStatus(in *infrav1.VSphereMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.Deprecated != nil {
		if in.Deprecated.V1Beta1 == nil || reflect.DeepEqual(in.Deprecated.V1Beta1, &infrav1.VSphereMachineV1Beta1DeprecatedStatus{}) {
			in.Deprecated = nil
		}
	}
}

func spokeVSphereMachineSpec(in *VSphereMachineSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.ProviderID != nil && *in.ProviderID == "" {
		in.ProviderID = nil
	}

	if in.FailureDomain != nil && *in.FailureDomain == "" {
		in.FailureDomain = nil
	}

	if in.NamingStrategy != nil && in.NamingStrategy.Template != nil && *in.NamingStrategy.Template == "" {
		in.NamingStrategy.Template = nil
	}

	if in.GuestSoftPowerOffTimeout != nil {
		in.GuestSoftPowerOffTimeout = ptr.To[metav1.Duration](metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second})
	}
}

func VSphereMachineTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeVSphereMachineSpec,
	}
}

func VSphereVMFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		spokeVSphereVMSpec,
	}
}

func spokeVSphereVMSpec(in *VSphereVMSpec, c randfill.Continue) {
	c.FillNoCustom(in)

	if in.GuestSoftPowerOffTimeout != nil {
		in.GuestSoftPowerOffTimeout = ptr.To[metav1.Duration](metav1.Duration{Duration: time.Duration(c.Int31()) * time.Second})
	}
}
