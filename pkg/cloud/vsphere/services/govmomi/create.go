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
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"net"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"

	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/certificates"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeadm"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/userdata"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
)

const (
	// localIPV4lookup resolves via cloudinit and looks up the instance's IP through the provider's metadata service.
	// See https://cloudinit.readthedocs.io/en/latest/topics/instancedata.html
	localIPV4Lookup = "{{ ds.meta_data.local_ipv4 }}"

	// hostnameLookup resolves via cloud init and uses cloud provider's metadata service to lookup its own hostname.
	hostnameLookup = "{{ ds.meta_data.hostname }}"

	// containerdSocket is the path to containerd socket.
	containerdSocket = "/var/run/containerd/containerd.sock"

	// nodeRole is the label assigned to every node in the cluster.
	nodeRole = "node-role.kubernetes.io/node="
)

// Create creates a new machine.
func Create(ctx *context.MachineContext, bootstrapToken string) error {

	if taskRef := ctx.MachineStatus.TaskRef; taskRef != "" {
		return verifyAndUpdateTask(ctx, taskRef)
	}

	// Before going for cloning, check if we can locate a VM with the InstanceUUID
	// as this Machine. If found, that VM is the right match for this machine
	vmRef, err := findVMByInstanceUUID(ctx)
	if err != nil {
		return errors.Wrapf(err, "unable to find VM by instance UUID")
	}
	if vmRef != "" {
		if vmRef == "creating" {
			return &clustererror.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds}
		}
		record.Eventf(ctx.Machine, "CreateSuccess", "created machine %q", ctx)
		ctx.MachineConfig.MachineRef = vmRef
		return nil
	}

	caCertHash, err := certificates.GenerateCertificateHash(ctx.ClusterConfig.CAKeyPair.Cert)
	if err != nil {
		return err
	}

	var controlPlaneEndpoint string
	if bootstrapToken != "" {
		var err error
		if controlPlaneEndpoint, err = ctx.ControlPlaneEndpoint(); err != nil {
			return errors.Wrapf(err, "unable to get control plane endpoint while creating machine %q", ctx)
		}
	}

	var userDataYAML string

	// apply values based on the role of the machine
	switch ctx.Role() {
	case context.ControlPlaneRole:

		if bootstrapToken != "" {
			ctx.Logger.V(2).Info("allowing a machine to join the control plane")

			bindPort := ctx.BindPort()

			kubeadm.SetJoinConfigurationOptions(
				&ctx.MachineConfig.KubeadmConfiguration.Join,
				kubeadm.WithBootstrapTokenDiscovery(
					kubeadm.NewBootstrapTokenDiscovery(
						kubeadm.WithAPIServerEndpoint(controlPlaneEndpoint),
						kubeadm.WithToken(bootstrapToken),
						kubeadm.WithCACertificateHash(caCertHash),
					),
				),
				kubeadm.WithJoinNodeRegistrationOptions(
					kubeadm.NewNodeRegistration(
						kubeadm.WithTaints(ctx.Machine.Spec.Taints),
						kubeadm.WithNodeRegistrationName(hostnameLookup),
						kubeadm.WithCRISocket(containerdSocket),
						//kubeadm.WithKubeletExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
					),
				),
				kubeadm.WithLocalAPIEndpointAndPort(localIPV4Lookup, int(bindPort)),
			)
			joinConfigurationYAML, err := kubeadm.ConfigurationToYAML(&ctx.MachineConfig.KubeadmConfiguration.Join)
			if err != nil {
				return err
			}

			userData, err := userdata.JoinControlPlane(&userdata.ContolPlaneJoinInput{
				SSHAuthorizedKeys: ctx.ClusterConfig.SSHAuthorizedKeys,
				CACert:            string(ctx.ClusterConfig.CAKeyPair.Cert),
				CAKey:             string(ctx.ClusterConfig.CAKeyPair.Key),
				EtcdCACert:        string(ctx.ClusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:         string(ctx.ClusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert:  string(ctx.ClusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:   string(ctx.ClusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:            string(ctx.ClusterConfig.SAKeyPair.Cert),
				SaKey:             string(ctx.ClusterConfig.SAKeyPair.Key),
				JoinConfiguration: joinConfigurationYAML,
			})
			if err != nil {
				return err
			}

			userDataYAML = userData
		} else {
			ctx.Logger.V(2).Info("initializing a new cluster")

			bindPort := ctx.MachineConfig.KubeadmConfiguration.Init.LocalAPIEndpoint.BindPort
			if bindPort == 0 {
				bindPort = constants.DefaultBindPort
			}
			certSans := []string{localIPV4Lookup}
			if v := ctx.ClusterConfig.ClusterConfiguration.ControlPlaneEndpoint; v != "" {
				host, _, err := net.SplitHostPort(v)
				if err != nil {
					return err
				}
				certSans = append(certSans, host)
			}

			klog.V(2).Info("machine is the first control plane machine for the cluster")
			if !ctx.ClusterConfig.CAKeyPair.HasCertAndKey() {
				return errors.New("failed to run controlplane, missing CAPrivateKey")
			}

			kubeadm.SetClusterConfigurationOptions(
				&ctx.ClusterConfig.ClusterConfiguration,
				kubeadm.WithControlPlaneEndpoint(fmt.Sprintf("%s:%d", localIPV4Lookup, bindPort)),
				kubeadm.WithAPIServerCertificateSANs(certSans...),
				//kubeadm.WithAPIServerExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
				//kubeadm.WithControllerManagerExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
				kubeadm.WithClusterName(ctx.Cluster.Name),
				kubeadm.WithClusterNetworkFromClusterNetworkingConfig(ctx.Cluster.Spec.ClusterNetwork),
				kubeadm.WithKubernetesVersion(ctx.Machine.Spec.Versions.ControlPlane),
			)
			clusterConfigYAML, err := kubeadm.ConfigurationToYAML(&ctx.ClusterConfig.ClusterConfiguration)
			if err != nil {
				return err
			}

			kubeadm.SetInitConfigurationOptions(
				&ctx.MachineConfig.KubeadmConfiguration.Init,
				kubeadm.WithNodeRegistrationOptions(
					kubeadm.NewNodeRegistration(
						kubeadm.WithTaints(ctx.Machine.Spec.Taints),
						kubeadm.WithNodeRegistrationName(hostnameLookup),
						kubeadm.WithCRISocket(containerdSocket),
						//kubeadm.WithKubeletExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
					),
				),
				kubeadm.WithInitLocalAPIEndpointAndPort(localIPV4Lookup, int(bindPort)),
			)
			initConfigYAML, err := kubeadm.ConfigurationToYAML(&ctx.MachineConfig.KubeadmConfiguration.Init)
			if err != nil {
				return err
			}

			userData, err := userdata.NewControlPlane(&userdata.ControlPlaneInput{
				SSHAuthorizedKeys:    ctx.ClusterConfig.SSHAuthorizedKeys,
				CACert:               string(ctx.ClusterConfig.CAKeyPair.Cert),
				CAKey:                string(ctx.ClusterConfig.CAKeyPair.Key),
				EtcdCACert:           string(ctx.ClusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:            string(ctx.ClusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert:     string(ctx.ClusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:      string(ctx.ClusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:               string(ctx.ClusterConfig.SAKeyPair.Cert),
				SaKey:                string(ctx.ClusterConfig.SAKeyPair.Key),
				ClusterConfiguration: clusterConfigYAML,
				InitConfiguration:    initConfigYAML,
			})
			if err != nil {
				return err
			}

			userDataYAML = userData
		}

	case context.NodeRole:
		ctx.Logger.V(2).Info("joining a worker node")

		kubeadm.SetJoinConfigurationOptions(
			&ctx.MachineConfig.KubeadmConfiguration.Join,
			kubeadm.WithBootstrapTokenDiscovery(
				kubeadm.NewBootstrapTokenDiscovery(
					kubeadm.WithAPIServerEndpoint(controlPlaneEndpoint),
					kubeadm.WithToken(bootstrapToken),
					kubeadm.WithCACertificateHash(caCertHash),
				),
			),
			kubeadm.WithJoinNodeRegistrationOptions(
				kubeadm.NewNodeRegistration(
					kubeadm.WithNodeRegistrationName(hostnameLookup),
					kubeadm.WithCRISocket(containerdSocket),
					//kubeadm.WithKubeletExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
					kubeadm.WithTaints(ctx.Machine.Spec.Taints),
					kubeadm.WithKubeletExtraArgs(map[string]string{"node-labels": nodeRole}),
				),
			),
		)
		joinConfigurationYAML, err := kubeadm.ConfigurationToYAML(&ctx.MachineConfig.KubeadmConfiguration.Join)
		if err != nil {
			return err
		}

		userData, err := userdata.NewNode(&userdata.NodeInput{
			SSHAuthorizedKeys: ctx.ClusterConfig.SSHAuthorizedKeys,
			JoinConfiguration: joinConfigurationYAML,
		})
		if err != nil {
			return nil
		}

		userDataYAML = userData

	default:
		return errors.Errorf("unknown role %q for machine %q", ctx.Role(), ctx)
	}

	userData64 := base64.StdEncoding.EncodeToString([]byte(userDataYAML))

	// Use the appropriate path if we're connected to a vCenter
	if ctx.Session.IsVC() {
		return cloneVirtualMachineOnVCenter(ctx, userData64)
	}

	// fallback in case we're connected to a standalone ESX host
	return errors.New("temporarily disabled esx cloning")
}

func findVMByInstanceUUID(ctx *context.MachineContext) (string, error) {
	ctx.Logger.V(4).Info("finding vm by instance UUID", "instance-uuid", ctx.Machine.UID)

	vmRef, err := ctx.Session.FindByInstanceUUID(ctx, string(ctx.Machine.UID))
	if err != nil {
		return "", errors.Wrapf(err, ctx.String())
	}

	if vmRef != nil {
		ctx.Logger.V(2).Info("found machine by instance UUID", "vmRef", vmRef.Reference().Value, "instance-uuid", ctx.Machine.UID)
		return vmRef.Reference().Value, nil
	}

	return ctx.MachineConfig.MachineRef, nil
}

func verifyAndUpdateTask(ctx *context.MachineContext, taskRef string) error {
	ctx.Logger.V(4).Info("verifyig and updating tasks")

	var obj mo.Task
	moRef := types.ManagedObjectReference{
		Type:  "task",
		Value: taskRef,
	}

	logger := ctx.Logger.WithName(taskRef)

	if err := ctx.Session.RetrieveOne(ctx, moRef, []string{"info"}, &obj); err != nil {
		logger.V(4).Info("task does not exist")
		ctx.MachineStatus.TaskRef = ""
		return nil
	}

	logger.V(4).Info("task found", "state", obj.Info.State)

	switch obj.Info.State {

	case types.TaskInfoStateQueued:
		logger.V(4).Info("task is still pending")
		return &clustererror.RequeueAfterError{RequeueAfter: time.Second * 5}

	case types.TaskInfoStateRunning:
		logger.V(4).Info("task is still running")
		return &clustererror.RequeueAfterError{RequeueAfter: time.Second * 5}

	case types.TaskInfoStateSuccess:
		logger.V(4).Info("task is a success")

		switch obj.Info.DescriptionId {

		case "folder.createVm":
			logger.V(4).Info("task is a create op")
			vmRef := obj.Info.Result.(types.ManagedObjectReference)
			vm := object.NewVirtualMachine(ctx.Session.Client.Client, vmRef)
			logger.V(4).Info("powering on VM")
			task, err := vm.PowerOn(ctx)
			if err != nil {
				return errors.Wrapf(err, "error triggering power on op for machine %q", ctx)
			}
			logger.V(4).Info("waiting for power on op to complete")
			if _, err := task.WaitForResult(ctx, nil); err != nil {
				return errors.Wrapf(err, "error powering on machine %q", ctx)
			}
			ctx.MachineConfig.MachineRef = vmRef.Value
			ctx.MachineStatus.TaskRef = ""
			record.Eventf(ctx.Machine, "CreateSuccess", "created machine %q", ctx)
			return nil

		case "virtualMachine.clone":
			logger.V(4).Info("task is a clone op")
			vmRef := obj.Info.Result.(types.ManagedObjectReference)
			ctx.MachineConfig.MachineRef = vmRef.Value
			ctx.MachineStatus.TaskRef = ""
			record.Eventf(ctx.Machine, "CloneSuccess", "cloned machine %q", ctx)
			return nil

		case "virtualMachine.reconfigure":
			record.Eventf(ctx.Machine, "ReconfigSuccess", "reconfigured machine %q", ctx)
			ctx.MachineStatus.TaskRef = ""
			return nil
		}

	case types.TaskInfoStateError:
		logger.V(2).Info("task failed", "description-id", obj.Info.DescriptionId)

		switch obj.Info.DescriptionId {

		case "virtualMachine.clone":
			record.Warnf(ctx.Machine, "CloneFailure", "clone machine failed %q", ctx)
			ctx.MachineStatus.TaskRef = ""

		case "folder.createVm":
			record.Warnf(ctx.Machine, "CreateFailure", "create machine failed %q", ctx)
			ctx.MachineStatus.TaskRef = ""
		}

	default:
		return errors.Errorf("task %q has unknown state %v", taskRef, obj.Info.State)
	}

	return nil
}

func cloneVirtualMachineOnVCenter(ctx *context.MachineContext, userData string) error {
	ctx.Logger.V(4).Info("starting the clone process on vCenter")

	// Let's check to make sure we can find the template earlier on... Plus, we need
	// the cluster/host info if we want to deploy direct to the cluster/host.
	var src *object.VirtualMachine

	if isValidUUID(ctx.MachineConfig.MachineSpec.VMTemplate) {
		ctx.Logger.V(4).Info("trying to resolve the VMTemplate as InstanceUUID", "instance-uuid", ctx.MachineConfig.MachineSpec.VMTemplate)

		tplRef, err := ctx.Session.FindByInstanceUUID(ctx, ctx.MachineConfig.MachineSpec.VMTemplate)
		if err != nil {
			return errors.Wrap(err, "error querying template by instance UUID")
		}
		if tplRef != nil {
			src = object.NewVirtualMachine(ctx.Session.Client.Client, tplRef.Reference())
		}
	}

	if src == nil {
		ctx.Logger.V(4).Info("trying to resolve the VMTemplate as name", "name", ctx.MachineConfig.MachineSpec.VMTemplate)
		tpl, err := ctx.Session.Finder.VirtualMachine(ctx, ctx.MachineConfig.MachineSpec.VMTemplate)
		if err != nil {
			return errors.Wrapf(err, "unable to find VMTemplate %q", ctx.MachineConfig.MachineSpec.VMTemplate)
		}
		src = tpl
	}

	host, err := src.HostSystem(ctx)
	if err != nil {
		return errors.Wrap(err, "hostSystem failed")
	}
	hostProps, err := PropertiesHost(ctx, host)
	if err != nil {
		return errors.Wrap(err, "unable to fetch host properties")
	}

	// Since it's assumed that the ResourcePool name has been provided in the config, if we
	// want to deploy directly to the cluster/host, then we need to override the ResourcePool
	// path before generating the Cloud Provider config. This is done below in:
	// getCloudInitUserData()
	// +--- getCloudProviderConfig()
	resourcePoolPath := ""
	if len(ctx.MachineConfig.MachineSpec.ResourcePool) == 0 {
		resourcePoolPath = fmt.Sprintf("/%s/host/%s/Resource", ctx.MachineConfig.MachineSpec.Datacenter, hostProps.Name)
		ctx.Logger.V(2).Info("attempting to deploy directly to cluster/host resource pool", "pool", resourcePoolPath)
	}

	metaData, err := getCloudInitMetaData(ctx)
	if err != nil {
		return errors.Wrapf(err, "unable to get cloud-init metadata for machine %q", ctx)
	}

	var spec types.VirtualMachineCloneSpec
	ctx.Logger.V(4).Info("preparing clone spec", "folder", ctx.MachineConfig.MachineSpec.VMFolder)

	vmFolder, err := ctx.Session.Finder.FolderOrDefault(ctx, ctx.MachineConfig.MachineSpec.VMFolder)
	if err != nil {
		return errors.Wrapf(err, "unable to get folder for machine %q", ctx)
	}

	datastore, err := ctx.Session.Finder.DatastoreOrDefault(ctx, ctx.MachineConfig.MachineSpec.Datastore)
	if err != nil {
		return errors.Wrapf(err, "unable to get folder for machine %q", ctx)
	}
	spec.Location.Datastore = types.NewReference(datastore.Reference())

	diskUUIDEnabled := true

	spec.Config = &types.VirtualMachineConfigSpec{
		// Use the object UID as the instanceUUID for the VM
		InstanceUuid: string(ctx.Machine.UID),
		Flags: &types.VirtualMachineFlagInfo{
			DiskUuidEnabled: &diskUUIDEnabled,
		},
	}

	ctx.Logger.V(4).Info("assigned VM instance UUID from machine UID", "uid", string(ctx.Machine.UID))

	if ctx.MachineConfig.MachineSpec.NumCPUs > 0 {
		spec.Config.NumCPUs = int32(ctx.MachineConfig.MachineSpec.NumCPUs)
	}
	if ctx.MachineConfig.MachineSpec.MemoryMB > 0 {
		spec.Config.MemoryMB = ctx.MachineConfig.MachineSpec.MemoryMB
	}
	spec.Config.Annotation = ctx.String()
	spec.Location.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndConsolidate)

	vmProps, err := PropertiesVM(ctx, src)
	if err != nil {
		return errors.Wrapf(err, "error fetching vm/template properties while creating machine %q", ctx)
	}

	if len(ctx.MachineConfig.MachineSpec.ResourcePool) > 0 {
		pool, err := ctx.Session.Finder.ResourcePoolOrDefault(ctx, ctx.MachineConfig.MachineSpec.ResourcePool)

		if _, ok := err.(*find.NotFoundError); ok {
			ctx.Logger.V(2).Info("failed to find resource pool, attempting to create it", "pool", ctx.MachineConfig.MachineSpec.ResourcePool)

			poolRoot, err := host.ResourcePool(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to find root resource pool")
			}

			ctx.Logger.V(4).Info("creating resource pool using default values")
			pool2, err := poolRoot.Create(ctx, ctx.MachineConfig.MachineSpec.ResourcePool, types.DefaultResourceConfigSpec())
			if err != nil {
				return errors.Wrap(err, "failed to create resource pool")
			}

			pool = pool2
		}

		spec.Location.Pool = types.NewReference(pool.Reference())
	} else {
		ctx.Logger.V(2).Info("attempting to use host resource pool")
		pool, err := host.ResourcePool(ctx)
		if err != nil {
			return errors.Wrap(err, "host resource pool failed")
		}
		spec.Location.Pool = types.NewReference(pool.Reference())
	}
	spec.PowerOn = true

	var extraconfigs []types.BaseOptionValue
	extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.metadata", Value: metaData})
	extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.metadata.encoding", Value: "base64"})
	extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.userdata", Value: userData})
	extraconfigs = append(extraconfigs, &types.OptionValue{Key: "guestinfo.userdata.encoding", Value: "base64"})
	spec.Config.ExtraConfig = extraconfigs

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
	}(ctx.MachineConfig.MachineSpec.Disks)
	diskChange := false
	for _, dev := range disks {
		disk := dev.(*types.VirtualDisk)
		if newSize, ok := diskMap[disk.DeviceInfo.GetDescription().Label]; ok {
			if disk.CapacityInBytes > giBToByte(newSize) {
				return errors.New("disk size provided should be more than actual disk size of the template")
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
	if !diskChange && len(ctx.MachineConfig.MachineSpec.Disks) > 0 {
		return errors.New("invalid disk configuration")
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
	for _, network := range ctx.MachineConfig.MachineSpec.Networks {
		netRef, err := ctx.Session.Finder.Network(ctx, network.NetworkName)
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

	ctx.Logger.V(6).Info("cloning machine", "clone-spec", spec)
	task, err := src.Clone(ctx, vmFolder, ctx.Machine.Name, spec)
	if err != nil {
		return errors.Wrapf(err, "error trigging clone op for machine %q", ctx)
	}

	ctx.MachineConfig.MachineRef = "creating"
	ctx.MachineStatus.TaskRef = task.Reference().Value

	return nil
}

// PropertiesVM is a convenience method that wraps fetching the VirtualMachine
// MO from its higher-level object.
func PropertiesVM(ctx *context.MachineContext, vm *object.VirtualMachine) (*mo.VirtualMachine, error) {
	var props mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

// PropertiesHost is a convenience method that wraps fetching the
// HostSystem MO from its higher-level object.
func PropertiesHost(ctx *context.MachineContext, host *object.HostSystem) (*mo.HostSystem, error) {
	var props mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

func getCloudInitMetaData(ctx *context.MachineContext) (string, error) {
	buf := &bytes.Buffer{}
	tpl := template.Must(template.New("t").Parse(metaDataFormat))

	if err := tpl.Execute(buf, struct {
		Hostname string
		Networks []vsphereconfigv1.NetworkSpec
	}{
		Hostname: ctx.Machine.Name,
		Networks: ctx.MachineConfig.MachineSpec.Networks,
	}); err != nil {
		return "", errors.Wrapf(err, "error getting cloud init metadata for machine %q", ctx)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func isValidUUID(str string) bool {
	_, err := uuid.Parse(str)
	return err == nil
}

// byteToGiB returns n/1024^3. The input must be an integer that can be
// appropriately divisible.
func byteToGiB(n int64) int64 {
	return n / int64(math.Pow(1024, 3))
}

// GiBTgiBToByteoByte returns n*1024^3.
func giBToByte(n int64) int64 {
	return int64(n * int64(math.Pow(1024, 3)))
}
