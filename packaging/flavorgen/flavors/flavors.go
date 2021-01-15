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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
	cloudprovidersvc "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
	"sigs.k8s.io/cluster-api/api/v1alpha3"
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
func MultiNodeTemplateWithHAProxy() []runtime.Object {
	lb := newHAProxyLoadBalancer()
	vsphereCluster := newVSphereCluster(&lb)
	machineTemplate := newVSphereMachineTemplate()
	controlPlane := newKubeadmControlplane(444, machineTemplate, []bootstrapv1.File{})
	kubeadmJoinTemplate := newKubeadmConfigTemplate()
	cluster := newCluster(vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, machineTemplate, kubeadmJoinTemplate)
	return []runtime.Object{
		&cluster,
		&lb,
		&vsphereCluster,
		&machineTemplate,
		&controlPlane,
		&kubeadmJoinTemplate,
		&machineDeployment,
	}
}

func MultiNodeTemplateWithKubeVIP() []runtime.Object {
	vsphereCluster := newVSphereCluster(nil)
	machineTemplate := newVSphereMachineTemplate()
	controlPlane := newKubeadmControlplane(444, machineTemplate, newKubeVIPFiles())
	kubeadmJoinTemplate := newKubeadmConfigTemplate()
	cluster := newCluster(vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, machineTemplate, kubeadmJoinTemplate)
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResources := createCrsResourceObjects(&clusterResourceSet, vsphereCluster, cluster)

	// removing Storage config so the cluster controller is not going not install CSI (it is installed by the clusterResourceSet)
	vsphereCluster.Spec.CloudProviderConfiguration.ProviderConfig.Storage = nil

	MultiNodeTemplate := []runtime.Object{
		&cluster,
		&vsphereCluster,
		&machineTemplate,
		&controlPlane,
		&kubeadmJoinTemplate,
		&machineDeployment,
		&clusterResourceSet,
	}
	return append(MultiNodeTemplate, crsResources...)
}

// createCrsResourceObjects creates the api objects necessary for CSI to function. Also appends the resources to the CRS
func createCrsResourceObjects(crs *addonsv1alpha3.ClusterResourceSet, vsphereCluster infrav1.VSphereCluster, cluster v1alpha3.Cluster) []runtime.Object {
	serviceAccount := cloudprovidersvc.CSIControllerServiceAccount()
	serviceAccount.TypeMeta = v1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: corev1.SchemeGroupVersion.String(),
	}
	serviceAccountSecret := newSecret(serviceAccount.Name, serviceAccount)
	appendSecretToCrsResource(crs, serviceAccountSecret)

	clusterRole := cloudprovider.CSIControllerClusterRole()
	clusterRole.TypeMeta = v1.TypeMeta{
		Kind:       "ClusterRole",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	clusterRoleConfigMap := newConfigMap(clusterRole.Name, clusterRole)
	appendConfigMapToCrsResource(crs, clusterRoleConfigMap)

	clusterRoleBinding := cloudprovider.CSIControllerClusterRoleBinding()
	clusterRoleBinding.TypeMeta = v1.TypeMeta{
		Kind:       "ClusterRoleBinding",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	clusterRoleBindingConfigMap := newConfigMap(clusterRoleBinding.Name, clusterRoleBinding)
	appendConfigMapToCrsResource(crs, clusterRoleBindingConfigMap)

	cloudConfig, err := cloudprovidersvc.ConfigForCSI(vsphereCluster, cluster, vSphereUsername, vSpherePassword).MarshalINI()
	if err != nil {
		panic(errors.Errorf("invalid cloudConfig"))
	}
	// cloud config secret is wrapped in another secret so it could be injected via CRS
	cloudConfigSecret := cloudprovidersvc.CSICloudConfigSecret(string(cloudConfig))
	cloudConfigSecret.TypeMeta = v1.TypeMeta{
		Kind:       "Secret",
		APIVersion: corev1.SchemeGroupVersion.String(),
	}
	cloudConfigSecretWrapper := newSecret(cloudConfigSecret.Name, cloudConfigSecret)
	appendSecretToCrsResource(crs, cloudConfigSecretWrapper)

	csiDriver := cloudprovider.CSIDriver()
	csiDriver.TypeMeta = v1.TypeMeta{
		Kind:       "CSIDriver",
		APIVersion: storagev1.SchemeGroupVersion.String(),
	}
	csiDriverConfigMap := newConfigMap(csiDriver.Name, csiDriver)
	appendConfigMapToCrsResource(crs, csiDriverConfigMap)

	storageConfig := createStorageConfig()
	daemonSet := cloudprovidersvc.VSphereCSINodeDaemonSet(storageConfig)
	daemonSet.TypeMeta = v1.TypeMeta{
		Kind:       "DaemonSet",
		APIVersion: appsv1.SchemeGroupVersion.String(),
	}
	daemonSetConfigMap := newConfigMap(daemonSet.Name, daemonSet)
	appendConfigMapToCrsResource(crs, daemonSetConfigMap)

	deployment := cloudprovider.CSIControllerDeployment(storageConfig)
	deployment.TypeMeta = v1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: appsv1.SchemeGroupVersion.String(),
	}
	deploymentConfigMap := newConfigMap(deployment.Name, deployment)
	appendConfigMapToCrsResource(crs, deploymentConfigMap)

	configMap := cloudprovider.CSIFeatureStatesConfigMap()
	featureStateConfigMap := newConfigMap(configMap.Name, configMap)

	return []runtime.Object{
		serviceAccountSecret,
		clusterRoleConfigMap,
		clusterRoleBindingConfigMap,
		cloudConfigSecretWrapper,
		csiDriverConfigMap,
		daemonSetConfigMap,
		deploymentConfigMap,
		featureStateConfigMap,
	}
}

func MultiNodeTemplateWithExternalLoadBalancer() []runtime.Object {
	vsphereCluster := newVSphereCluster(nil)
	machineTemplate := newVSphereMachineTemplate()
	controlPlane := newKubeadmControlplane(444, machineTemplate, nil)
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
