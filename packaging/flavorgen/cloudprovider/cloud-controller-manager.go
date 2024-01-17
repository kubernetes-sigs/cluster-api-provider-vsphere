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

package cloudprovider

import (
	"errors"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
)

// NOTE: the contents of this file are derived from https://github.com/kubernetes/cloud-provider-vsphere/tree/master/manifests/controller-manager

const (
	CPIControllerImageURL = "gcr.io/cloud-provider-vsphere/cpi/release/manager"
)

// CloudControllerManagerManifests returns the yaml manifests generated via
// `helm template` from https://github.com/kubernetes/cloud-provider-vsphere/tree/master/charts/vsphere-cpi
func CloudControllerManagerManifests() (string, error) {
	objectList := []string{}

	// Replace the hardcoded image tag by a variable for clusterctl. This way we can dynamically configure the version of CPI.
	imageTagRemoval := regexp.MustCompile(`(image: .*):.*`)
	withoutTag := imageTagRemoval.ReplaceAll([]byte(cpiManifests), []byte(`${1}:$`+env.CPIImageKubernetesVersionVar))

	// ClusterResourceSet does not support `kind: List`, we have to adjust the manifests
	// to remove those lists and directly add the items instead.
	for _, object := range strings.Split(string(withoutTag), "---\n") {
		// Ignore empty strings
		if object == "" {
			continue
		}
		// Marshal to unstructured so we can check the type
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(object), u); err != nil {
			return "", err
		}
		// If it is not a List we can directly add the object to the slice
		if u.GetKind() != "List" {
			objectList = append(objectList, object)
			continue
		}

		// If it is a list we extract the slice at `items`
		items, isSlice, err := unstructured.NestedSlice(u.UnstructuredContent(), "items")
		if err != nil {
			return "", err
		}
		if !isSlice {
			return "", errors.New("expected List.items to be a slice")
		}

		// Loop over all items, marshal them and add it to objects.
		for _, item := range items {
			marshaledItem, err := yaml.Marshal(item)
			if err != nil {
				return "", err
			}
			objectList = append(objectList, string(marshaledItem))
		}
	}

	return "---\n" + strings.Join(objectList, "---\n"), nil
}

// CloudControllerManagerConfigMap returns a ConfigMap containing data for the cloud config file.
func CloudControllerManagerConfigMap(cloudConfig string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloud-config",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"vsphere.conf": cloudConfig,
		},
	}
}
