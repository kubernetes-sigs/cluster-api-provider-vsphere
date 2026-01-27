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

package v1alpha2

import (
	"testing"

	vmoprv1alpha2 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/randfill"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/test"
)

func TestFuzzyConversion(t *testing.T) {
	converter := conversion.NewConverter()
	utilruntime.Must(vmoprvhub.AddToConverter(converter))
	utilruntime.Must(AddToConverter(converter))

	t.Run("for VirtualMachine", test.RoundTripTest(test.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachine{},
		Spoke:     &vmoprv1alpha2.VirtualMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{
			virtualMachineFuncs,
		},
		CheckTypes: test.RoundTripCheckTypesInput{
			FieldNameMap: map[string]string{
				"VirtualMachine.Status.NodeName": "Host",
			},
		},
	}))
	t.Run("for VirtualMachineClass", test.RoundTripTest(test.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineClass{},
		Spoke:     &vmoprv1alpha2.VirtualMachineClass{},
	}))
	t.Run("for VirtualMachineGroup", test.RoundTripTest(test.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineGroup{},
		Spoke:     &vmoprv1alpha2.VirtualMachineGroup{},
	}))
	t.Run("for VirtualMachineImage", test.RoundTripTest(test.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineImage{},
		Spoke:     &vmoprv1alpha2.VirtualMachineImage{},
	}))
	t.Run("for VirtualMachineService", test.RoundTripTest(test.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineService{},
		Spoke:     &vmoprv1alpha2.VirtualMachineService{},
	}))
	t.Run("for VirtualMachineSetResourcePolicy", test.RoundTripTest(test.RoundTripTestInput{
		Converter: converter,
		Hub:       &vmoprvhub.VirtualMachineSetResourcePolicy{},
		Spoke:     &vmoprv1alpha2.VirtualMachineSetResourcePolicy{},
	}))
}

func virtualMachineFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubPersistentVolumeClaimVolumeSource,
	}
}

func hubPersistentVolumeClaimVolumeSource(in *vmoprvhub.PersistentVolumeClaimVolumeSource, c randfill.Continue) {
	c.FillNoCustom(in)
	// Fields existing in hub but not in v1alpha2.PersistentVolumeClaim
	in.ApplicationType = ""
	in.ControllerBusNumber = nil
	in.ControllerType = ""
	in.DiskMode = ""
	in.SharingMode = ""
	in.UnitNumber = nil
	in.UnmanagedVolumeClaim = nil
}
