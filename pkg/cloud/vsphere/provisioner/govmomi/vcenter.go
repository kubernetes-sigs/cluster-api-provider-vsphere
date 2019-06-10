package govmomi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/vmware/govmomi/find"

	"github.com/vmware/govmomi/object"

	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"

	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
)

type GovmomiVcenter struct {
	s *SessionContext
}

func runSSHCommand(cmd string, host string, username string, password string) error {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			// ssh.Password needs to be explictly allowed, and by default ESXi only allows public + keyboard interactive
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				// Just send the password back for all questions
				answers := make([]string, len(questions))
				for i, _ := range answers {
					answers[i] = password
				}

				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	fmt.Printf("[clone] Connecting to %s@%s", host, username)
	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return err
	}
	defer conn.Close()

	var sess *ssh.Session
	var sessStdOut, sessStderr io.Reader
	if sess, err = conn.NewSession(); err != nil {
		return err
	}

	defer sess.Close()
	if sessStdOut, err = sess.StdoutPipe(); err != nil {
		return err
	}
	go io.Copy(os.Stdout, sessStdOut)
	if sessStderr, err = sess.StderrPipe(); err != nil {
		return err
	}
	go io.Copy(os.Stderr, sessStderr)
	return sess.Run(cmd)
}

// CloneVirtualMachine clones the template to a virtual machine.
func (pv *Provisioner) cloneVirtualMachineOnVCenter(ctx context.Context, machine *GovmomiMachine) error {
	klog.V(4).Infof("[clone] Starting on vCenter")

	dc, err := machine.s.finder.DatacenterOrDefault(ctx, machine.config.MachineSpec.Datacenter)
	if err != nil {
		return err
	}
	machine.s.finder.SetDatacenter(dc)

	// Let's check to make sure we can find the template earlier on... Plus, we need
	// the cluster/host info if we want to deploy direct to the cluster/host.
	src, err := machine.GetCluster().FindVM(ctx, dc, machine.config.MachineSpec.VMTemplate)
	if err != nil {
		return err
	}

	host, err := src.HostSystem(ctx)
	if err != nil {
		return fmt.Errorf("HostSystem failed. err=%s", err)
	}
	hostProps, err := machine.GetCluster().GetHostMO(host)
	if err != nil {
		return fmt.Errorf("error fetching host properties: %s", err)
	}

	// Since it's assumed that the ResourcePool name has been provided in the config, if we
	// want to deploy directly to the cluster/host, then we need to override the ResourcePool
	// path before generating the Cloud Provider config. This is done below in:
	// getCloudInitUserData()
	// +--- getCloudProviderConfig()
	resourcePoolPath := ""
	if len(machine.config.MachineSpec.ResourcePool) == 0 {
		resourcePoolPath = fmt.Sprintf("/%s/host/%s/Resource", machine.config.MachineSpec.Datacenter, hostProps.Name)
		klog.Infof("[clone] Attempting to deploy directly to cluster/host RP: %s", resourcePoolPath)
	}

	// Fetch the user-data for the cloud-init first, so that we can fail fast before even trying to connect to pv
	userData, err := pv.getCloudInitUserData(machine.cluster, machine.machine, resourcePoolPath, true)
	if err != nil {
		// err returned by the getCloudInitUserData would be of type RequeueAfterError in case kubeadm is not ready yet
		return err
	}
	metaData, err := pv.getCloudInitMetaData(machine.cluster, machine.machine)
	if err != nil {
		// err returned by the getCloudInitMetaData would be of type RequeueAfterError in case kubeadm is not ready yet
		return err
	}

	var spec types.VirtualMachineCloneSpec
	klog.V(4).Infof("[clone] Preparing clone spec for VM %s/%s", machine.config.MachineSpec.VMFolder, machine.Name)
	vmFolder, err := machine.s.finder.FolderOrDefault(ctx, machine.config.MachineSpec.VMFolder)
	if err != nil {
		return err
	}

	ds, err := machine.s.finder.DatastoreOrDefault(ctx, machine.config.MachineSpec.Datastore)
	if err != nil {
		return err
	}
	spec.Location.Datastore = types.NewReference(ds.Reference())

	spec.Config = &types.VirtualMachineConfigSpec{}
	// Use the object UID as the instanceUUID for the VM
	spec.Config.InstanceUuid = string(machine.machine.UID)
	diskUUIDEnabled := true
	spec.Config.Flags = &types.VirtualMachineFlagInfo{
		DiskUuidEnabled: &diskUUIDEnabled,
	}
	if machine.config.MachineSpec.NumCPUs > 0 {
		spec.Config.NumCPUs = int32(machine.config.MachineSpec.NumCPUs)
	}
	if machine.config.MachineSpec.MemoryMB > 0 {
		spec.Config.MemoryMB = machine.config.MachineSpec.MemoryMB
	}
	spec.Config.Annotation = fmt.Sprintf("Virtual Machine is part of the cluster %s managed by cluster-api", machine.cluster.Name)
	spec.Location.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndConsolidate)

	vmProps, err := machine.GetCluster().GetVirtualMachineMO(src)
	if err != nil {
		return fmt.Errorf("error fetching vm/template properties: %s", err)
	}

	if len(machine.config.MachineSpec.ResourcePool) > 0 {
		pool, err := machine.s.finder.ResourcePoolOrDefault(ctx, machine.config.MachineSpec.ResourcePool)

		if _, ok := err.(*find.NotFoundError); ok {
			klog.Warningf("[provision] Failed to find ResourcePool=%s err=%s. Attempting to create it.", machine.config.MachineSpec.ResourcePool, err)

			poolRoot, errRoot := host.ResourcePool(ctx)
			if errRoot != nil {
				return fmt.Errorf("[provision] Failed to find root ResourcePool. err=%s", errRoot)
			}

			klog.Info("[provision] Creating ResourcePool using default values. These values can be modified after ResourcePool creation.")
			pool, err = poolRoot.Create(ctx, machine.config.MachineSpec.ResourcePool, types.DefaultResourceConfigSpec())
			if err != nil {
				return fmt.Errorf("[provision] Create ResourcePool failed. err=%s", err)
			}
		}

		spec.Location.Pool = types.NewReference(pool.Reference())
	} else {
		klog.Infof("Attempting to use Host ResourcePool")
		pool, err := host.ResourcePool(ctx)

		if err != nil {
			return fmt.Errorf("Host ResourcePool failed. err=%s", err)
		}

		spec.Location.Pool = types.NewReference(pool.Reference())
	}
	spec.PowerOn = true

	if machine.config.MachineSpec.VsphereCloudInit {
		// In case of vsphere cloud-init datasource present, set the appropriate extraconfig options
		var extraconfigs []types.BaseOptionValue
		extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.metadata", Value: metaData})
		extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.metadata.encoding", Value: "base64"})
		extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.userdata", Value: userData})
		extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.userdata.encoding", Value: "base64"})
		spec.Config.ExtraConfig = extraconfigs
	} else {
		// This case is to support backwords compatibility, where we are using the ubuntu cloud image ovf properties
		// to drive the cloud-init workflow. Once the vsphere cloud-init datastore is merged as part of the official
		// cloud-init, then we can potentially remove this flag from the spec as then all the native cloud images
		// available for the different distros will include this new datasource.
		// See (https://github.com/akutz/cloud-init-vmware-guestinfo/ - vmware cloud-init datasource) for details
		if vmProps.Config.VAppConfig == nil {
			return fmt.Errorf("this source VM lacks a vApp configuration and cannot have vApp properties set on it")
		}
		allProperties := vmProps.Config.VAppConfig.GetVmConfigInfo().Property
		var props []types.VAppPropertySpec
		for _, p := range allProperties {
			defaultValue := " "
			if p.DefaultValue != "" {
				defaultValue = p.DefaultValue
			}
			prop := types.VAppPropertySpec{
				ArrayUpdateSpec: types.ArrayUpdateSpec{
					Operation: types.ArrayUpdateOperationEdit,
				},
				Info: &types.VAppPropertyInfo{
					Key:   p.Key,
					Id:    p.Id,
					Value: defaultValue,
				},
			}
			if p.Id == "user-data" {
				prop.Info.Value = userData
			}
			if p.Id == "public-keys" {
				prop.Info.Value, err = pv.GetSSHPublicKey(machine.cluster)
				if err != nil {
					return err
				}
			}
			if p.Id == "hostname" {
				prop.Info.Value = machine.Name
			}
			props = append(props, prop)
		}
		spec.Config.VAppConfig = &types.VmConfigSpec{
			Property: props,
		}
	}

	l := object.VirtualDeviceList(vmProps.Config.Hardware.Device)
	deviceSpecs := []types.BaseVirtualDeviceConfigSpec{}
	disks := l.SelectByType((*types.VirtualDisk)(nil))
	// For the disks listed under the MachineSpec.Disks property, they are used
	// only for resizing a maching disk on the template. Currently, no new disk
	// is added. Only the matched disks via the DiskLabel are resized. If the
	// MachineSpec.Disks is specified but none of the disks matched to the disks
	// present in the VM Template then error is returned. This is to avoid the
	// case when the user did want to resize but accidentally passed a wrong
	// disk label. A 100% matching of disks in not enforced as the user might be
	// interested in resizing only a subset of disks and thus we don't want to
	// force the user to list all the disk and sizes if they don't want to change
	// all.
	diskMap := func(diskSpecs []vsphereconfigv1.DiskSpec) map[string]int64 {
		diskMap := make(map[string]int64)
		for _, s := range diskSpecs {
			diskMap[s.DiskLabel] = s.DiskSizeGB
		}
		return diskMap
	}(machine.config.MachineSpec.Disks)
	diskChange := false
	for _, dev := range disks {
		disk := dev.(*types.VirtualDisk)
		if newSize, ok := diskMap[disk.DeviceInfo.GetDescription().Label]; ok {
			if disk.CapacityInBytes > vsphereutils.GiBToByte(newSize) {
				return errors.New("[resize] [FATAL] Disk size provided should be more than actual disk size of the template. Please correct the machineSpec to proceed")
			}
			klog.V(4).Infof("[resize] Resizing the disk \"%s\" to new size \"%d\"", disk.DeviceInfo.GetDescription().Label, newSize)
			diskChange = true
			disk.CapacityInBytes = vsphereutils.GiBToByte(newSize)
			diskspec := &types.VirtualDeviceConfigSpec{}
			diskspec.Operation = types.VirtualDeviceConfigSpecOperationEdit
			diskspec.Device = disk
			deviceSpecs = append(deviceSpecs, diskspec)
		}
	}
	if !diskChange && len(machine.config.MachineSpec.Disks) > 0 {
		return fmt.Errorf("[resize] None of the disks specified in the MachineSpec matched with the disks on the template %s", machine.config.MachineSpec.VMTemplate)
	}

	nics := l.SelectByType((*types.VirtualEthernetCard)(nil))
	// Remove any existing nics on the source vm
	for _, dev := range nics {
		nic := dev.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()
		nicspec := &types.VirtualDeviceConfigSpec{}
		nicspec.Operation = types.VirtualDeviceConfigSpecOperationRemove
		nicspec.Device = nic
		deviceSpecs = append(deviceSpecs, nicspec)
	}
	// Add new nics based on the user info
	nicid := int32(-100)
	for _, network := range machine.config.MachineSpec.Networks {
		netRef, err := machine.s.finder.Network(ctx, network.NetworkName)
		if err != nil {
			return err
		}
		nic := types.VirtualVmxnet3{}
		nic.Key = nicid
		nic.Backing, err = netRef.EthernetCardBackingInfo(ctx)
		if err != nil {
			return err
		}
		nicspec := &types.VirtualDeviceConfigSpec{}
		nicspec.Operation = types.VirtualDeviceConfigSpecOperationAdd
		nicspec.Device = &nic
		deviceSpecs = append(deviceSpecs, nicspec)
		nicid--
	}
	spec.Config.DeviceChange = deviceSpecs
	machine.Eventf("Creating", "Creating Machine %v", machine.Name)

	task, err := src.Clone(ctx, vmFolder, machine.Name, spec)
	klog.V(6).Infof("[clone] with spec %v", spec)
	if err != nil {
		return err
	}
	return machine.SetTaskRef(task.Reference().Value)
}

func (pv *Provisioner) thickCopyDisks(ctx context.Context, machine *GovmomiMachine, srcPath string, dstPath string) error {
	var ds *object.Datastore
	var dc *object.Datacenter
	var err error

	if ds, err = machine.s.finder.DatastoreOrDefault(ctx, machine.config.MachineSpec.Datastore); err != nil {
		return err
	}

	if dc, err = machine.s.finder.DefaultDatacenter(ctx); err != nil {
		return err
	}

	m := ds.NewFileManager(dc, false)

	// copy happens here
	klog.V(4).Infof("[copy] Copying template disk to %s", dstPath)
	cp := m.Copy
	if err := cp(ctx, srcPath, dstPath); err != nil {
		klog.V(4).Infof("[copy] thick copied vmdk, src(%s) ->  dst(%s)", srcPath, dstPath)
	} else {
		return err
	}
	return nil

}
