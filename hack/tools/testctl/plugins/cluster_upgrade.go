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
	"time"

	pkgerrors "github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/testctl/core"
)

const clusterUpgradePluginKey = "upgrade.cluster.testctl"

// ClusterUpgradePlugin can be used to upgrade a Cluster.
type ClusterUpgradePlugin struct{}

var _ core.ExecutorPlugin = &ClusterUpgradePlugin{}
var _ core.ConfigurablePlugin = &ClusterUpgradePlugin{}
var _ core.CallStackValidatorPlugin = &ClusterUpgradePlugin{}
var _ core.MessageGeneratorPlugin = &ClusterUpgradePlugin{}

// ClusterUpgradePluginConfig defines the config for the ClusterUpgradePlugin.
type ClusterUpgradePluginConfig struct {
	// Version is the version to upgrade to.
	Version string `json:"version"`
}

func init() {
	registerPlugin(clusterUpgradePluginKey, &ClusterUpgradePlugin{})
}

// Exec executes the plugin for the given TestObjects.
func (p ClusterUpgradePlugin) Exec(ctx context.Context, c client.Client, objects core.TestObjects, pluginConfigUntyped any, runConfig core.RunConfig) error {
	config, cluster, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return err
	}

	if !cluster.Spec.Topology.IsDefined() {
		return pkgerrors.Errorf("unable to upgrade Cluster %s. support for clusters without spec.topology not implemented yet", klog.KObj(cluster))
	}

	log := ctrl.LoggerFrom(ctx).WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	currentVersion := cluster.Spec.Topology.Version
	if currentVersion == config.Version {
		log.Info(fmt.Sprintf("Upgrade Cluster action skipped, Cluster already have version %s", config.Version))
		return nil
	}

	log.Info(fmt.Sprintf("Upgrading Cluster from version %s to %s", currentVersion, config.Version))
	if ptr.Deref(runConfig.DryRun, false) {
		return nil
	}

	original := cluster.DeepCopy()
	cluster.Spec.Topology.Version = config.Version
	if err := c.Patch(ctx, cluster, client.MergeFrom(original)); err != nil {
		return pkgerrors.Wrapf(err, "failed to patch Cluster %s", klog.KObj(cluster))
	}

	if ptr.Deref(runConfig.SkipWait, false) {
		return nil
	}

	log.Info(fmt.Sprintf("Waiting for Cluster to have version %s", config.Version))
	controlPlane, err := getControlPlane(ctx, c, cluster)
	if err != nil {
		return err
	}
	machineDeployments, err := getMachineDeployments(ctx, c, cluster, "", nil)
	if err != nil {
		return err
	}
	var retryErr error
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, false, func(ctx context.Context) (done bool, err error) {
		retryErr = nil
		done, err, retryErr = waitForControlPlaneMachines(ctx, c, cluster, controlPlane, ptr.Deref(controlPlane.Spec.Replicas, 0), cluster.Spec.Topology.Version)
		if err != nil {
			return done, err
		}
		if retryErr != nil {
			return false, nil //nolint:nilerr
		}

		for _, machineDeployment := range machineDeployments {
			done, err, retryErr = waitForMachineDeploymentMachines(ctx, c, cluster, machineDeployment, ptr.Deref(machineDeployment.Spec.Replicas, 0), cluster.Spec.Topology.Version)
			if err != nil {
				return done, err
			}
			if retryErr != nil {
				return false, nil //nolint:nilerr
			}
		}
		return true, nil
	}); err != nil || retryErr != nil {
		return kerrors.NewAggregate([]error{retryErr, err})
	}

	log.Info(fmt.Sprintf("Upgrade Cluster from version %s to %s completed", currentVersion, config.Version))

	return nil
}

// ParseConfig parses the config for this plugin.
func (p ClusterUpgradePlugin) ParseConfig(_ context.Context, rawPluginConfig []byte) (any, error) {
	config := &ClusterUpgradePluginConfig{}
	if err := yaml.UnmarshalStrict(rawPluginConfig, config); err != nil {
		return nil, err
	}
	return config, nil
}

// GenerateMessage generates a message text for the plugin call.
func (p ClusterUpgradePlugin) GenerateMessage(objects core.TestObjects, pluginConfigUntyped any) (string, error) {
	config, cluster, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Upgrade Cluster %s to version %s", klog.KObj(cluster), config.Version), nil
}

// ValidateCallStack validates the call stack for the plugin.
func (p ClusterUpgradePlugin) ValidateCallStack(_ context.Context, callStack []string) error {
	if !slices.Contains(callStack, clusterSelectorPluginKey) {
		return pkgerrors.Errorf("this plugin can only be called as a child of %s", clusterSelectorPluginKey)
	}
	return nil
}

func (p ClusterUpgradePlugin) unwrapConfigAndTestObjects(objects core.TestObjects, pluginConfigUntyped any) (*ClusterUpgradePluginConfig, *clusterv1.Cluster, error) {
	config := &ClusterUpgradePluginConfig{}
	if pluginConfigUntyped != nil {
		config = pluginConfigUntyped.(*ClusterUpgradePluginConfig)
	}

	cluster, err := UnwrapClusterTestObject(objects)
	if err != nil {
		return config, nil, err
	}
	return config, cluster, nil
}
