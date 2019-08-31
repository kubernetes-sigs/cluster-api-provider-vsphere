package govmomi

import (
	"encoding/base64"

	"github.com/pkg/errors"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/extra"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/net"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/util"
)

const (
	ethCardType  = "vmxnet3"
	diskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndConsolidate)
)

// VMService provdes API to interact with the VMs using govmomi
type VMService struct{}

// ReconcileVM makes sure that the VM is in the desired state by:
//   1. Creating the VM if it does not exist, then...
//   2. Updating the VM with the bootstrap data, such as the cloud-init meta and user data, before...
//   3. Powering on the VM, and finally...
//   4. Returning the real-time state of the VM to the caller
func (vms *VMService) ReconcileVM(ctx *context.MachineContext) (infrav1.VirtualMachine, error) {

	// Create a VM object
	vm := infrav1.VirtualMachine{
		Name:  ctx.VSphereMachine.Name,
		State: infrav1.VirtualMachineStatePending,
	}

	// If there is no pending task or no machine ref then no VM exits, create one
	if ctx.VSphereMachine.Status.TaskRef == "" && ctx.VSphereMachine.Spec.MachineRef == "" {
		ref, err := findVMByInstanceUUID(ctx)
		if err != nil {
			return vm, err
		}

		if ref != "" {
			return vm, errors.Errorf("vm with the same Instance UUID already exists %q", ctx.VSphereMachine.Name)
		}

		// no VM exits, goahead and create a VM
		if err := createVM(ctx, []byte(*ctx.Machine.Spec.Bootstrap.Data)); err != nil {
			return vm, err
		}

		return vm, nil
	}

	// The VM exists at this point, so let's address steps two through four
	// from this function's documented workflow (please see the function's
	// GoDoc comments for more information)

	// Check for in-flight tasks
	if inflight, err := hasInFlightTask(ctx); err != nil || inflight {
		return vm, err
	}

	// Update the MachineRef if not already present
	if ctx.VSphereMachine.Spec.MachineRef == "" {
		moRefID, err := findVMByInstanceUUID(ctx)
		if err != nil {
			return vm, err
		}
		if moRefID != "" {
			ctx.VSphereMachine.Spec.MachineRef = moRefID
			ctx.Logger.V(6).Info("discovered moref id", "moref-id", ctx.VSphereMachine.Spec.MachineRef)
		}
	}

	// Verify if the VM exists
	obj, err := getVMObject(ctx)
	if err != nil {
		// The name lookup fails, therefore the VM does not exist.
		ctx.VSphereMachine.Spec.MachineRef = ""
		return vm, err
	}

	if err := vms.reconcileNetworkStatus(ctx, &vm); err != nil {
		return vm, nil
	}

	if ok, err := vms.reconcileMetadata(ctx, vm); err != nil || !ok {
		return vm, err
	}

	if ok, err := vms.reconcilePowerState(ctx); err != nil || !ok {
		return vm, err
	}

	if err := vms.reconcileUUIUDs(ctx, &vm, obj); err != nil {
		return vm, err
	}

	vm.State = infrav1.VirtualMachineStateReady
	return vm, nil
}

// DestroyVM powers off and destroys a virtual machine.
func (vms *VMService) DestroyVM(ctx *context.MachineContext) (infrav1.VirtualMachine, error) {

	vm := infrav1.VirtualMachine{
		Name:  ctx.VSphereMachine.Name,
		State: infrav1.VirtualMachineStatePending,
	}

	if ctx.VSphereMachine.Spec.MachineRef == "" && ctx.VSphereMachine.Status.TaskRef == "" {
		// vm already deleted
		vm.State = infrav1.VirtualMachineStateNotFound
		return vm, nil
	}

	// check for in-flight tasks
	if inflight, err := hasInFlightTask(ctx); err != nil || inflight {
		return vm, err
	}

	// Power off the VM if needed
	powerState, err := vms.getPowerState(ctx)
	if err != nil {
		return vm, err
	}
	if powerState == infrav1.VirtualMachinePowerStatePoweredOn {
		task, err := vms.powerOffVM(ctx)
		if err != nil {
			return vm, err
		}
		ctx.VSphereMachine.Status.TaskRef = task
		// requeue for VM to be powered off
		ctx.Logger.V(6).Info("reenqueue to wait for power off op")
		return vm, nil
	}

	// At this point the VM is not powered on and can be destroyed. Store the
	// destroy task's reference and return a requeue error.
	ctx.Logger.V(6).Info("destroying vm")
	task, err := vms.destroyVM(ctx)
	if err != nil {
		return vm, err
	}
	ctx.VSphereMachine.Status.TaskRef = task

	// Requeue
	ctx.Logger.V(6).Info("reenqueue to wait for destroy op")
	return vm, nil
}

func (vms *VMService) reconcileNetworkStatus(ctx *context.MachineContext, vm *infrav1.VirtualMachine) error {
	netStatus, err := vms.getNetworkStatus(ctx)
	if err != nil {
		return err
	}

	vm.Network = netStatus
	return nil
}

func (vms *VMService) reconcileMetadata(ctx *context.MachineContext, vm infrav1.VirtualMachine) (bool, error) {
	existingMetadata, err := vms.getMetadata(ctx)
	if err != nil {
		return false, err
	}

	newMetadata, err := util.GetMachineMetadata(*ctx.VSphereMachine, vm.Network...)
	if err != nil {
		return false, err
	}

	// If the metadata is the same then return early.
	if string(newMetadata) == existingMetadata {
		return true, nil
	}

	ctx.Logger.V(4).Info("updating metadata")
	task, err := vms.setMetadata(ctx, newMetadata)
	if err != nil {
		return false, errors.Wrapf(err, "unable to set metadata on vm %q", ctx)
	}

	// update taskref
	ctx.VSphereMachine.Status.TaskRef = task
	ctx.Logger.V(6).Info("reenqueue to track update metadata task")
	return false, nil
}

func (vms *VMService) reconcilePowerState(ctx *context.MachineContext) (bool, error) {
	powerState, err := vms.getPowerState(ctx)
	if err != nil {
		return false, err
	}
	switch powerState {
	case infrav1.VirtualMachinePowerStatePoweredOff:
		ctx.Logger.V(4).Info("powering on")
		task, err := vms.powerOnVM(ctx)
		if err != nil {
			return false, errors.Wrapf(err, "failed to trigger power on op for vm %q", ctx)
		}
		// update the tak ref to track
		ctx.VSphereMachine.Status.TaskRef = task
		ctx.Logger.V(6).Info("reenqueue to wait for power on state")
		return false, nil
	case infrav1.VirtualMachinePowerStatePoweredOn:
		ctx.Logger.V(6).Info("powered on")
	default:
		return false, errors.Errorf("unexpected power state %q for vm %q", powerState, ctx)
	}

	return true, nil
}

func (vms *VMService) reconcileUUIUDs(ctx *context.MachineContext, vm *infrav1.VirtualMachine, obj mo.VirtualMachine) error {
	// Temporarily removing this. It is calling a panic (nil pointer reference).
	// we dont use this anywhere so ti should be fine.
	// vm.InstanceUUID = obj.Config.InstanceUuid

	biosUUID, err := vms.getBiosUUID(ctx)
	if err != nil {
		return err
	}
	vm.BiosUUID = biosUUID
	return nil
}

func (vms *VMService) getPowerState(ctx *context.MachineContext) (infrav1.VirtualMachinePowerState, error) {

	vm, err := getVMfromMachineRef(ctx)
	if err != nil {
		return "", err
	}

	pState, err := vm.PowerState(ctx)
	if err != nil {
		return "", err
	}

	switch pState {
	case types.VirtualMachinePowerStatePoweredOn:
		return infrav1.VirtualMachinePowerStatePoweredOn, nil
	case types.VirtualMachinePowerStatePoweredOff:
		return infrav1.VirtualMachinePowerStatePoweredOff, nil
	case types.VirtualMachinePowerStateSuspended:
		return infrav1.VirtualMachinePowerStateSuspended, nil
	default:
		return "", errors.Errorf("unexpected power state %q for vm %q", pState, ctx)
	}
}

func (vms *VMService) getMetadata(ctx *context.MachineContext) (string, error) {
	var (
		obj mo.VirtualMachine

		moRef = *(getMoRef(ctx))
		pc    = property.DefaultCollector(ctx.Session.Client.Client)
		props = []string{"config.extraConfig"}
	)

	if err := pc.RetrieveOne(ctx, moRef, props, &obj); err != nil {
		return "", errors.Wrapf(err, "unable to fetch props %v for vm %v", props, moRef)
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
		return "", errors.Wrapf(err, "unable to decode metadata for %q", ctx)
	}

	return string(metadataBuf), nil
}

func (vms *VMService) setMetadata(ctx *context.MachineContext, metadata []byte) (string, error) {
	var extraConfig extra.Config
	extraConfig.SetCloudInitMetadata(metadata)

	vm, err := getVMfromMachineRef(ctx)
	if err != nil {
		return "", err
	}

	task, err := vm.Reconfigure(ctx, types.VirtualMachineConfigSpec{
		ExtraConfig: extraConfig,
	})
	if err != nil {
		return "", errors.Wrapf(err, "unable to set metadata on vm %q", ctx)
	}

	return task.Reference().Value, nil
}

func (vms *VMService) getNetworkStatus(ctx *context.MachineContext) ([]infrav1.NetworkStatus, error) {
	allNetStatus, err := net.GetNetworkStatus(ctx, ctx.Session.Client.Client, *(getMoRef(ctx)))
	if err != nil {
		return nil, err
	}
	ctx.Logger.V(6).Info("got allNetStatus", "status", allNetStatus)
	apiNetStatus := []infrav1.NetworkStatus{}
	for _, s := range allNetStatus {
		apiNetStatus = append(apiNetStatus, infrav1.NetworkStatus{
			Connected:   s.Connected,
			IPAddrs:     sanitizeIPAddrs(ctx, s.IPAddrs),
			MACAddr:     s.MACAddr,
			NetworkName: s.NetworkName,
		})
	}
	return apiNetStatus, nil
}

func (vms *VMService) getBiosUUID(ctx *context.MachineContext) (string, error) {
	vm, err := getVMfromMachineRef(ctx)
	if err != nil {
		return "", err
	}

	return vm.UUID(ctx), nil
}

func (vms *VMService) powerOnVM(ctx *context.MachineContext) (string, error) {
	vm, err := getVMfromMachineRef(ctx)
	if err != nil {
		return "", err
	}

	task, err := vm.PowerOn(ctx)
	if err != nil {
		return "", err
	}

	return task.Reference().Value, nil
}

func (vms *VMService) powerOffVM(ctx *context.MachineContext) (string, error) {
	vm, err := getVMfromMachineRef(ctx)
	if err != nil {
		return "", err
	}

	task, err := vm.PowerOff(ctx)
	if err != nil {
		return "", err
	}

	return task.Reference().Value, nil
}

func (vms *VMService) destroyVM(ctx *context.MachineContext) (string, error) {
	vm, err := getVMfromMachineRef(ctx)
	if err != nil {
		return "", err
	}

	task, err := vm.Destroy(ctx)
	if err != nil {
		return "", err
	}

	return task.Reference().Value, nil
}
