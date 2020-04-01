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

package vcenter

import (
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/extra"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/template"
)

const (
	fullCloneDiskMoveType = types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndConsolidate
	linkCloneDiskMoveType = types.VirtualMachineRelocateDiskMoveOptionsCreateNewChildDiskBacking
)

// Clone kicks off a clone operation on vCenter to create a new virtual machine.
// nolint:gocognit
func Clone(ctx *context.VMContext, bootstrapData []byte) error {
	ctx = &context.VMContext{
		ControllerContext: ctx.ControllerContext,
		VSphereVM:         ctx.VSphereVM,
		Session:           ctx.Session,
		Logger:            ctx.Logger.WithName("vcenter"),
		PatchHelper:       ctx.PatchHelper,
	}
	ctx.Logger.Info("starting clone process")

	var extraConfig extra.Config
	if len(bootstrapData) > 0 {
		ctx.Logger.Info("applied bootstrap data to VM clone spec")
		if err := extraConfig.SetCloudInitUserData(bootstrapData); err != nil {
			return err
		}
	}

	tpl, err := template.FindTemplate(ctx, ctx.VSphereVM.Spec.Template)
	if err != nil {
		return err
	}

	// If a linked clone is requested then a MoRef for a snapshot must be
	// found with which to perform the linked clone.
	var snapshotRef *types.ManagedObjectReference
	if ctx.VSphereVM.Spec.CloneMode == "" || ctx.VSphereVM.Spec.CloneMode == infrav1.LinkedClone {
		ctx.Logger.Info("linked clone requested")
		// If the name of a snapshot was not provided then find the template's
		// current snapshot.
		if snapshotName := ctx.VSphereVM.Spec.Snapshot; snapshotName == "" {
			ctx.Logger.Info("searching for current snapshot")
			var vm mo.VirtualMachine
			if err := tpl.Properties(ctx, tpl.Reference(), []string{"snapshot"}, &vm); err != nil {
				return errors.Wrapf(err, "error getting snapshot information for template %s", ctx.VSphereVM.Spec.Template)
			}
			if vm.Snapshot != nil {
				snapshotRef = vm.Snapshot.CurrentSnapshot
			}
		} else {
			ctx.Logger.Info("searching for snapshot by name", "snapshotName", snapshotName)
			var err error
			snapshotRef, err = tpl.FindSnapshot(ctx, snapshotName)
			if err != nil {
				ctx.Logger.Info("failed to find snapshot", "snapshotName", snapshotName)
			}
		}
	}

	// The type of clone operation depends on whether or not there is a snapshot
	// from which to do a linked clone.
	diskMoveType := fullCloneDiskMoveType
	ctx.VSphereVM.Status.CloneMode = infrav1.FullClone
	if snapshotRef != nil {
		// Record the actual type of clone mode used as well as the name of
		// the snapshot (if not the current snapshot).
		ctx.VSphereVM.Status.CloneMode = infrav1.LinkedClone
		ctx.VSphereVM.Status.Snapshot = snapshotRef.Value
		diskMoveType = linkCloneDiskMoveType
	}

	folder, err := ctx.Session.Finder.FolderOrDefault(ctx, ctx.VSphereVM.Spec.Folder)
	if err != nil {
		return errors.Wrapf(err, "unable to get folder for %q", ctx)
	}

	datastore, err := ctx.Session.Finder.DatastoreOrDefault(ctx, ctx.VSphereVM.Spec.Datastore)
	if err != nil {
		return errors.Wrapf(err, "unable to get datastore for %q", ctx)
	}

	pool, err := ctx.Session.Finder.ResourcePoolOrDefault(ctx, ctx.VSphereVM.Spec.ResourcePool)
	if err != nil {
		return errors.Wrapf(err, "unable to get resource pool for %q", ctx)
	}

	devices, err := tpl.Device(ctx)
	if err != nil {
		return errors.Wrapf(err, "error getting devices for %q", ctx)
	}

	// Create a new list of device specs for cloning the VM.
	deviceSpecs := []types.BaseVirtualDeviceConfigSpec{}

	// Only non-linked clones may expand the size of the template's disk.
	if snapshotRef == nil {
		diskSpec, err := getDiskSpec(ctx, devices)
		if err != nil {
			return errors.Wrapf(err, "error getting disk spec for %q", ctx)
		}
		deviceSpecs = append(deviceSpecs, diskSpec)
	}

	networkSpecs, err := getNetworkSpecs(ctx, devices)
	if err != nil {
		return errors.Wrapf(err, "error getting network specs for %q", ctx)
	}
	deviceSpecs = append(deviceSpecs, networkSpecs...)

	numCPUs := ctx.VSphereVM.Spec.NumCPUs
	if numCPUs < 2 {
		numCPUs = 2
	}
	numCoresPerSocket := ctx.VSphereVM.Spec.NumCoresPerSocket
	if numCoresPerSocket == 0 {
		numCoresPerSocket = numCPUs
	}
	memMiB := ctx.VSphereVM.Spec.MemoryMiB
	if memMiB == 0 {
		memMiB = 2048
	}

	spec := types.VirtualMachineCloneSpec{
		Config: &types.VirtualMachineConfigSpec{
			// Assign the clone's InstanceUUID the value of the Kubernetes Machine
			// object's UID. This allows lookup of the cloned VM prior to knowing
			// the VM's UUID.
			InstanceUuid:      string(ctx.VSphereVM.UID),
			Flags:             newVMFlagInfo(),
			DeviceChange:      deviceSpecs,
			ExtraConfig:       extraConfig,
			NumCPUs:           numCPUs,
			NumCoresPerSocket: numCoresPerSocket,
			MemoryMB:          memMiB,
		},
		Location: types.VirtualMachineRelocateSpec{
			Datastore:    types.NewReference(datastore.Reference()),
			DiskMoveType: string(diskMoveType),
			Folder:       types.NewReference(folder.Reference()),
			Pool:         types.NewReference(pool.Reference()),
		},
		// This is implicit, but making it explicit as it is important to not
		// power the VM on before its virtual hardware is created and the MAC
		// address(es) used to build and inject the VM with cloud-init metadata
		// are generated.
		PowerOn:  false,
		Snapshot: snapshotRef,
	}

	ctx.Logger.Info("cloning machine", "namespace", ctx.VSphereVM.Namespace, "name", ctx.VSphereVM.Name, "cloneType", ctx.VSphereVM.Status.CloneMode)
	task, err := tpl.Clone(ctx, folder, ctx.VSphereVM.Name, spec)
	if err != nil {
		return errors.Wrapf(err, "error trigging clone op for machine %s", ctx)
	}

	ctx.VSphereVM.Status.TaskRef = task.Reference().Value

	// patch the vsphereVM here to ensure that the task is
	// reflected in the status right away, this avoid situations
	// of concurrent clones
	if err := ctx.Patch(); err != nil {
		ctx.Logger.Error(err, "patch failed", "vspherevm", ctx.VSphereVM)
	}
	return nil
}

func newVMFlagInfo() *types.VirtualMachineFlagInfo {
	diskUUIDEnabled := true
	return &types.VirtualMachineFlagInfo{
		DiskUuidEnabled: &diskUUIDEnabled,
	}
}

func getDiskSpec(
	ctx *context.VMContext,
	devices object.VirtualDeviceList) (types.BaseVirtualDeviceConfigSpec, error) {

	disks := devices.SelectByType((*types.VirtualDisk)(nil))
	if len(disks) != 1 {
		return nil, errors.Errorf("invalid disk count: %d", len(disks))
	}

	disk := disks[0].(*types.VirtualDisk)
	disk.CapacityInKB = int64(ctx.VSphereVM.Spec.DiskGiB) * 1024 * 1024

	return &types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationEdit,
		Device:    disk,
	}, nil
}

const ethCardType = "vmxnet3"

func getNetworkSpecs(
	ctx *context.VMContext,
	devices object.VirtualDeviceList) ([]types.BaseVirtualDeviceConfigSpec, error) {

	deviceSpecs := []types.BaseVirtualDeviceConfigSpec{}

	// Remove any existing NICs
	for _, dev := range devices.SelectByType((*types.VirtualEthernetCard)(nil)) {
		deviceSpecs = append(deviceSpecs, &types.VirtualDeviceConfigSpec{
			Device:    dev,
			Operation: types.VirtualDeviceConfigSpecOperationRemove,
		})
	}

	// Add new NICs based on the machine config.
	key := int32(-100)
	for i := range ctx.VSphereVM.Spec.Network.Devices {
		netSpec := &ctx.VSphereVM.Spec.Network.Devices[i]
		ref, err := ctx.Session.Finder.Network(ctx, netSpec.NetworkName)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to find network %q", netSpec.NetworkName)
		}
		backing, err := ref.EthernetCardBackingInfo(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to create new ethernet card backing info for network %q on %q", netSpec.NetworkName, ctx)
		}
		dev, err := object.EthernetCardTypes().CreateEthernetCard(ethCardType, backing)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to create new ethernet card %q for network %q on %q", ethCardType, netSpec.NetworkName, ctx)
		}

		// Get the actual NIC object. This is safe to assert without a check
		// because "object.EthernetCardTypes().CreateEthernetCard" returns a
		// "types.BaseVirtualEthernetCard" as a "types.BaseVirtualDevice".
		nic := dev.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()

		if netSpec.MACAddr != "" {
			nic.MacAddress = netSpec.MACAddr
			// Please see https://www.vmware.com/support/developer/converter-sdk/conv60_apireference/vim.vm.device.VirtualEthernetCard.html#addressType
			// for the valid values for this field.
			nic.AddressType = string(types.VirtualEthernetCardMacTypeManual)
			ctx.Logger.V(4).Info("configured manual mac address", "mac-addr", nic.MacAddress)
		}

		// Assign a temporary device key to ensure that a unique one will be
		// generated when the device is created.
		nic.Key = key

		deviceSpecs = append(deviceSpecs, &types.VirtualDeviceConfigSpec{
			Device:    dev,
			Operation: types.VirtualDeviceConfigSpecOperationAdd,
		})
		ctx.Logger.V(4).Info("created network device", "eth-card-type", ethCardType, "network-spec", netSpec)
		key--
	}

	return deviceSpecs, nil
}
