/*
Copyright 2022 The Kubernetes Authors.

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

package fake

import (
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
)

type vmService struct {
	fakeMachine infrav1.VirtualMachine
	err         error
}

func NewVMServiceWithVM(fakeMachine infrav1.VirtualMachine) services.VirtualMachineService {
	return vmService{
		fakeMachine: fakeMachine,
	}
}

func (v vmService) ReconcileVM(_ *context.VMContext) (infrav1.VirtualMachine, error) {
	return v.fakeMachine, v.err
}

func (v vmService) DestroyVM(ctx *context.VMContext) (infrav1.VirtualMachine, error) {
	return v.fakeMachine, v.err
}
