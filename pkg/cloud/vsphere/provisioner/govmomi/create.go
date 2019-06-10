package govmomi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"k8s.io/klog"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"
)

func (pv *Provisioner) Create(ctx context.Context, cluster *clusterv1.Cluster, _machine *clusterv1.Machine) error {
	machine, err := pv.NewGovmomiMachine(cluster, _machine)
	if err != nil {
		return err
	}

	task := machine.status.TaskRef
	if task != "" {
		// In case an active task is going on, wait for its completion
		return pv.verifyAndUpdateTask(ctx, machine, task)
	}
	// Before going for cloning, check if we can locate a VM with the InstanceUUID
	// as this Machine. If found, that VM is the right match for this machine
	vmRef, err := machine.GetCluster().findVMByInstanceUUID(ctx, string(machine.machine.UID))
	if err != nil {
		return err
	}
	if vmRef != "" {
		machine.Eventf("Created", "Created Machine %s(%s)", machine.Name, vmRef)
		if err := machine.UpdateVMReference(vmRef); err != nil {
			return err
		}
	}

	// Use the appropriate path if we're connected to a vCenter
	if machine.s.session.IsVC() {
		return pv.cloneVirtualMachineOnVCenter(ctx, machine)
	}

	// fallback in case we're connected to a standalone ESX host
	return pv.cloneVirtualMachineOnESX(ctx, machine)
}

func (pv *Provisioner) verifyAndUpdateTask(ctx context.Context, machine *GovmomiMachine, taskmoref string) error {
	task := machine.GetCluster().GetTask(ctx, taskmoref)

	if task == nil {
		machine.SetTaskRef("")
		return nil
	}

	vmref := task.Info.Result.(types.ManagedObjectReference)
	if machine.config.MachineRef != "" && vmref.Value != machine.config.MachineRef {
		return fmt.Errorf("assertion failed task %s for vm %s, referenced from %s", taskmoref, machine.config.MachineRef, vmref)
	}

	klog.V(4).Infof("[%s] %s = %s ", machine.Name, task.Info.DescriptionId, task.Info.State)
	switch task.Info.State {
	// Queued or Running
	case types.TaskInfoStateQueued, types.TaskInfoStateRunning:
		// Requeue the machine update to check back in 5 seconds on the task
		return &clustererror.RequeueAfterError{RequeueAfter: time.Second * 5}
	// Successful
	case types.TaskInfoStateSuccess:
		if task.Info.DescriptionId == "Folder.createVm" {
			if err := machine.PowerOn(ctx).WaitFor(); err != nil {
				return err
			}

			machine.Eventf("Created", "Created Machine %s(%s)", machine.Name, vmref.Value)
			err := machine.UpdateVMReference(vmref.Value)
			if err != nil {
				return err
			}
			return machine.SetTaskRef("")
		} else if task.Info.DescriptionId == "VirtualMachine.clone" {
			vmref := task.Info.Result.(types.ManagedObjectReference)
			machine.Eventf("Created", "Created Machine %s(%s)", machine.Name, vmref.Value)
			// Update the Machine object with the VM Reference annotation
			err := machine.UpdateVMReference(vmref.Value)
			if err != nil {
				return err
			}
			return machine.SetTaskRef("")
		} else if task.Info.DescriptionId == "VirtualMachine.reconfigure" {
			machine.Eventf("Reconfigured", "Reconfigured Machine %s", task.Info.EntityName)
		}
		return machine.SetTaskRef("")
	case types.TaskInfoStateError:
		klog.Infof("[DEBUG] task error condition, description = %s", task.Info.DescriptionId)
		// If the machine was created via the ESXi "cloning", the description id will likely be "Folder.createVm"
		if task.Info.DescriptionId == "VirtualMachine.clone" || task.Info.DescriptionId == "Folder.createVm" {
			machine.Eventf("Failed", "Creation failed for Machine %v", machine.Name)
			// Clear the reference to the failed task so that the next reconcile loop can re-create it
			return machine.SetTaskRef("")
		}
	default:
		return fmt.Errorf("Unknown state %s for task %s detected", taskmoref, task.Info.State)
	}
	return nil
}

func (pv *Provisioner) addMachineBase(ctx context.Context, machine *GovmomiMachine, spec *types.VirtualMachineConfigSpec, vmProps *mo.VirtualMachine) error {
	// Fetch the user-data for the cloud-init first, so that we can fail fast before even trying to connect to VC
	userData, err := pv.getCloudInitUserData(machine.cluster, machine.machine, "", false)
	if err != nil {
		// err returned by the getCloudInitUserData would be of type RequeueAfterError in case kubeadm is not ready yet
		return err
	}
	metaData, err := pv.getCloudInitMetaData(machine.cluster, machine.machine)
	if err != nil {
		// err returned by the getCloudInitUserData would be of type RequeueAfterError in case kubeadm is not ready yet
		return err
	}

	diskUUIDEnabled := true
	spec.Flags = &types.VirtualMachineFlagInfo{
		DiskUuidEnabled: &diskUUIDEnabled,
	}

	spec.Name = machine.Name                      // set name from cluster configuration
	spec.GuestId = vmProps.Config.GuestId         // set GuestId from template
	spec.Firmware = vmProps.Config.Firmware       // set Firmware from template
	spec.BootOptions = vmProps.Config.BootOptions // set BootOptions from Template

	if machine.config.MachineSpec.NumCPUs > 0 {
		spec.NumCPUs = int32(machine.config.MachineSpec.NumCPUs)
	}
	if machine.config.MachineSpec.MemoryMB > 0 {
		spec.MemoryMB = machine.config.MachineSpec.MemoryMB
	}

	spec.Annotation = fmt.Sprintf("Virtual Machine is part of the cluster %s managed by cluster-api", machine.cluster.Name)

	spec.ExtraConfig = pv.addOvfEnv(machine, machine.config, vmProps, userData, metaData)

	spec.Files = &types.VirtualMachineFileInfo{
		VmPathName: fmt.Sprintf("[%s]", machine.config.MachineSpec.Datastore),
	}

	return nil
}

func (pv *Provisioner) copyDisks(ctx context.Context, machine *GovmomiMachine, devices object.VirtualDeviceList, vmProps *mo.VirtualMachine) (object.VirtualDeviceList, error) {
	ds, err := machine.s.finder.DatastoreOrDefault(ctx, machine.config.MachineSpec.Datastore)
	if err != nil {
		return devices, err
	}
	dc, err := machine.s.finder.DefaultDatacenter(ctx)
	if err != nil {
		return devices, err
	}

	machine.s.finder.SetDatacenter(dc)
	dstf := fmt.Sprintf("[%s] %s", machine.config.MachineSpec.Datastore, machine.Name)
	m := ds.NewFileManager(dc, false)
	err = m.FileManager.MakeDirectory(ctx, dstf, dc, true)
	if err != nil {
		if soap.IsSoapFault(err) {
			soapFault := soap.ToSoapFault(err)
			// Exit with error only if it's not EEXIST
			if _, ok := soapFault.VimFault().(types.FileAlreadyExists); !ok {
				return devices, err
			}
		}
	}

	// Fetch disk info from Template VM
	l := object.VirtualDeviceList(vmProps.Config.Hardware.Device)
	disks := l.SelectByType((*types.VirtualDisk)(nil))

	scsi, err := devices.CreateSCSIController("")
	if err != nil {
		return devices, err
	}

	devices = append(devices, scsi)
	controller, err := devices.FindDiskController(devices.Name(scsi))
	if err != nil {
		return devices, err
	}

	// Iterate through the machine spec and then iterate over the VM's disks
	for _, diskSpec := range machine.config.MachineSpec.Disks {
		for _, dev := range disks {
			srcdisk := dev.(*types.VirtualDisk)

			if srcdisk.DeviceInfo.GetDescription().Label == diskSpec.DiskLabel {
				newSize := diskSpec.DiskSizeGB
				if srcdisk.CapacityInBytes > vsphereutils.GiBToByte(newSize) {
					return devices, errors.New("Disk size provided should be more than actual disk size of the template")
				}
				srcdisk.CapacityInBytes = vsphereutils.GiBToByte(newSize)
			}
			root := fmt.Sprintf("/vmfs/volumes/%s/%s", ds.Reference().Value, machine.Name)
			dstPath := fmt.Sprintf("%s/%s-%s.vmdk", root, machine.Name, diskSpec.DiskLabel)
			src := srcdisk.Backing.(types.BaseVirtualDeviceFileBackingInfo).GetVirtualDeviceFileBackingInfo()
			srcPath := fmt.Sprintf("/vmfs/volumes/%s/%s", src.Datastore.Value, strings.Split(src.FileName, " ")[1])

			if err := pv.thinCopyDisks(ctx, machine, srcPath, dstPath); err != nil {
				return devices, err
			}
			// attach disk to VM
			disk := devices.CreateDisk(controller, ds.Reference(), ds.Path(fmt.Sprintf("%s/%s-%s.vmdk", machine.Name, machine.Name, diskSpec.DiskLabel)))
			devices = append(devices, disk)
		}
	}

	return devices, nil
}

func (pv *Provisioner) addNetworking(ctx context.Context, machine *GovmomiMachine, devices object.VirtualDeviceList) (object.VirtualDeviceList, error) {
	klog.V(4).Infof("[provision] Adding NICs")
	for _, network := range machine.config.MachineSpec.Networks {
		netRef, err := machine.s.finder.Network(ctx, network.NetworkName)
		if err != nil {
			return devices, err
		}

		backing, err := netRef.EthernetCardBackingInfo(ctx)
		if err != nil {
			return devices, err
		}

		netdev, err := object.EthernetCardTypes().CreateEthernetCard("vmxnet3", backing)
		if err != nil {
			return devices, err
		}
		devices = append(devices, netdev)
	}

	return devices, nil
}

func (pv *Provisioner) addSerialPort(ctx context.Context, devices object.VirtualDeviceList, vmProps *mo.VirtualMachine) (object.VirtualDeviceList, error) {
	// Add SIO Controller
	klog.V(4).Infof("[provision] Adding SIO controller")
	l := object.VirtualDeviceList(vmProps.Config.Hardware.Device)
	controllers := l.SelectByType((*types.VirtualSIOController)(nil))

	if len(controllers) == 0 {
		// Add a serial port
		klog.V(4).Infof("[provision] Adding SIO controller")
		c := &types.VirtualSIOController{}
		c.Key = devices.NewKey()
		devices = append(devices, c)
	}

	for _, d := range controllers {
		c := d.(*types.VirtualSIOController)
		c.Key = devices.NewKey()
		devices = append(devices, c)
	}

	// Add serial port
	ports := l.SelectByType((*types.VirtualSerialPort)(nil))

	if len(ports) == 0 {
		klog.V(4).Infof("[provision] Adding serial port")
		portSpec, err := devices.CreateSerialPort()
		if err != nil {
			return nil, fmt.Errorf("[provision] Failed to add serial port: %s", err)
		}
		portSpec.Key = devices.NewKey()
		devices = append(devices, portSpec)
	}

	for _, d := range ports {
		p := d.(*types.VirtualSerialPort)
		p.Key = devices.NewKey()
		devices = append(devices, p)
	}

	return devices, nil
}
