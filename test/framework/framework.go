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

// Package framework implements utils for CAPV e2e tests.
package framework

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/yaml"
)

type ProviderConfig clusterctl.ProviderConfig

// Util functions to interact with the clusterctl e2e framework.

type configOverrides struct {
	Variables map[string]string   `json:"variables,omitempty"`
	Intervals map[string][]string `json:"intervals,omitempty"`
}

func LoadE2EConfig(ctx context.Context, configPath string, configOverridesPath, testTarget, testMode string) (*clusterctl.E2EConfig, error) {
	config := clusterctl.LoadE2EConfig(ctx, clusterctl.LoadE2EConfigInput{ConfigPath: configPath})
	if config == nil {
		return nil, fmt.Errorf("cannot load E2E config found at %s", configPath)
	}

	// If defined, load configOverrides.
	// This can be used e.g. when working with a custom vCenter server for local testing (instead of the one in VMC used in CI).
	if configOverridesPath != "" {
		Expect(configOverridesPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config-overrides should be an existing file.")

		Byf("Merging with e2e config overrides from %q", configOverridesPath)
		configData, err := os.ReadFile(configOverridesPath) //nolint:gosec
		Expect(err).ToNot(HaveOccurred(), "Failed to read e2e config overrides")
		Expect(configData).ToNot(BeEmpty(), "The e2e config overrides should not be empty")

		configOverrides := &configOverrides{}
		Expect(yaml.Unmarshal(configData, configOverrides)).To(Succeed(), "Failed to convert e2e config overrides to yaml")

		for k, v := range configOverrides.Variables {
			config.Variables[k] = v
		}
		for k, v := range configOverrides.Intervals {
			config.Intervals[k] = v
		}
	}

	if testTarget == "vcenter" {
		// In case we are not testing vcsim, then drop the vcsim controller from providers and images.
		// This ensures that all the tests not yet allowing to explicitly set vsphere as target infra provider keep working.
		Byf("Dropping vcsim provider from the e2e config")
		for i := range config.Providers {
			if config.Providers[i].Name == "vcsim" {
				config.Providers = append(config.Providers[:i], config.Providers[i+1:]...)
				break
			}
		}

		for i := range config.Images {
			if strings.Contains(config.Images[i].Name, "cluster-api-vcsim-controller") {
				config.Images = append(config.Images[:i], config.Images[i+1:]...)
				break
			}
		}
	} else {
		// In case we are testing with vcsim, then drop the in-cluster ipam provider from providers and images.
		Byf("Dropping in-cluster provider from the e2e config")
		for i := range config.Providers {
			if config.Providers[i].Name == "in-cluster" {
				config.Providers = append(config.Providers[:i], config.Providers[i+1:]...)
				break
			}
		}

		for i := range config.Images {
			if strings.Contains(config.Images[i].Name, "cluster-api-ipam-in-cluster-controller") {
				config.Images = append(config.Images[:i], config.Images[i+1:]...)
				break
			}
		}
	}

	if testMode == "govmomi" {
		// In case we are not testing supervisor, then drop the vm-operator controller from providers and images.
		Byf("Dropping vm-operator from the e2e config")
		for i := range config.Providers {
			if config.Providers[i].Name == "vm-operator" {
				config.Providers = append(config.Providers[:i], config.Providers[i+1:]...)
				break
			}
		}

		for i := range config.Images {
			if strings.Contains(config.Images[i].Name, "vm-operator") {
				config.Images = append(config.Images[:i], config.Images[i+1:]...)
				break
			}
		}

		Byf("Dropping net-operator from the e2e config")
		for i := range config.Providers {
			if config.Providers[i].Name == "net-operator" {
				config.Providers = append(config.Providers[:i], config.Providers[i+1:]...)
				break
			}
		}

		for i := range config.Images {
			if strings.Contains(config.Images[i].Name, "net-operator") {
				config.Images = append(config.Images[:i], config.Images[i+1:]...)
				break
			}
		}
	} else {
		// In case we are testing supervisor, change the folder we build manifest from
		Byf("Overriding source folder for vsphere provider to /config/supervisor in the e2e config")
		for i := range config.Providers {
			if config.Providers[i].Name == "vsphere" {
				// Replace relativ path for latest version.
				config.Providers[i].Versions[0].Value = strings.ReplaceAll(config.Providers[i].Versions[0].Value, "/config/default", "/config/supervisor")
				// Replace target file in github.
				for j, version := range config.Providers[i].Versions {
					if strings.HasSuffix(version.Value, "infrastructure-components.yaml") {
						version.Value = fmt.Sprintf("%s-supervisor.yaml", strings.TrimSuffix(version.Value, ".yaml"))
						config.Providers[i].Versions[j] = version
					}
				}
				break
			}
		}
	}

	return config, nil
}

func CreateClusterctlLocalRepository(ctx context.Context, config *clusterctl.E2EConfig, repositoryFolder string, cniEnabled bool) (string, error) {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}
	if cniEnabled {
		// Ensuring a CNI file is defined in the config and register a FileTransformation to inject the referenced file as in place of the CNI_RESOURCES envSubst variable.
		cniPath, ok := config.Variables[capi_e2e.CNIPath]
		if !ok {
			return "", fmt.Errorf("missing %s variable in the config", capi_e2e.CNIPath)
		}

		if _, err := os.Stat(cniPath); err != nil {
			return "", fmt.Errorf("the %s variable should resolve to an existing file", capi_e2e.CNIPath)
		}
		createRepositoryInput.RegisterClusterResourceSetConfigMapTransformation(cniPath, capi_e2e.CNIResources)
	}

	clusterctlConfig := clusterctl.CreateRepository(ctx, createRepositoryInput)
	if _, err := os.Stat(clusterctlConfig); err != nil {
		return "", fmt.Errorf("the clusterctl config file does not exists in the local repository %s", repositoryFolder)
	}
	return clusterctlConfig, nil
}

func SetupBootstrapCluster(ctx context.Context, config *clusterctl.E2EConfig, scheme *runtime.Scheme, useExistingCluster bool) (bootstrap.ClusterProvider, framework.ClusterProxy, error) {
	var clusterProvider bootstrap.ClusterProvider
	kubeconfigPath := ""
	if !useExistingCluster {
		clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               config.ManagementClusterName,
			RequiresDockerSock: config.HasDockerProvider(),
			KubernetesVersion:  config.MustGetVariable(capi_e2e.KubernetesVersionManagement),
			Images:             config.Images,
		})

		kubeconfigPath = clusterProvider.GetKubeconfigPath()
		if _, err := os.Stat(kubeconfigPath); err != nil {
			return nil, nil, errors.New("failed to get the kubeconfig file for the bootstrap cluster")
		}
	}

	clusterProxy := framework.NewClusterProxy("bootstrap", kubeconfigPath, scheme)

	return clusterProvider, clusterProxy, nil
}

func InitBootstrapCluster(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig, clusterctlConfig, artifactFolder string) {
	clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:              bootstrapClusterProxy,
		ClusterctlConfigPath:      clusterctlConfig,
		InfrastructureProviders:   config.InfrastructureProviders(),
		LogFolder:                 filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
		IPAMProviders:             config.IPAMProviders(),
		RuntimeExtensionProviders: config.RuntimeExtensionProviders(),
	}, config.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
}

func TearDown(ctx context.Context, bootstrapClusterProvider bootstrap.ClusterProvider, bootstrapClusterProxy framework.ClusterProxy) {
	if bootstrapClusterProxy != nil {
		bootstrapClusterProxy.Dispose(ctx)
	}
	if bootstrapClusterProvider != nil {
		bootstrapClusterProvider.Dispose(ctx)
	}
}
