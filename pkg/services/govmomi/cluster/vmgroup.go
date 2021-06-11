/*
Copyright 2021 The Kubernetes Authors.

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

package cluster

import (
	"context"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func AddVMToGroup(ctx computeClusterContext, clusterName, vmGroupName, vm string) error {
	ccr, err := ctx.GetSession().Finder.ClusterComputeResource(ctx, clusterName)
	if err != nil {
		return err
	}

	vms, err := listVMs(ctx, ccr, vmGroupName)
	if err != nil {
		return err
	}

	vmObj, err := ctx.GetSession().Finder.VirtualMachine(ctx, vm)
	if err != nil {
		return err
	}
	vms = append(vms, vmObj.Reference())

	info := &types.ClusterVmGroup{
		ClusterGroupInfo: types.ClusterGroupInfo{
			Name: vmGroupName,
		},
		Vm: vms,
	}
	spec := &types.ClusterConfigSpecEx{
		GroupSpec: []types.ClusterGroupSpec{
			{
				ArrayUpdateSpec: types.ArrayUpdateSpec{
					Operation: types.ArrayUpdateOperationEdit,
				},
				Info: info,
			},
		},
	}
	return reconfigure(ctx, ccr, spec)
}

func listVMs(ctx context.Context, ccr *object.ClusterComputeResource, vmGroupName string) ([]types.ManagedObjectReference, error) {
	clusterConfigInfoEx, err := ccr.Configuration(ctx)
	if err != nil {
		return nil, err
	}

	var refs []types.ManagedObjectReference
	for _, group := range clusterConfigInfoEx.Group {
		if clusterVMGroup, ok := group.(*types.ClusterVmGroup); ok {
			if clusterVMGroup.Name == vmGroupName {
				return clusterVMGroup.Vm, nil
			}
		}
	}
	return refs, nil
}
