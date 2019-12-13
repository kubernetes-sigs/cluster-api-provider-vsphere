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
	"encoding/base64"

	"github.com/pkg/errors"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/extra"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/net"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// VMService provdes API to interact with the VMs using govmomi
type VMService struct{}

// ReconcileVM makes sure that the VM is in the desired state by:
//   1. Creating the VM if it does not exist, then...
//   2. Updating the VM with the bootstrap data, such as the cloud-init meta and user data, before...
//   3. Powering on the VM, and finally...
//   4. Returning the real-time state of the VM to the caller
func (vms *VMService) ReconcileVM(ctx *context.MachineContext) (vm infrav1.VirtualMachine, _ error) {

	// Initialize the result.
	vm = infrav1.VirtualMachine{
		Name:  ctx.Machine.Name,
		State: infrav1.VirtualMachineStatePending,
	}

	// If there is an in-flight task associated with this VM then do not
	// reconcile the VM until the task is completed.
	if inFlight, err := reconcileInFlightTask(ctx); err != nil || inFlight {
		return vm, err
	}

	// This deferred function will trigger a reconcile event for the
	// VSphereMachine resource once its associated task completes. If
	// there is no task for the VSphereMachine resource then no reconcile
	// event is triggered.
	defer reconcileVSphereMachineOnTaskCompletion(ctx)

	// Before going further, we need the VM's managed object reference.
	vmRef, err := findVM(ctx)
	if err != nil {
		if !isNotFound(err) {
			return vm, err
		}
		// If VM's MoRef could not be found then the VM does not exist,
		// and the VM should be created.
		return vm, createVM(ctx, []byte(*ctx.Machine.Spec.Bootstrap.Data))
	}

	//
	// At this point we know the VM exists, so it needs to be updated.
	//

	// Create a new virtualMachineContext to reconcile the VM.
	vmCtx := &virtualMachineContext{
		MachineContext: *ctx,
		Obj:            object.NewVirtualMachine(ctx.Session.Client.Client, vmRef),
		Ref:            vmRef,
		State:          &vm,
	}

	if err := vms.reconcileUUID(vmCtx); err != nil {
		return vm, err
	}

	if err := vms.reconcileNetworkStatus(vmCtx); err != nil {
		return vm, nil
	}

	if ok, err := vms.reconcileMetadata(vmCtx); err != nil || !ok {
		return vm, err
	}

	if ok, err := vms.reconcilePowerState(vmCtx); err != nil || !ok {
		return vm, err
	}

	vm.State = infrav1.VirtualMachineStateReady
	return vm, nil
}

// DestroyVM powers off and destroys a virtual machine.
func (vms *VMService) DestroyVM(ctx *context.MachineContext) (infrav1.VirtualMachine, error) {

	vm := infrav1.VirtualMachine{
		Name:  ctx.Machine.Name,
		State: infrav1.VirtualMachineStatePending,
	}

	// If there is an in-flight task associated with this VM then do not
	// reconcile the VM until the task is completed.
	if inFlight, err := reconcileInFlightTask(ctx); err != nil || inFlight {
		return vm, err
	}

	// This deferred function will trigger a reconcile event for the
	// VSphereMachine resource once its associated task completes. If
	// there is no task for the VSphereMachine resource then no reconcile
	// event is triggered.
	defer reconcileVSphereMachineOnTaskCompletion(ctx)

	// Before going further, we need the VM's managed object reference.
	vmRef, err := findVM(ctx)
	if err != nil {
		// If the VM's MoRef could not be found then the VM no longer exists. This
		// is the desired state.
		if isNotFound(err) {
			vm.State = infrav1.VirtualMachineStateNotFound
			return vm, nil
		}
		return vm, err
	}

	//
	// At this point we know the VM exists, so it needs to be destroyed.
	//

	// Create a new virtualMachineContext to reconcile the VM.
	vmCtx := &virtualMachineContext{
		MachineContext: *ctx,
		Obj:            object.NewVirtualMachine(ctx.Session.Client.Client, vmRef),
		Ref:            vmRef,
		State:          &vm,
	}

	// Power off the VM.
	powerState, err := vms.getPowerState(vmCtx)
	if err != nil {
		return vm, err
	}
	if powerState == infrav1.VirtualMachinePowerStatePoweredOn {
		task, err := vmCtx.Obj.PowerOff(ctx)
		if err != nil {
			return vm, err
		}
		ctx.VSphereMachine.Status.TaskRef = task.Reference().Value
		ctx.Logger.Info("wait for VM to be powered off")
		return vm, nil
	}

	// At this point the VM is not powered on and can be destroyed. Store the
	// destroy task's reference and return a requeue error.
	ctx.Logger.Info("destroying vm")
	task, err := vmCtx.Obj.Destroy(ctx)
	if err != nil {
		return vm, err
	}
	ctx.VSphereMachine.Status.TaskRef = task.Reference().Value
	ctx.Logger.Info("wait for VM to be destroyed")
	return vm, nil
}

func (vms *VMService) reconcileNetworkStatus(ctx *virtualMachineContext) error {
	netStatus, err := vms.getNetworkStatus(ctx)
	if err != nil {
		return err
	}
	ctx.State.Network = netStatus
	return nil
}

func (vms *VMService) reconcileMetadata(ctx *virtualMachineContext) (bool, error) {
	existingMetadata, err := vms.getMetadata(ctx)
	if err != nil {
		return false, err
	}

	newMetadata, err := util.GetMachineMetadata(ctx.Machine.Name, *ctx.VSphereMachine, ctx.State.Network...)
	if err != nil {
		return false, err
	}

	// If the metadata is the same then return early.
	if string(newMetadata) == existingMetadata {
		return true, nil
	}

	ctx.Logger.Info("updating metadata")
	taskRef, err := vms.setMetadata(ctx, newMetadata)
	if err != nil {
		return false, errors.Wrapf(err, "unable to set metadata on vm %s", ctx)
	}

	ctx.VSphereMachine.Status.TaskRef = taskRef
	ctx.Logger.Info("wait for VM metadata to be updated")
	return false, nil
}

func (vms *VMService) reconcilePowerState(ctx *virtualMachineContext) (bool, error) {
	powerState, err := vms.getPowerState(ctx)
	if err != nil {
		return false, err
	}
	switch powerState {
	case infrav1.VirtualMachinePowerStatePoweredOff:
		ctx.Logger.Info("powering on")
		task, err := ctx.Obj.PowerOn(ctx)
		if err != nil {
			return false, errors.Wrapf(err, "failed to trigger power on op for vm %s", ctx)
		}

		// Update the VSphereMachine.Status.TaskRef to track the power-on task.
		ctx.VSphereMachine.Status.TaskRef = task.Reference().Value

		// Once the VM is successfully powered on, a reconcile request should be
		// triggered once the VM reports IP addresses are available.
		reconcileVSphereMachineWhenNetworkIsReady(ctx, task)

		ctx.Logger.Info("wait for VM to be powered on")
		return false, nil
	case infrav1.VirtualMachinePowerStatePoweredOn:
		ctx.Logger.Info("powered on")
		return true, nil
	default:
		return false, errors.Errorf("unexpected power state %q for vm %s", powerState, ctx)
	}
}

func (vms *VMService) reconcileUUID(ctx *virtualMachineContext) error {
	ctx.State.BiosUUID = ctx.Obj.UUID(ctx)
	return nil
}

func (vms *VMService) getPowerState(ctx *virtualMachineContext) (infrav1.VirtualMachinePowerState, error) {
	powerState, err := ctx.Obj.PowerState(ctx)
	if err != nil {
		return "", err
	}

	switch powerState {
	case types.VirtualMachinePowerStatePoweredOn:
		return infrav1.VirtualMachinePowerStatePoweredOn, nil
	case types.VirtualMachinePowerStatePoweredOff:
		return infrav1.VirtualMachinePowerStatePoweredOff, nil
	case types.VirtualMachinePowerStateSuspended:
		return infrav1.VirtualMachinePowerStateSuspended, nil
	default:
		return "", errors.Errorf("unexpected power state %q for vm %s", powerState, ctx)
	}
}

func (vms *VMService) getMetadata(ctx *virtualMachineContext) (string, error) {
	var (
		obj mo.VirtualMachine

		pc    = property.DefaultCollector(ctx.Session.Client.Client)
		props = []string{"config.extraConfig"}
	)

	if err := pc.RetrieveOne(ctx, ctx.Ref, props, &obj); err != nil {
		return "", errors.Wrapf(err, "unable to fetch props %v for vm %s", props, ctx)
	}
	if obj.Config == nil {
		return "", nil
	}

	var metadataBase64 string
	for _, ec := range obj.Config.ExtraConfig {
		if optVal := ec.GetOptionValue(); optVal != nil {
			// TODO(akutz) Using a switch instead of if in case we ever
			//             want to check the metadata encoding as well.
			//             Since the image stamped images always use
			//             base64, it should be okay to not check.
			// nolint
			switch optVal.Key {
			case guestInfoKeyMetadata:
				if v, ok := optVal.Value.(string); ok {
					metadataBase64 = v
				}
			}
		}
	}

	if metadataBase64 == "" {
		return "", nil
	}

	metadataBuf, err := base64.StdEncoding.DecodeString(metadataBase64)
	if err != nil {
		return "", errors.Wrapf(err, "unable to decode metadata for %s", ctx)
	}

	return string(metadataBuf), nil
}

func (vms *VMService) setMetadata(ctx *virtualMachineContext, metadata []byte) (string, error) {
	var extraConfig extra.Config
	extraConfig.SetCloudInitMetadata(metadata)

	task, err := ctx.Obj.Reconfigure(ctx, types.VirtualMachineConfigSpec{
		ExtraConfig: extraConfig,
	})
	if err != nil {
		return "", errors.Wrapf(err, "unable to set metadata on vm %s", ctx)
	}

	return task.Reference().Value, nil
}

func (vms *VMService) getNetworkStatus(ctx *virtualMachineContext) ([]infrav1.NetworkStatus, error) {
	allNetStatus, err := net.GetNetworkStatus(ctx, ctx.Session.Client.Client, ctx.Ref)
	if err != nil {
		return nil, err
	}
	ctx.Logger.V(4).Info("got allNetStatus", "status", allNetStatus)
	apiNetStatus := []infrav1.NetworkStatus{}
	for _, s := range allNetStatus {
		apiNetStatus = append(apiNetStatus, infrav1.NetworkStatus{
			Connected:   s.Connected,
			IPAddrs:     sanitizeIPAddrs(&ctx.MachineContext, s.IPAddrs),
			MACAddr:     s.MACAddr,
			NetworkName: s.NetworkName,
		})
	}
	return apiNetStatus, nil
}
