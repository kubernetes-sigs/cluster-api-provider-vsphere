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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// NewMachineContext returns a fake VIMMachineContext for unit testing
// reconcilers with a fake client.
func NewMachineContext(ctx *context.ClusterContext) *context.VIMMachineContext {
	// Create the machine resources.
	machine := newMachineV1a4()
	vsphereMachine := newVSphereMachine(machine)

	// Add the cluster resources to the fake cluster client.
	if err := ctx.Client.Create(ctx, &machine); err != nil {
		panic(err)
	}
	if err := ctx.Client.Create(ctx, &vsphereMachine); err != nil {
		panic(err)
	}

	return &context.VIMMachineContext{
		BaseMachineContext: &context.BaseMachineContext{
			ControllerContext: ctx.ControllerContext,
			Cluster:           ctx.Cluster,
			Machine:           &machine,
			Logger:            ctx.Logger.WithName(vsphereMachine.Name),
		},
		VSphereCluster: ctx.VSphereCluster,
		VSphereMachine: &vsphereMachine,
	}
}

func newMachineV1a4() clusterv1.Machine {
	return clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      Clusterv1a2Name,
			UID:       Clusterv1a2UUID,
		},
		Spec: clusterv1.MachineSpec{},
	}
}

func newVSphereMachine(owner clusterv1.Machine) infrav1.VSphereMachine {
	return infrav1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: owner.Namespace,
			Name:      owner.Name,
			UID:       VSphereMachineUUID,
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
			VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
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
		},
	}
}
