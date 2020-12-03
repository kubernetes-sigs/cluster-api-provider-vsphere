package crs

import (
	"fmt"

	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
	cloudprovidersvc "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/cloudprovider"
	addonsv1alpha4 "sigs.k8s.io/cluster-api/exp/addons/api/v1alpha4"
)

// CreateCrsResourceObjectsCPI creates the api objects necessary for CSI to function. Also appends the resources to the CRS
func CreateCrsResourceObjectsCPI(crs *addonsv1alpha4.ClusterResourceSet) []runtime.Object {
	serviceAccount := cloudprovidersvc.CloudControllerManagerServiceAccount()
	serviceAccount.TypeMeta = v1.TypeMeta{
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
	clusterRole.TypeMeta = v1.TypeMeta{
		Kind:       "ClusterRole",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, clusterRole)

	clusterRoleBinding := cloudprovider.CloudControllerManagerClusterRoleBinding()
	clusterRoleBinding.TypeMeta = v1.TypeMeta{
		Kind:       "ClusterRoleBinding",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, clusterRoleBinding)

	cloudConfig, err := CPIConfigString()
	if err != nil {
		panic(errors.Errorf("invalid cloudConfig"))
	}
	// cloud config secret is wrapped in another secret so it could be injected via CRS
	cloudConfigConfigMap := cloudprovidersvc.CloudControllerManagerConfigMap(cloudConfig)
	cloudConfigConfigMap.TypeMeta = v1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: corev1.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, cloudConfigConfigMap)

	roleBinding := cloudprovider.CloudControllerManagerRoleBinding()
	roleBinding.TypeMeta = v1.TypeMeta{
		Kind:       "RoleBinding",
		APIVersion: rbac.SchemeGroupVersion.String(),
	}
	cpiObjects = append(cpiObjects, roleBinding)

	cpiService := cloudprovider.CloudControllerManagerService()
	cpiService.TypeMeta = v1.TypeMeta{
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
	cpiDaemonSet.TypeMeta = v1.TypeMeta{
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
	cpiConfig := newCPIConfig()

	cpiConfigString, err := cpiConfig.MarshalINI()
	if err != nil {
		return "", err
	}

	return string(cpiConfigString), nil
}

func cpiCredentials(credentials map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: v1.TypeMeta{
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
