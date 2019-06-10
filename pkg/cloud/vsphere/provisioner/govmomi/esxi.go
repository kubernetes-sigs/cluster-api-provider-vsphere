package govmomi

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog"
)

func (pv *Provisioner) thinCopyDisks(ctx context.Context, machine *GovmomiMachine, srcPath string, dstPath string) error {
	var err error
	var username, password string
	if username, password, err = machine.GetCluster().GetCredentials(); err != nil {
		return err
	}

	vsphereConfig := machine.GetCluster().config
	klog.V(4).Infof("[clone] thin cloning disk to %s", dstPath)

	cmd := fmt.Sprintf("vmkfstools -i \"%s\" \"%s\" -d thin", srcPath, dstPath)
	klog.V(5).Infof("[clone] ssh: %s", cmd)
	host := vsphereConfig.VsphereServer + ":22"
	err = runSSHCommand(cmd, host, username, password)
	klog.V(4).Infof("[clone] thin cloned vmdk, src(%s) ->  dst(%s)", srcPath, dstPath)
	return err
}

// cloneVirtualMachineOnESX clones the template to a virtual machine.
func (pv *Provisioner) cloneVirtualMachineOnESX(ctx context.Context, machine *GovmomiMachine) error {
	klog.V(4).Infof("[clone] via ESX for VM %s", machine.Name)

	dc, err := machine.s.finder.DefaultDatacenter(ctx)
	if err != nil {
		return err
	}
	machine.s.finder.SetDatacenter(dc)

	folders, err := dc.Folders(ctx)
	if err != nil {
		return err
	}

	pool, err := machine.s.finder.ResourcePoolOrDefault(ctx, machine.config.MachineSpec.ResourcePool)
	if err != nil {
		return err
	}

	// Fetch info from templateVM
	src, err := machine.GetCluster().FindVM(ctx, dc, machine.config.MachineSpec.VMTemplate)
	if err != nil {
		return err
	}
	var vmProps *mo.VirtualMachine
	vmProps, err = machine.GetCluster().GetVirtualMachineMO(src)
	if err != nil {
		return fmt.Errorf("error fetching virtual machine or template properties: %s", err)
	}

	spec := &types.VirtualMachineConfigSpec{}
	var devices object.VirtualDeviceList

	if err := pv.addMachineBase(ctx, machine, spec, vmProps); err != nil {
		return err
	}

	if devices, err = pv.copyDisks(ctx, machine, devices, vmProps); err != nil {
		return err
	}

	if devices, err = pv.addNetworking(ctx, machine, devices); err != nil {
		return err
	}

	if devices, err = pv.addSerialPort(ctx, devices, vmProps); err != nil {
		return err
	}

	deviceChange, err := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	if err != nil {
		return err
	}

	spec.DeviceChange = deviceChange

	// get current hostsystem from source vm
	ch, err := src.HostSystem(ctx)
	if err != nil {
		return err
	}

	machine.Eventf("Creating", "Creating Machine %v", machine.Name)

	task, err := folders.VmFolder.CreateVM(ctx, *spec, pool, ch)
	if err != nil {
		return fmt.Errorf("[DEBUG] VmFolder.CreateVM() FAILED: %s", err.Error())
	}

	return machine.SetTaskRef(task.Reference().Value)

}
