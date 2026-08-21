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
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/testctl/core"
)

const (
	controlPlaneSelectorPluginKey = "select.controlPlane.testctl"

	// ControlPlaneObjectKey is the key used to identify the ControlPlane object in TestObjects.
	ControlPlaneObjectKey = "ControlPlane"
)

// ControlPlaneSelectorPlugin can be used to select the Cluster's ControlPlane.
type ControlPlaneSelectorPlugin struct{}

var _ core.SelectorPlugin = &ControlPlaneSelectorPlugin{}
var _ core.MessageGeneratorPlugin = &ControlPlaneSelectorPlugin{}
var _ core.CallStackValidatorPlugin = &ControlPlaneSelectorPlugin{}

func init() {
	registerPlugin(controlPlaneSelectorPluginKey, &ControlPlaneSelectorPlugin{})
}

// Select selects the TestObjects to apply the test to, in this case the Cluster's ControlPlane.
func (p *ControlPlaneSelectorPlugin) Select(ctx context.Context, c client.Client, objects core.TestObjects, _ any, _ core.RunConfig) ([]core.TestObjects, error) {
	cluster, err := p.unwrapTestObjects(objects)
	if err != nil {
		return nil, err
	}

	controlPlane, err := getControlPlane(ctx, c, cluster)
	if err != nil {
		return nil, err
	}

	return []core.TestObjects{
		{
			ClusterObjectKey:      cluster.DeepCopy(),
			ControlPlaneObjectKey: controlPlane,
		},
	}, nil
}

// GenerateMessage generates a message text for the plugin call.
func (p ControlPlaneSelectorPlugin) GenerateMessage(objects core.TestObjects, _ any) (string, error) {
	cluster, err := p.unwrapTestObjects(objects)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Select control plane in Cluster %s", klog.KObj(cluster)), nil
}

// ValidateCallStack validates the call stack for the plugin.
func (p ControlPlaneSelectorPlugin) ValidateCallStack(_ context.Context, callStack []string) error {
	if !slices.Contains(callStack, clusterSelectorPluginKey) {
		return pkgerrors.Errorf("this plugin can only be called as a child of %s", clusterSelectorPluginKey)
	}
	return nil
}

func (p *ControlPlaneSelectorPlugin) unwrapTestObjects(objects core.TestObjects) (*clusterv1.Cluster, error) {
	cluster, err := UnwrapClusterTestObject(objects)
	if err != nil {
		return nil, err
	}

	if cluster.Spec.ControlPlaneRef.Kind != "KubeadmControlPlane" {
		return nil, pkgerrors.Errorf("unable to select control plane for Cluster %s. Support for control plane with kind different than KubeadmControlPlane not implemented yet", klog.KObj(cluster))
	}

	return cluster, nil
}

// UnwrapControlPlaneTestObject extracts the ControlPlane from TestObjects.
func UnwrapControlPlaneTestObject(objects core.TestObjects) (*controlplanev1.KubeadmControlPlane, error) {
	c, ok := objects[ControlPlaneObjectKey]
	if !ok {
		return nil, pkgerrors.Errorf("failed to get the %s object from test objects. You must run the %s plugin before invoking this plugin", ControlPlaneObjectKey, controlPlaneSelectorPluginKey)
	}

	controlPlane, ok := c.(*controlplanev1.KubeadmControlPlane)
	if !ok {
		return nil, pkgerrors.Errorf("failed to cast the %s object to the target type", ControlPlaneObjectKey)
	}
	return controlPlane, nil
}
