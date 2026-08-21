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
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/testctl/core"
)

const controlPlaneScalePluginKey = "scale.controlPlane.testctl"

// ControlPlaneScalePlugin can be used to scale the Cluster's ControlPlane.
type ControlPlaneScalePlugin struct{}

var _ core.ExecutorPlugin = &ControlPlaneScalePlugin{}
var _ core.MessageGeneratorPlugin = &ControlPlaneScalePlugin{}
var _ core.CallStackValidatorPlugin = &ControlPlaneScalePlugin{}

// ControlPlaneScalePluginConfig defines the config for the ControlPlaneScalePlugin.
type ControlPlaneScalePluginConfig struct {
	// Replicas is the target number of KubeadmControlPlane replicas.
	Replicas int32 `json:"replicas"`
}

func init() {
	registerPlugin(controlPlaneScalePluginKey, &ControlPlaneScalePlugin{})
}

// Exec executes the plugin for the given TestObjects.
func (p ControlPlaneScalePlugin) Exec(ctx context.Context, c client.Client, objects core.TestObjects, pluginConfigUntyped any, runConfig core.RunConfig) error {
	config, cluster, controlPlane, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return err
	}

	log := ctrl.LoggerFrom(ctx).WithValues("Cluster", klog.KObj(cluster), "KubeadmControlPlane", klog.KObj(controlPlane))
	ctx = ctrl.LoggerInto(ctx, log)

	// TODO: check if the replicas are already fully provisioned so we can still wait for the operation to complete when necessary.
	replicas := config.Replicas
	currentReplicas := ptr.Deref(controlPlane.Status.Replicas, 0)
	if currentReplicas == config.Replicas && ptr.Deref(cluster.Spec.Topology.ControlPlane.Replicas, 0) == replicas {
		log.Info(fmt.Sprintf("Scaling KubeadmControlPlane action skipped, KubeadmControlPlane already have %d replicas", replicas))
		return nil
	}

	log.Info(fmt.Sprintf("Scaling KubeadmControlPlane from %d to %d replicas", currentReplicas, replicas))
	if ptr.Deref(runConfig.DryRun, false) {
		return nil
	}

	if ptr.Deref(cluster.Spec.Topology.ControlPlane.Replicas, 0) != replicas {
		original := cluster.DeepCopy()
		cluster.Spec.Topology.ControlPlane.Replicas = ptr.To(replicas)
		if err := c.Patch(ctx, cluster, client.MergeFrom(original)); err != nil {
			return pkgerrors.Wrapf(err, "failed to patch Cluster %s", klog.KObj(cluster))
		}
	}

	if ptr.Deref(runConfig.SkipWait, false) {
		return nil
	}

	log.Info(fmt.Sprintf("Waiting for KubeadmControlPlane to have %d replicas", replicas))
	var retryErr error
	timeout := 5 * time.Minute
	if runConfig.Timeout != nil {
		timeout = runConfig.Timeout.Duration
	}
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, false, func(ctx context.Context) (done bool, err error) {
		retryErr = nil
		done, err, retryErr = waitForControlPlaneMachines(ctx, c, cluster, controlPlane, replicas, cluster.Spec.Topology.Version)
		return done, err
	}); err != nil || retryErr != nil {
		return kerrors.NewAggregate([]error{retryErr, err})
	}

	log.Info(fmt.Sprintf("Scale KubeadmControlPlane from %d to %d replicas completed", currentReplicas, replicas))

	return nil
}

// ParseConfig parses the config for this plugin.
func (p ControlPlaneScalePlugin) ParseConfig(_ context.Context, rawPluginConfig []byte) (any, error) {
	config := &ControlPlaneScalePluginConfig{}
	if err := yaml.UnmarshalStrict(rawPluginConfig, config); err != nil {
		return nil, err
	}
	return config, nil
}

// GenerateMessage generates a message text for the plugin call.
func (p ControlPlaneScalePlugin) GenerateMessage(objects core.TestObjects, pluginConfigUntyped any) (string, error) {
	config, cluster, controlPlane, err := p.unwrapConfigAndTestObjects(objects, pluginConfigUntyped)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Scale KubeadmControlPlane %s to %d replicas in Cluster %s", klog.KObj(controlPlane), config.Replicas, klog.KObj(cluster)), nil
}

// ValidateCallStack validates the call stack for the plugin.
func (p ControlPlaneScalePlugin) ValidateCallStack(_ context.Context, callStack []string) error {
	if !slices.Contains(callStack, controlPlaneSelectorPluginKey) {
		return pkgerrors.Errorf("this plugin can only be called as a child of %s", controlPlaneSelectorPluginKey)
	}
	return nil
}

func (p ControlPlaneScalePlugin) unwrapConfigAndTestObjects(objects core.TestObjects, pluginConfigUntyped any) (*ControlPlaneScalePluginConfig, *clusterv1.Cluster, *controlplanev1.KubeadmControlPlane, error) {
	config := &ControlPlaneScalePluginConfig{}
	if pluginConfigUntyped != nil {
		config = pluginConfigUntyped.(*ControlPlaneScalePluginConfig)
	}

	cluster, err := UnwrapClusterTestObject(objects)
	if err != nil {
		return config, nil, nil, err
	}

	if !cluster.Spec.Topology.IsDefined() {
		return config, nil, nil, pkgerrors.Errorf("unable to scale control plane for Cluster %s. support for clusters without spec.topology not implemented yet", klog.KObj(cluster))
	}

	controlPlane, err := UnwrapControlPlaneTestObject(objects)
	if err != nil {
		return config, nil, nil, err
	}
	return config, cluster, controlPlane, nil
}
