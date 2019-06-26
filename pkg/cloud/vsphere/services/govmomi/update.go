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
	"github.com/vmware/govmomi/vim25/types"

	corev1 "k8s.io/api/core/v1"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/extra"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/metadata"
)

// Update updates the machine from the backend platform.
func Update(ctx *context.MachineContext) error {

	// Check to see if the VM exists first since no error is returned if the VM
	// does not exist, only when there's an error checking or when the op should
	// be requeued, like when the VM has an in-flight task.
	vm, err := findVM(ctx)
	if err != nil {
		return err
	}

	// A VM is supposed to exist by this point. Otherwise return an error.
	if vm == nil {
		return errors.Errorf("vm is supposed to exist %q", ctx)
	}

	if err := reconcileNetwork(ctx, vm); err != nil {
		return err
	}

	if err := reconcileMetadata(ctx, vm); err != nil {
		return err
	}

	if err := reconcilePowerState(ctx, vm); err != nil {
		return err
	}

	return nil
}

// reconcileMetadata updates the metadata on the VM if it is missing or different
// than new metadata
func reconcileMetadata(ctx *context.MachineContext, vm *object.VirtualMachine) error {
	existingMetadata, err := getExistingMetadata(ctx)
	if err != nil {
		return err
	}

	newMetadata, err := metadata.New(ctx)
	if err != nil {
		return err
	}

	// If the metadata is the same then return early.
	if string(newMetadata) == existingMetadata {
		return nil
	}

	// Update the VM's metadata, track the task, and reenqueue this op
	ctx.Logger.V(4).Info("updating metadata")
	var extraConfig extra.Config
	extraConfig.SetCloudInitMetadata(newMetadata)
	task, err := vm.Reconfigure(ctx, types.VirtualMachineConfigSpec{
		ExtraConfig: extraConfig,
	})
	if err != nil {
		return errors.Wrapf(err, "unable to set metadata on vm %q", ctx)
	}
	ctx.MachineStatus.TaskRef = task.Reference().Value
	ctx.Logger.V(6).Info("reenqueue to track update metadata task")
	return &clustererror.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
}

// reconcileNetwork updates the machine's network spec and status
func reconcileNetwork(ctx *context.MachineContext, vm *object.VirtualMachine) error {

	// Validate the number of reported networks match the number of configured
	// networks.
	allNetStatus, err := getNetworkStatus(ctx)
	if err != nil {
		return errors.Wrapf(err, "unable to get vm's network status %q", ctx)
	}
	expNetCount, actNetCount := len(ctx.MachineConfig.MachineSpec.Network.Devices), len(allNetStatus)
	if expNetCount != actNetCount {
		return errors.Errorf("invalid network count for %q: exp=%d act=%d", ctx, expNetCount, actNetCount)
	}
	ctx.MachineStatus.Network = allNetStatus

	// Update the MAC addresses in the machine's network config as well. This
	// is required in order to generate the metadata.
	for i := range ctx.MachineConfig.MachineSpec.Network.Devices {
		devSpec := &ctx.MachineConfig.MachineSpec.Network.Devices[i]
		oldMac, newMac := devSpec.MACAddr, allNetStatus[i].MACAddr
		if oldMac != newMac {
			devSpec.MACAddr = newMac
			ctx.Logger.V(6).Info("updating MAC address for device", "network-name", devSpec.NetworkName, "old-mac-addr", oldMac, "new-mac-addr", newMac)
		}
	}

	// If the VM is powered on then issue requeues until all of the VM's
	// networks have IP addresses.
	var ipAddrs []corev1.NodeAddress
	powerState, err := vm.PowerState(ctx)
	if err != nil {
		return errors.Wrapf(err, "unable to get vm's power state %q", ctx)
	}
	if powerState == types.VirtualMachinePowerStatePoweredOn {
		for _, netStatus := range ctx.MachineStatus.Network {
			if len(netStatus.IPAddrs) == 0 {
				ctx.Logger.V(6).Info("reenqueue to wait on IP addresses")
				return &clustererror.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
			}
			for _, ip := range netStatus.IPAddrs {
				ipAddrs = append(ipAddrs, corev1.NodeAddress{
					Type:    corev1.NodeInternalIP,
					Address: ip,
				})
			}
		}
	}

	// Use the collected IP addresses to assign the Machine's addresses.
	ctx.Machine.Status.Addresses = ipAddrs

	return nil
}

// reconcilePowerState powers on the VM if it is powered off
func reconcilePowerState(ctx *context.MachineContext, vm *object.VirtualMachine) error {
	powerState, err := vm.PowerState(ctx)
	if err != nil {
		return errors.Wrapf(err, "unable to get vm's power state %q", ctx)
	}
	switch powerState {
	case types.VirtualMachinePowerStatePoweredOff:
		ctx.Logger.V(4).Info("powering on")
		task, err := vm.PowerOn(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to trigger power on op for vm %q", ctx)
		}
		ctx.MachineStatus.TaskRef = task.Reference().Value
		ctx.Logger.V(6).Info("reenqueue to wait for power on state")
		return &clustererror.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	case types.VirtualMachinePowerStatePoweredOn:
		ctx.Logger.V(6).Info("powered on")
	default:
		return errors.Errorf("unexpected power state %q for vm %q", powerState, ctx)
	}
	return nil
}
