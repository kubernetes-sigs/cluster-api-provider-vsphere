package govmomi

import (
	"fmt"
	"os"
	"path"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/vmware/govmomi/find"

	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	. "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

type VmSpecBuilder struct {
	Context    context.Context
	Logger     logr.Logger
	Machine    *MachineContext
	Src        *object.VirtualMachine
	Host       *object.HostSystem
	Folder     *object.Folder
	Datastore  *object.Datastore
	Datacenter *object.Datacenter
	Pool       *object.ResourcePool
	Session    *Session
}

// GetVirtualMachineMO is a convenience method that wraps fetching the VirtualMachine
// MO from its higher-level object.
func (ctx *VmSpecBuilder) GetVirtualMachineMO() (*mo.VirtualMachine, error) {
	var props mo.VirtualMachine
	if err := ctx.Src.Properties(ctx.Context, ctx.Src.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

// GetHostMO is a convenience method that wraps fetching the
// HostSystem MO from its higher-level object.
func (ctx *VmSpecBuilder) GetHostMO() (*mo.HostSystem, error) {
	var props mo.HostSystem
	if err := ctx.Host.Properties(ctx.Context, ctx.Host.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

func findVm(ctx *MachineContext) (*object.VirtualMachine, error) {
	// Let's check to make sure we can find the template earlier on... Plus, we need
	// the cluster/host info if we want to deploy direct to the cluster/host.
	var src *object.VirtualMachine
	template := ctx.MachineConfig.MachineSpec.VMTemplate
	if isValidUUID(template) {
		ctx.Logger.V(4).Info("trying to resolve the VMTemplate as InstanceUUID", "instance-uuid", template)

		tplRef, err := ctx.Session.FindByInstanceUUID(ctx, template)
		if err != nil {
			return nil, errors.Wrap(err, "error querying template by instance UUID")
		}
		if tplRef != nil {
			src = object.NewVirtualMachine(ctx.Session.Client.Client, tplRef.Reference())
		}
	}

	if src == nil {
		ctx.Logger.V(4).Info("trying to resolve the VMTemplate as name", "name", template)
		tpl, err := ctx.Session.Finder.VirtualMachine(ctx, template)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to find VMTemplate %q", template)
		}
		src = tpl
	}
	return src, nil
}

func NewVmSpecBuilder(ctx *MachineContext) (*VmSpecBuilder, error) {

	src, err := findVm(ctx)
	if err != nil {
		return nil, err
	}

	host, err := src.HostSystem(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "hostSystem failed")
	}

	vmFolder, err := ctx.Session.Finder.FolderOrDefault(ctx, ctx.MachineConfig.MachineSpec.VMFolder)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get folder for machine %q", ctx)
	}

	datastore, err := ctx.Session.Finder.DatastoreOrDefault(ctx, ctx.MachineConfig.MachineSpec.Datastore)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get datastore for machine %q", ctx)
	}

	datacenter, err := ctx.Session.Finder.DatacenterOrDefault(ctx, ctx.MachineConfig.MachineSpec.Datacenter)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get datacenter for machine %q", ctx)
	}

	builder := &VmSpecBuilder{
		Context:    ctx.Context,
		Machine:    ctx,
		Logger:     ctx.Logger.WithName("spec-builder"),
		Src:        src,
		Host:       host,
		Datastore:  datastore,
		Datacenter: datacenter,
		Folder:     vmFolder,
		Session:    ctx.Session,
	}
	builder.Pool, err = builder.FindPool()
	if err != nil {
		return nil, err
	}
	return builder, nil
}

func (ctx *VmSpecBuilder) FindPool() (*object.ResourcePool, error) {

	name := ctx.Machine.MachineConfig.MachineSpec.ResourcePool

	// Since it's assumed that the ResourcePool name has been provided in the config, if we
	// want to deploy directly to the cluster/host, then we need to override the ResourcePool
	// path before generating the Cloud Provider config. This is done below in:
	// getCloudInitUserData()
	// +--- getCloudProviderConfig()
	resourcePoolPath := ""
	if len(name) == 0 {
		mo, err := ctx.GetHostMO()
		if err != nil {
			return nil, err
		}
		resourcePoolPath = fmt.Sprintf("/%s/host/%s/Resource", ctx.Machine.MachineConfig.MachineSpec.Datacenter, mo.Name)
		ctx.Logger.V(2).Info("attempting to deploy directly to cluster/host resource pool", "pool", resourcePoolPath)
	}

	var pool *object.ResourcePool
	var err error
	if len(name) > 0 {
		pool, err = ctx.Session.Finder.ResourcePoolOrDefault(ctx.Context, name)

		if _, ok := err.(*find.NotFoundError); ok {
			ctx.Logger.V(2).Info("failed to find resource pool, attempting to create it", "pool", name)
			poolRoot, err := ctx.Host.ResourcePool(ctx.Context)
			if err != nil {
				return nil, errors.Wrap(err, "failed to find root resource pool")
			}

			ctx.Logger.V(4).Info("creating resource pool using default values")
			pool2, err := poolRoot.Create(ctx.Context, name, types.DefaultResourceConfigSpec())
			if err != nil {
				return nil, errors.Wrap(err, "failed to create resource pool")
			}

			pool = pool2
		}
	} else {
		ctx.Logger.V(2).Info("attempting to use host resource pool")
		pool, err = ctx.Host.ResourcePool(ctx.Context)
		if err != nil {
			return nil, errors.Wrap(err, "host resource pool failed")
		}
	}
	return pool, nil
}

func (ctx *VmSpecBuilder) GetVirtualDisks() ([]types.VirtualDisk, error) {
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

func (ctx *VmSpecBuilder) GetDisks() ([]types.BaseVirtualDeviceConfigSpec, error) {
	mo, err := ctx.GetVirtualMachineMO()
	if err != nil {
		return nil, err
	}
	l := object.VirtualDeviceList(mo.Config.Hardware.Device)
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
	}(ctx.Machine.MachineConfig.MachineSpec.Disks)
	diskChange := false
	for _, dev := range disks {
		disk := dev.(*types.VirtualDisk)
		if newSize, ok := diskMap[disk.DeviceInfo.GetDescription().Label]; ok {
			if disk.CapacityInBytes > giBToByte(newSize) {
				return nil, errors.New("disk size provided should be more than actual disk size of the template")
			}
			ctx.Logger.V(4).Info("resizing the disk", "disk-label", disk.DeviceInfo.GetDescription().Label, "new-size", newSize)
			diskChange = true
			disk.CapacityInBytes = giBToByte(newSize)
			diskspec := &types.VirtualDeviceConfigSpec{}
			diskspec.Operation = types.VirtualDeviceConfigSpecOperationEdit
			diskspec.Device = disk
			deviceSpecs = append(deviceSpecs, diskspec)
		}
	}
	if !diskChange && len(ctx.Machine.MachineConfig.MachineSpec.Disks) > 0 {
		return nil, errors.New("invalid disk configuration")
	}
	return deviceSpecs, nil
}

func (ctx *VmSpecBuilder) GetNics() ([]types.BaseVirtualDeviceConfigSpec, error) {
	var devices object.VirtualDeviceList
	for _, network := range ctx.Machine.MachineConfig.MachineSpec.Networks {
		netRef, err := ctx.Machine.Session.Finder.Network(ctx.Context, network.NetworkName)
		if err != nil {
			return nil, err
		}

		backing, err := netRef.EthernetCardBackingInfo(ctx.Context)
		if err != nil {
			return nil, err
		}

		netdev, err := object.EthernetCardTypes().CreateEthernetCard("vmxnet3", backing)
		if err != nil {
			return nil, err
		}
		devices = append(devices, netdev)
	}
	return devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
}

func (b *VmSpecBuilder) GetCloneSpec() *types.VirtualMachineCloneSpec {
	var spec types.VirtualMachineCloneSpec

	return &spec
}

func (b *VmSpecBuilder) GetExtraOptions(userData, metaData string) []types.BaseOptionValue {
	var extraconfigs []types.BaseOptionValue
	extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.metadata", Value: metaData})
	extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.metadata.encoding", Value: "base64"})
	extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.userdata", Value: userData})
	extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.userdata.encoding", Value: "base64"})
	return extraconfigs
}

func (ctx *VmSpecBuilder) GetVirtualMachineConfigSpec() *types.VirtualMachineConfigSpec {
	diskUUIDEnabled := true
	config := &types.VirtualMachineConfigSpec{
		// Use the object UID as the instanceUUID for the VM
		InstanceUuid: string(ctx.Machine.Machine.UID),
		Flags: &types.VirtualMachineFlagInfo{
			DiskUuidEnabled: &diskUUIDEnabled,
		},
	}

	mo, _ := ctx.GetVirtualMachineMO()
	config.GuestId = mo.Config.GuestId         // set GuestId from template
	config.Firmware = mo.Config.Firmware       // set Firmware from template
	config.BootOptions = mo.Config.BootOptions // set BootOptions from Template

	ctx.Logger.V(4).Info("assigned VM instance UUID from machine UID", "uid", string(ctx.Machine.Machine.UID))
	if ctx.Machine.MachineConfig.MachineSpec.NumCPUs > 0 {
		config.NumCPUs = int32(ctx.Machine.MachineConfig.MachineSpec.NumCPUs)
	}
	if ctx.Machine.MachineConfig.MachineSpec.MemoryMB > 0 {
		config.MemoryMB = ctx.Machine.MachineConfig.MachineSpec.MemoryMB
	}
	config.Annotation = ctx.Machine.String()
	return config
}

func (ctx *VmSpecBuilder) GetSerialPort(devices object.VirtualDeviceList) (object.VirtualDeviceList, error) {
	ctx.Logger.V(6).Info("Adding SIO controller")
	c := &types.VirtualSIOController{}
	c.Key = -301
	devices = append(devices, c)

	ctx.Logger.V(6).Info("Adding Serial Port")

	portSpec, err := devices.CreateSerialPort()
	if err != nil {
		return nil, fmt.Errorf("Failed to add serial port: %s", err)
	}
	portSpec.Key = -302
	portSpec.ControllerKey = c.Key
	devices = append(devices, portSpec)

	return devices, nil
}

func (ctx *VmSpecBuilder) ThinCloneDisks() ([]types.BaseVirtualDeviceConfigSpec, error) {
	devices := object.VirtualDeviceList{}
	disks, err := ctx.GetVirtualDisks()
	if err != nil {
		return nil, err
	}
	ctx.Logger.V(4).Info("cloning", "machine", ctx.Machine.Machine)
	name := ctx.Machine.Machine.Name
	scsi, err := devices.CreateSCSIController("")
	if err != nil {
		return nil, err
	}

	devices = append(devices, scsi)
	controller, err := devices.FindDiskController(devices.Name(scsi))
	if err != nil {
		return nil, err
	}
	for _, disk := range disks {
		root := fmt.Sprintf("/vmfs/volumes/%s/%s", ctx.Datastore.Reference().Value, name)
		dstPath := fmt.Sprintf("%s/%s.vmdk", root, name)
		src := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo).GetVirtualDeviceFileBackingInfo()
		srcPath := fmt.Sprintf("/vmfs/volumes/%s/%s", src.Datastore.Value, strings.Split(src.FileName, " ")[1])
		cmd := fmt.Sprintf("vmkfstools -i \"%s\" \"%s\" -d thin", srcPath, dstPath)
		ssh := ctx.Machine.ClusterContext.GetSSH()
		ctx.Logger.V(4).Info("[clone] ssh: " + cmd)
		if err := ssh.Exec("mkdir -p %s", path.Dir(dstPath)); err != nil {
			return nil, errors.Wrapf(err, "Cannot create directory %s ", path.Base(dstPath))
		}
		if err := ssh.Exec(cmd); err != nil && 1 == 2 {
			return nil, errors.Wrapf(err, "Failed to thin clone disk")
		}

		ctx.Logger.V(4).Info(fmt.Sprintf("[clone] thin cloned vmdk, src(%s) ->  dst(%s)", srcPath, dstPath))
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

func (ctx *VmSpecBuilder) BuildCreateSpec(userData string) (*types.VirtualMachineConfigSpec, error) {
	ctx.Logger.V(4).Info("starting the clone process on ESX")
	ctx.Logger.V(4).Info("preparing clone spec", "folder", ctx.Machine.MachineConfig.MachineSpec.VMFolder)

	metaData, err := getCloudInitMetaData(ctx.Machine)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get cloud-init metadata for machine %q", ctx)
	}

	spec := ctx.GetVirtualMachineConfigSpec()
	spec.Name = ctx.Machine.Machine.Name
	spec.ExtraConfig = ctx.GetExtraOptions(userData, metaData)
	deviceSpecs := []types.BaseVirtualDeviceConfigSpec{}

	disks, err := ctx.ThinCloneDisks()
	if err != nil {
		return nil, err
	}
	for _, disk := range disks {
		deviceSpecs = append(deviceSpecs, disk)
	}

	nics, err := ctx.GetNics()
	if err != nil {
		return nil, err
	}
	for _, device := range nics {
		deviceSpecs = append(deviceSpecs, device)
	}

	var devices object.VirtualDeviceList
	if devices, err = ctx.GetSerialPort(devices); err != nil {
		return nil, err
	}
	devicesConfigs, _ := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	for _, device := range devicesConfigs {
		deviceSpecs = append(deviceSpecs, device)
	}

	spec.DeviceChange = deviceSpecs
	spec.Files = &types.VirtualMachineFileInfo{
		VmPathName: fmt.Sprintf("[%s]", ctx.Datastore.Name()),
	}

	if ctx.Logger.V(6).Enabled() {
		//pretty print config
		data, _ := yaml.Marshal(spec)
		os.Stderr.WriteString("\n" + string(data))
	}

	return spec, nil
}

func (ctx *VmSpecBuilder) BuildCloneSpec(userData string) (*types.VirtualMachineCloneSpec, error) {
	ctx.Logger.V(4).Info("starting the clone process on vCenter")
	ctx.Logger.V(4).Info("preparing clone spec", "folder", ctx.Machine.MachineConfig.MachineSpec.VMFolder)

	metaData, err := getCloudInitMetaData(ctx.Machine)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get cloud-init metadata for machine %q", ctx)
	}

	var spec types.VirtualMachineCloneSpec
	spec.Location.Datastore = types.NewReference(ctx.Datastore.Reference())
	spec.Location.Pool = types.NewReference(ctx.Pool.Reference())
	spec.Location.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndConsolidate)
	spec.Config = ctx.GetVirtualMachineConfigSpec()
	spec.PowerOn = true

	spec.Config.ExtraConfig = ctx.GetExtraOptions(userData, metaData)

	deviceSpecs := []types.BaseVirtualDeviceConfigSpec{}

	disks, err := ctx.GetDisks()
	if err != nil {
		return nil, err
	}

	for _, device := range disks {
		deviceSpecs = append(deviceSpecs, device)
	}

	nics, err := ctx.GetNics()
	if err != nil {
		return nil, err
	}

	for _, device := range nics {
		deviceSpecs = append(deviceSpecs, device)
	}

	spec.Config.DeviceChange = deviceSpecs

	return &spec, nil
}
