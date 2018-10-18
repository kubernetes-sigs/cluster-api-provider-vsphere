package govmomi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	vpshereprovisionercommon "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/provisioner/common"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"
	apierrors "sigs.k8s.io/cluster-api/pkg/errors"
	"sigs.k8s.io/cluster-api/pkg/util"
)

func (pv *Provisioner) Create(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	glog.V(4).Infof("govmomi.Actuator.Create %s", machine.Spec.Name)
	s, err := pv.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(*s.context)
	defer cancel()
	usersession, err := s.session.SessionManager.UserSession(ctx)
	if err != nil {
		return err
	}
	glog.V(4).Infof("Using session %v", usersession)
	task := vsphereutils.GetActiveTasks(machine)
	if task != "" {
		// In case an active task is going on, wait for its completion
		return pv.verifyAndUpdateTask(s, machine, task)
	}
	pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Creating", "Creating Machine %v", machine.Name)
	return pv.cloneVirtualMachine(s, cluster, machine)
}

func (pv *Provisioner) verifyAndUpdateTask(s *SessionContext, machine *clusterv1.Machine, taskmoref string) error {
	ctx, cancel := context.WithCancel(*s.context)
	defer cancel()
	// If a task does exist on the
	var taskmo mo.Task
	taskref := types.ManagedObjectReference{
		Type:  "Task",
		Value: taskmoref,
	}
	err := s.session.RetrieveOne(ctx, taskref, []string{"info"}, &taskmo)
	if err != nil {
		//TODO: inspect the error and act appropriately.
		// Naive assumption is that the task does not exist any more, thus clear that from the machine
		return pv.setTaskRef(machine, "")
	}
	switch taskmo.Info.State {
	// Queued or Running
	case types.TaskInfoStateQueued, types.TaskInfoStateRunning:
		// Requeue the machine update to check back in 5 seconds on the task
		return &clustererror.RequeueAfterError{RequeueAfter: time.Second * 5}
	// Successful
	case types.TaskInfoStateSuccess:
		if taskmo.Info.DescriptionId == "VirtualMachine.clone" {
			vmref := taskmo.Info.Result.(types.ManagedObjectReference)
			pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Created", "Created Machine %s(%s)", machine.Name, vmref.Value)
			// Update the Machine object with the VM Reference annotation
			updatedmachine, err := pv.updateVMReference(machine, vmref.Value)
			if err != nil {
				return err
			}
			// This is needed otherwise the update status on the original machine object would fail as the resource has been updated by the previous call
			// Note: We are not mutating the object retrieved from the informer ever. The updatedmachine is the updated resource generated using DeepCopy
			// This would just update the reference to be the newer object so that the status update works
			machine = updatedmachine
			return pv.setTaskRef(machine, "")
		} else if taskmo.Info.DescriptionId == "VirtualMachine.reconfigure" {
			pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Reconfigured", "Reconfigured Machine %s", taskmo.Info.EntityName)
		}
		return pv.setTaskRef(machine, "")
	case types.TaskInfoStateError:
		if taskmo.Info.DescriptionId == "VirtualMachine.clone" {
			pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Failed", "Creation failed for Machine %v", machine.Name)
			// Clear the reference to the failed task so that the next reconcile loop can re-create it
			return pv.setTaskRef(machine, "")
		}
	default:
		glog.Warningf("unknown state %s for task %s detected", taskmoref, taskmo.Info.State)
		return fmt.Errorf("Unknown state %s for task %s detected", taskmoref, taskmo.Info.State)
	}
	return nil
}

// CloneVirtualMachine clones the template to a virtual machine.
func (pv *Provisioner) cloneVirtualMachine(s *SessionContext, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	// Fetch the user-data for the cloud-init first, so that we can fail fast before even trying to connect to pv
	userData, err := pv.getCloudInitUserData(cluster, machine)
	if err != nil {
		// err returned by the getCloudInitUserData would be of type RequeueAfterError in case kubeadm is not ready yet
		return err
	}
	ctx, cancel := context.WithCancel(*s.context)
	defer cancel()

	var spec types.VirtualMachineCloneSpec
	machineConfig, err := vsphereutils.GetMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return err
	}
	glog.V(4).Infof("[DEBUG] ExpandVirtualMachineCloneSpec: Preparing clone spec for VM")

	dc, err := s.finder.DatacenterOrDefault(ctx, machineConfig.MachineVariables[constants.ProviderDatacenter])
	if err != nil {
		return err
	}
	s.finder.SetDatacenter(dc)

	vmFolder, err := s.finder.FolderOrDefault(ctx, machineConfig.MachineVariables[constants.ProviderVmFolder])
	if err != nil {
		return err
	}

	ds, err := s.finder.DatastoreOrDefault(ctx, machineConfig.MachineVariables[constants.ProviderDatastore])
	if err != nil {
		return err
	}
	spec.Location.Datastore = types.NewReference(ds.Reference())

	pool, err := s.finder.ResourcePoolOrDefault(ctx, machineConfig.MachineVariables[constants.ProviderResPool])
	if err != nil {
		return err
	}
	spec.Location.Pool = types.NewReference(pool.Reference())
	spec.PowerOn = true

	spec.Config = &types.VirtualMachineConfigSpec{}
	diskUUIDEnabled := true
	spec.Config.Flags = &types.VirtualMachineFlagInfo{
		DiskUuidEnabled: &diskUUIDEnabled,
	}
	// var extraconfigs []types.BaseOptionValue
	// extraconfigs = append(extraconfigs, &types.OptionValue{Key: "govmomi.Test", Value: "Yay"})
	// spec.Config.ExtraConfig = extraconfigs
	if scpu, ok := machineConfig.MachineVariables["num_cpus"]; ok {
		cpu, err := strconv.ParseInt(scpu, 10, 32)
		if err != nil {
			return err
		}
		spec.Config.NumCPUs = int32(cpu)
	}
	if smemory, ok := machineConfig.MachineVariables["memory"]; ok {
		memory, err := strconv.ParseInt(smemory, 10, 64)
		if err != nil {
			return err
		}
		spec.Config.MemoryMB = memory
	}
	spec.Config.Annotation = fmt.Sprintf("Virtual Machine is part of the cluster %s managed by cluster-api", cluster.Name)
	spec.Location.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndAllowSharing)
	src, err := s.finder.VirtualMachine(ctx, machineConfig.MachineVariables[constants.ProviderTemplate])
	if err != nil {
		return err
	}
	vmProps, err := Properties(src)
	if err != nil {
		return fmt.Errorf("error fetching virtual machine or template properties: %s", err)
	}
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
			prop.Info.Value, err = pv.GetSSHPublicKey(cluster)
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
	// reconfigure disks as needed
	l := object.VirtualDeviceList(vmProps.Config.Hardware.Device)
	diskSpecs := []types.BaseVirtualDeviceConfigSpec{}
	disks := l.SelectByType((*types.VirtualDisk)(nil))
	var targetdisk *types.VirtualDisk
	disklabel := machineConfig.MachineVariables["disk_label"]
	for _, dev := range disks {
		disk := dev.(*types.VirtualDisk)
		if disk.DeviceInfo.GetDescription().Label == disklabel {
			newsize, err := strconv.ParseInt(machineConfig.MachineVariables["disk_size"], 10, 64)
			if err != nil {
				return err
			}
			if disk.CapacityInBytes > vsphereutils.GiBToByte(newsize) {
				return errors.New("Disk size provided should be more than actual disk size of the template")
			}
			disk.CapacityInBytes = vsphereutils.GiBToByte(newsize)
			targetdisk = disk
			// Currently we only have 1 disk support so break out here
			break
		}
	}
	if targetdisk == nil {
		return fmt.Errorf("Could not locate the disk with label %s", disklabel)
	}
	diskspec := &types.VirtualDeviceConfigSpec{}
	diskspec.Operation = types.VirtualDeviceConfigSpecOperationEdit
	diskspec.Device = targetdisk
	diskSpecs = append(diskSpecs, diskspec)
	spec.Config.DeviceChange = diskSpecs
	task, err := src.Clone(ctx, vmFolder, machine.Name, spec)
	if err != nil {
		return err
	}
	return pv.setTaskRef(machine, task.Reference().Value)

}

// Properties is a convenience method that wraps fetching the
// VirtualMachine MO from its higher-level object.
func Properties(vm *object.VirtualMachine) (*mo.VirtualMachine, error) {
	glog.Infof("[DEBUG] Fetching properties for VM %q", vm.InventoryPath)
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultAPITimeout)
	defer cancel()
	var props mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

// Removes the current task reference from the Machine object
func (pv *Provisioner) removeTaskRef(machine *clusterv1.Machine) error {
	nmachine := machine.DeepCopy()
	if nmachine.ObjectMeta.Annotations == nil {
		return nil
	}
	delete(nmachine.ObjectMeta.Annotations, constants.VirtualMachineTaskRef)
	_, err := pv.clusterV1alpha1.Machines(nmachine.Namespace).Update(nmachine)
	return err
}

func (vc *Provisioner) updateVMReference(machine *clusterv1.Machine, vmref string) (*clusterv1.Machine, error) {
	providerConfig, err := vsphereutils.GetMachineProviderConfig(machine.Spec.ProviderConfig)
	providerConfig.MachineRef = vmref
	// Set the Kind and APIVersion again since they are not returned
	// See the following Issues for details:
	// https://github.com/kubernetes/client-go/issues/308
	// https://github.com/kubernetes/kubernetes/issues/3030
	providerConfig.Kind = reflect.TypeOf(*providerConfig).Name()
	providerConfig.APIVersion = vsphereconfigv1.SchemeGroupVersion.String()
	newMachine := machine.DeepCopy()
	out, err := json.Marshal(providerConfig)
	if err != nil {
		glog.Infof("Error marshaling ProviderConfig: %s", err)
		return machine, err
	}
	newMachine.Spec.ProviderConfig.Value = &runtime.RawExtension{Raw: out}
	newMachine, err = vc.clusterV1alpha1.Machines(newMachine.Namespace).Update(newMachine)
	if err != nil {
		glog.Infof("Error in updating the machine ref: %s", err)
		return machine, err
	}
	return newMachine, nil
}

func (pv *Provisioner) setTaskRef(machine *clusterv1.Machine, taskref string) error {
	oldProviderStatus, err := vsphereutils.GetMachineProviderStatus(machine)
	if err != nil {
		return err
	}

	if oldProviderStatus != nil && oldProviderStatus.TaskRef == taskref {
		// Nothing to update
		return nil
	}
	newProviderStatus := &vsphereconfigv1.VsphereMachineProviderStatus{}
	// create a copy of the old status so that any other fields except the ones we want to change can be retained
	if oldProviderStatus != nil {
		newProviderStatus = oldProviderStatus.DeepCopy()
	}
	newProviderStatus.TaskRef = taskref
	newProviderStatus.LastUpdated = time.Now().UTC().String()
	out, err := json.Marshal(newProviderStatus)
	newMachine := machine.DeepCopy()
	newMachine.Status.ProviderStatus = &runtime.RawExtension{Raw: out}
	_, err = pv.clusterV1alpha1.Machines(newMachine.Namespace).UpdateStatus(newMachine)
	if err != nil {
		glog.Infof("Error in updating the machine ref: %s", err)
		return err
	}
	return nil
}

// We are storing these as annotations and not in Machine Status because that's intended for
// "Provider-specific status" that will usually be used to detect updates. Additionally,
// Status requires yet another version API resource which is too heavy to store IP and TF state.
func (pv *Provisioner) updateAnnotations(cluster *clusterv1.Cluster, machine *clusterv1.Machine, vmIP string, vm *object.VirtualMachine) error {
	glog.Infof("Updating annotations for machine %s", machine.ObjectMeta.Name)
	nmachine := machine.DeepCopy()
	if nmachine.ObjectMeta.Annotations == nil {
		nmachine.ObjectMeta.Annotations = make(map[string]string)
	}
	glog.V(4).Infof("updateAnnotations - IP = %s", vmIP)
	nmachine.ObjectMeta.Annotations[constants.VmIpAnnotationKey] = vmIP
	nmachine.ObjectMeta.Annotations[constants.ControlPlaneVersionAnnotationKey] = nmachine.Spec.Versions.ControlPlane
	nmachine.ObjectMeta.Annotations[constants.KubeletVersionAnnotationKey] = nmachine.Spec.Versions.Kubelet

	_, err := pv.clusterV1alpha1.Machines(nmachine.Namespace).Update(nmachine)
	if err != nil {
		return err
	}
	// Update the cluster status with updated time stamp for tracking purposes
	ncluster := cluster.DeepCopy()
	status := &vsphereconfigv1.VsphereClusterProviderStatus{LastUpdated: time.Now().UTC().String()}
	out, err := json.Marshal(status)
	if err != nil {
		return err
	}
	ncluster.Status.ProviderStatus = &runtime.RawExtension{Raw: out}
	_, err = pv.clusterV1alpha1.Clusters(ncluster.Namespace).UpdateStatus(ncluster)
	if err != nil {
		glog.Infof("Error in updating the status: %s", err)
		return err
	}
	return nil
}

func (pv *Provisioner) getCloudInitUserData(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	script, err := pv.getStartupScript(cluster, machine)
	if err != nil {
		return "", err
	}
	config, err := pv.getCloudProviderConfig(cluster, machine)
	if err != nil {
		return "", err
	}
	userdata, err := vpshereprovisionercommon.GetCloudInitUserData(
		vpshereprovisionercommon.CloudInitTemplate{
			Script:              script,
			IsMaster:            util.IsMaster(machine),
			CloudProviderConfig: config,
		},
	)
	if err != nil {
		return "", err
	}
	userdata = base64.StdEncoding.EncodeToString([]byte(userdata))
	return userdata, nil
}

func (pv *Provisioner) getCloudProviderConfig(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	clusterConfig, err := vsphereutils.GetClusterProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return "", err
	}
	machineconfig, err := vsphereutils.GetMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return "", err
	}
	// TODO(ssurana): revisit once we solve https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues/60
	cloudProviderConfig, err := vpshereprovisionercommon.GetCloudProviderConfigConfig(
		vpshereprovisionercommon.CloudProviderConfigTemplate{
			Datacenter:   machineconfig.MachineVariables[constants.ProviderDatacenter],
			Server:       clusterConfig.VsphereServer,
			Insecure:     true, // TODO(ssurana): Needs to be a user input
			UserName:     clusterConfig.VsphereUser,
			Password:     clusterConfig.VspherePassword,
			ResourcePool: machineconfig.MachineVariables[constants.ProviderResPool],
			Datastore:    machineconfig.MachineVariables[constants.ProviderDatastore],
			Network:      machineconfig.MachineVariables[constants.ProviderNetwork],
		},
	)
	if err != nil {
		return "", err
	}
	cloudProviderConfig = base64.StdEncoding.EncodeToString([]byte(cloudProviderConfig))
	return cloudProviderConfig, nil
}

// Builds and returns the startup script for the passed machine and cluster.
// Returns the full path of the saved startup script and possible error.
func (pv *Provisioner) getStartupScript(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	config, err := vsphereutils.GetMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return "", pv.HandleMachineError(machine, apierrors.InvalidMachineConfiguration(
			"Cannot unmarshal providerConfig field: %v", err), constants.CreateEventAction)
	}
	preloaded := false
	if val, ok := config.MachineVariables["preloaded"]; ok {
		preloaded, err = strconv.ParseBool(val)
		if err != nil {
			return "", pv.HandleMachineError(machine, apierrors.InvalidMachineConfiguration(
				"Invalid value for preloaded: %v", err), constants.CreateEventAction)
		}
	}
	var startupScript string
	if util.IsMaster(machine) {
		if machine.Spec.Versions.ControlPlane == "" {
			return "", pv.HandleMachineError(machine, apierrors.InvalidMachineConfiguration(
				"invalid master configuration: missing Machine.Spec.Versions.ControlPlane"), constants.CreateEventAction)
		}
		var err error
		startupScript, err = vpshereprovisionercommon.GetMasterStartupScript(
			vpshereprovisionercommon.TemplateParams{
				Cluster:   cluster,
				Machine:   machine,
				Preloaded: preloaded,
			},
		)
		if err != nil {
			return "", err
		}
	} else {
		if len(cluster.Status.APIEndpoints) == 0 {
			glog.Infof("invalid cluster state: cannot create a Kubernetes node without an API endpoint")
			return "", &clustererror.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds}
		}
		kubeadmToken, err := pv.GetKubeadmToken(cluster)
		if err != nil {
			glog.Infof("Error generating kubeadm token, will requeue: %s", err.Error())
			return "", &clustererror.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds}
		}
		startupScript, err = vpshereprovisionercommon.GetNodeStartupScript(
			vpshereprovisionercommon.TemplateParams{
				Token:     kubeadmToken,
				Cluster:   cluster,
				Machine:   machine,
				Preloaded: preloaded,
			},
		)
		if err != nil {
			return "", err
		}
	}
	startupScript = base64.StdEncoding.EncodeToString([]byte(startupScript))
	return startupScript, nil
}
