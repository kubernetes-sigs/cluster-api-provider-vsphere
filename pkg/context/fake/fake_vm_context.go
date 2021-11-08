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
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// NewVMContext returns a fake VMContext for unit testing
// reconcilers with a fake client.
func NewVMContext(ctx *context.ControllerContext) *context.VMContext {
	// Create the resources.
	vsphereVM := newVSphereVM()

	// Add the resources to the fake client.
	if err := ctx.Client.Create(ctx, &vsphereVM); err != nil {
		panic(err)
	}

	helper, err := patch.NewHelper(&vsphereVM, ctx.Client)
	if err != nil {
		panic(err)
	}

	return &context.VMContext{
		ControllerContext: ctx,
		VSphereVM:         &vsphereVM,
		Logger:            ctx.Logger.WithName(vsphereVM.Name),
		PatchHelper:       helper,
	}
}

func newVSphereVM() infrav1.VSphereVM {
	return infrav1.VSphereVM{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      VSphereVMName,
			UID:       types.UID(VSphereVMUUID),
		},
		Spec: infrav1.VSphereVMSpec{
			VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
				Server:     "10.10.10.10",
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
