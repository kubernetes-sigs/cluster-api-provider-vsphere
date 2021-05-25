package crs

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
	addonsv1alpha4 "sigs.k8s.io/cluster-api/exp/addons/api/v1alpha4"
)

func newSecret(name string, o runtime.Object) *v1.Secret {
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: env.NamespaceVar,
		},
		StringData: map[string]string{
			"data": util.GenerateObjectYAML(o, []util.Replacement{}),
		},
		Type: addonsv1alpha4.ClusterResourceSetSecretType,
	}
}

func appendSecretToCrsResource(crs *addonsv1alpha4.ClusterResourceSet, generatedSecret *v1.Secret) {
	crs.Spec.Resources = append(crs.Spec.Resources, addonsv1alpha4.ResourceRef{
		Name: generatedSecret.Name,
		Kind: "Secret",
	})
}

func newConfigMap(name string, o runtime.Object) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: env.NamespaceVar,
		},
		Data: map[string]string{
			"data": util.GenerateObjectYAML(o, []util.Replacement{}),
		},
	}
}
func newConfigMapManifests(name string, o []runtime.Object) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: env.NamespaceVar,
		},
		Data: map[string]string{
			"data": util.GenerateManifestYaml(o),
		},
	}
}

func appendConfigMapToCrsResource(crs *addonsv1alpha4.ClusterResourceSet, generatedConfigMap *v1.ConfigMap) {
	crs.Spec.Resources = append(crs.Spec.Resources, addonsv1alpha4.ResourceRef{
		Name: generatedConfigMap.Name,
		Kind: "ConfigMap",
	})
}

func newCPIConfig() *infrav1.CPIConfig {
	return &infrav1.CPIConfig{
		Global: infrav1.CPIGlobalConfig{
			SecretName:      "cloud-provider-vsphere-credentials",
			SecretNamespace: metav1.NamespaceSystem,
			Thumbprint:      env.VSphereThumbprint,
		},
		VCenter: map[string]infrav1.CPIVCenterConfig{
			env.VSphereServerVar: {
				Datacenters: env.VSphereDataCenterVar,
				Thumbprint:  env.VSphereThumbprint,
			},
		},
		Network: infrav1.CPINetworkConfig{
			Name: env.VSphereNetworkVar,
		},
		Workspace: infrav1.CPIWorkspaceConfig{
			Server:       env.VSphereServerVar,
			Datacenter:   env.VSphereDataCenterVar,
			Datastore:    env.VSphereDatastoreVar,
			ResourcePool: env.VSphereResourcePoolVar,
			Folder:       env.VSphereFolderVar,
		},
	}
}
