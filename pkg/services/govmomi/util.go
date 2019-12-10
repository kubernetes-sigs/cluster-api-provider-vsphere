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
	"sigs.k8s.io/controller-runtime/pkg/event"

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
			ctx.Logger.V(4).Info("ignoring IP address", "reason", err.Error())
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
	if ctx.VSphereMachine.Status.TaskRef == "" {
		return nil
	}
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
		logger.Info("task is a success", "description-id", task.Info.DescriptionId)
		ctx.VSphereMachine.Status.TaskRef = ""
		return false, nil
	case types.TaskInfoStateError:
		logger.Info("task failed", "description-id", task.Info.DescriptionId)
		ctx.VSphereMachine.Status.TaskRef = ""
		return false, nil
	default:
		return false, errors.Errorf("unknown task state %q for %q", task.Info.State, ctx)
	}
}

func reconcileVSphereMachineWhenNetworkIsReady(
	ctx *virtualMachineContext,
	powerOnTask *object.Task) {

	reconcileVSphereMachineOnFuncCompletion(
		&ctx.MachineContext,
		func() ([]interface{}, error) {
			taskInfo, err := powerOnTask.WaitForResult(ctx)
			if err != nil && taskInfo == nil {
				return nil, errors.Wrapf(err, "failed to wait for power on op for vm %s", ctx)
			}
			powerState, err := ctx.Obj.PowerState(ctx)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get power state for vm %s", ctx)
			}
			if powerState != types.VirtualMachinePowerStatePoweredOn {
				return nil, errors.Errorf(
					"unexpected power state %v for vm %s",
					powerState, ctx)
			}
			if _, err := ctx.Obj.WaitForNetIP(ctx, false); err != nil {
				return nil, errors.Wrapf(err, "failed to wait for networking for vm %s", ctx)
			}
			return []interface{}{
				"reason", "network",
			}, nil
		})
}

func reconcileVSphereMachineOnTaskCompletion(ctx *context.MachineContext) {
	task := getTask(ctx)
	if task == nil {
		ctx.Logger.V(4).Info(
			"skipping reconcile VSphereMachine on task completion",
			"reason", "no-task")
		return
	}
	taskRef := task.Reference()
	taskHelper := object.NewTask(ctx.Session.Client.Client, taskRef)

	ctx.Logger.Info(
		"enqueuing reconcile request on task completion",
		"task-ref", taskRef,
		"task-name", task.Info.Name,
		"task-entity-name", task.Info.EntityName,
		"task-description-id", task.Info.DescriptionId)

	reconcileVSphereMachineOnFuncCompletion(ctx, func() ([]interface{}, error) {
		taskInfo, err := taskHelper.WaitForResult(ctx)

		// An error is only returned if the process of waiting for the result
		// failed, *not* if the task itself failed.
		if err != nil && taskInfo == nil {
			return nil, err
		}

		return []interface{}{
			"reason", "task",
			"task-ref", taskRef,
			"task-name", taskInfo.Name,
			"task-entity-name", taskInfo.EntityName,
			"task-state", taskInfo.State,
			"task-description-id", taskInfo.DescriptionId,
		}, nil
	})
}

func reconcileVSphereMachineOnFuncCompletion(
	ctx *context.MachineContext,
	waitFn func() (loggerKeysAndValues []interface{}, _ error)) {

	obj := ctx.VSphereMachine.DeepCopy()
	gvk := obj.GetObjectKind().GroupVersionKind()

	// Wait on the function to complete in a background goroutine.
	go func() {
		loggerKeysAndValues, err := waitFn()
		if err != nil {
			ctx.Logger.Error(err, "failed to wait on func")
			return
		}

		// Once the task has completed (successfully or otherwise), trigger
		// a reconcile event for the associated resource by sending a
		// GenericEvent into the event channel for the resource type.
		ctx.Logger.Info("triggering GenericEvent", loggerKeysAndValues...)
		eventChannel := ctx.GetGenericEventChannelFor(gvk)
		eventChannel <- event.GenericEvent{
			Meta:   obj,
			Object: obj,
		}
	}()
}
