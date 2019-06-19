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

package govmomi

import (
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

// Exists returns a flag indicating whether or not a machine exists.
func Exists(ctx *context.MachineContext) (bool, error) {
	if ctx.MachineConfig.MachineRef == "" {
		ctx.Logger.V(6).Info("exists is false due to lack of machine ref")
		return false, nil
	}

	moRef := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: ctx.MachineConfig.MachineRef,
	}

	var obj mo.VirtualMachine
	if err := ctx.Session.RetrieveOne(ctx, moRef, []string{"name"}, &obj); err != nil {
		ctx.Logger.V(6).Info("exists is false due to lookup failure")
		return false, nil
	}

	return true, nil
}
