package govmomi

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"

	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	vpshereprovisionercommon "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/provisioner/common"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/certificates"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeadm"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/userdata"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
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

func (pv *Provisioner) Create(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine,
	bootstrapToken string) error {

	if cluster == nil {
		return errors.New(constants.ClusterIsNullErr)
	}

	machineRole := vsphereutils.GetMachineRole(machine)
	if machineRole == "" {
		return errors.Errorf(
			"unable to get machine role while creating machine with GoVmomi "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	klog.V(4).Infof("creating machine with GoVmomi %s=%s %s=%s %s=%s %s=%s %s=%s",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Namespace,
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name,
		"machine-role", machineRole)

	s, err := pv.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		return errors.Wrapf(
			err,
			"unable to get session while creating machine with GoVmomi "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	createctx, cancel := context.WithCancel(*s.context)
	defer cancel()

	userSession, err := s.session.SessionManager.UserSession(createctx)
	if err != nil {
		return errors.Wrapf(
			err,
			"unable to get user session while creating machine with GoVmomi "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	klog.V(4).Infof("got user session while creating machine with GoVmomi "+
		"%s=%s %s=%s %s=%s %s=%s %s=%s %s=%v",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Namespace,
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name,
		"machine-role", machineRole,
		"user-session", userSession)

	task := vsphereutils.GetActiveTasks(machine)
	if task != "" {
		// In case an active task is going on, wait for its completion
		return pv.verifyAndUpdateTask(s, machine, task)
	}
	// Before going for cloning, check if we can locate a VM with the InstanceUUID
	// as this Machine. If found, that VM is the right match for this machine
	vmRef, err := pv.findVMByInstanceUUID(ctx, s, machine)
	if err != nil {
		return errors.Wrapf(
			err,
			"unable to get find VM by instance UUID while creating machine with GoVmomi "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}
	if vmRef != "" {
		pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "created", "created Machine %s(%s)", machine.Name, vmRef)
		// Update the Machine object with the VM Reference annotation
		if _, err := pv.updateVMReference(machine, vmRef); err != nil {
			return errors.Wrapf(
				err,
				"unable to update VM reference while creating machine with GoVmomi "+
					"%s=%s %s=%s %s=%s %s=%s",
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Namespace,
				"machine-namespace", machine.Namespace,
				"machine-name", machine.Name)
		}
		return nil
	}

	clusterConfig, err := vsphereconfigv1.ClusterConfigFromProviderSpec(&cluster.Spec.ProviderSpec)
	if err != nil {
		return errors.Wrapf(
			err,
			"unable to get cluster provider config while creating machine with GoVmomi "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	machineConfig, err := vsphereconfigv1.MachineConfigFromProviderSpec(&machine.Spec.ProviderSpec)
	if err != nil {
		return errors.Wrapf(
			err,
			"unable to get machine provider config while creating machine with GoVmomi "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	caCertHash, err := certificates.GenerateCertificateHash(clusterConfig.CAKeyPair.Cert)
	if err != nil {
		return err
	}

	var controlPlaneEndpoint string
	if bootstrapToken != "" {
		var err error
		if controlPlaneEndpoint, err = vsphereutils.GetControlPlaneEndpoint(cluster, nil); err != nil {
			return errors.Wrapf(
				err,
				"unable to get control plane endpoint while creating machine with GoVmomi "+
					"%s=%s %s=%s %s=%s %s=%s",
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Namespace,
				"machine-namespace", machine.Namespace,
				"machine-name", machine.Name)
		}
	}

	var userDataYAML string

	// apply values based on the role of the machine
	switch machineRole {
	case "controlplane":

		if bootstrapToken != "" {
			klog.V(2).Infof("allowing a machine to join the control plane "+
				"%s=%s %s=%s %s=%s %s=%s",
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Namespace,
				"machine-namespace", machine.Namespace,
				"machine-name", machine.Name)

			bindPort := vsphereutils.GetAPIServerBindPort(machineConfig)

			kubeadm.SetJoinConfigurationOptions(
				&machineConfig.KubeadmConfiguration.Join,
				kubeadm.WithBootstrapTokenDiscovery(
					kubeadm.NewBootstrapTokenDiscovery(
						kubeadm.WithAPIServerEndpoint(controlPlaneEndpoint),
						kubeadm.WithToken(bootstrapToken),
						kubeadm.WithCACertificateHash(caCertHash),
					),
				),
				kubeadm.WithJoinNodeRegistrationOptions(
					kubeadm.NewNodeRegistration(
						kubeadm.WithTaints(machine.Spec.Taints),
						kubeadm.WithNodeRegistrationName(hostnameLookup),
						kubeadm.WithCRISocket(containerdSocket),
						//kubeadm.WithKubeletExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
					),
				),
				kubeadm.WithLocalAPIEndpointAndPort(localIPV4Lookup, int(bindPort)),
			)
			joinConfigurationYAML, err := kubeadm.ConfigurationToYAML(&machineConfig.KubeadmConfiguration.Join)
			if err != nil {
				return err
			}

			userData, err := userdata.JoinControlPlane(&userdata.ContolPlaneJoinInput{
				CACert:            string(clusterConfig.CAKeyPair.Cert),
				CAKey:             string(clusterConfig.CAKeyPair.Key),
				EtcdCACert:        string(clusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:         string(clusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert:  string(clusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:   string(clusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:            string(clusterConfig.SAKeyPair.Cert),
				SaKey:             string(clusterConfig.SAKeyPair.Key),
				JoinConfiguration: joinConfigurationYAML,
			})
			if err != nil {
				return err
			}

			userDataYAML = userData
		} else {
			klog.V(2).Infof("initializing a new cluster with %s=%s %s=%s %s=%s %s=%s",
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Namespace,
				"machine-namespace", machine.Namespace,
				"machine-name", machine.Name)

			bindPort := machineConfig.KubeadmConfiguration.Init.LocalAPIEndpoint.BindPort
			certSans := []string{"localIPV4Lookup"}
			if v := clusterConfig.ClusterConfiguration.ControlPlaneEndpoint; v != "" {
				host, _, err := net.SplitHostPort(v)
				if err != nil {
					return err
				}
				certSans = append(certSans, host)
			}

			klog.V(2).Info("machine is the first control plane machine for the cluster")
			if !clusterConfig.CAKeyPair.HasCertAndKey() {
				return errors.New("failed to run controlplane, missing CAPrivateKey")
			}

			kubeadm.SetClusterConfigurationOptions(
				&clusterConfig.ClusterConfiguration,
				kubeadm.WithControlPlaneEndpoint(fmt.Sprintf("%s:%d", localIPV4Lookup, bindPort)),
				kubeadm.WithAPIServerCertificateSANs(certSans...),
				//kubeadm.WithAPIServerExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
				//kubeadm.WithControllerManagerExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
				kubeadm.WithClusterName(cluster.Name),
				kubeadm.WithClusterNetworkFromClusterNetworkingConfig(cluster.Spec.ClusterNetwork),
				kubeadm.WithKubernetesVersion(machine.Spec.Versions.ControlPlane),
			)
			clusterConfigYAML, err := kubeadm.ConfigurationToYAML(&clusterConfig.ClusterConfiguration)
			if err != nil {
				return err
			}

			kubeadm.SetInitConfigurationOptions(
				&machineConfig.KubeadmConfiguration.Init,
				kubeadm.WithNodeRegistrationOptions(
					kubeadm.NewNodeRegistration(
						kubeadm.WithTaints(machine.Spec.Taints),
						kubeadm.WithNodeRegistrationName(hostnameLookup),
						kubeadm.WithCRISocket(containerdSocket),
						//kubeadm.WithKubeletExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
					),
				),
			)
			initConfigYAML, err := kubeadm.ConfigurationToYAML(&machineConfig.KubeadmConfiguration.Init)
			if err != nil {
				return err
			}

			userData, err := userdata.NewControlPlane(&userdata.ControlPlaneInput{
				CACert:               string(clusterConfig.CAKeyPair.Cert),
				CAKey:                string(clusterConfig.CAKeyPair.Key),
				EtcdCACert:           string(clusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:            string(clusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert:     string(clusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:      string(clusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:               string(clusterConfig.SAKeyPair.Cert),
				SaKey:                string(clusterConfig.SAKeyPair.Key),
				ClusterConfiguration: clusterConfigYAML,
				InitConfiguration:    initConfigYAML,
			})
			if err != nil {
				return err
			}

			userDataYAML = userData
		}
	case "node":
		klog.V(2).Infof("joining a worker node to the cluster "+
			"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)

		kubeadm.SetJoinConfigurationOptions(
			&machineConfig.KubeadmConfiguration.Join,
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
					kubeadm.WithTaints(machine.Spec.Taints),
					kubeadm.WithKubeletExtraArgs(map[string]string{"node-labels": nodeRole}),
				),
			),
		)
		joinConfigurationYAML, err := kubeadm.ConfigurationToYAML(&machineConfig.KubeadmConfiguration.Join)
		if err != nil {
			return err
		}

		userData, err := userdata.NewNode(&userdata.NodeInput{
			JoinConfiguration: joinConfigurationYAML,
		})
		if err != nil {
			return nil
		}

		userDataYAML = userData

	default:
		return errors.Errorf("unknown node role %q", machineRole)
	}

	userData64 := base64.StdEncoding.EncodeToString([]byte(userDataYAML))

	// Use the appropriate path if we're connected to a vCenter
	if s.session.IsVC() {
		return pv.cloneVirtualMachineOnVCenter(s, cluster, machine, userData64)
	}

	// fallback in case we're connected to a standalone ESX host
	return pv.cloneVirtualMachineOnESX(s, cluster, machine, userData64)
}

func (pv *Provisioner) findVMByInstanceUUID(ctx context.Context, s *SessionContext, machine *clusterv1.Machine) (string, error) {
	klog.V(4).Infof("trying to check existence of the VM via InstanceUUID %s", machine.UID)
	si := object.NewSearchIndex(s.session.Client)
	instanceUUID := true
	vmRef, err := si.FindByUuid(ctx, nil, string(machine.UID), true, &instanceUUID)
	if err != nil {
		return "", fmt.Errorf("error quering virtual machine or template using FindByUuid: %s", err)
	}
	if vmRef != nil {
		return vmRef.Reference().Value, nil
	}
	return "", nil
}

func (pv *Provisioner) verifyAndUpdateTask(s *SessionContext, machine *clusterv1.Machine, taskmoref string) error {
	klog.V(4).Infof("[DEBUG] Verifying and updating Tasks")
	ctx, cancel := context.WithCancel(*s.context)
	defer cancel()
	// If a task does exist on the
	var taskmo mo.Task
	taskref := types.ManagedObjectReference{
		Type:  "task",
		Value: taskmoref,
	}
	klog.V(4).Infof("[DEBUG] Retrieving the Task")
	err := s.session.RetrieveOne(ctx, taskref, []string{"info"}, &taskmo)
	if err != nil {
		klog.V(4).Infof("[DEBUG] task does not exist for moref %s", taskmoref)
		// The task does not exist any more, thus no point tracking it. Thus clear it from the machine
		return pv.setTaskRef(machine, "")
	}
	klog.V(4).Infof("[DEBUG] task state = %s", taskmo.Info.State)
	switch taskmo.Info.State {
	// Queued or Running
	case types.TaskInfoStateQueued, types.TaskInfoStateRunning:
		// Requeue the machine update to check back in 5 seconds on the task
		return &clustererror.RequeueAfterError{RequeueAfter: time.Second * 5}
	// Successful
	case types.TaskInfoStateSuccess:
		klog.V(4).Infof("[DEBUG] Task is a Success")
		if taskmo.Info.DescriptionId == "folder.createVm" {
			klog.V(4).Infof("[DEBUG] Task is a CreateVM")
			vmref := taskmo.Info.Result.(types.ManagedObjectReference)
			vm := object.NewVirtualMachine(s.session.Client, vmref)
			klog.V(4).Infof("[DEBUG] Powering On the VM")
			task, err := vm.PowerOn(ctx)
			if err != nil {
				return err
			}
			klog.V(4).Infof("[DEBUG] Waiting for PowerOn to happen")
			_, err = task.WaitForResult(ctx, nil)
			if err != nil {
				return err
			}

			klog.V(4).Infof("[DEBUG] Recording the event")
			pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "created", "created Machine %s(%s)", machine.Name, vmref.Value)
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
		} else if taskmo.Info.DescriptionId == "virtualMachine.clone" {
			vmref := taskmo.Info.Result.(types.ManagedObjectReference)
			pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "created", "created Machine %s(%s)", machine.Name, vmref.Value)
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
		} else if taskmo.Info.DescriptionId == "virtualMachine.reconfigure" {
			pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "reconfigured", "reconfigured Machine %s", taskmo.Info.EntityName)
		}
		return pv.setTaskRef(machine, "")
	case types.TaskInfoStateError:
		klog.Infof("[DEBUG] task error condition, description = %s", taskmo.Info.DescriptionId)
		// If the machine was created via the ESXi "cloning", the description id will likely be "folder.createVm"
		if taskmo.Info.DescriptionId == "virtualMachine.clone" || taskmo.Info.DescriptionId == "folder.createVm" {
			pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "failed", "creation failed for Machine %v", machine.Name)
			// Clear the reference to the failed task so that the next reconcile loop can re-create it
			return pv.setTaskRef(machine, "")
		}
	default:
		klog.Warningf("unknown state %s for task %s detected", taskmoref, taskmo.Info.State)
		return fmt.Errorf("unknown state %s for task %s detected", taskmoref, taskmo.Info.State)
	}
	return nil
}

// CloneVirtualMachine clones the template to a virtual machine.
func (pv *Provisioner) cloneVirtualMachineOnVCenter(s *SessionContext, cluster *clusterv1.Cluster, machine *clusterv1.Machine, userData string) error {
	klog.V(4).Infof("starting the clone process on vCenter")
	ctx, cancel := context.WithCancel(*s.context)
	defer cancel()

	machineConfig, err := vsphereconfigv1.MachineConfigFromProviderSpec(&machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	dc, err := s.finder.DatacenterOrDefault(ctx, machineConfig.MachineSpec.Datacenter)
	if err != nil {
		return err
	}
	s.finder.SetDatacenter(dc)

	// Let's check to make sure we can find the template earlier on... Plus, we need
	// the cluster/host info if we want to deploy direct to the cluster/host.
	var src *object.VirtualMachine
	if vsphereutils.IsValidUUID(machineConfig.MachineSpec.VMTemplate) {
		// If the passed VMTemplate is a valid UUID, then first try to find it treating that as InstanceUUID
		// In case if are not able to locate a matching VM then fall back to searching using the VMTemplate
		// as a name
		klog.V(4).Infof("trying to resolve the VMTemplate as InstanceUUID %s", machineConfig.MachineSpec.VMTemplate)
		si := object.NewSearchIndex(s.session.Client)
		instanceUUID := true
		templateref, err := si.FindByUuid(ctx, dc, machineConfig.MachineSpec.VMTemplate, true, &instanceUUID)
		if err != nil {
			return fmt.Errorf("error querying virtual machine or template using FindByUuid: %s", err)
		}
		if templateref != nil {
			src = object.NewVirtualMachine(s.session.Client, templateref.Reference())
		}
	}
	if src == nil {
		klog.V(4).Infof("trying to resolve the VMTemplate as Name %s", machineConfig.MachineSpec.VMTemplate)
		src, err = s.finder.VirtualMachine(ctx, machineConfig.MachineSpec.VMTemplate)
		if err != nil {
			klog.Errorf("virtualMachine finder failed. err=%s", err)
			return err
		}
	}

	host, err := src.HostSystem(ctx)
	if err != nil {
		klog.Errorf("hostSystem failed. err=%s", err)
		return err
	}
	hostProps, err := PropertiesHost(host)
	if err != nil {
		return fmt.Errorf("error fetching host properties: %s", err)
	}

	// Since it's assumed that the ResourcePool name has been provided in the config, if we
	// want to deploy directly to the cluster/host, then we need to override the ResourcePool
	// path before generating the Cloud Provider config. This is done below in:
	// getCloudInitUserData()
	// +--- getCloudProviderConfig()
	resourcePoolPath := ""
	if len(machineConfig.MachineSpec.ResourcePool) == 0 {

		resourcePoolPath = fmt.Sprintf("/%s/host/%s/Resource", machineConfig.MachineSpec.Datacenter, hostProps.Name)
		klog.Infof("attempting to deploy directly to cluster/host RP: %s", resourcePoolPath)
	}

	metaData, err := pv.getCloudInitMetaData(cluster, machine)
	if err != nil {
		// err returned by the getCloudInitMetaData would be of type RequeueAfterError in case kubeadm is not ready yet
		return err
	}

	var spec types.VirtualMachineCloneSpec
	klog.V(4).Infof("[cloneVirtualMachine]: Preparing clone spec for VM %s", machine.Name)
	klog.V(4).Infof("clone VM to folder %s", machineConfig.MachineSpec.VMFolder)
	vmFolder, err := s.finder.FolderOrDefault(ctx, machineConfig.MachineSpec.VMFolder)
	if err != nil {
		return err
	}

	ds, err := s.finder.DatastoreOrDefault(ctx, machineConfig.MachineSpec.Datastore)
	if err != nil {
		return err
	}
	spec.Location.Datastore = types.NewReference(ds.Reference())

	spec.Config = &types.VirtualMachineConfigSpec{}
	// Use the object UID as the instanceUUID for the VM
	spec.Config.InstanceUuid = string(machine.UID)
	klog.V(4).Infof("assigned VM instanceUUID=%q", machine.UID)
	diskUUIDEnabled := true
	spec.Config.Flags = &types.VirtualMachineFlagInfo{
		DiskUuidEnabled: &diskUUIDEnabled,
	}
	if machineConfig.MachineSpec.NumCPUs > 0 {
		spec.Config.NumCPUs = int32(machineConfig.MachineSpec.NumCPUs)
	}
	if machineConfig.MachineSpec.MemoryMB > 0 {
		spec.Config.MemoryMB = machineConfig.MachineSpec.MemoryMB
	}
	spec.Config.Annotation = fmt.Sprintf("virtual Machine is part of the cluster %s managed by cluster-api", cluster.Name)
	spec.Location.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndConsolidate)

	vmProps, err := PropertiesVM(src)
	if err != nil {
		return fmt.Errorf("error fetching vm/template properties: %s", err)
	}

	if len(machineConfig.MachineSpec.ResourcePool) > 0 {
		pool, err := s.finder.ResourcePoolOrDefault(ctx, machineConfig.MachineSpec.ResourcePool)

		if _, ok := err.(*find.NotFoundError); ok {
			klog.Warningf("failed to find ResourcePool=%s err=%s. Attempting to create it.", machineConfig.MachineSpec.ResourcePool, err)

			poolRoot, errRoot := host.ResourcePool(ctx)
			if errRoot != nil {
				klog.Errorf("failed to find root ResourcePool. err=%s", errRoot)
				return errRoot
			}

			klog.Info("creating ResourcePool using default values. These values can be modified after ResourcePool creation.")
			pool, err = poolRoot.Create(ctx, machineConfig.MachineSpec.ResourcePool, types.DefaultResourceConfigSpec())
			if err != nil {
				klog.Errorf("create ResourcePool failed. err=%s", err)
				return err
			}
		}

		spec.Location.Pool = types.NewReference(pool.Reference())
	} else {
		klog.Infof("attempting to use Host ResourcePool")
		pool, err := host.ResourcePool(ctx)

		if err != nil {
			klog.Errorf("host ResourcePool failed. err=%s", err)
			return err
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
	}(machineConfig.MachineSpec.Disks)
	diskChange := false
	for _, dev := range disks {
		disk := dev.(*types.VirtualDisk)
		if newSize, ok := diskMap[disk.DeviceInfo.GetDescription().Label]; ok {
			if disk.CapacityInBytes > vsphereutils.GiBToByte(newSize) {
				return errors.New("[FATAL] Disk size provided should be more than actual disk size of the template. Please correct the machineSpec to proceed")
			}
			klog.V(4).Infof("[cloneVirtualMachine] Resizing the disk \"%s\" to new size \"%d\"", disk.DeviceInfo.GetDescription().Label, newSize)
			diskChange = true
			disk.CapacityInBytes = vsphereutils.GiBToByte(newSize)
			diskspec := &types.VirtualDeviceConfigSpec{}
			diskspec.Operation = types.VirtualDeviceConfigSpecOperationEdit
			diskspec.Device = disk
			deviceSpecs = append(deviceSpecs, diskspec)
		}
	}
	if !diskChange && len(machineConfig.MachineSpec.Disks) > 0 {
		klog.V(4).Info("[cloneVirtualMachine] No disks were resized while cloning from template")
		return fmt.Errorf("[FATAL] None of the disks specified in the MachineSpec matched with the disks on the template %s", machineConfig.MachineSpec.VMTemplate)
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
	for _, network := range machineConfig.MachineSpec.Networks {
		netRef, err := s.finder.Network(ctx, network.NetworkName)
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
	if pv.eventRecorder != nil { // TODO: currently supporting nil for testing
		pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "creating", "creating Machine %v", machine.Name)
	}
	task, err := src.Clone(ctx, vmFolder, machine.Name, spec)
	klog.V(6).Infof("clone VM with spec %v", spec)
	if err != nil {
		return err
	}
	return pv.setTaskRef(machine, task.Reference().Value)
}

// cloneVirtualMachineOnESX clones the template to a virtual machine.
func (pv *Provisioner) cloneVirtualMachineOnESX(s *SessionContext, cluster *clusterv1.Cluster, machine *clusterv1.Machine, userData string) error {
	klog.V(4).Infof("starting the clone process on standalone ESX")
	ctx, cancel := context.WithCancel(*s.context)
	defer cancel()

	machineConfig, err := vsphereconfigv1.MachineConfigFromProviderSpec(&machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}
	klog.V(4).Infof("[cloneVirtualMachineOnESX]: Preparing clone spec for VM %s", machine.Name)

	dc, err := s.finder.DefaultDatacenter(ctx)
	//dc, err := s.finder.DatacenterOrDefault(ctx, machineConfig.MachineSpec.Datacenter)
	if err != nil {
		return err
	}
	s.finder.SetDatacenter(dc)

	folders, err := dc.Folders(ctx)
	if err != nil {
		return err
	}

	pool, err := s.finder.ResourcePoolOrDefault(ctx, machineConfig.MachineSpec.ResourcePool)
	if err != nil {
		return err
	}

	// Fetch info from templateVM
	src, err := s.finder.VirtualMachine(ctx, machineConfig.MachineSpec.VMTemplate)
	if err != nil {
		return err
	}
	vmProps, err := PropertiesVM(src)
	if err != nil {
		return fmt.Errorf("error fetching virtual machine or template properties: %s", err)
	}

	spec := &types.VirtualMachineConfigSpec{}
	var devices object.VirtualDeviceList

	if err := pv.addMachineBase(s, cluster, machine, machineConfig, spec, vmProps, userData); err != nil {
		return err
	}

	if devices, err = pv.copyDisks(ctx, s, machine, machineConfig, devices, vmProps); err != nil {
		return err
	}

	if devices, err = pv.addNetworking(ctx, s, machineConfig, devices); err != nil {
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

	pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "creating", "creating Machine %v", machine.Name)

	task, err := folders.VmFolder.CreateVM(ctx, *spec, pool, ch)
	if err != nil {
		klog.Infof("[DEBUG] VmFolder.CreateVM() FAILED: %s", err.Error())
		return err
	}

	return pv.setTaskRef(machine, task.Reference().Value)

}

func (pv *Provisioner) addMachineBase(s *SessionContext, cluster *clusterv1.Cluster, machine *clusterv1.Machine, machineConfig *vsphereconfigv1.VsphereMachineProviderConfig, spec *types.VirtualMachineConfigSpec, vmProps *mo.VirtualMachine, userData string) error {
	metaData, err := pv.getCloudInitMetaData(cluster, machine)
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

	if machineConfig.MachineSpec.NumCPUs > 0 {
		spec.NumCPUs = int32(machineConfig.MachineSpec.NumCPUs)
	}
	if machineConfig.MachineSpec.MemoryMB > 0 {
		spec.MemoryMB = machineConfig.MachineSpec.MemoryMB
	}

	//spec.PowerOpInfo.
	spec.Annotation = fmt.Sprintf("virtual Machine is part of the cluster %s managed by cluster-api", cluster.Name)

	// OVF Environment
	// build up Environment in order to marshal to xml

	var opts []types.BaseOptionValue
	opts = append(opts, &types.OptionValue{Key: "guestinfo.metadata", Value: metaData})
	opts = append(opts, &types.OptionValue{Key: "guestinfo.metadata.encoding", Value: "base64"})
	opts = append(opts, &types.OptionValue{Key: "guestinfo.userdata", Value: userData})
	opts = append(opts, &types.OptionValue{Key: "guestinfo.userdata.encoding", Value: "base64"})
	spec.ExtraConfig = opts

	spec.Files = &types.VirtualMachineFileInfo{
		VmPathName: fmt.Sprintf("[%s]", machineConfig.MachineSpec.Datastore),
	}

	return nil
}

func (pv *Provisioner) copyDisks(ctx context.Context, s *SessionContext, machine *clusterv1.Machine, machineConfig *vsphereconfigv1.VsphereMachineProviderConfig, devices object.VirtualDeviceList, vmProps *mo.VirtualMachine) (object.VirtualDeviceList, error) {
	ds, err := s.finder.DatastoreOrDefault(ctx, machineConfig.MachineSpec.Datastore)
	if err != nil {
		return devices, err
	}
	dc, err := s.finder.DefaultDatacenter(ctx)
	if err != nil {
		return devices, err
	}
	s.finder.SetDatacenter(dc)

	// Create datastore directory for our VM
	dstf := fmt.Sprintf("[%s] %s", machineConfig.MachineSpec.Datastore, machine.Name)
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

	var dstdisk string

	// Iterate through the machine spec and then iterate over the VM's disks
	for _, diskSpec := range machineConfig.MachineSpec.Disks {
		for _, dev := range disks {
			srcdisk := dev.(*types.VirtualDisk)

			if srcdisk.DeviceInfo.GetDescription().Label == diskSpec.DiskLabel {
				newSize := diskSpec.DiskSizeGB
				if srcdisk.CapacityInBytes > vsphereutils.GiBToByte(newSize) {
					return devices, errors.New("disk size provided should be more than actual disk size of the template")
				}
				srcdisk.CapacityInBytes = vsphereutils.GiBToByte(newSize)
			}

			dstdisk = fmt.Sprintf("%s/%s-%s.vmdk", dstf, machine.Name, diskSpec.DiskLabel)
			srcfile := srcdisk.Backing.(types.BaseVirtualDeviceFileBackingInfo)

			// copy happens here
			klog.V(4).Infof("[DEBUG] Cloning template disk to %s", dstdisk)
			cp := m.Copy
			if err := cp(ctx, srcfile.GetVirtualDeviceFileBackingInfo().FileName, dstdisk); err != nil {
				klog.V(4).Infof("[DEBUG] Copying vmdk, src(%s), dst(%s)", srcfile.GetVirtualDeviceFileBackingInfo().FileName, dstdisk)
				return devices, err
			}

			// attach disk to VM
			disk := devices.CreateDisk(controller, ds.Reference(), ds.Path(fmt.Sprintf("%s/%s-%s.vmdk", machine.Name, machine.Name, diskSpec.DiskLabel)))
			devices = append(devices, disk)
		}
	}

	return devices, nil
}

func (pv *Provisioner) addNetworking(ctx context.Context, s *SessionContext, machineConfig *vsphereconfigv1.VsphereMachineProviderConfig, devices object.VirtualDeviceList) (object.VirtualDeviceList, error) {
	klog.V(4).Infof("[DEBUG] Adding NICs")
	for _, network := range machineConfig.MachineSpec.Networks {
		netRef, err := s.finder.Network(ctx, network.NetworkName)
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
	klog.V(4).Infof("[DEBUG] Adding SIO controller")
	l := object.VirtualDeviceList(vmProps.Config.Hardware.Device)
	controllers := l.SelectByType((*types.VirtualSIOController)(nil))

	if len(controllers) == 0 {
		// Add a serial port
		klog.V(4).Infof("[DEBUG] Adding SIO controller")
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
		klog.V(4).Infof("[DEBUG] Adding serial port")
		portSpec, err := devices.CreateSerialPort()
		if err != nil {
			klog.V(4).Infof("[DEBUG] Failed to add serial port: %s", err.Error())
			return devices, err
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

// Properties is a convenience method that wraps fetching the
// VirtualMachine MO from its higher-level object.
func PropertiesVM(vm *object.VirtualMachine) (*mo.VirtualMachine, error) {
	klog.V(4).Infof("[DEBUG] Fetching properties for VM %q", vm.InventoryPath)
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultAPITimeout)
	defer cancel()
	var props mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

// PropertiesHost is a convenience method that wraps fetching the
// HostSystem MO from its higher-level object.
func PropertiesHost(host *object.HostSystem) (*mo.HostSystem, error) {
	klog.V(4).Infof("[DEBUG] Fetching properties for host %q", host.InventoryPath)
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultAPITimeout)
	defer cancel()
	var props mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

func (vc *Provisioner) updateVMReference(machine *clusterv1.Machine, vmref string) (*clusterv1.Machine, error) {
	machineConfig, err := vsphereconfigv1.MachineConfigFromProviderSpec(&machine.Spec.ProviderSpec)
	if err != nil {
		klog.Infof("error fetching MachineProviderConfig: %s", err)
		return machine, err
	}
	machineConfig.MachineRef = vmref
	return machine, nil
}

func (pv *Provisioner) setTaskRef(machine *clusterv1.Machine, taskRef string) error {
	machineStatus, err := vsphereconfigv1.MachineStatusFromProviderStatus(&machine.Status)
	if err != nil {
		return err
	}
	machineStatus.TaskRef = taskRef
	return nil
}

func (pv *Provisioner) getCloudInitMetaData(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	machineconfig, err := vsphereconfigv1.MachineConfigFromProviderSpec(&machine.Spec.ProviderSpec)
	if err != nil {
		return "", err
	}
	metadata, err := vpshereprovisionercommon.GetCloudInitMetaData(machine.Name, machineconfig)
	if err != nil {
		return "", err
	}
	metadata = base64.StdEncoding.EncodeToString([]byte(metadata))
	return metadata, nil
}

func (pv *Provisioner) getCloudProviderConfig(cluster *clusterv1.Cluster, machine *clusterv1.Machine,
	resourcePoolPath string, deployOnVC bool) (string, error) {
	clusterConfig, err := vsphereconfigv1.ClusterConfigFromProviderSpec(&cluster.Spec.ProviderSpec)
	if err != nil {
		return "", err
	}
	machineconfig, err := vsphereconfigv1.MachineConfigFromProviderSpec(&machine.Spec.ProviderSpec)
	if err != nil {
		return "", err
	}

	// cloud provider requires bare IP:port, so if it is parseable as a url with a scheme, then
	// strip the scheme and path.  Otherwise continue.  TODO replace with better input validation.
	var server string
	serverURL, err := url.Parse(clusterConfig.VsphereServer)
	if err == nil && serverURL.Host != "" {
		server = serverURL.Host
		klog.Infof("extracted vSphere server url: %s", server)
	} else {
		server = clusterConfig.VsphereServer
		klog.Infof("using input vSphere server url: %s", server)
	}

	username, password, err := pv.GetVsphereCredentials(cluster)
	if err != nil {
		return "", err
	}

	// TODO(ssurana): revisit once we solve https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues/60
	cpc := vpshereprovisionercommon.CloudProviderConfigTemplate{
		Datacenter:   machineconfig.MachineSpec.Datacenter,
		Server:       server,
		Insecure:     true, // TODO(ssurana): Needs to be a user input
		UserName:     username,
		Password:     password,
		ResourcePool: machineconfig.MachineSpec.ResourcePool,
		Datastore:    machineconfig.MachineSpec.Datastore,
		Network:      "",
	}
	if len(resourcePoolPath) > 0 {
		cpc.ResourcePool = resourcePoolPath
	}
	if len(machineconfig.MachineSpec.Networks) > 0 {
		cpc.Network = machineconfig.MachineSpec.Networks[0].NetworkName
	}

	cloudProviderConfig, err := vpshereprovisionercommon.GetCloudProviderConfigConfig(cpc, deployOnVC)
	if err != nil {
		return "", err
	}
	cloudProviderConfig = base64.StdEncoding.EncodeToString([]byte(cloudProviderConfig))
	return cloudProviderConfig, nil
}
