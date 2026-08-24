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
	"net/http"
	"slices"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/testctl/core"
)

const clusterControlPlaneEndpointPluginKey = "update.controlPlaneEndpoint.cluster.testctl"

// ClusterControlPlaneEndpointPlugin can be used to control the status of a Cluster's ControlPlaneEndpoint.
type ClusterControlPlaneEndpointPlugin struct{}

var _ core.ExecutorPlugin = &ClusterControlPlaneEndpointPlugin{}
var _ core.ConfigurablePlugin = &ClusterControlPlaneEndpointPlugin{}
var _ core.MessageGeneratorPlugin = &ClusterControlPlaneEndpointPlugin{}
var _ core.CallStackValidatorPlugin = &ClusterControlPlaneEndpointPlugin{}

// ClusterControlPlaneEndpointPluginConfig defines the config for the ClusterControlPlaneEndpointPlugin.
type ClusterControlPlaneEndpointPluginConfig struct {
	// Status is the target status for the Cluster's ControlPlaneEndpoint listener.
	Status ClusterControlPlaneEndpointStatus `status:"start,omitempty"`
}

// ClusterControlPlaneEndpointStatus defines the ControlPlaneEndpoint status.
type ClusterControlPlaneEndpointStatus string

const (
	// ClusterControlPlaneEndpointStatusRunning represents ControlPlaneEndpoint in running status.
	ClusterControlPlaneEndpointStatusRunning = ClusterControlPlaneEndpointStatus("Running")

	// ClusterControlPlaneEndpointStatusNotRunning represents ControlPlaneEndpoint not in running status.
	ClusterControlPlaneEndpointStatusNotRunning = ClusterControlPlaneEndpointStatus("NotRunning")
)

func init() {
	registerPlugin(clusterControlPlaneEndpointPluginKey, &ClusterControlPlaneEndpointPlugin{})
}

// Exec executes the plugin for the given TestObjects.
func (p ClusterControlPlaneEndpointPlugin) Exec(ctx context.Context, _ client.Client, objects core.TestObjects, pluginConfigUntyped any, runConfig core.RunConfig) error {
	config, cluster, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return err
	}

	log := ctrl.LoggerFrom(ctx).WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	action, logAction := targetControlPlaneEndpointStatus(config)

	log.Info(fmt.Sprintf("%s listener for the Cluster control plane endpoint", logAction))
	if ptr.Deref(runConfig.DryRun, false) {
		return nil
	}

	// TODO: make ip and port configurable
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://127.0.0.1:19000//namespaces/%s/clusters/%s/listener", cluster.Namespace, cluster.Name), strings.NewReader(action))
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to create request to call the endpoint to %s listener for Cluster %s", action, klog.KObj(cluster))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to call the endpoint to %s listener for Cluster %s", action, klog.KObj(cluster))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return pkgerrors.Errorf("failed %s listener for Cluster %s", action, klog.KObj(cluster))
	}

	log.Info(fmt.Sprintf("%s listener for Cluster control plane endpoint completed", action))

	return nil
}

// ParseConfig parses the config for this plugin.
func (p ClusterControlPlaneEndpointPlugin) ParseConfig(_ context.Context, rawPluginConfig []byte) (any, error) {
	config := &ClusterControlPlaneEndpointPluginConfig{}
	if err := yaml.UnmarshalStrict(rawPluginConfig, config); err != nil {
		return nil, err
	}
	return config, nil
}

// GenerateMessage generates a message text for the plugin call.
func (p ClusterControlPlaneEndpointPlugin) GenerateMessage(objects core.TestObjects, pluginConfigUntyped any) (string, error) {
	config, cluster, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return "", err
	}

	_, logAction := targetControlPlaneEndpointStatus(config)
	return fmt.Sprintf("%s control plane endpoint status in Cluster %s", logAction, klog.KObj(cluster)), nil
}

// ValidateCallStack validates the call stack for the plugin.
func (p ClusterControlPlaneEndpointPlugin) ValidateCallStack(_ context.Context, callStack []string) error {
	if !slices.Contains(callStack, clusterSelectorPluginKey) {
		return pkgerrors.Errorf("this plugin can only be called as a child of %s", clusterSelectorPluginKey)
	}
	return nil
}

func (p ClusterControlPlaneEndpointPlugin) unwrapConfigAndTestObjects(objects core.TestObjects, pluginConfigUntyped any) (*ClusterControlPlaneEndpointPluginConfig, *clusterv1.Cluster, error) {
	config := &ClusterControlPlaneEndpointPluginConfig{}
	if pluginConfigUntyped != nil {
		config = pluginConfigUntyped.(*ClusterControlPlaneEndpointPluginConfig)
	}

	cluster, err := UnwrapClusterTestObject(objects)
	if err != nil {
		return config, nil, err
	}

	// TODO: implement a check ensuring that it is a cluster with an in-memory backend.

	return config, cluster, nil
}

func targetControlPlaneEndpointStatus(config *ClusterControlPlaneEndpointPluginConfig) (string, string) {
	action := "start"
	logAction := "Starting"
	if config.Status == ClusterControlPlaneEndpointStatusNotRunning {
		action = "stop"
		logAction = "Stopping"
	}
	return action, logAction
}
