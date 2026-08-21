/*
Copyright The Kubernetes Authors.

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

package plugins

import (
	"context"
	"fmt"
	"slices"

	pkgerrors "github.com/pkg/errors"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/testctl/core"
)

const (
	machineDeploymentSelectorPluginKey = "select.machineDeployment.testctl"

	// MachineDeploymentSelectorObjectKey is the key used to identify the MachineDeployment object in TestObjects.
	MachineDeploymentSelectorObjectKey = "MachineDeployment"
)

// MachineDeploymentSelectorPlugin can be used to select MachineDeployments for a Cluster.
type MachineDeploymentSelectorPlugin struct{}

var _ core.SelectorPlugin = &MachineDeploymentSelectorPlugin{}
var _ core.ConfigurablePlugin = &MachineDeploymentSelectorPlugin{}
var _ core.MessageGeneratorPlugin = &MachineDeploymentSelectorPlugin{}
var _ core.CallStackValidatorPlugin = &MachineDeploymentSelectorPlugin{}

// MachineDeploymentSelectorPluginConfig defines the config for the MachineDeploymentSelectorPlugin.
type MachineDeploymentSelectorPluginConfig struct {
	// TopologyNameRegex defines a regex used to select among the list of MachineDeployments which are candidate.
	// If TopologyNameRegex is not set, all candidate MachineDeployments will be considered.
	TopologyNameRegex string `json:"topologyNameRegex,omitempty"`
}

func init() {
	registerPlugin(machineDeploymentSelectorPluginKey, &MachineDeploymentSelectorPlugin{})
}

// Select selects the TestObjects to apply the test to, in this case a Cluster's MachineDeployments.
func (p *MachineDeploymentSelectorPlugin) Select(ctx context.Context, c client.Client, objects core.TestObjects, pluginConfigUntyped any, runConfig core.RunConfig) ([]core.TestObjects, error) {
	config, cluster, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return nil, err
	}

	machineDeployments, err := getMachineDeployments(ctx, c, cluster, config.TopologyNameRegex, runConfig.Limit)
	if err != nil {
		return nil, err
	}

	ret := make([]core.TestObjects, len(machineDeployments))
	for i := range machineDeployments {
		ret[i] = map[string]any{
			ClusterObjectKey:                   cluster.DeepCopy(),
			MachineDeploymentSelectorObjectKey: machineDeployments[i],
		}
	}

	return ret, nil
}

// ParseConfig parses the config for this plugin.
func (p MachineDeploymentSelectorPlugin) ParseConfig(_ context.Context, rawPluginConfig []byte) (any, error) {
	config := &MachineDeploymentSelectorPluginConfig{}
	if err := yaml.UnmarshalStrict(rawPluginConfig, config); err != nil {
		return nil, err
	}

	// TODO: validate regex

	return config, nil
}

// GenerateMessage generates a message text for the plugin call.
func (p MachineDeploymentSelectorPlugin) GenerateMessage(objects core.TestObjects, pluginConfigUntyped any) (string, error) {
	_, cluster, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Select MachineDeployments in Cluster %s", klog.KObj(cluster)), nil
}

// ValidateCallStack validates the call stack for the plugin.
func (p MachineDeploymentSelectorPlugin) ValidateCallStack(_ context.Context, callStack []string) error {
	if !slices.Contains(callStack, clusterSelectorPluginKey) {
		return pkgerrors.Errorf("this plugin can only be called as a child of %s", clusterSelectorPluginKey)
	}
	return nil
}

func (p *MachineDeploymentSelectorPlugin) unwrapConfigAndTestObjects(objects core.TestObjects, pluginConfigUntyped any) (*MachineDeploymentSelectorPluginConfig, *clusterv1.Cluster, error) {
	config := &MachineDeploymentSelectorPluginConfig{}
	if pluginConfigUntyped != nil {
		config = pluginConfigUntyped.(*MachineDeploymentSelectorPluginConfig)
	}

	cluster, err := UnwrapClusterTestObject(objects)
	if err != nil {
		return config, nil, err
	}

	return config, cluster, nil
}

// UnwrapMachineDeploymentTestObject extracts the MachineDeployment from TestObjects.
func UnwrapMachineDeploymentTestObject(objects core.TestObjects) (*clusterv1.MachineDeployment, error) {
	c, ok := objects[MachineDeploymentSelectorObjectKey]
	if !ok {
		return nil, pkgerrors.Errorf("failed to get the %s object from test objects. You must run the %s plugin before invoking this plugin", MachineDeploymentSelectorObjectKey, machineDeploymentSelectorPluginKey)
	}

	machineDeployment, ok := c.(*clusterv1.MachineDeployment)
	if !ok {
		return nil, pkgerrors.Errorf("failed to cast the %s object to the target type", MachineDeploymentSelectorObjectKey)
	}
	return machineDeployment, nil
}
