/*
Copyright 2017 The Kubernetes Authors.

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

package terraform

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	vsphereconfig "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/namedmachines"
	vpshereprovisionercommon "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/provisioner/common"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	"sigs.k8s.io/cluster-api/clusterctl/clusterdeployer"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	v1alpha1 "sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
	apierrors "sigs.k8s.io/cluster-api/pkg/errors"
	"sigs.k8s.io/cluster-api/pkg/kubeadm"
	"sigs.k8s.io/cluster-api/pkg/util"
)

const (
	// The contents of the tfstate file for a machine.
	StatusMachineTerraformState = "tf-state"
	StageDir                    = "/tmp/cluster-api/machines"
	MachinePathStageFormat      = "/tmp/cluster-api/machines/%s/"
	TfConfigFilename            = "terraform.tf"
	TfVarsFilename              = "variables.tfvars"
	TfStateFilename             = "terraform.tfstate"
	startupScriptFilename       = "machine-startup.sh"
)

type Provisioner struct {
	clusterV1alpha1   clusterv1alpha1.ClusterV1alpha1Interface
	scheme            *runtime.Scheme
	lister            v1alpha1.Interface
	namedMachineWatch *namedmachines.ConfigWatch
	eventRecorder     record.EventRecorder
	// Once the vsphere-deployer is deleted, both DeploymentClient and VsphereClient can depend on
	// something that implements GetIP instead of the VsphereClient depending on DeploymentClient.
	deploymentClient clusterdeployer.ProviderDeployer
}

func New(clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface, lister v1alpha1.Interface, eventRecorder record.EventRecorder, namedMachinePath string, depClient clusterdeployer.ProviderDeployer) (*Provisioner, error) {
	scheme, _, err := vsphereconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}

	// DEPRECATED: Remove when vsphere-deployer is deleted.
	var nmWatch *namedmachines.ConfigWatch
	nmWatch, err = namedmachines.NewConfigWatch(namedMachinePath)
	if err != nil {
		glog.Errorf("error creating named machine config watch: %+v", err)
	}
	return &Provisioner{
		clusterV1alpha1:   clusterV1alpha1,
		scheme:            scheme,
		lister:            lister,
		namedMachineWatch: nmWatch,
		eventRecorder:     eventRecorder,
		deploymentClient:  depClient,
	}, nil
}

func saveFile(contents, path string, perm os.FileMode) error {
	return ioutil.WriteFile(path, []byte(contents), perm)
}

// Stage the machine for running terraform.
// Return: machine's staging dir path, error
func (pv *Provisioner) prepareStageMachineDir(machine *clusterv1.Machine, eventAction string) (string, error) {
	err := pv.cleanUpStagingDir(machine)
	if err != nil {
		return "", err
	}

	machineName := machine.ObjectMeta.Name
	config, err := vsphereutils.GetMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return "", pv.handleMachineError(machine, apierrors.InvalidMachineConfiguration(
			"Cannot unmarshal providerConfig field: %v", err), eventAction)
	}

	machinePath := fmt.Sprintf(MachinePathStageFormat, machineName)
	if _, err := os.Stat(machinePath); os.IsNotExist(err) {
		os.MkdirAll(machinePath, 0755)
	}

	// Save the config file and variables file to the machinePath
	tfConfigPath := path.Join(machinePath, TfConfigFilename)
	tfVarsPath := path.Join(machinePath, TfVarsFilename)

	namedMachines, err := pv.namedMachineWatch.NamedMachines()
	if err != nil {
		return "", err
	}
	matchedMachine, err := namedMachines.MatchMachine(config.VsphereMachine)
	if err != nil {
		return "", err
	}
	if err := saveFile(matchedMachine.MachineHcl, tfConfigPath, 0644); err != nil {
		return "", err
	}

	var tfVarsContents []string
	for key, value := range config.MachineVariables {
		tfVarsContents = append(tfVarsContents, fmt.Sprintf("%s=\"%s\"", key, value))
	}
	if err := saveFile(strings.Join(tfVarsContents, "\n"), tfVarsPath, 0644); err != nil {
		return "", err
	}

	// Save the tfstate file (if not bootstrapping).
	_, err = pv.stageTfState(machine)
	if err != nil {
		return "", err
	}

	return machinePath, nil
}

// Returns the path to the tfstate file staged from the tf state in annotations.
func (pv *Provisioner) stageTfState(machine *clusterv1.Machine) (string, error) {
	machinePath := fmt.Sprintf(MachinePathStageFormat, machine.ObjectMeta.Name)
	tfStateFilePath := path.Join(machinePath, TfStateFilename)

	if _, err := os.Stat(machinePath); os.IsNotExist(err) {
		os.MkdirAll(machinePath, 0755)
	}

	// Attempt to stage the file from annotations.
	glog.Infof("Attempting to stage tf state for machine %s", machine.ObjectMeta.Name)
	if machine.ObjectMeta.Annotations == nil {
		glog.Infof("machine does not have annotations, state does not exist")
		return "", nil
	}

	if _, ok := machine.ObjectMeta.Annotations[StatusMachineTerraformState]; !ok {
		glog.Info("machine does not have annotation for tf state.")
		return "", nil
	}

	tfStateB64, _ := machine.ObjectMeta.Annotations[StatusMachineTerraformState]
	tfState, err := base64.StdEncoding.DecodeString(tfStateB64)
	if err != nil {
		glog.Errorf("error decoding tfstate while staging. %+v", err)
		return "", err
	}
	if err := saveFile(string(tfState), tfStateFilePath, 0644); err != nil {
		return "", err
	}

	return tfStateFilePath, nil
}

// Cleans up the staging directory.
func (pv *Provisioner) cleanUpStagingDir(machine *clusterv1.Machine) error {
	glog.Infof("Cleaning up the staging dir for machine %s", machine.ObjectMeta.Name)
	return os.RemoveAll(fmt.Sprintf(MachinePathStageFormat, machine.ObjectMeta.Name))
}

// Builds and saves the startup script for the passed machine and cluster.
// Returns the full path of the saved startup script and possible error.
func (pv *Provisioner) saveStartupScript(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	config, err := vsphereutils.GetMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return "", pv.handleMachineError(machine, apierrors.InvalidMachineConfiguration(
			"Cannot unmarshal providerConfig field: %v", err), constants.CreateEventAction)
	}
	preloaded := false
	if val, ok := config.MachineVariables["preloaded"]; ok {
		preloaded, err = strconv.ParseBool(val)
		if err != nil {
			return "", pv.handleMachineError(machine, apierrors.InvalidMachineConfiguration(
				"Invalid value for preloaded: %v", err), constants.CreateEventAction)
		}
	}

	var startupScript string

	if util.IsMaster(machine) {
		if machine.Spec.Versions.ControlPlane == "" {
			return "", pv.handleMachineError(machine, apierrors.InvalidMachineConfiguration(
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
			return "", errors.New("invalid cluster state: cannot create a Kubernetes node without an API endpoint")
		}
		kubeadmToken, err := pv.getKubeadmToken(cluster)
		if err != nil {
			return "", err
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

	glog.Infof("Saving startup script for machine %s", machine.ObjectMeta.Name)
	// Save the startup script.
	machinePath := fmt.Sprintf(MachinePathStageFormat, machine.Name)
	startupScriptPath := path.Join(machinePath, startupScriptFilename)
	if err := saveFile(startupScript, startupScriptPath, 0644); err != nil {
		return "", errors.New(fmt.Sprintf("Could not write startup script %s", err))
	}

	return startupScriptPath, nil
}

func (pv *Provisioner) Create(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	config, err := vsphereutils.GetMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return pv.handleMachineError(machine, apierrors.InvalidMachineConfiguration(
			"Cannot unmarshal providerConfig field: %v", err), constants.CreateEventAction)
	}

	clusterConfig, err := vsphereutils.GetClusterProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return err
	}

	if verr := pv.validateMachine(machine, config); verr != nil {
		return pv.handleMachineError(machine, verr, constants.CreateEventAction)
	}

	if verr := pv.validateCluster(cluster); verr != nil {
		return verr
	}

	machinePath, err := pv.prepareStageMachineDir(machine, constants.CreateEventAction)
	if err != nil {
		return errors.New(fmt.Sprintf("error while staging machine: %+v", err))
	}

	glog.Infof("Staged for machine create at %s", machinePath)

	// Save the startup script.
	startupScriptPath, err := pv.saveStartupScript(cluster, machine)
	if err != nil {
		return errors.New(fmt.Sprintf("could not write startup script %+v", err))
	}
	defer cleanUpStartupScript(machine.Name, startupScriptPath)

	glog.Infof("Checking if machine %s exists", machine.ObjectMeta.Name)
	instance, err := pv.instanceIfExists(machine)
	if err != nil {
		return err
	}

	if instance == nil {
		glog.Infof("Machine instance does not exist. will create %s", machine.ObjectMeta.Name)

		var args []string
		args = append(args, "apply")
		args = append(args, "-auto-approve")
		args = append(args, "-input=false")
		args = append(args, "-var")
		args = append(args, fmt.Sprintf("vm_name=%s", machine.ObjectMeta.Name))
		args = append(args, "-var")
		args = append(args, fmt.Sprintf("vsphere_user=%s", clusterConfig.VsphereUser))
		args = append(args, "-var")
		args = append(args, fmt.Sprintf("vsphere_password=%s", clusterConfig.VspherePassword))
		args = append(args, "-var")
		args = append(args, fmt.Sprintf("vsphere_server=%s", clusterConfig.VsphereServer))
		args = append(args, fmt.Sprintf("-var-file=%s", path.Join(machinePath, TfVarsFilename)))
		args = append(args, "-var")
		args = append(args, fmt.Sprintf("startup_script_path=%s", startupScriptPath))

		_, cmdErr := runTerraformCmd(false, machinePath, args...)
		if cmdErr != nil {
			return errors.New(fmt.Sprintf("could not run terraform to create machine: %s", cmdErr))
		}

		// Get the IP address
		out, cmdErr := runTerraformCmd(false, machinePath, "output", "ip_address")
		if cmdErr != nil {
			return fmt.Errorf("could not obtain 'ip_address' output variable: %s", cmdErr)
		}
		vmIp := strings.TrimSpace(out.String())
		glog.Infof("Machine %s created with ip address %s", machine.ObjectMeta.Name, vmIp)

		// Annotate the machine so that we remember exactly what VM we created for it.
		tfState, _ := pv.GetTfState(machine)
		pv.cleanUpStagingDir(machine)
		pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Created", "Created Machine %v", machine.Name)
		return pv.updateAnnotations(cluster, machine, vmIp, tfState)
	} else {
		glog.Infof("Skipped creating a VM for machine %s that already exists.", machine.ObjectMeta.Name)
	}

	return nil
}

// Assumes the staging dir (workingDir) is set up correctly. That means have the .tf, .tfstate and .tfvars
// there as needed.
// Set stdout=true to redirect process's standard out to sys stdout.
// Otherwise returns a byte buffer of stdout.
func runTerraformCmd(stdout bool, workingDir string, arg ...string) (bytes.Buffer, error) {
	var out bytes.Buffer

	terraformPath, err := exec.LookPath("terraform")
	if err != nil {
		return bytes.Buffer{}, errors.New("terraform binary not found")
	}
	glog.Infof("terraform path: %s", terraformPath)

	// Check if we need to terraform init
	tfInitExists, e := pathExists(path.Join(workingDir, ".terraform/"))
	if e != nil {
		return bytes.Buffer{}, errors.New(fmt.Sprintf("Could not get the path of .terraform for machine: %+v", e))
	}
	if !tfInitExists {
		glog.Infof("Terraform not initialized. Running terraform init.")
		cmd := exec.Command(terraformPath, "init")
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		cmd.Dir = workingDir

		initErr := cmd.Run()
		if initErr != nil {
			return bytes.Buffer{}, errors.New(fmt.Sprintf("Could not run terraform: %+v", initErr))
		}
	}

	cmd := exec.Command(terraformPath, arg...)
	// If stdout, only show in stdout
	if stdout {
		cmd.Stdout = os.Stdout
	} else {
		// Otherwise, save to buffer, and to a local log file.
		logFileName := fmt.Sprintf("/tmp/cluster-api-%s.log", util.RandomToken())
		f, _ := os.Create(logFileName)
		glog.Infof("Executing terraform. Logs are saved in %s", logFileName)
		multiWriter := io.MultiWriter(&out, f)
		cmd.Stdout = multiWriter
	}
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Dir = workingDir
	err = cmd.Run()
	if err != nil {
		return out, err
	}

	return out, nil
}

func (pv *Provisioner) Delete(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	// Check if the instance exists, return if it doesn't
	instance, err := pv.instanceIfExists(machine)
	if err != nil {
		return err
	}
	if instance == nil {
		return nil
	}

	clusterConfig, err := vsphereutils.GetClusterProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return err
	}

	machinePath, err := pv.prepareStageMachineDir(machine, constants.DeleteEventAction)

	// destroy it
	args := []string{
		"destroy",
		"-auto-approve",
		"-input=false",
		"-var", fmt.Sprintf("vm_name=%s", machine.ObjectMeta.Name),
		"-var", fmt.Sprintf("vsphere_user=%s", clusterConfig.VsphereUser),
		"-var", fmt.Sprintf("vsphere_password=%s", clusterConfig.VspherePassword),
		"-var", fmt.Sprintf("vsphere_server=%s", clusterConfig.VsphereServer),
		"-var", "startup_script_path=/dev/null",
		"-var-file=variables.tfvars",
	}
	_, err = runTerraformCmd(false, machinePath, args...)
	if err != nil {
		return fmt.Errorf("could not run terraform: %s", err)
	}

	pv.cleanUpStagingDir(machine)

	// Update annotation for the state.
	machine.ObjectMeta.Annotations[StatusMachineTerraformState] = ""
	_, err = pv.clusterV1alpha1.Machines(machine.Namespace).Update(machine)

	if err == nil {
		pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Killing", "Killing machine %v", machine.Name)
	}

	return err
}

func (pv *Provisioner) PostDelete(cluster *clusterv1.Cluster) error {
	return nil
}

func (pv *Provisioner) Update(cluster *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	// Check if the annotations we want to track exist, if not, the user likely created a master machine with their own annotation.
	if _, ok := goalMachine.ObjectMeta.Annotations[constants.ControlPlaneVersionAnnotationKey]; !ok {
		ip, _ := pv.deploymentClient.GetIP(nil, goalMachine)
		glog.Info("Version annotations do not exist. Populating existing state for bootstrapped machine.")
		tfState, _ := pv.GetTfState(goalMachine)
		return pv.updateAnnotations(cluster, goalMachine, ip, tfState)
	}

	if util.IsMaster(goalMachine) {
		// Master upgrades
		glog.Info("Upgrade for master machine.. Check if upgrade needed.")

		// If the saved versions and new versions differ, do in-place upgrade.
		if pv.needsMasterUpdate(goalMachine) {
			glog.Infof("Doing in-place upgrade for master from v%s to v%s", goalMachine.Annotations[constants.ControlPlaneVersionAnnotationKey], goalMachine.Spec.Versions.ControlPlane)
			err := pv.updateMasterInPlace(goalMachine)
			if err != nil {
				glog.Errorf("Master in-place upgrade failed: %+v", err)
				return err
			}
		} else {
			glog.Info("UNSUPPORTED MASTER UPDATE.")
		}
	} else {
		if pv.needsNodeUpdate(goalMachine) {
			// Node upgrades
			if err := pv.updateNode(cluster, goalMachine); err != nil {
				glog.Errorf("Node %s update failed: %+v", goalMachine.ObjectMeta.Name, err)
				return err
			}
		} else {
			glog.Info("UNSUPPORTED NODE UPDATE.")
		}
	}

	return nil
}

func (pv *Provisioner) needsControlPlaneUpdate(machine *clusterv1.Machine) bool {
	return machine.Spec.Versions.ControlPlane != machine.Annotations[constants.ControlPlaneVersionAnnotationKey]
}

func (pv *Provisioner) needsKubeletUpdate(machine *clusterv1.Machine) bool {
	return machine.Spec.Versions.Kubelet != machine.Annotations[constants.KubeletVersionAnnotationKey]
}

// Returns true if the node is needed to be upgraded.
func (pv *Provisioner) needsNodeUpdate(machine *clusterv1.Machine) bool {
	return !util.IsMaster(machine) &&
		pv.needsKubeletUpdate(machine)
}

// Returns true if the master is needed to be upgraded.
func (pv *Provisioner) needsMasterUpdate(machine *clusterv1.Machine) bool {
	return util.IsMaster(machine) &&
		pv.needsControlPlaneUpdate(machine)
	// TODO: we should support kubelet upgrades here as well.
}

func (pv *Provisioner) updateKubelet(machine *clusterv1.Machine) error {
	if pv.needsKubeletUpdate(machine) {
		// Kubelet packages are versioned 1.10.1-00 and so on.
		kubeletAptVersion := machine.Spec.Versions.Kubelet + "-00"
		cmd := fmt.Sprintf("sudo apt-get install kubelet=%s", kubeletAptVersion)
		if _, err := pv.remoteSshCommand(machine, cmd, "~/.ssh/vsphere_tmp", "ubuntu"); err != nil {
			glog.Errorf("remoteSshCommand while installing new kubelet version: %v", err)
			return err
		}
	}
	return nil
}

func (pv *Provisioner) updateControlPlane(machine *clusterv1.Machine) error {
	if pv.needsControlPlaneUpdate(machine) {
		// Pull the kudeadm for target version K8s.
		cmd := fmt.Sprintf("curl -sSL https://dl.k8s.io/release/v%s/bin/linux/amd64/kubeadm | sudo tee /usr/bin/kubeadm > /dev/null; "+
			"sudo chmod a+rx /usr/bin/kubeadm", machine.Spec.Versions.ControlPlane)
		if _, err := pv.remoteSshCommand(machine, cmd, "~/.ssh/vsphere_tmp", "ubuntu"); err != nil {
			glog.Infof("remoteSshCommand failed while downloading new kubeadm: %+v", err)
			return err
		}

		// Next upgrade control plane
		cmd = fmt.Sprintf("sudo kubeadm upgrade apply %s -y", "v"+machine.Spec.Versions.ControlPlane)
		if _, err := pv.remoteSshCommand(machine, cmd, "~/.ssh/vsphere_tmp", "ubuntu"); err != nil {
			glog.Infof("remoteSshCommand failed while upgrading control plane: %+v", err)
			return err
		}
	}
	return nil
}

// Update the passed node machine by recreating it.
func (pv *Provisioner) updateNode(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if err := pv.Delete(cluster, machine); err != nil {
		return err
	}

	if err := pv.Create(cluster, machine); err != nil {
		return err
	}
	return nil
}

// Assumes that update is needed.
// For now support only K8s control plane upgrades.
func (pv *Provisioner) updateMasterInPlace(machine *clusterv1.Machine) error {
	// Execute a control plane upgrade.
	if err := pv.updateControlPlane(machine); err != nil {
		return err
	}

	// Update annotation for version.
	machine.ObjectMeta.Annotations[constants.ControlPlaneVersionAnnotationKey] = machine.Spec.Versions.ControlPlane
	if _, err := pv.clusterV1alpha1.Machines(machine.Namespace).Update(machine); err != nil {
		return err
	}
	return nil
}

func (pv *Provisioner) remoteSshCommand(m *clusterv1.Machine, cmd, privateKeyPath, sshUser string) (string, error) {
	glog.Infof("Remote SSH execution '%s' on %s", cmd, m.ObjectMeta.Name)

	publicIP, err := pv.deploymentClient.GetIP(nil, m)
	if err != nil {
		return "", err
	}

	command := fmt.Sprintf("echo STARTFILE; %s", cmd)
	c := exec.Command("ssh", "-i", privateKeyPath, sshUser+"@"+publicIP,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		command)
	out, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error: %v, output: %s", err, string(out))
	}
	result := strings.TrimSpace(string(out))
	parts := strings.Split(result, "STARTFILE")
	glog.Infof("\t Result of command %s ========= %+v", cmd, parts)
	if len(parts) != 2 {
		return "", nil
	}
	// TODO: Check error.
	return strings.TrimSpace(parts[1]), nil
}

func (pv *Provisioner) Exists(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	i, err := pv.instanceIfExists(machine)
	if err != nil {
		return false, err
	}
	return i != nil, err
}

func (pv *Provisioner) GetTfState(machine *clusterv1.Machine) (string, error) {
	if machine.ObjectMeta.Annotations != nil {
		if tfStateB64, ok := machine.ObjectMeta.Annotations[StatusMachineTerraformState]; ok {
			glog.Infof("Returning tfstate for machine %s from annotation", machine.ObjectMeta.Name)
			tfState, err := base64.StdEncoding.DecodeString(tfStateB64)
			if err != nil {
				glog.Errorf("error decoding tfstate in annotation. %+v", err)
				return "", err
			}
			return string(tfState), nil
		}
	}

	tfStateBytes, _ := ioutil.ReadFile(path.Join(fmt.Sprintf(MachinePathStageFormat, machine.ObjectMeta.Name), TfStateFilename))
	if tfStateBytes != nil {
		glog.Infof("Returning tfstate for machine %s from staging file", machine.ObjectMeta.Name)
		return string(tfStateBytes), nil
	}

	return "", errors.New("could not get tfstate")
}

// We are storing these as annotations and not in Machine Status because that's intended for
// "Provider-specific status" that will usually be used to detect updates. Additionally,
// Status requires yet another version API resource which is too heavy to store IP and TF state.
func (pv *Provisioner) updateAnnotations(cluster *clusterv1.Cluster, machine *clusterv1.Machine, vmIP, tfState string) error {
	glog.Infof("Updating annotations for machine %s", machine.ObjectMeta.Name)
	if machine.ObjectMeta.Annotations == nil {
		machine.ObjectMeta.Annotations = make(map[string]string)
	}

	tfStateB64 := base64.StdEncoding.EncodeToString([]byte(tfState))

	machine.ObjectMeta.Annotations[constants.VmIpAnnotationKey] = vmIP
	machine.ObjectMeta.Annotations[constants.ControlPlaneVersionAnnotationKey] = machine.Spec.Versions.ControlPlane
	machine.ObjectMeta.Annotations[constants.KubeletVersionAnnotationKey] = machine.Spec.Versions.Kubelet
	machine.ObjectMeta.Annotations[StatusMachineTerraformState] = tfStateB64

	_, err := pv.clusterV1alpha1.Machines(machine.Namespace).Update(machine)
	if err != nil {
		return err
	}
	// Update the cluster status with updated time stamp for tracking purposes
	status := &vsphereconfig.VsphereClusterProviderStatus{LastUpdated: time.Now().UTC().String()}
	out, err := json.Marshal(status)
	cluster.Status.ProviderStatus = &runtime.RawExtension{Raw: out}
	_, err = pv.clusterV1alpha1.Clusters(cluster.Namespace).UpdateStatus(cluster)
	if err != nil {
		glog.Infof("Error in updating the status: %s", err)
		return err
	}
	return nil
}

// Returns the machine object if the passed machine exists in terraform state.
func (pv *Provisioner) instanceIfExists(machine *clusterv1.Machine) (*clusterv1.Machine, error) {
	machinePath := fmt.Sprintf(MachinePathStageFormat, machine.ObjectMeta.Name)
	tfStateFilePath, err := pv.stageTfState(machine)
	if err != nil {
		return nil, err
	}
	glog.Infof("Instance existance checked in directory %+v", tfStateFilePath)
	if tfStateFilePath == "" {
		return nil, nil
	}

	out, tfCmdErr := runTerraformCmd(false, machinePath, "show")
	if tfCmdErr != nil {
		glog.Infof("Ignore terraform error in instanceIfExists: %+v", err)
		return nil, nil
	}
	re := regexp.MustCompile(fmt.Sprintf("\n[[:space:]]*(name = %s)\n", machine.ObjectMeta.Name))
	if re.MatchString(out.String()) {
		return machine, nil
	}
	return nil, nil
}

func (pv *Provisioner) validateMachine(machine *clusterv1.Machine, config *vsphereconfig.VsphereMachineProviderConfig) *apierrors.MachineError {
	if machine.Spec.Versions.Kubelet == "" {
		return apierrors.InvalidMachineConfiguration("spec.versions.kubelet can't be empty")
	}
	return nil
}

func (pv *Provisioner) validateCluster(cluster *clusterv1.Cluster) error {
	if cluster.Spec.ClusterNetwork.ServiceDomain == "" {
		return errors.New("invalid cluster configuration: missing Cluster.Spec.ClusterNetwork.ServiceDomain")
	}
	if vsphereutils.GetSubnet(cluster.Spec.ClusterNetwork.Pods) == "" {
		return errors.New("invalid cluster configuration: missing Cluster.Spec.ClusterNetwork.Pods")
	}
	if vsphereutils.GetSubnet(cluster.Spec.ClusterNetwork.Services) == "" {
		return errors.New("invalid cluster configuration: missing Cluster.Spec.ClusterNetwork.Services")
	}

	clusterConfig, err := vsphereutils.GetClusterProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return err
	}
	if clusterConfig.VsphereUser == "" || clusterConfig.VspherePassword == "" || clusterConfig.VsphereServer == "" {
		return errors.New("vsphere_user, vsphere_password, vsphere_server are required fields in Cluster spec.")
	}
	return nil
}

func (pv *Provisioner) getKubeadmToken(cluster *clusterv1.Cluster) (string, error) {
	// From the cluster locate the master node
	master, err := pv.getMasterForCluster(cluster)
	if err != nil {
		return "", err
	}
	kubeconfig, err := pv.deploymentClient.GetKubeConfig(cluster, master)
	if err != nil {
		return "", err
	}
	tmpconfig, err := createTempFile(kubeconfig)
	if err != nil {
		return "", err
	}
	tokenParams := kubeadm.TokenCreateParams{
		KubeConfig: tmpconfig,
		Ttl:        constants.KubeadmTokenTtl,
	}
	output, err := kubeadm.New().TokenCreate(tokenParams)
	if err != nil {
		return "", fmt.Errorf("unable to create kubeadm token: %v", err)
	}
	return strings.TrimSpace(output), err
}

func (pv *Provisioner) getMasterForCluster(cluster *clusterv1.Cluster) (*clusterv1.Machine, error) {
	machines, err := pv.lister.Machines().Lister().Machines(cluster.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		//glog.Infof("%v", machine)
		if util.IsMaster(machine) {
			// Return the first master for now. Need to handle the multi-master case better
			glog.Infof("Found the master VM %s", machine.Name)
			return machine, nil
		}
	}
	return nil, fmt.Errorf("No master node found for the cluster %s", cluster.Name)
}

func createTempFile(contents string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		glog.Warningf("Error creating temporary file")
		return "", err
	}
	// For any error in this method, clean up the temp file
	defer func(pErr *error, path string) {
		if *pErr != nil {
			if err := os.Remove(path); err != nil {
				glog.Warningf("Error removing file '%s': %v", path, err)
			}
		}
	}(&err, tmpFile.Name())

	if _, err = tmpFile.Write([]byte(contents)); err != nil {
		glog.Warningf("Error writing to temporary file '%s'", tmpFile.Name())
		return "", err
	}
	if err = tmpFile.Close(); err != nil {
		return "", err
	}
	if err = os.Chmod(tmpFile.Name(), 0644); err != nil {
		glog.Warningf("Error setting file permission to 0644 for the temporary file '%s'", tmpFile.Name())
		return "", err
	}
	return tmpFile.Name(), nil
}

// If the Provisioner has a client for updating Machine objects, this will set
// the appropriate reason/message on the Machine.Status. If not, such as during
// cluster installation, it will operate as a no-op. It also returns the
// original error for convenience, so callers can do "return handleMachineError(...)".
func (pv *Provisioner) handleMachineError(machine *clusterv1.Machine, err *apierrors.MachineError, eventAction string) error {
	if pv.clusterV1alpha1 != nil {
		reason := err.Reason
		message := err.Message
		machine.Status.ErrorReason = &reason
		machine.Status.ErrorMessage = &message
		pv.clusterV1alpha1.Machines(machine.Namespace).UpdateStatus(machine)
	}
	if eventAction != "" {
		pv.eventRecorder.Eventf(machine, corev1.EventTypeWarning, "Failed"+eventAction, "%v", err.Reason)
	}

	glog.Errorf("Machine error: %v", err.Message)
	return err
}

func run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(out))
	}
	return nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func cleanUpStartupScript(machineName, fullPath string) {
	glog.Infof("cleaning up startup script for %v: %v", machineName, fullPath)
	if err := os.RemoveAll(path.Dir(fullPath)); err != nil {
		glog.Error(err)
	}
}
