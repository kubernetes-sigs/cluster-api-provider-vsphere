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
	"time"

	"github.com/vmware/govmomi/vim25/types"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

// Exists returns a flag indicating whether or not a machine exists.
func Exists(ctx *context.MachineContext) (bool, error) {
	// Check to see if the VM exists.
	vm, err := findVM(ctx)
	if err != nil {
		return false, err
	}
	if vm != nil {
		return true, nil
	}

	// If there is no task reference then the VM cannot be determined to exist.
	if ctx.MachineStatus.TaskRef == "" {
		return false, nil
	}

	// Update the logger with the task ID.
	ctx = context.NewMachineLoggerContext(ctx, ctx.MachineStatus.TaskRef)

	// Check to see if a task exists.
	task := getTask(ctx)
	if task == nil {
		ctx.Logger.V(4).Info("task does not exist")
		ctx.MachineStatus.TaskRef = ""
		return false, nil
	}

	// Since a task was discovered, let's find out if it indicates a VM is
	// being, or has been, created/cloned.
	ctx.Logger.V(4).Info("task found", "state", task.Info.State)
	switch task.Info.State {
	case types.TaskInfoStateQueued:
		ctx.Logger.V(4).Info("task is still pending")
	case types.TaskInfoStateRunning:
		ctx.Logger.V(4).Info("task is still running")
	case types.TaskInfoStateSuccess:
		ctx.Logger.V(4).Info("task is a success")
		if ref, ok := task.Info.Result.(types.ManagedObjectReference); ok {
			if ref.Type == "VirtualMachine" {
				ctx.MachineStatus.TaskRef = ""
				ctx.MachineConfig.MachineRef = ref.Value
			}
		}
		// Remove the machine reference if this was a destroy task.
		if task.Info.DescriptionId == taskVMDestroy {
			ctx.MachineConfig.MachineRef = ""
		}
	case types.TaskInfoStateError:
		ctx.Logger.V(2).Info("task failed", "description-id", task.Info.DescriptionId)
		ctx.MachineStatus.TaskRef = ""
	}

	// If a MachineRef was discovered or there is still a TaskRef set on
	// the machine, then requeue this method so the next time it is called it
	// can use the MachineRef or TaskRef to try and figure out the state of
	// the VM.
	if ctx.MachineConfig.MachineRef != "" || ctx.MachineStatus.TaskRef != "" {
		ctx.Logger.V(6).Info("reenqueue to wait for VM exists check")
		return false, &clustererror.RequeueAfterError{RequeueAfter: time.Second * 5}
	}

	// No VM could be found.
	return false, nil
}
