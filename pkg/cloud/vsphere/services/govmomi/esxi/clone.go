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

package esxi

import (
	"fmt"
	"path"
	"strings"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/net"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	vspherecontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/extra"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/ssh"
)

// Clone kicks off a clone operation on ESXi to create a new virtual machine.
func Clone(ctx *vspherecontext.MachineContext, userData []byte) error {
	ctx = vspherecontext.NewMachineLoggerContext(ctx, "esxi")
	ctx.Logger.V(6).Info("starting clone process")
	vm, err := context.NewGovmomiContext(ctx)
	if err != nil {
		return err
	}

	diskUUIDEnabled := true
	spec := &types.VirtualMachineConfigSpec{
		// Use the object UID as the instanceUUID for the VM
		InstanceUuid: string(ctx.Machine.UID),
		Flags: &types.VirtualMachineFlagInfo{
			DiskUuidEnabled: &diskUUIDEnabled,
		},
	}

	mo, _ := vm.GetVirtualMachineMO()
	spec.GuestId = mo.Config.GuestId         // set GuestId from template
	spec.Firmware = mo.Config.Firmware       // set Firmware from template
	spec.BootOptions = mo.Config.BootOptions // set BootOptions from Template

	ctx.Logger.V(4).Info("assigned VM instance UUID from machine UID", "uid", string(ctx.Machine.UID))
	if ctx.MachineConfig.MachineSpec.NumCPUs > 0 {
		spec.NumCPUs = int32(vm.MachineConfig.MachineSpec.NumCPUs)
	}
	if ctx.MachineConfig.MachineSpec.MemoryMB > 0 {
		spec.MemoryMB = vm.MachineConfig.MachineSpec.MemoryMB
	}
	spec.Annotation = vm.String()
	spec.Name = vm.Machine.Name
	deviceSpecs := []types.BaseVirtualDeviceConfigSpec{}

	var devices object.VirtualDeviceList
	disks, err := thinCloneDisks(vm)
	if err != nil {
		return err
	}
	for _, disk := range disks {
		deviceSpecs = append(deviceSpecs, disk)
	}

	nics, err := net.GetNetworkSpecs(vm, devices)
	if err != nil {
		return err
	}
	for _, device := range nics {
		deviceSpecs = append(deviceSpecs, device)
	}

	if devices, err = getSerialPort(vm, devices); err != nil {
		return err
	}
	devicesConfigs, _ := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	for _, device := range devicesConfigs {
		deviceSpecs = append(deviceSpecs, device)
	}

	spec.DeviceChange = deviceSpecs
	spec.Files = &types.VirtualMachineFileInfo{
		VmPathName: fmt.Sprintf("[%s]", vm.Datastore.Name()),
	}

	var extraConfig extra.Config
	extraConfig.SetCloudInitUserData(userData)
	spec.ExtraConfig = extraConfig

	task, err := createVM(vm, spec)
	if err != nil {
		return errors.Wrapf(err, "Failed to create vm")
	}
	ctx.Logger.V(4).Info("Task submitted: " + task.Reference().Value)
	ctx.MachineStatus.TaskRef = task.Reference().Value
	return nil
}

// createVM submits the folder.createVM task
func createVM(ctx *context.GovmomiContext, spec *types.VirtualMachineConfigSpec) (*object.Task, error) {
	ctx.Logger.V(6).Info("esxi cloning machine", "clone-spec", spec)

	folders, err := ctx.Datacenter.Folders(ctx)
	if err != nil {
		return nil, err
	}
	return folders.VmFolder.CreateVM(ctx, *spec, ctx.Pool, ctx.Host)
}

// getSerialPort creates and returns a new serial port to help with entropy
func getSerialPort(ctx *context.GovmomiContext, devices object.VirtualDeviceList) (object.VirtualDeviceList, error) {
	ctx.Logger.V(6).Info("Adding SIO controller")
	c := &types.VirtualSIOController{}
	c.Key = -301
	devices = append(devices, c)

	ctx.Logger.V(6).Info("Adding Serial Port")

	portSpec, err := devices.CreateSerialPort()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to add serial port")
	}
	portSpec.Key = -302
	portSpec.ControllerKey = c.Key
	devices = append(devices, portSpec)

	return devices, nil
}

// getVirtualDisks returns the list of existing virtual disks on the source template
func getVirtualDisks(ctx *context.GovmomiContext) ([]types.VirtualDisk, error) {
	mo, err := ctx.GetVirtualMachineMO()
	if err != nil {
		return nil, err
	}
	l := object.VirtualDeviceList(mo.Config.Hardware.Device)
	var disks []types.VirtualDisk
	for _, device := range l.SelectByType((*types.VirtualDisk)(nil)) {
		disk := device.(*types.VirtualDisk)
		disks = append(disks, *disk)
	}
	return disks, nil
}

// thinCloneDisks clones the disks specified in the template by SSHing into the node and using vmkfstools
func thinCloneDisks(ctx *context.GovmomiContext) ([]types.BaseVirtualDeviceConfigSpec, error) {
	devices := object.VirtualDeviceList{}
	disks, err := getVirtualDisks(ctx)
	if err != nil {
		return nil, err
	}
	ctx.Logger.V(4).Info("cloning", "machine", ctx.Machine)
	name := ctx.Machine.Name
	scsi, err := devices.CreateSCSIController("")
	if err != nil {
		return nil, err
	}

	devices = append(devices, scsi)
	controller, err := devices.FindDiskController(devices.Name(scsi))
	if err != nil {
		return nil, err
	}
	ssh := getSSH(ctx)
	for _, disk := range disks {
		root := fmt.Sprintf("/vmfs/volumes/%s/%s", ctx.Datastore.Reference().Value, name)
		dstPath := fmt.Sprintf("%s/%s.vmdk", root, name)
		src := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo).GetVirtualDeviceFileBackingInfo()
		srcPath := fmt.Sprintf("/vmfs/volumes/%s/%s", src.Datastore.Value, strings.Split(src.FileName, " ")[1])
		cmd := fmt.Sprintf("vmkfstools -i \"%s\" \"%s\" -d thin", srcPath, dstPath)
		ssh.Logger.V(6).Info("cmd: " + cmd)
		if err := ssh.Exec("mkdir -p %s", path.Dir(dstPath)); err != nil {
			return nil, errors.Wrapf(err, "Cannot create directory %s ", path.Base(dstPath))
		}
		if err := ssh.Exec(cmd); err != nil && 1 == 2 {
			return nil, errors.Wrapf(err, "Failed to thin clone disk")
		}

		ssh.Logger.V(4).Info(fmt.Sprintf("thin cloned vmdk, src(%s) ->  dst(%s)", srcPath, dstPath))
		cloned := devices.CreateDisk(controller, ctx.Datastore.Reference(), ctx.Datastore.Path(fmt.Sprintf("%s/%s.vmdk", name, name)))
		cloned.CapacityInBytes = disk.CapacityInBytes
		cloned.DeviceInfo = disk.DeviceInfo

		devices = append(devices, cloned)
	}
	deviceChange, err := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	if err != nil {
		return nil, err
	}
	return deviceChange, nil
}

// getSSH returns a SSH client that can be used to execute commands against the vSphere endpoint
func getSSH(ctx *context.GovmomiContext) *ssh.SSH {
	return &ssh.SSH{
		Logger: ctx.Logger.WithName("ssh"),
		User:   ctx.ClusterContext.User(),
		Pass:   ctx.ClusterContext.Pass(),
		Host:   ctx.ClusterContext.ClusterConfig.VsphereServer + ":22",
	}
}
