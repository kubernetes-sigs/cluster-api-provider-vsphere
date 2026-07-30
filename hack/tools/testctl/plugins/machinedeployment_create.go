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
	"math"
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

const machineDeploymentCreatePluginKey = "create.machineDeployment.testctl"

// MachineDeploymentCreatePlugin can be used to create MachineDeployments for a Cluster.
type MachineDeploymentCreatePlugin struct{}

var _ core.ExecutorPlugin = &MachineDeploymentCreatePlugin{}
var _ core.ConfigurablePlugin = &MachineDeploymentCreatePlugin{}
var _ core.MessageGeneratorPlugin = &MachineDeploymentCreatePlugin{}
var _ core.CallStackValidatorPlugin = &MachineDeploymentCreatePlugin{}

// MachineDeploymentCreatePluginConfig defines the config for the MachineDeploymentCreatePlugin.
type MachineDeploymentCreatePluginConfig struct {
	// Count is the number of MachineDeployments to create.
	Count int32 `json:"count"`

	// GenerateName is an optional prefix used to build the name of each generated
	// MachineDeploymentTopology (<GenerateName><n>), used only if Template.Name is not set.
	GenerateName string `json:"generateName,omitempty"`

	// Template is the MachineDeploymentTopology used as a template for each created MachineDeployment.
	Template clusterv1.MachineDeploymentTopology `json:"template,omitempty"`
}

func init() {
	registerPlugin(machineDeploymentCreatePluginKey, &MachineDeploymentCreatePlugin{})
}

// Exec executes the plugin for the given TestObjects.
func (p MachineDeploymentCreatePlugin) Exec(ctx context.Context, c client.Client, objects core.TestObjects, pluginConfigUntyped any, runConfig core.RunConfig) error {
	config, cluster, err := p.getInfo(objects, pluginConfigUntyped)
	if err != nil {
		return err
	}

	log := ctrl.LoggerFrom(ctx).WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	count := min(config.Count, ptr.Deref(runConfig.Limit, math.MaxInt32))
	var machineDeployments []clusterv1.MachineDeploymentTopology
	for i := int32(0); i < count; i++ {
		mdTopology := config.Template.DeepCopy()
		if mdTopology.Name == "" {
			mdTopology.Name = fmt.Sprintf("%s%d", config.GenerateName, i+1)
		}
		machineDeployments = append(machineDeployments, *mdTopology)
	}

	log.Info(fmt.Sprintf("Creating %d MachineDeployments", len(machineDeployments)))
	if ptr.Deref(runConfig.DryRun, false) {
		log.Info("MachineDeployment creation cannot be simulated in dry run mode, skipping (also nested run will be skipped)")
		return nil
	}

	for _, md := range machineDeployments {
		if err := p.create(ctx, c, cluster, md, runConfig); err != nil {
			return err
		}
	}
	return nil
}

// ParseConfig parses the config for this plugin.
func (p MachineDeploymentCreatePlugin) ParseConfig(_ context.Context, rawPluginConfig []byte) (any, error) {
	config := &MachineDeploymentCreatePluginConfig{}
	if err := yaml.UnmarshalStrict(rawPluginConfig, config); err != nil {
		return nil, err
	}
	return config, nil
}

// GenerateMessage generates a message text for the plugin call.
func (p MachineDeploymentCreatePlugin) GenerateMessage(objects core.TestObjects, pluginConfigUntyped any) (string, error) {
	config, cluster, err := p.getInfo(objects, pluginConfigUntyped)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Create %d MachineDeployments in Cluster %s", config.Count, klog.KObj(cluster)), nil
}

// ValidateCallStack validates the call stack for the plugin.
func (p MachineDeploymentCreatePlugin) ValidateCallStack(_ context.Context, callStack []string) error {
	if !slices.Contains(callStack, clusterSelectorPluginKey) {
		return pkgerrors.Errorf("this plugin can only be called as a child of %s", clusterSelectorPluginKey)
	}
	return nil
}

func (p MachineDeploymentCreatePlugin) getInfo(objects core.TestObjects, pluginConfigUntyped any) (*MachineDeploymentCreatePluginConfig, *clusterv1.Cluster, error) {
	config := &MachineDeploymentCreatePluginConfig{}
	if pluginConfigUntyped != nil {
		config = pluginConfigUntyped.(*MachineDeploymentCreatePluginConfig)
	}

	cluster, err := UnwrapClusterTestObject(objects)
	if err != nil {
		return config, nil, err
	}

	if !cluster.Spec.Topology.IsDefined() {
		return config, nil, pkgerrors.Errorf("unable to create MachineDeployments for Cluster %s. support for clusters without spec.topology not implemented yet", klog.KObj(cluster))
	}
	return config, cluster, nil
}

func (p *MachineDeploymentCreatePlugin) create(ctx context.Context, c client.Client, cluster *clusterv1.Cluster, machineDeploymentTopology clusterv1.MachineDeploymentTopology, runConfig core.RunConfig) error {
	// Add new MachineDeployments to cluster.Spec.Topology.Workers.
	original := cluster.DeepCopy()

	log := ctrl.LoggerFrom(ctx).WithValues("MachineDeployment.TopologyName", machineDeploymentTopology.Name)
	ctx = ctrl.LoggerInto(ctx, log)

	mdTopologyIndex := getMachineDeploymentTopologyIndex(cluster, machineDeploymentTopology.Name)

	if mdTopologyIndex >= 0 {
		log.Info("Creating MachineDeployment action skipped, MachineDeployment already exists")
		return nil
	}

	cluster.Spec.Topology.Workers.MachineDeployments = append(cluster.Spec.Topology.Workers.MachineDeployments, machineDeploymentTopology)

	if err := c.Patch(ctx, cluster, client.MergeFrom(original)); err != nil {
		return pkgerrors.Wrapf(err, "failed to patch Cluster %s", klog.KObj(cluster))
	}

	if ptr.Deref(runConfig.SkipWait, false) {
		return nil
	}

	log.Info("Waiting for MachineDeployments to be created")
	var retryErr error
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, false, func(ctx context.Context) (done bool, err error) {
		retryErr = nil
		md, err := getMachineDeployment(ctx, c, cluster, machineDeploymentTopology.Name)
		if err != nil {
			retryErr = err
			return false, nil //nolint:nilerr
		}

		if md == nil {
			retryErr = pkgerrors.Errorf("MachineDeployment with the %s=%s label does not exist", clusterv1.ClusterTopologyMachineDeploymentNameLabel, machineDeploymentTopology.Name)
			return false, nil
		}

		done, err, retryErr = waitForMachineDeploymentMachines(ctx, c, cluster, md, ptr.Deref(machineDeploymentTopology.Replicas, 0), cluster.Spec.Topology.Version)
		return done, err
	}); err != nil || retryErr != nil {
		return kerrors.NewAggregate([]error{retryErr, err})
	}

	log.Info("Create MachineDeployment completed")
	return nil
}
