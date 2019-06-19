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
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"

	gocontext "context"

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
	if ctx.HasControlPlaneRole() {

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

	} else {
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
	}

	userData64 := base64.StdEncoding.EncodeToString([]byte(userDataYAML))
	var task *object.Task

	builder, err := NewVmSpecBuilder(ctx)
	if err != nil {
		return err
	}

	if ctx.Session.IsVC() {
		// Use the appropriate path if we're connected to a vCenter
		spec, err := builder.BuildCloneSpec(userData64)
		if err != nil {
			return err
		}
		ctx.Logger.V(6).Info("vcenter cloning machine", "clone-spec", spec)
		task, err = builder.Src.Clone(ctx, builder.Folder, ctx.Machine.Name, *spec)
		if err != nil {
			return errors.Wrapf(err, "error trigging clone op for machine %q", ctx)
		}

	} else {
		// ESXi does not support cloning, we need to "ghetto" clone the VM
		spec, err := builder.BuildCreateSpec(userData64)
		if err != nil {
			return err
		}
		ctx.Logger.V(6).Info("esxi cloning machine", "clone-spec", spec)

		dc, err := ctx.Session.Finder.DefaultDatacenter(ctx.Context)
		if err != nil {
			return err
		}
		folders, err := dc.Folders(ctx)
		if err != nil {
			return err
		}
		task, err = folders.VmFolder.CreateVM(ctx, *spec, builder.Pool, builder.Host)
		ctx.Logger.V(4).Info("Task submitted: " + task.Reference().Value)
		if err != nil {
			return errors.Wrapf(err, "Failed to submit createVM task")
		}

		// CreateVM does not take PowerOn param, so we need to power on only
		// after the creation has completed, *but* we don't want to block
		// on creation either
		go func() {
			result, err := task.WaitForResult(gocontext.TODO(), nil)
			if err == nil {
				return
			}
			vmRef := result.Result.(types.ManagedObjectReference)
			vm := object.NewVirtualMachine(ctx.Session.Client.Client, vmRef)
			ctx.Logger.V(4).Info("powering on VM")
			_, err = vm.PowerOn(gocontext.TODO())
			if err != nil {
				ctx.Logger.V(1).Info("Failed to power-on", "err", err)
			}
		}()

	}

	ctx.MachineConfig.MachineRef = "creating"
	ctx.MachineStatus.TaskRef = task.Reference().Value
	return nil
}

func findVMByInstanceUUID(ctx *context.MachineContext) (string, error) {
	ctx.Logger.V(6).Info("finding vm by instance UUID", "instance-uuid", ctx.Machine.UID)

	vmRef, err := ctx.Session.FindByInstanceUUID(ctx, string(ctx.Machine.UID))
	if err != nil {
		return "", errors.Wrapf(err, ctx.String())
	}

	if vmRef != nil {
		ctx.Logger.V(5).Info("found machine by instance UUID", "vmRef", vmRef.Reference().Value, "instance-uuid", ctx.Machine.UID)
		return vmRef.Reference().Value, nil
	}

	return ctx.MachineConfig.MachineRef, nil
}

func verifyAndUpdateTask(ctx *context.MachineContext, taskRef string) error {
	ctx.Logger.V(6).Info("verifying and updating tasks")

	var obj mo.Task
	moRef := types.ManagedObjectReference{
		Type:  morefTypeTask,
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

		case taskFolderCreateVM:
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

		case taskVMClone:
			logger.V(4).Info("task is a clone op")
			vmRef := obj.Info.Result.(types.ManagedObjectReference)
			ctx.MachineConfig.MachineRef = vmRef.Value
			ctx.MachineStatus.TaskRef = ""
			record.Eventf(ctx.Machine, "CloneSuccess", "cloned machine %q", ctx)

		case taskVMReconfigure:
			record.Eventf(ctx.Machine, "ReconfigSuccess", "reconfigured machine %q", ctx)
			ctx.MachineStatus.TaskRef = ""
		}

		// The task on the VM completed successfully.
		// Requeue the machine after one second so Exists==true and Update
		// will be called, causing the VM's new state to be patched into
		// the machine object.
		return &clustererror.RequeueAfterError{RequeueAfter: time.Second * 1}

	case types.TaskInfoStateError:
		logger.V(2).Info("task failed", "description-id", obj.Info.DescriptionId)

		switch obj.Info.DescriptionId {

		case taskVMClone:
			record.Warnf(ctx.Machine, "CloneFailure", "clone machine failed %q", ctx)
			ctx.MachineStatus.TaskRef = ""

		case taskFolderCreateVM:
			record.Warnf(ctx.Machine, "CreateFailure", "create machine failed %q", ctx)
			ctx.MachineStatus.TaskRef = ""
		}

	default:
		return errors.Errorf("task %q has unknown state %v", taskRef, obj.Info.State)
	}

	return nil
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
