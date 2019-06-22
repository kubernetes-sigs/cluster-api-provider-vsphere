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
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"
)

// Delete deletes the machine from the backend platform.
func Delete(ctx *context.MachineContext) error {

	// Check to see if the VM exists first since no error is returned if the VM
	// does not exist, only when there's an error checking or when the op should
	// be requeued, like when the VM has an in-flight task.
	if _, err := Exists(ctx); err != nil {
		return err
	}

	// Try to get the VM. If it does not exist, return success.
	vm := getVM(ctx)
	if vm == nil {
		return nil
	}

	// If the VM is powered on then power it off, store the power off task's
	// reference, and and requeue this operation.
	powerState, err := vm.PowerState(ctx)
	if err != nil {
		return errors.Wrapf(err, "error determining power state when deleting %q", ctx)
	}
	ctx.Logger.V(6).Info("powering off vm")
	if powerState == types.VirtualMachinePowerStatePoweredOn {
		task, err := vm.PowerOff(ctx)
		if err != nil {
			return errors.Wrapf(err, "error triggering power off op for %q", ctx)
		}
		ctx.MachineStatus.TaskRef = task.Reference().Value
		ctx.Logger.V(6).Info("reenqueue to wait for power off op")
		return &clustererror.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	// At this point the VM is not powered on and can be destroyed. Store the
	// destroy task's reference and return a requeue error.
	ctx.Logger.V(6).Info("destroying vm")
	task, err := vm.Destroy(ctx)
	if err != nil {
		return errors.Wrapf(err, "error triggering destroy for %q", ctx)
	}
	ctx.MachineStatus.TaskRef = task.Reference().Value
	ctx.Logger.V(6).Info("reenqueue to wait for destroy op")
	return &clustererror.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
}
