/*
Copyright 2019 The Kubernetes Authors.

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

package services

import (
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// VirtualMachineService is a service for creating/updating/deleting virtual
// machines on vSphere.
type VirtualMachineService interface {
	// ReconcileVM reconciles a VM with the intended state.
	ReconcileVM(ctx *context.MachineContext) (infrav1.VirtualMachine, error)

	// DestroyVM powers off and removes a VM from the inventory.
	DestroyVM(ctx *context.MachineContext) (infrav1.VirtualMachine, error)
}
