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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/testctl/core"
)

const clusterDeletePluginKey = "delete.cluster.testctl"

// ClusterDeletePlugin can be used to delete a Cluster.
type ClusterDeletePlugin struct{}

var _ core.ExecutorPlugin = &ClusterDeletePlugin{}
var _ core.MessageGeneratorPlugin = &ClusterDeletePlugin{}
var _ core.CallStackValidatorPlugin = &ClusterDeletePlugin{}

// Exec executes the plugin for the given TestObjects.
func (p ClusterDeletePlugin) Exec(ctx context.Context, c client.Client, objects core.TestObjects, _ any, runConfig core.RunConfig) error {
	cluster, err := p.unwrapTestObjects(objects)
	if err != nil {
		return err
	}

	log := ctrl.LoggerFrom(ctx).WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// NOTE: if the plugin is processing a Cluster, it is not fully deleted yet.

	log.Info("Deleting Cluster")
	if ptr.Deref(runConfig.DryRun, false) {
		return nil
	}

	if err := c.Delete(ctx, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Deleted Cluster")

			return nil
		}
		return pkgerrors.Wrapf(err, "failed to delete Cluster %s", klog.KObj(cluster))
	}

	if ptr.Deref(runConfig.SkipWait, false) {
		return nil
	}

	log.Info("Waiting for Cluster to be deleted")
	timeout := 5 * time.Minute
	if runConfig.Timeout != nil {
		timeout = runConfig.Timeout.Duration
	}
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, false, func(ctx context.Context) (done bool, err error) {
		tmpCluster := &clusterv1.Cluster{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(cluster), tmpCluster); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}); err != nil {
		return err
	}

	log.Info("Deleted Cluster")

	return nil
}

func init() {
	registerPlugin(clusterDeletePluginKey, &ClusterDeletePlugin{})
}

// GenerateMessage generates a message text for the plugin call.
func (p ClusterDeletePlugin) GenerateMessage(objects core.TestObjects, _ any) (string, error) {
	cluster, err := p.unwrapTestObjects(objects)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Delete Cluster %s", klog.KObj(cluster)), nil
}

// ValidateCallStack validates the call stack for the plugin.
func (p ClusterDeletePlugin) ValidateCallStack(_ context.Context, callStack []string) error {
	if !slices.Contains(callStack, clusterSelectorPluginKey) {
		return pkgerrors.Errorf("this plugin can only be called as a child of %s", clusterSelectorPluginKey)
	}
	// TODO: Think about adding a check that ensures that no other actions are run on a cluster after delete
	return nil
}

func (p ClusterDeletePlugin) unwrapTestObjects(objects core.TestObjects) (*clusterv1.Cluster, error) {
	cluster, err := UnwrapClusterTestObject(objects)
	if err != nil {
		return nil, err
	}
	return cluster, nil
}
