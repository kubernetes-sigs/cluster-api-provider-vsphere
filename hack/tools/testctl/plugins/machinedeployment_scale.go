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

const machineDeploymentScalePluginKey = "scale.machineDeployment.testctl"

// MachineDeploymentScalePlugin can be used to scale a MachineDeployments for a Cluster.
type MachineDeploymentScalePlugin struct{}

var _ core.ExecutorPlugin = &MachineDeploymentScalePlugin{}
var _ core.ConfigurablePlugin = &MachineDeploymentScalePlugin{}
var _ core.MessageGeneratorPlugin = &MachineDeploymentScalePlugin{}
var _ core.CallStackValidatorPlugin = &MachineDeploymentScalePlugin{}

// MachineDeploymentScalePluginConfig defines the config for the MachineDeploymentScalePlugin.
type MachineDeploymentScalePluginConfig struct {
	// Replicas is the absolute number of replicas to scale to. Takes precedence over ReplicasDiff if both are set.
	Replicas *int32 `json:"replicas,omitempty"`

	// ReplicasDiff is a relative delta (positive or negative) applied to the current number of replicas.
	ReplicasDiff *int32 `json:"replicasDiff,omitempty"`
}

func init() {
	registerPlugin(machineDeploymentScalePluginKey, &MachineDeploymentScalePlugin{})
}

// Exec executes the plugin for the given TestObjects.
func (p MachineDeploymentScalePlugin) Exec(ctx context.Context, c client.Client, objects core.TestObjects, pluginConfigUntyped any, runConfig core.RunConfig) error {
	config, cluster, machineDeployment, mdTopologyIndex, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return err
	}

	log := ctrl.LoggerFrom(ctx).WithValues("Cluster", klog.KObj(cluster), "MachineDeployment", klog.KObj(machineDeployment))
	ctx = ctrl.LoggerInto(ctx, log)

	replicas := targetMachineDeploymentReplicas(cluster, mdTopologyIndex, config)

	currentReplicas := ptr.Deref(machineDeployment.Status.Replicas, 0)
	if currentReplicas == replicas && ptr.Deref(cluster.Spec.Topology.Workers.MachineDeployments[mdTopologyIndex].Replicas, 0) == replicas {
		log.Info(fmt.Sprintf("Scaling MachineDeployment action skipped, MachineDeployment already have %d replicas", replicas))
		return nil
	}

	log.Info(fmt.Sprintf("Scaling MachineDeployment from %d to %d replicas", currentReplicas, replicas))
	if ptr.Deref(runConfig.DryRun, false) {
		return nil
	}

	original := cluster.DeepCopy()
	cluster.Spec.Topology.Workers.MachineDeployments[mdTopologyIndex].Replicas = ptr.To(replicas)
	if err := c.Patch(ctx, cluster, client.MergeFrom(original)); err != nil {
		return pkgerrors.Wrapf(err, "failed to patch Cluster %s", klog.KObj(cluster))
	}

	if ptr.Deref(runConfig.SkipWait, false) {
		return nil
	}

	log.Info(fmt.Sprintf("Waiting for MachineDeployment to have %d replicas", replicas))
	var retryErr error
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, false, func(ctx context.Context) (done bool, err error) {
		retryErr = nil
		done, err, retryErr = waitForMachineDeploymentMachines(ctx, c, cluster, machineDeployment, replicas, cluster.Spec.Topology.Version)
		return done, err
	}); err != nil || retryErr != nil {
		return kerrors.NewAggregate([]error{retryErr, err})
	}

	log.Info(fmt.Sprintf("Scale MachineDeployment from %d to %d replicas completed", currentReplicas, replicas))

	return nil
}

// ParseConfig parses the config for this plugin.
func (p MachineDeploymentScalePlugin) ParseConfig(_ context.Context, rawPluginConfig []byte) (any, error) {
	config := &MachineDeploymentScalePluginConfig{}
	if err := yaml.UnmarshalStrict(rawPluginConfig, config); err != nil {
		return nil, err
	}
	return config, nil
}

// GenerateMessage generates a message text for the plugin call.
func (p MachineDeploymentScalePlugin) GenerateMessage(objects core.TestObjects, pluginConfigUntyped any) (string, error) {
	config, cluster, machineDeployment, mdTopologyIndex, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return "", err
	}

	replicas := targetMachineDeploymentReplicas(cluster, mdTopologyIndex, config)
	return fmt.Sprintf("Scale MachineDeployments %s to %d replicas in Cluster %s", klog.KObj(machineDeployment), replicas, klog.KObj(cluster)), nil
}

// ValidateCallStack validates the call stack for the plugin.
func (p MachineDeploymentScalePlugin) ValidateCallStack(_ context.Context, callStack []string) error {
	if !slices.Contains(callStack, machineDeploymentSelectorPluginKey) {
		return pkgerrors.Errorf("this plugin can only be called as a child of %s", machineDeploymentSelectorPluginKey)
	}
	return nil
}

func (p MachineDeploymentScalePlugin) unwrapConfigAndTestObjects(objects core.TestObjects, pluginConfigUntyped any) (*MachineDeploymentScalePluginConfig, *clusterv1.Cluster, *clusterv1.MachineDeployment, int, error) {
	config := &MachineDeploymentScalePluginConfig{}
	if pluginConfigUntyped != nil {
		config = pluginConfigUntyped.(*MachineDeploymentScalePluginConfig)
	}

	cluster, err := UnwrapClusterTestObject(objects)
	if err != nil {
		return config, nil, nil, 0, err
	}

	if !cluster.Spec.Topology.IsDefined() {
		return config, nil, nil, 0, pkgerrors.Errorf("unable to scale MachineDeployments for Cluster %s. support for clusters without spec.topology not implemented yet", klog.KObj(cluster))
	}

	machineDeployment, err := UnwrapMachineDeploymentTestObject(objects)
	if err != nil {
		return config, nil, nil, 0, err
	}

	mdTopologyName, ok := machineDeployment.Labels[clusterv1.ClusterTopologyMachineDeploymentNameLabel]
	if !ok {
		return config, nil, nil, 0, pkgerrors.Errorf("MachineDeployment doesn't have the %s label", clusterv1.ClusterTopologyMachineDeploymentNameLabel)
	}

	mdTopologyIndex := getMachineDeploymentTopologyIndex(cluster, mdTopologyName)
	if mdTopologyIndex == -1 {
		return config, nil, nil, 0, pkgerrors.Errorf("Cannot find a MachineDeployment with name %s in cluster.spec.topology.workers.machineDeployments", mdTopologyName)
	}
	return config, cluster, machineDeployment, mdTopologyIndex, nil
}

func targetMachineDeploymentReplicas(cluster *clusterv1.Cluster, mdTopologyIndex int, config *MachineDeploymentScalePluginConfig) int32 {
	replicas := ptr.Deref(cluster.Spec.Topology.Workers.MachineDeployments[mdTopologyIndex].Replicas, 0)
	if config.ReplicasDiff != nil {
		replicas += *config.ReplicasDiff
	}
	if config.Replicas != nil {
		replicas = *config.Replicas
	}
	return replicas
}
