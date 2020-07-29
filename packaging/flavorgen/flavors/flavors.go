/*
Copyright 2020 The Kubernetes Authors.

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

package flavors

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
	cloudprovidersvc "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	addonsv1alpha3 "sigs.k8s.io/cluster-api/exp/addons/api/v1alpha3"
)

// create StorageConfig to be used by tkg template
func createStorageConfig() *infrav1.CPIStorageConfig {
	return &infrav1.CPIStorageConfig{
		ControllerImage:     cloudprovidersvc.DefaultCSIControllerImage,
		NodeDriverImage:     cloudprovidersvc.DefaultCSINodeDriverImage,
		AttacherImage:       cloudprovidersvc.DefaultCSIAttacherImage,
		ProvisionerImage:    cloudprovidersvc.DefaultCSIProvisionerImage,
		MetadataSyncerImage: cloudprovidersvc.DefaultCSIMetadataSyncerImage,
		LivenessProbeImage:  cloudprovidersvc.DefaultCSILivenessProbeImage,
		RegistrarImage:      cloudprovidersvc.DefaultCSIRegistrarImage,
	}
}
func addGeneratedSecretToCRS(clusterresourceSet *addonsv1alpha3.ClusterResourceSet, clusterResourceSetBinding *addonsv1alpha3.ClusterResourceSetBinding) {
	serviceAccount := cloudprovidersvc.CSIControllerServiceAccount()
	serviceAccountMarshalled, err := serviceAccount.Marshal()
	if err != nil {
		panic(errors.Errorf("invalid serviceAccount"))
	}
	// generate secret for above types

	serviceAccountSecret := cloudprovidersvc.CSIComponentConfigSecret(serviceAccount.Name, string(serviceAccountMarshalled))
	//serviceAccountSecret.SetGroupVersionKind("Secret")
	appendResourceSecretToCRS(clusterresourceSet, serviceAccountSecret)
	// add to binding
	appendResourceSetToBinding(clusterResourceSetBinding, serviceAccountSecret, clusterresourceSet)

	clusterRole := cloudprovidersvc.CSIControllerClusterRole()
	clusterRoleMarshalled, err := clusterRole.Marshal()

	if err != nil {
		panic(errors.Errorf("invalid clusterRole"))
	}
	clusterRoleSecret := cloudprovidersvc.CSIComponentConfigSecret(clusterRole.Name, string(clusterRoleMarshalled))
	appendResourceSecretToCRS(clusterresourceSet, clusterRoleSecret)
	// add to bining
	appendResourceSetToBinding(clusterResourceSetBinding, clusterRoleSecret, clusterresourceSet)

	clusterRoleBinding := cloudprovidersvc.CSIControllerClusterRoleBinding()
	clusterRoleBindingMarshalled, err := clusterRoleBinding.Marshal()
	if err != nil {
		panic(errors.Errorf("invalid clusterRoleBinding"))
	}
	clusterRoleBindingSecret := cloudprovidersvc.CSIComponentConfigSecret(clusterRoleBinding.Name, string(clusterRoleBindingMarshalled))
	appendResourceSecretToCRS(clusterresourceSet, clusterRoleBindingSecret)
	// add to bining
	appendResourceSetToBinding(clusterResourceSetBinding, clusterRoleBindingSecret, clusterresourceSet)

	csiDriver := cloudprovidersvc.CSIDriver()
	csiDriverMarshalled, err := csiDriver.Marshal()
	if err != nil {
		panic(errors.Errorf("invalid csiDriver"))
	}
	csiDriverSecret := cloudprovidersvc.CSIComponentConfigSecret(csiDriver.Name, string(csiDriverMarshalled))
	appendResourceSecretToCRS(clusterresourceSet, csiDriverSecret)
	// add to bining
	appendResourceSetToBinding(clusterResourceSetBinding, csiDriverSecret, clusterresourceSet)

	storageConfig := createStorageConfig()
	daemonSet := cloudprovidersvc.VSphereCSINodeDaemonSet(storageConfig)
	daemonSetMarshalled, err := cloudprovidersvc.VSphereCSINodeDaemonSet(storageConfig).Marshal()
	if err != nil {
		panic(errors.Errorf("invalid daemonSet"))
	}
	daemonSetSecret := cloudprovidersvc.CSIComponentConfigSecret(daemonSet.Name, string(daemonSetMarshalled))
	appendResourceSecretToCRS(clusterresourceSet, daemonSetSecret)
	// add to bining
	appendResourceSetToBinding(clusterResourceSetBinding, daemonSetSecret, clusterresourceSet)

	deployment := cloudprovider.CSIControllerDeployment(storageConfig)
	deploymentMarshalled, err := deployment.Marshal()
	if err != nil {
		panic(errors.Errorf("invalid deployment"))
	}
	deploymentSecret := cloudprovider.CSIComponentConfigSecret(deployment.Name, string(deploymentMarshalled))
	appendResourceSecretToCRS(clusterresourceSet, deploymentSecret)
	// add to bining
	appendResourceSetToBinding(clusterResourceSetBinding, deploymentSecret, clusterresourceSet)

}
func MultiNodeTemplateWithHAProxy() []runtime.Object {

	var MultiNodeTemplate []runtime.Object

	lb := newHAProxyLoadBalancer()
	vsphereCluster := newVSphereCluster(&lb)
	machineTemplate := newVSphereMachineTemplate()
	controlPlane := newKubeadmControlplane(444, machineTemplate, []bootstrapv1.File{})
	kubeadmJoinTemplate := newKubeadmConfigTemplate()
	cluster := newCluster(vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, machineTemplate, kubeadmJoinTemplate)

	cloudConfig, err := cloudprovidersvc.ConfigForCSI(vsphereCluster, cluster).MarshalINI()
	if err != nil {
		panic(errors.Errorf("invalid cloudConfig"))
	}
	cloudConfigSecret := cloudprovidersvc.CSICloudConfigSecret(string(cloudConfig))

	// create ClusterResouceSet that contains cloudConfigSecret
	clusterresourceSet := newClusterResourceSet(cluster, cloudConfigSecret)
	clusterResourceSetBinding := newClusterResourceSetBinding(&cluster, cloudConfigSecret, &clusterresourceSet)
	// add geenrated secret, and cloudConfigSecret to the crs and CRSBinding
	addGeneratedSecretToCRS(&clusterresourceSet, &clusterResourceSetBinding)

	MultiNodeTemplate = []runtime.Object{
		&cluster,
		&lb,
		&vsphereCluster,
		&machineTemplate,
		&controlPlane,
		&kubeadmJoinTemplate,
		&machineDeployment,
		&clusterresourceSet,
		&clusterResourceSetBinding,
	}
	return MultiNodeTemplate

}

func MultiNodeTemplateWithKubeVIP() []runtime.Object {
	vsphereCluster := newVSphereCluster(nil)
	machineTemplate := newVSphereMachineTemplate()
	controlPlane := newKubeadmControlplane(444, machineTemplate, newKubeVIPFiles())
	kubeadmJoinTemplate := newKubeadmConfigTemplate()
	cluster := newCluster(vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, machineTemplate, kubeadmJoinTemplate)
	return []runtime.Object{
		&cluster,
		&vsphereCluster,
		&machineTemplate,
		&controlPlane,
		&kubeadmJoinTemplate,
		&machineDeployment,
	}
}
