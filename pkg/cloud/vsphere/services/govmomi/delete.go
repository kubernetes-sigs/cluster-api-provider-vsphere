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
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

// Delete deletes the machine from the backend platform.
func Delete(ctx *context.MachineContext) error {
	if ctx.MachineConfig.MachineRef == "" {
		return errors.Errorf("machine ref is empty while deleting machine %q", ctx)
	}

	moRef := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: ctx.MachineConfig.MachineRef,
	}

	var obj mo.VirtualMachine
	if err := ctx.Session.RetrieveOne(ctx, moRef, []string{"name", "runtime"}, &obj); err != nil {
		return errors.Errorf("machine does not exist %q", ctx)
	}

	vm := object.NewVirtualMachine(ctx.Session.Client.Client, moRef)
	if obj.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
		task, err := vm.PowerOff(ctx)
		if err != nil {
			return errors.Wrapf(err, "error triggering power off op on machine %q", ctx)
		}
		if err := task.Wait(ctx); err != nil {
			return errors.Wrapf(err, "error powering off machine %q", ctx)
		}
	}

	task, err := vm.Destroy(ctx)
	if err != nil {
		return errors.Wrapf(err, "error triggering delete op on machine %q", ctx)
	}

	taskInfo, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return errors.Wrapf(err, "error deleting machine %q", ctx)
	}

	if taskInfo.State != types.TaskInfoStateSuccess {
		return errors.Errorf("error deleting machine %q", ctx)
	}

	return nil
}
