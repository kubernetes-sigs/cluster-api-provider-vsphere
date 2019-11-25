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

package fake

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1a2 "sigs.k8s.io/cluster-api/api/v1alpha2"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// NewMachineContext returns a fake MachineContext for unit testing
// reconcilers with a fake client.
func NewMachineContext(ctx *context.ClusterContext) *context.MachineContext {

	// Create the machine resources.
	machine := newMachineV1a2()
	vsphereMachine := newVSphereMachine(machine)

	// Add the cluster resources to the fake cluster client.
	if err := ctx.Client.Create(ctx, &machine); err != nil {
		panic(err)
	}
	if err := ctx.Client.Create(ctx, &vsphereMachine); err != nil {
		panic(err)
	}

	return &context.MachineContext{
		ClusterContext: ctx,
		Machine:        &machine,
		VSphereMachine: &vsphereMachine,
		Logger:         ctx.Logger.WithName(vsphereMachine.Name),
	}
}

func newMachineV1a2() clusterv1a2.Machine {
	return clusterv1a2.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      Clusterv1a2Name,
			UID:       types.UID(Clusterv1a2UUID),
		},
		Spec: clusterv1a2.MachineSpec{},
	}
}

func newVSphereMachine(owner clusterv1a2.Machine) infrav1.VSphereMachine {
	return infrav1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: owner.Namespace,
			Name:      owner.Name,
			UID:       types.UID(VSphereMachineUUID),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         owner.APIVersion,
					Kind:               owner.Kind,
					Name:               owner.Name,
					UID:                owner.UID,
					BlockOwnerDeletion: &boolTrue,
					Controller:         &boolTrue,
				},
			},
		},
		Spec: infrav1.VSphereMachineSpec{
			Datacenter: "dc0",
			Network: infrav1.NetworkSpec{
				Devices: []infrav1.NetworkDeviceSpec{
					{
						NetworkName: "VM Network",
						DHCP4:       true,
						DHCP6:       true,
					},
				},
			},
			NumCPUs:   2,
			MemoryMiB: 2048,
			DiskGiB:   20,
		},
	}
}
