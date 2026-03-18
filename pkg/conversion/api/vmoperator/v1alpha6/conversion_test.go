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

package v1alpha6

import (
	"testing"

	vmoprv1alpha6 "github.com/vmware-tanzu/vm-operator/api/v1alpha6"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/randfill"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
	conversiontest "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/test"
)

func TestFuzzyConversion(t *testing.T) {
	converter := conversion.NewConverter(func(_ schema.GroupKind) (string, error) {
		return vmoprv1alpha6.GroupVersion.Version, nil
	})
	utilruntime.Must(vmoprvhub.AddToConverter(converter))
	utilruntime.Must(AddToConverter(converter))

	t.Run("for VirtualMachine", conversiontest.RoundTripTest(conversiontest.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachine{},
		Spoke:     &vmoprv1alpha6.VirtualMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{
			virtualMachineFuncs,
		},
		CheckTypes: conversiontest.RoundTripCheckTypesInput{
			// Volume fields (Removable, ApplicationType, ControllerBusNumber, etc.) are at
			// different structural levels in hub (PersistentVolumeClaimVolumeSource) vs spoke
			// (VirtualMachineVolume). Skip type check until this is reconciled.
			Skip: true,
		},
	}))
	t.Run("for VirtualMachineClass", conversiontest.RoundTripTest(conversiontest.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineClass{},
		Spoke:     &vmoprv1alpha6.VirtualMachineClass{},
	}))
	t.Run("for VirtualMachineGroup", conversiontest.RoundTripTest(conversiontest.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineGroup{},
		Spoke:     &vmoprv1alpha6.VirtualMachineGroup{},
	}))
	t.Run("for VirtualMachineImage", conversiontest.RoundTripTest(conversiontest.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineImage{},
		Spoke:     &vmoprv1alpha6.VirtualMachineImage{},
	}))
	t.Run("for VirtualMachineService", conversiontest.RoundTripTest(conversiontest.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineService{},
		Spoke:     &vmoprv1alpha6.VirtualMachineService{},
	}))
	t.Run("for VirtualMachineSetResourcePolicy", conversiontest.RoundTripTest(conversiontest.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineSetResourcePolicy{},
		Spoke:     &vmoprv1alpha6.VirtualMachineSetResourcePolicy{},
	}))
	t.Run("for ClusterVirtualMachineImage", conversiontest.RoundTripTest(conversiontest.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.ClusterVirtualMachineImage{},
		Spoke:     &vmoprv1alpha6.ClusterVirtualMachineImage{},
	}))
}

func virtualMachineFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubPersistentVolumeClaimVolumeSource,
	}
}

func hubPersistentVolumeClaimVolumeSource(in *vmoprvhub.PersistentVolumeClaimVolumeSource, c randfill.Continue) {
	c.FillNoCustom(in)
	in.UnmanagedVolumeClaim = nil
}
