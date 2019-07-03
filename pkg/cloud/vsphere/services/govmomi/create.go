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
	"fmt"
	"net"

	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/certificates"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/esxi"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/vcenter"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeadm"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/userdata"
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

	// the Kubernetes cloud provider to use
	cloudProvider = "vsphere"

	// the cloud config path read by the cloud provider
	cloudConfigPath = "/etc/kubernetes/vsphere.conf"
)

// Create creates a new machine.
func Create(ctx *context.MachineContext, bootstrapToken string) error {
	// Check to see if the VM exists first since no error is returned if the VM
	// does not exist, only when there's an error checking or when the op should
	// be requeued, like when the VM has an in-flight task.
	vm, err := lookupVM(ctx)
	if err != nil {
		return err
	}
	if vm != nil {
		return errors.Errorf("vm already exists for %q", ctx)
	}

	userData, err := generateUserData(ctx, bootstrapToken)
	if err != nil {
		return errors.Wrapf(err, "error generating user data for %q", ctx)
	}

	if ctx.Session.IsVC() {
		return vcenter.Clone(ctx, userData)
	}
	return esxi.Clone(ctx, userData)
}

func generateUserData(ctx *context.MachineContext, bootstrapToken string) ([]byte, error) {
	caCertHash, err := certificates.GenerateCertificateHash(ctx.ClusterConfig.CAKeyPair.Cert)
	if err != nil {
		return nil, err
	}

	var controlPlaneEndpoint string
	if bootstrapToken != "" {
		var err error
		if controlPlaneEndpoint, err = ctx.ControlPlaneEndpoint(); err != nil {
			return nil, errors.Wrapf(err, "unable to get control plane endpoint while creating machine %q", ctx)
		}
	}

	var userDataYAML string

	// apply values based on the role of the machine
	if ctx.HasControlPlaneRole() {

		// Cloud init needs the a valid vmfolder in cloudconfig
		// Check the vmfolder and replace with default if not present
		folder, err := ctx.Session.Finder.FolderOrDefault(ctx, ctx.MachineConfig.MachineSpec.VMFolder)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get folder for %q", ctx)
		}

		cloudConfig, err := userdata.NewCloudConfig(&userdata.CloudConfigInput{
			User:         ctx.ClusterConfig.VsphereUser,
			Password:     ctx.ClusterConfig.VspherePassword,
			Server:       ctx.ClusterConfig.VsphereServer,
			Datacenter:   ctx.MachineConfig.MachineSpec.Datacenter,
			ResourcePool: ctx.MachineConfig.MachineSpec.ResourcePool,
			Folder:       folder.Name(),
			Datastore:    ctx.MachineConfig.MachineSpec.Datastore,
			// assume the first VM network found for the vSphere cloud provider
			Network: ctx.MachineConfig.MachineSpec.Network.Devices[0].NetworkName,
		})
		if err != nil {
			return nil, err
		}

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
					kubeadm.SetNodeRegistration(
						&ctx.MachineConfig.KubeadmConfiguration.Join.NodeRegistration,
						kubeadm.WithTaints(ctx.Machine.Spec.Taints),
						kubeadm.WithNodeRegistrationName(hostnameLookup),
						kubeadm.WithCRISocket(containerdSocket),
						kubeadm.WithKubeletExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
					),
				),
				kubeadm.WithLocalAPIEndpointAndPort(localIPV4Lookup, int(bindPort)),
			)
			joinConfigurationYAML, err := kubeadm.ConfigurationToYAML(&ctx.MachineConfig.KubeadmConfiguration.Join)
			if err != nil {
				return nil, err
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
				CloudConfig:       cloudConfig,
				JoinConfiguration: joinConfigurationYAML,
			})
			if err != nil {
				return nil, err
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
					return nil, err
				}
				certSans = append(certSans, host)
			}

			ctx.Logger.V(2).Info("machine is the first control plane machine for the cluster")
			if !ctx.ClusterConfig.CAKeyPair.HasCertAndKey() {
				return nil, errors.New("failed to run controlplane, missing CAPrivateKey")
			}

			kubeadm.SetClusterConfigurationOptions(
				&ctx.ClusterConfig.ClusterConfiguration,
				kubeadm.WithControlPlaneEndpoint(fmt.Sprintf("%s:%d", localIPV4Lookup, bindPort)),
				kubeadm.WithAPIServerCertificateSANs(certSans...),
				kubeadm.WithAPIServerExtraArgs(map[string]string{
					"cloud-provider": cloudProvider,
					"cloud-config":   cloudConfigPath,
				}),
				kubeadm.WithControllerManagerExtraArgs(map[string]string{
					"cloud-provider": cloudProvider,
					"cloud-config":   cloudConfigPath,
				}),
				kubeadm.WithAPIServerExtraVolumes("cloud-config", cloudConfigPath, cloudConfigPath),
				kubeadm.WithControllerManagerExtraVolumes("cloud-config", cloudConfigPath, cloudConfigPath),
				kubeadm.WithClusterName(ctx.Cluster.Name),
				kubeadm.WithClusterNetworkFromClusterNetworkingConfig(ctx.Cluster.Spec.ClusterNetwork),
				kubeadm.WithKubernetesVersion(ctx.Machine.Spec.Versions.ControlPlane),
			)
			clusterConfigYAML, err := kubeadm.ConfigurationToYAML(&ctx.ClusterConfig.ClusterConfiguration)
			if err != nil {
				return nil, err
			}

			kubeadm.SetInitConfigurationOptions(
				&ctx.MachineConfig.KubeadmConfiguration.Init,
				kubeadm.WithNodeRegistrationOptions(
					kubeadm.SetNodeRegistration(
						&ctx.MachineConfig.KubeadmConfiguration.Init.NodeRegistration,
						kubeadm.WithTaints(ctx.Machine.Spec.Taints),
						kubeadm.WithNodeRegistrationName(hostnameLookup),
						kubeadm.WithCRISocket(containerdSocket),
						kubeadm.WithKubeletExtraArgs(map[string]string{"cloud-provider": cloudProvider}),
					),
				),
				kubeadm.WithInitLocalAPIEndpointAndPort(localIPV4Lookup, int(bindPort)),
			)
			initConfigYAML, err := kubeadm.ConfigurationToYAML(&ctx.MachineConfig.KubeadmConfiguration.Init)
			if err != nil {
				return nil, err
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
				CloudConfig:          cloudConfig,
				ClusterConfiguration: clusterConfigYAML,
				InitConfiguration:    initConfigYAML,
			})
			if err != nil {
				return nil, err
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
				kubeadm.SetNodeRegistration(
					&ctx.MachineConfig.KubeadmConfiguration.Join.NodeRegistration,
					kubeadm.WithNodeRegistrationName(hostnameLookup),
					kubeadm.WithCRISocket(containerdSocket),
					kubeadm.WithKubeletExtraArgs(map[string]string{
						"cloud-provider": cloudProvider,
					}),
					kubeadm.WithTaints(ctx.Machine.Spec.Taints),
					kubeadm.WithKubeletExtraArgs(map[string]string{"node-labels": nodeRole}),
				),
			),
		)
		joinConfigurationYAML, err := kubeadm.ConfigurationToYAML(&ctx.MachineConfig.KubeadmConfiguration.Join)
		if err != nil {
			return nil, err
		}

		userData, err := userdata.NewNode(&userdata.NodeInput{
			SSHAuthorizedKeys: ctx.ClusterConfig.SSHAuthorizedKeys,
			JoinConfiguration: joinConfigurationYAML,
		})
		if err != nil {
			return nil, err
		}

		userDataYAML = userData
	}

	return []byte(userDataYAML), nil
}
