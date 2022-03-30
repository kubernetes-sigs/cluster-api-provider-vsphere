/*
Copyright 2021 The Kubernetes Authors.

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

package crs

import (
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/crs/types"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
)

// CreateCrsResourceObjectsCSI creates the api objects necessary for CSI to function.
// Also appends the resources to the CRS.
func CreateCrsResourceObjectsCSI(crs *addonsv1.ClusterResourceSet) []runtime.Object {
	serviceAccount := cloudprovider.CSIControllerServiceAccount()
	serviceAccount.TypeMeta = metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: corev1.SchemeGroupVersion.String(),
	}
	serviceAccountSecret := newSecret(serviceAccount.Name, serviceAccount)
	appendSecretToCrsResource(crs, serviceAccountSecret)

	clusterRole := cloudprovider.CSIControllerClusterRole()
	clusterRole.TypeMeta = metav1.TypeMeta{
		Kind:       "ClusterRole",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	clusterRoleConfigMap := newConfigMap(clusterRole.Name, clusterRole)
	appendConfigMapToCrsResource(crs, clusterRoleConfigMap)

	clusterRoleBinding := cloudprovider.CSIControllerClusterRoleBinding()
	clusterRoleBinding.TypeMeta = metav1.TypeMeta{
		Kind:       "ClusterRoleBinding",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	clusterRoleBindingConfigMap := newConfigMap(clusterRoleBinding.Name, clusterRoleBinding)
	appendConfigMapToCrsResource(crs, clusterRoleBindingConfigMap)

	cloudConfig, err := ConfigForCSI().MarshalINI()
	if err != nil {
		panic(errors.Errorf("invalid cloudConfig"))
	}
	// cloud config secret is wrapped in another secret so it could be injected via CRS
	cloudConfigSecret := cloudprovider.CSICloudConfigSecret(string(cloudConfig))
	cloudConfigSecret.TypeMeta = metav1.TypeMeta{
		Kind:       "Secret",
		APIVersion: corev1.SchemeGroupVersion.String(),
	}
	cloudConfigSecretWrapper := newSecret(cloudConfigSecret.Name, cloudConfigSecret)
	appendSecretToCrsResource(crs, cloudConfigSecretWrapper)

	csiDriver := cloudprovider.CSIDriver()
	csiDriver.TypeMeta = metav1.TypeMeta{
		Kind:       "CSIDriver",
		APIVersion: storagev1.SchemeGroupVersion.String(),
	}
	csiDriverConfigMap := newConfigMap(csiDriver.Name, csiDriver)
	appendConfigMapToCrsResource(crs, csiDriverConfigMap)

	storageConfig := createStorageConfig()
	daemonSet := cloudprovider.VSphereCSINodeDaemonSet(storageConfig)
	daemonSet.TypeMeta = metav1.TypeMeta{
		Kind:       "DaemonSet",
		APIVersion: appsv1.SchemeGroupVersion.String(),
	}
	daemonSetConfigMap := newConfigMap(daemonSet.Name, daemonSet)
	appendConfigMapToCrsResource(crs, daemonSetConfigMap)

	deployment := cloudprovider.CSIControllerDeployment(storageConfig)
	deployment.TypeMeta = metav1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: appsv1.SchemeGroupVersion.String(),
	}
	deploymentConfigMap := newConfigMap(deployment.Name, deployment)
	appendConfigMapToCrsResource(crs, deploymentConfigMap)

	return []runtime.Object{
		serviceAccountSecret,
		clusterRoleConfigMap,
		clusterRoleBindingConfigMap,
		cloudConfigSecretWrapper,
		csiDriverConfigMap,
		daemonSetConfigMap,
		deploymentConfigMap,
	}
}

// createStorageConfig to be used by tkg template.
func createStorageConfig() *types.CPIStorageConfig {
	return &types.CPIStorageConfig{
		ControllerImage:     cloudprovider.DefaultCSIControllerImage,
		NodeDriverImage:     cloudprovider.DefaultCSINodeDriverImage,
		AttacherImage:       cloudprovider.DefaultCSIAttacherImage,
		ProvisionerImage:    cloudprovider.DefaultCSIProvisionerImage,
		MetadataSyncerImage: cloudprovider.DefaultCSIMetadataSyncerImage,
		LivenessProbeImage:  cloudprovider.DefaultCSILivenessProbeImage,
		RegistrarImage:      cloudprovider.DefaultCSIRegistrarImage,
	}
}

// ConfigForCSI returns a cloudprovider.CPIConfig specific to the vSphere CSI driver until
// it supports using Secrets for vCenter credentials.
func ConfigForCSI() *types.CPIConfig {
	config := &types.CPIConfig{}

	config.Global.ClusterID = fmt.Sprintf("%s/%s", env.NamespaceVar, env.ClusterNameVar)
	config.Network.Name = env.VSphereNetworkVar

	config.VCenter = map[string]types.CPIVCenterConfig{
		env.VSphereServerVar: {
			Username:    env.VSphereUsername,
			Password:    env.VSpherePassword,
			Datacenters: env.VSphereDataCenterVar,
		},
	}

	return config
}
