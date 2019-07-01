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
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/extra"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/net"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/template"
)

const (
	diskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndConsolidate)
)

// Clone kicks off a clone operation on vCenter to create a new virtual machine.
func Clone(ctx *context.MachineContext, userData []byte) error {
	ctx = context.NewMachineLoggerContext(ctx, "vcenter")
	ctx.Logger.V(6).Info("starting clone process")

	var extraConfig extra.Config
	extraConfig.SetCloudInitUserData(userData)

	tpl, err := template.FindTemplate(ctx, ctx.MachineConfig.MachineSpec.VMTemplate)
	if err != nil {
		return err
	}

	folder, err := ctx.Session.Finder.FolderOrDefault(ctx, ctx.MachineConfig.MachineSpec.VMFolder)
	if err != nil {
		return errors.Wrapf(err, "unable to get folder for %q", ctx)
	}

	datastore, err := ctx.Session.Finder.DatastoreOrDefault(ctx, ctx.MachineConfig.MachineSpec.Datastore)
	if err != nil {
		return errors.Wrapf(err, "unable to get datastore for %q", ctx)
	}

	pool, err := ctx.Session.Finder.ResourcePoolOrDefault(ctx, ctx.MachineConfig.MachineSpec.ResourcePool)
	if err != nil {
		return errors.Wrapf(err, "unable to get resource pool for %q", ctx)
	}

	devices, err := tpl.Device(ctx)

	if err != nil {
		return errors.Wrapf(err, "error getting devices for %q", ctx)
	}

	diskSpec, err := getDiskSpec(ctx, devices)
	if err != nil {
		return errors.Wrapf(err, "error getting disk spec for %q", ctx)
	}

	networkSpecs, err := net.GetNetworkSpecs(ctx, devices)
	if err != nil {
		return errors.Wrapf(err, "error getting network specs for %q", ctx)
	}

	deviceSpecs := []types.BaseVirtualDeviceConfigSpec{diskSpec}
	deviceSpecs = append(deviceSpecs, networkSpecs...)

	numCPUs := ctx.MachineConfig.MachineSpec.NumCPUs
	if numCPUs < 2 {
		numCPUs = 2
	}
	memMiB := ctx.MachineConfig.MachineSpec.MemoryMB
	if memMiB == 0 {
		memMiB = 2048
	}

	spec := types.VirtualMachineCloneSpec{
		Config: &types.VirtualMachineConfigSpec{
			Annotation: ctx.String(),
			// Assign the clone's InstanceUUID the value of the Kubernetes Machine
			// object's UID. This allows lookup of the cloned VM prior to knowing
			// the VM's UUID.
			InstanceUuid: string(ctx.Machine.UID),
			Flags:        newVMFlagInfo(),
			DeviceChange: deviceSpecs,
			ExtraConfig:  extraConfig,
			NumCPUs:      numCPUs,
			MemoryMB:     memMiB,
		},
		Location: types.VirtualMachineRelocateSpec{
			Datastore:    types.NewReference(datastore.Reference()),
			DiskMoveType: diskMoveType,
			Folder:       types.NewReference(folder.Reference()),
			Pool:         types.NewReference(pool.Reference()),
		},
		// This is implicit, but making it explicit as it is important to not
		// power the VM on before its virtual hardware is created and the MAC
		// address(es) used to build and inject the VM with cloud-init metadata
		// are generated.
		PowerOn: false,
	}

	ctx.Logger.V(6).Info("cloning machine", "clone-spec", spec)
	task, err := tpl.Clone(ctx, folder, ctx.Machine.Name, spec)
	if err != nil {
		return errors.Wrapf(err, "error trigging clone op for machine %q", ctx)
	}

	ctx.MachineStatus.TaskRef = task.Reference().Value

	return nil
}

func newVMFlagInfo() *types.VirtualMachineFlagInfo {
	diskUUIDEnabled := true
	return &types.VirtualMachineFlagInfo{
		DiskUuidEnabled: &diskUUIDEnabled,
	}
}

func getDiskSpec(
	ctx *context.MachineContext,
	devices object.VirtualDeviceList) (types.BaseVirtualDeviceConfigSpec, error) {

	disks := devices.SelectByType((*types.VirtualDisk)(nil))
	if len(disks) != 1 {
		return nil, errors.Errorf("invalid disk count: %d", len(disks))
	}

	disk := disks[0].(*types.VirtualDisk)
	disk.CapacityInKB = int64(ctx.MachineConfig.MachineSpec.DiskGiB) * 1024 * 1024

	return &types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationEdit,
		Device:    disk,
	}, nil
}
