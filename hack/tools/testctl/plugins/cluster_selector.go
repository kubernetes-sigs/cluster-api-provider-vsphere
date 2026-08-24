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

	pkgerrors "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/testctl/core"
)

const clusterSelectorPluginKey = "select.cluster.testctl"

// ClusterObjectKey is the key used to identify the Cluster object in TestObjects.
const ClusterObjectKey = "Cluster"

// ClusterSelectorPlugin can be used to select Clusters.
type ClusterSelectorPlugin struct{}

var _ core.SelectorPlugin = &ClusterSelectorPlugin{}
var _ core.ConfigurablePlugin = &ClusterSelectorPlugin{}
var _ core.MessageGeneratorPlugin = &ClusterSelectorPlugin{}

// ClusterSelectorPluginConfig defines the config for the ClusterSelectorPlugin.
type ClusterSelectorPluginConfig struct {
	// ClusterSelectors defines the list of Clusters which are candidate for this test.
	//
	// If ClusterSelectors is not set, the test applies to all Clusters.
	// If ClusterSelectors contains multiple selectors, the results are ORed.
	ClusterSelectors []metav1.LabelSelector `json:"clusterSelectors,omitempty"`

	// NameRegex defines a regex used to select among the list of Clusters which are candidate.
	// If NameRegex is not set, all candidate Clusters will be considered.
	NameRegex string `json:"nameRegex,omitempty"`
}

func init() {
	registerPlugin(clusterSelectorPluginKey, &ClusterSelectorPlugin{})
}

// Select selects the TestObjects to apply the test to, in this case Clusters.
func (p *ClusterSelectorPlugin) Select(ctx context.Context, c client.Client, _ core.TestObjects, pluginConfigUntyped any, runConfig core.RunConfig) ([]core.TestObjects, error) {
	log := ctrl.LoggerFrom(ctx)
	config := p.unwrapConfig(pluginConfigUntyped)

	clusters, err := getClusters(ctx, c, config.NameRegex, runConfig.Limit)
	if err != nil {
		return nil, err
	}

	ret := make([]core.TestObjects, len(clusters))
	for i := range clusters {
		ret[i] = map[string]any{
			ClusterObjectKey: clusters[i],
		}
	}

	log.Info(fmt.Sprintf("%d candidate clusters", len(clusters)))

	return ret, nil
}

// ParseConfig parses the config for this plugin.
func (p ClusterSelectorPlugin) ParseConfig(_ context.Context, rawPluginConfig []byte) (any, error) {
	config := &ClusterSelectorPluginConfig{}
	if err := yaml.UnmarshalStrict(rawPluginConfig, config); err != nil {
		return nil, err
	}

	// TODO: validate regex

	return config, nil
}

// GenerateMessage generates a message text for the plugin call.
func (p ClusterSelectorPlugin) GenerateMessage(_ core.TestObjects, _ any) (string, error) {
	return "Select Clusters", nil
}

func (p *ClusterSelectorPlugin) unwrapConfig(pluginConfigUntyped any) *ClusterSelectorPluginConfig {
	config := &ClusterSelectorPluginConfig{}
	if pluginConfigUntyped != nil {
		config = pluginConfigUntyped.(*ClusterSelectorPluginConfig)
	}
	return config
}

// UnwrapClusterTestObject extracts the Cluster from TestObjects.
func UnwrapClusterTestObject(objects core.TestObjects) (*clusterv1.Cluster, error) {
	c, ok := objects[ClusterObjectKey]
	if !ok {
		return nil, pkgerrors.Errorf("failed to get the %s object from test objects. You must run the %s plugin before invoking this plugin", ClusterObjectKey, clusterSelectorPluginKey)
	}

	cluster, ok := c.(*clusterv1.Cluster)
	if !ok {
		return nil, pkgerrors.Errorf("failed to cast the %s object to the target type", ClusterObjectKey)
	}
	return cluster, nil
}
