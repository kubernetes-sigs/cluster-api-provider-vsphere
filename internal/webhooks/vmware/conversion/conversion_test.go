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

package conversion

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/randfill"

	vmwarev1beta1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api-provider-vsphere/internal/conversion"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(vmwarev1beta1.AddToScheme(scheme)).To(Succeed())
	g.Expect(vmwarev1.AddToScheme(scheme)).To(Succeed())

	t.Run("for VSphereCluster", utilconversion.SpokeConverterFuzzTestFunc(utilconversion.SpokeConverterFuzzTestFuncInput[*vmwarev1.VSphereCluster, *vmwarev1beta1.VSphereCluster]{
		Scheme:                scheme,
		ConvertSpokeToHubFunc: ConvertVSphereClusterV1Beta1ToHub,
		ConvertHubToSpokeFunc: ConvertVSphereClusterHubToV1Beta1,
		FuzzerFuncs:           []fuzzer.FuzzerFuncs{VSphereClusterFuzzFuncs},
	}))
	t.Run("for VSphereClusterTemplate", utilconversion.SpokeConverterFuzzTestFunc(utilconversion.SpokeConverterFuzzTestFuncInput[*vmwarev1.VSphereClusterTemplate, *vmwarev1beta1.VSphereClusterTemplate]{
		Scheme:                scheme,
		ConvertSpokeToHubFunc: ConvertVSphereClusterTemplateV1Beta1ToHub,
		ConvertHubToSpokeFunc: ConvertVSphereClusterTemplateHubToV1Beta1,
		FuzzerFuncs:           []fuzzer.FuzzerFuncs{VSphereClusterTemplateFuzzFuncs},
	}))
	t.Run("for VSphereMachine", utilconversion.SpokeConverterFuzzTestFunc(utilconversion.SpokeConverterFuzzTestFuncInput[*vmwarev1.VSphereMachine, *vmwarev1beta1.VSphereMachine]{
		Scheme:                scheme,
		ConvertSpokeToHubFunc: ConvertVSphereMachineV1Beta1ToHub,
		ConvertHubToSpokeFunc: ConvertVSphereMachineHubToV1Beta1,
		FuzzerFuncs:           []fuzzer.FuzzerFuncs{VSphereMachineFuzzFuncs},
	}))
	t.Run("for VSphereMachineTemplate", utilconversion.SpokeConverterFuzzTestFunc(utilconversion.SpokeConverterFuzzTestFuncInput[*vmwarev1.VSphereMachineTemplate, *vmwarev1beta1.VSphereMachineTemplate]{
		Scheme:                scheme,
		ConvertSpokeToHubFunc: ConvertVSphereMachineTemplateV1Beta1ToHub,
		ConvertHubToSpokeFunc: ConvertVSphereMachineTemplateHubToV1Beta1,
		FuzzerFuncs:           []fuzzer.FuzzerFuncs{VSphereMachineTemplateFuzzFuncs},
	}))
	t.Run("for ProviderServiceAccount", utilconversion.SpokeConverterFuzzTestFunc(utilconversion.SpokeConverterFuzzTestFuncInput[*vmwarev1.ProviderServiceAccount, *vmwarev1beta1.ProviderServiceAccount]{
		Scheme:                scheme,
		ConvertSpokeToHubFunc: ConvertProviderServiceAccountV1Beta1ToHub,
		ConvertHubToSpokeFunc: ConvertProviderServiceAccountHubToV1Beta1,
		FuzzerFuncs:           []fuzzer.FuzzerFuncs{ProviderServiceAccountFuzzFuncs},
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

func spokeVSphereClusterStatus(in *vmwarev1beta1.VSphereClusterStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &vmwarev1beta1.VSphereClusterV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}

	in.ResourcePolicyName = "" // Field does not exist in v1beta1.
}

func VSphereClusterTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereClusterTemplateResource,
	}
}

func hubVSphereClusterTemplateResource(in *vmwarev1.VSphereClusterTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)

	in.ObjectMeta = clusterv1.ObjectMeta{} // Field does not exist in v1beta1.
}

func VSphereMachineFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereMachineStatus,
		spokeVSphereMachineSpec,
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

	if c.Bool() {
		phaseValues := []vmwarev1.VSphereMachinePhase{
			vmwarev1.VSphereMachinePhaseNotFound,
			vmwarev1.VSphereMachinePhaseCreated,
			vmwarev1.VSphereMachinePhasePoweredOn,
			vmwarev1.VSphereMachinePhasePending,
			vmwarev1.VSphereMachinePhaseReady,
			vmwarev1.VSphereMachinePhaseDeleting,
			vmwarev1.VSphereMachinePhaseError,
		}
		in.Phase = phaseValues[c.Int31n(int32(len(phaseValues)))]
	} else {
		in.Phase = ""
	}
}

func spokeVSphereMachineSpec(in *vmwarev1beta1.VSphereMachineSpec, c randfill.Continue) {
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

	if in.NamingStrategy != nil && reflect.DeepEqual(in.NamingStrategy, &vmwarev1beta1.VirtualMachineNamingStrategy{}) {
		in.NamingStrategy = nil
	}

	in.FailureDomain = nil // field has been dropped in v1beta2
}

func spokeVSphereMachineStatus(in *vmwarev1beta1.VSphereMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	// Drop empty structs with only omit empty fields.
	if in.V1Beta2 != nil {
		if reflect.DeepEqual(in.V1Beta2, &vmwarev1beta1.VSphereMachineV1Beta2Status{}) {
			in.V1Beta2 = nil
		}
	}

	if in.ID != nil && *in.ID == "" {
		in.ID = nil
	}

	in.IPAddr = "" // IPAddr has been removed in v1beta2.

	if c.Bool() {
		vmStatusValues := []vmwarev1beta1.VirtualMachineState{
			vmwarev1beta1.VirtualMachineStateNotFound,
			vmwarev1beta1.VirtualMachineStateCreated,
			vmwarev1beta1.VirtualMachineStatePoweredOn,
			vmwarev1beta1.VirtualMachineStatePending,
			vmwarev1beta1.VirtualMachineStateReady,
			vmwarev1beta1.VirtualMachineStateDeleting,
			vmwarev1beta1.VirtualMachineStateError,
		}
		in.VMStatus = vmStatusValues[c.Int31n(int32(len(vmStatusValues)))]
	} else {
		in.VMStatus = ""
	}
}

func VSphereMachineTemplateFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubVSphereMachineTemplateResource,
		spokeVSphereMachineSpec,
	}
}

func hubVSphereMachineTemplateResource(in *vmwarev1.VSphereMachineTemplateResource, c randfill.Continue) {
	c.FillNoCustom(in)

	in.ObjectMeta = clusterv1.ObjectMeta{} // Field does not exist in v1beta1.
}

func ProviderServiceAccountFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{}
}
