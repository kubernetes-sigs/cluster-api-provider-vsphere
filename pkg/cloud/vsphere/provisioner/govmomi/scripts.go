package govmomi

import (
	"encoding/base64"
	"fmt"

	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/klog"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	vpshereprovisionercommon "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/provisioner/common"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"
	apierrors "sigs.k8s.io/cluster-api/pkg/errors"
	"sigs.k8s.io/cluster-api/pkg/util"
)

func (pv *Provisioner) getCloudInitMetaData(cluster *clusterv1.Cluster, _machine *clusterv1.Machine) (string, error) {
	machine, err := pv.NewGovmomiMachine(cluster, _machine)
	if err != nil {
		return "", err
	}
	metadata, err := vpshereprovisionercommon.GetCloudInitMetaData(machine.Name, machine.config)
	if err != nil {
		return "", err
	}
	metadata = base64.StdEncoding.EncodeToString([]byte(metadata))
	return metadata, nil
}

func (pv *Provisioner) getCloudInitUserData(cluster *clusterv1.Cluster, _machine *clusterv1.Machine,
	resourcePoolPath string, deployOnVC bool) (string, error) {
	machine, err := pv.NewGovmomiMachine(cluster, _machine)
	script, err := pv.getStartupScript(cluster, _machine, deployOnVC)
	if err != nil {
		return "", err
	}
	config, err := pv.getCloudProviderConfig(cluster, _machine, resourcePoolPath, deployOnVC)
	if err != nil {
		return "", err
	}
	publicKey, err := pv.GetSSHPublicKey(cluster)
	if err != nil {
		return "", err
	}
	if err != nil {
		return "", err
	}
	userdata, err := vpshereprovisionercommon.GetCloudInitUserData(
		vpshereprovisionercommon.CloudInitTemplate{
			Script:              script,
			IsMaster:            util.IsControlPlaneMachine(machine.machine),
			CloudProviderConfig: config,
			SSHPublicKey:        publicKey,
			TrustedCerts:        machine.config.MachineSpec.TrustedCerts,
			NTPServers:          machine.config.MachineSpec.NTPServers,
		},
		deployOnVC,
	)
	if err != nil {
		return "", err
	}
	userdata = base64.StdEncoding.EncodeToString([]byte(userdata))
	return userdata, nil
}

func (pv *Provisioner) getCloudProviderConfig(cluster *clusterv1.Cluster, _machine *clusterv1.Machine,
	resourcePoolPath string, deployOnVC bool) (string, error) {
	machine, err := pv.NewGovmomiMachine(cluster, _machine)
	username, password, err := machine.GetCluster().GetCredentials()
	if err != nil {
		return "", err
	}

	// TODO(ssurana): revisit once we solve https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues/60
	cpc := vpshereprovisionercommon.CloudProviderConfigTemplate{
		Datacenter:   machine.config.MachineSpec.Datacenter,
		Server:       machine.GetCluster().GetHost(),
		Insecure:     true, // TODO(ssurana): Needs to be a user input
		UserName:     username,
		Password:     password,
		ResourcePool: machine.config.MachineSpec.ResourcePool,
		Datastore:    machine.config.MachineSpec.Datastore,
		Network:      "",
	}
	if len(resourcePoolPath) > 0 {
		cpc.ResourcePool = resourcePoolPath
	}
	if len(machine.config.MachineSpec.Networks) > 0 {
		cpc.Network = machine.config.MachineSpec.Networks[0].NetworkName
	}

	cloudProviderConfig, err := vpshereprovisionercommon.GetCloudProviderConfigConfig(cpc, deployOnVC)
	if err != nil {
		return "", err
	}
	cloudProviderConfig = base64.StdEncoding.EncodeToString([]byte(cloudProviderConfig))
	return cloudProviderConfig, nil
}

// Builds and returns the startup script for the passed machine and cluster.
// Returns the full path of the saved startup script and possible error.
func (pv *Provisioner) getStartupScript(cluster *clusterv1.Cluster, _machine *clusterv1.Machine, deployOnVC bool) (string, error) {
	machine, err := pv.NewGovmomiMachine(cluster, _machine)
	if err != nil {
		return "", pv.HandleMachineError(machine.machine, apierrors.InvalidMachineConfiguration(
			"Cannot unmarshal providerSpec field: %v", err), constants.CreateEventAction)
	}
	preloaded := machine.config.MachineSpec.Preloaded
	var startupScript string
	if util.IsControlPlaneMachine(machine.machine) {
		if machine.machine.Spec.Versions.ControlPlane == "" {
			return "", pv.HandleMachineError(machine.machine, apierrors.InvalidMachineConfiguration(
				"invalid master configuration: missing Machine.Spec.Versions.ControlPlane"), constants.CreateEventAction)
		}
		parsedVersion, err := version.ParseSemantic(machine.machine.Spec.Versions.ControlPlane)
		if err != nil {
			return "", err
		}

		startupScript, err = vpshereprovisionercommon.GetMasterStartupScript(
			vpshereprovisionercommon.TemplateParams{
				MajorMinorVersion: fmt.Sprintf("%d.%d", parsedVersion.Major(), parsedVersion.Minor()),
				Cluster:           cluster,
				Machine:           machine.machine,
				Preloaded:         preloaded,
			},
			deployOnVC,
		)
		if err != nil {
			return "", err
		}
	} else {
		clusterstatus, err := vsphereutils.GetClusterProviderStatus(cluster)
		if err != nil {
			klog.Infof("Error fetching cluster ProviderStatus field: %s", err)
			return "", err
		}
		if clusterstatus == nil || clusterstatus.APIStatus != vsphereconfigv1.ApiReady {
			duration := vsphereutils.GetNextBackOff()
			klog.Infof("[pending] Waiting for Kubernetes API Status to be \"Ready\". Retrying in %s", duration)
			return "", &clustererror.RequeueAfterError{RequeueAfter: duration}
		}
		kubeadmToken, err := pv.GetKubeadmToken(cluster)
		if err != nil {
			duration := vsphereutils.GetNextBackOff()
			klog.Infof("Error generating kubeadm token, will retry in %s error: %s", duration, err.Error())
			return "", &clustererror.RequeueAfterError{RequeueAfter: duration}
		}
		parsedVersion, err := version.ParseSemantic(machine.machine.Spec.Versions.Kubelet)
		if err != nil {
			return "", err
		}
		startupScript, err = vpshereprovisionercommon.GetNodeStartupScript(
			vpshereprovisionercommon.TemplateParams{
				Token:             kubeadmToken,
				MajorMinorVersion: fmt.Sprintf("%d.%d", parsedVersion.Major(), parsedVersion.Minor()),
				Cluster:           cluster,
				Machine:           machine.machine,
				Preloaded:         preloaded,
			},
			deployOnVC,
		)
		if err != nil {
			return "", err
		}
	}
	startupScript = base64.StdEncoding.EncodeToString([]byte(startupScript))
	return startupScript, nil
}
