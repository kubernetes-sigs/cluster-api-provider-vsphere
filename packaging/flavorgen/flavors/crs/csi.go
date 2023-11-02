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
	"path"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/yaml"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/cloudprovider"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/crs/types"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
)

// CreateCrsResourceObjectsCSI creates the api objects necessary for CSI to function.
// Also appends the resources to the CRS.
func CreateCrsResourceObjectsCSI(crs *addonsv1.ClusterResourceSet) ([]runtime.Object, error) {
	// Load kustomization.yaml from disk
	fSys := filesys.MakeFsInMemory()
	entries, err := cloudprovider.CSIKustomizationTemplates.ReadDir("csi")
	if err != nil {
		return nil, err
	}

	for i := range entries {
		fileBuf, err := cloudprovider.CSIKustomizationTemplates.ReadFile(path.Join("csi", entries[i].Name()))
		if err != nil {
			return nil, err
		}

		err = fSys.WriteFile(entries[i].Name(), fileBuf)
		if err != nil {
			return nil, err
		}
	}

	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	resources, err := k.Run(fSys, "/")
	if err != nil {
		return nil, err
	}

	// Get YAML string
	yamlString, err := resources.AsYaml()
	if err != nil {
		return nil, err
	}

	resourceObjs, err := yaml.ToUnstructured(yamlString)
	if err != nil {
		return nil, err
	}

	objs := make([]runtime.Object, 0, len(resourceObjs))
	for i := range resourceObjs {
		obj := resourceObjs[i]
		objs = append(objs, &obj)
	}

	cloudConfig, err := ConfigForCSI().MarshalINI()
	if err != nil {
		return nil, errors.Wrapf(err, "invalid cloudConfig")
	}

	// cloud config secret is wrapped in another secret so it could be injected via CRS
	cloudConfigSecret := cloudprovider.CSICloudConfigSecret(string(cloudConfig))
	cloudConfigSecretWrapper := newSecret(cloudConfigSecret.Name, cloudConfigSecret)
	appendSecretToCrsResource(crs, cloudConfigSecretWrapper)

	manifestConfigMap := newConfigMapManifests("csi-manifests", objs)
	appendConfigMapToCrsResource(crs, manifestConfigMap)

	return []runtime.Object{
		cloudConfigSecretWrapper,
		manifestConfigMap,
	}, nil
}

// ConfigForCSI returns a cloudprovider.CPIConfig specific to the vSphere CSI driver until
// it supports using Secrets for vCenter credentials.
func ConfigForCSI() *types.CPIConfig {
	config := &types.CPIConfig{}

	config.Global.ClusterID = fmt.Sprintf("%s/%s", env.NamespaceVar, env.ClusterNameVar)
	config.Global.Thumbprint = env.VSphereThumbprint
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
