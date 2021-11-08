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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
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
		Type: addonsv1.ClusterResourceSetSecretType,
	}
}

func appendSecretToCrsResource(crs *addonsv1.ClusterResourceSet, generatedSecret *v1.Secret) {
	crs.Spec.Resources = append(crs.Spec.Resources, addonsv1.ResourceRef{
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

func appendConfigMapToCrsResource(crs *addonsv1.ClusterResourceSet, generatedConfigMap *v1.ConfigMap) {
	crs.Spec.Resources = append(crs.Spec.Resources, addonsv1.ResourceRef{
		Name: generatedConfigMap.Name,
		Kind: "ConfigMap",
	})
}

func newCPIConfig() ([]byte, error) {
	config := map[string]interface{}{
		"global": map[string]interface{}{
			"secretName":      "cloud-provider-vsphere-credentials",
			"secretNamespace": metav1.NamespaceSystem,
			"thumbprint":      env.VSphereThumbprint,
		},
		"vcenter": map[string]interface{}{
			env.VSphereServerVar: map[string]interface{}{
				"server":          env.VSphereServerVar,
				"datacenters":     []string{env.VSphereDataCenterVar},
				"thumbprint":      env.VSphereThumbprint,
				"secretName":      "cloud-provider-vsphere-credentials",
				"secretNamespace": metav1.NamespaceSystem,
			},
		},
	}
	configBytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	return configBytes, nil
}
