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
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/net"
)

func sanitizeIPAddrs(ctx *context.MachineContext, ipAddrs []string) []string {
	if len(ipAddrs) == 0 {
		return nil
	}
	newIPAddrs := []string{}
	for _, addr := range ipAddrs {
		if err := net.ErrOnLocalOnlyIPAddr(addr); err != nil {
			ctx.Logger.V(8).Info("ignoring IP address", "reason", err.Error())
		} else {
			newIPAddrs = append(newIPAddrs, addr)
		}
	}
	return newIPAddrs
}

func findVMByInstanceUUID(ctx *context.MachineContext) (string, error) {
	ctx.Logger.V(6).Info("finding vm by instance UUID", "instance-uuid", ctx.Machine.UID)
	ref, err := ctx.Session.FindByInstanceUUID(ctx, string(ctx.Machine.UID))
	if err != nil {
		return "", err
	}
	if ref != nil {
		ctx.Logger.V(6).Info("found vm by instance UUID", "instance-uuid", ctx.Machine.UID)
		return ref.Reference().Value, nil
	}
	return "", nil
}

func getTask(ctx *context.MachineContext) *mo.Task {
	var obj mo.Task
	moRef := types.ManagedObjectReference{
		Type:  morefTypeTask,
		Value: ctx.VSphereMachine.Status.TaskRef,
	}
	if err := ctx.Session.RetrieveOne(ctx, moRef, []string{"info"}, &obj); err != nil {
		return nil
	}
	return &obj
}

func hasInFlightTask(ctx *context.MachineContext) (bool, error) {
	// Check to see if there is an in-flight task.
	if task := getTask(ctx); task == nil {
		// no task associated
		ctx.VSphereMachine.Status.TaskRef = ""
	} else {
		// check if the status of task is in a favourable state
		// If task is in completed or error state we can process further.
		// if task is in other states requeue
		ctx := context.NewMachineLoggerContext(ctx, task.Reference().Value)

		ctx.Logger.V(4).Info("task found", "state", task.Info.State, "description-id", task.Info.DescriptionId)
		switch task.Info.State {
		case types.TaskInfoStateQueued:
			ctx.Logger.V(4).Info("task is still pending", "description-id", task.Info.DescriptionId)
			return true, nil
		case types.TaskInfoStateRunning:
			ctx.Logger.V(4).Info("task is still running", "description-id", task.Info.DescriptionId)
			return true, nil
		case types.TaskInfoStateSuccess:
			ctx.Logger.V(4).Info("task is a success", "description-id", task.Info.DescriptionId)
			ctx.VSphereMachine.Status.TaskRef = ""
			return false, nil
		case types.TaskInfoStateError:
			ctx.Logger.V(2).Info("task failed", "description-id", task.Info.DescriptionId)
			ctx.VSphereMachine.Status.TaskRef = ""
			return false, nil
		default:
			return false, errors.Errorf("unknown task state %q for %q", task.Info.State, ctx)
		}
	}

	return false, nil
}

func getMoRef(ctx *context.MachineContext) *types.ManagedObjectReference {
	if ctx.VSphereMachine.Spec.MachineRef != "" {
		return &types.ManagedObjectReference{
			Type:  "VirtualMachine",
			Value: ctx.VSphereMachine.Spec.MachineRef,
		}
	}

	return nil
}

func getVMfromMachineRef(ctx *context.MachineContext) (*object.VirtualMachine, error) {
	ref := getMoRef(ctx)
	if ref == nil {
		return nil, errors.Errorf("machine ref not set")
	}

	return object.NewVirtualMachine(ctx.Session.Client.Client, *ref), nil
}

func getVMObject(ctx *context.MachineContext) (mo.VirtualMachine, error) {
	moRef := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: ctx.VSphereMachine.Spec.MachineRef,
	}
	var obj mo.VirtualMachine
	err := ctx.Session.RetrieveOne(ctx, moRef, []string{"name"}, &obj)
	return obj, err
}
