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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
)

// CreateCrsResourceObjectsCPI creates the api objects necessary for CSI to function.
// Also appends the resources to the CRS.
func CreateCrsResourceObjectsCPI(crs *addonsv1.ClusterResourceSet) []runtime.Object {
	serviceAccount := cloudprovider.CloudControllerManagerServiceAccount()
	serviceAccount.TypeMeta = metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: corev1.SchemeGroupVersion.String(),
	}
	serviceAccountSecret := newSecret(serviceAccount.Name, serviceAccount)
	appendSecretToCrsResource(crs, serviceAccountSecret)

	credentials := map[string]string{}
	credentials[fmt.Sprintf("%s.username", env.VSphereServerVar)] = env.VSphereUsername
	credentials[fmt.Sprintf("%s.password", env.VSphereServerVar)] = env.VSpherePassword
	cpiSecret := cpiCredentials(credentials)
	cpiSecretWrapper := newSecret(cpiSecret.Name, cpiSecret)
	appendSecretToCrsResource(crs, cpiSecretWrapper)

	cpiObjects := []runtime.Object{}
	clusterRole := cloudprovider.CloudControllerManagerClusterRole()
	clusterRole.TypeMeta = metav1.TypeMeta{
		Kind:       "ClusterRole",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, clusterRole)

	clusterRoleBinding := cloudprovider.CloudControllerManagerClusterRoleBinding()
	clusterRoleBinding.TypeMeta = metav1.TypeMeta{
		Kind:       "ClusterRoleBinding",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, clusterRoleBinding)

	cloudConfig, err := CPIConfigString()
	if err != nil {
		panic(errors.Errorf("invalid cloudConfig"))
	}
	// cloud config secret is wrapped in another secret so it could be injected via CRS
	cloudConfigConfigMap := cloudprovider.CloudControllerManagerConfigMap(cloudConfig)
	cloudConfigConfigMap.TypeMeta = metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: corev1.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, cloudConfigConfigMap)

	roleBinding := cloudprovider.CloudControllerManagerRoleBinding()
	roleBinding.TypeMeta = metav1.TypeMeta{
		Kind:       "RoleBinding",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, roleBinding)

	cpiService := cloudprovider.CloudControllerManagerService()
	cpiService.TypeMeta = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: corev1.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, cpiService)

	extraArgs := []string{
		"--v=2",
		"--cloud-provider=vsphere",
		"--cloud-config=/etc/cloud/vsphere.conf",
	}
	cpiDaemonSet := cloudprovider.CloudControllerManagerDaemonSet(cloudprovider.DefaultCPIControllerImage, extraArgs)
	cpiDaemonSet.TypeMeta = metav1.TypeMeta{
		Kind:       "DaemonSet",
		APIVersion: appsv1.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, cpiDaemonSet)

	manifestsCm := newConfigMapManifests("cpi-manifests", cpiObjects)
	appendConfigMapToCrsResource(crs, manifestsCm)
	// Define the kubeconfig secret for the target cluster.

	return []runtime.Object{
		serviceAccountSecret,
		cpiSecretWrapper,
		manifestsCm,
	}
}

func CPIConfigString() (string, error) {
	cpiConfig, err := newCPIConfig()
	if err != nil {
		return "", err
	}

	return string(cpiConfig), nil
}

func cpiCredentials(credentials map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "cloud-provider-vsphere-credentials",
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: credentials,
	}
}
