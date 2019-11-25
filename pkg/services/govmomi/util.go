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
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/net"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
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

// findVM searches for a VM in one of two ways:
//   1. If the ProviderID is available, then the VM is queried by its
//      BIOS UUID.
//   2. Lacking the ProviderID, the VM is queried by its instance UUID,
//      which was assigned the value of the Machine resource's UID string.
func findVM(ctx *context.MachineContext) (types.ManagedObjectReference, error) {
	if providerID := ctx.VSphereMachine.Spec.ProviderID; providerID != nil && *providerID != "" {
		uuid := util.ConvertProviderIDToUUID(providerID)
		if uuid == "" {
			return types.ManagedObjectReference{}, errors.Errorf("invalid providerID %s", *providerID)
		}
		objRef, err := ctx.Session.FindByBIOSUUID(ctx, uuid)
		if err != nil {
			return types.ManagedObjectReference{}, err
		}
		if objRef == nil {
			return types.ManagedObjectReference{}, errNotFound{uuid: uuid}
		}
		return objRef.Reference(), nil
	}

	uuid := string(ctx.Machine.UID)
	objRef, err := ctx.Session.FindByInstanceUUID(ctx, uuid)
	if err != nil {
		return types.ManagedObjectReference{}, err
	}
	if objRef == nil {
		return types.ManagedObjectReference{}, errNotFound{instanceUUID: true, uuid: uuid}
	}
	return objRef.Reference(), nil
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

func reconcileInFlightTask(ctx *context.MachineContext) (bool, error) {
	// Check to see if there is an in-flight task.
	task := getTask(ctx)

	// If no task was found then make sure to clear the VSphereMachine
	// resource's Status.TaskRef field.
	if task == nil {
		ctx.VSphereMachine.Status.TaskRef = ""
		return false, nil
	}

	// Otherwise the course of action is determined by the state of the task.
	logger := ctx.Logger.WithName(task.Reference().Value)
	logger.V(4).Info("task found", "state", task.Info.State, "description-id", task.Info.DescriptionId)
	switch task.Info.State {
	case types.TaskInfoStateQueued:
		logger.V(4).Info("task is still pending", "description-id", task.Info.DescriptionId)
		return true, nil
	case types.TaskInfoStateRunning:
		logger.V(4).Info("task is still running", "description-id", task.Info.DescriptionId)
		return true, nil
	case types.TaskInfoStateSuccess:
		logger.V(4).Info("task is a success", "description-id", task.Info.DescriptionId)
		ctx.VSphereMachine.Status.TaskRef = ""
		return false, nil
	case types.TaskInfoStateError:
		logger.V(2).Info("task failed", "description-id", task.Info.DescriptionId)
		ctx.VSphereMachine.Status.TaskRef = ""
		return false, nil
	default:
		return false, errors.Errorf("unknown task state %q for %q", task.Info.State, ctx)
	}
}
