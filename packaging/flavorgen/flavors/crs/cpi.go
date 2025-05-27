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

// Package crs contains tools to create a ClusterResourceSet for the CPI.
package crs

import (
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonsv1 "sigs.k8s.io/cluster-api/api/addons/v1beta2"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/cloudprovider"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
)

func addCPILabels(o metav1.Object, cpiInfraLabelValue string) {
	labels := map[string]string{}
	if o.GetLabels() != nil {
		labels = o.GetLabels()
	}
	labels["vsphere-cpi-infra"] = cpiInfraLabelValue
	labels["component"] = "cloud-controller-manager"
	o.SetLabels(labels)
}

// CreateCrsResourceObjectsCPI creates the api objects necessary for CSI to function.
// Also appends the resources to the CRS.
func CreateCrsResourceObjectsCPI(crs *addonsv1.ClusterResourceSet) []runtime.Object {
	credentials := map[string]string{}
	credentials[fmt.Sprintf("%s.username", env.VSphereServerVar)] = env.VSphereUsername
	credentials[fmt.Sprintf("%s.password", env.VSphereServerVar)] = env.VSpherePassword
	cpiSecret := cpiCredentials(credentials)
	addCPILabels(cpiSecret, "secret")
	cpiSecretWrapper := newSecret(cpiSecret.Name, cpiSecret)
	appendSecretToCrsResource(crs, cpiSecretWrapper)

	cpiManifests, err := cloudprovider.CloudControllerManagerManifests()
	if err != nil {
		panic(errors.Wrapf(err, "creating cloudcontrollermanager manifests"))
	}

	cpiObjects := []runtime.Object{}

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

	manifestsCm := newConfigMapManifests("cpi-manifests", cpiObjects)
	manifestsCm.Data["data"] = cpiManifests + "---\n" + manifestsCm.Data["data"]

	appendConfigMapToCrsResource(crs, manifestsCm)
	// Define the kubeconfig secret for the target cluster.

	return []runtime.Object{
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
			Name:      "cloud-provider-vsphere-credentials", // NOTE: this name is used in E2E tests.
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: credentials,
	}
}
