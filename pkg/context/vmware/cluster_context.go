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

// Package vmware contains fake contexts used for testing.
package vmware

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

// ClusterContext is a Go context used with a CAPI cluster.
type ClusterContext struct {
	Cluster        *clusterv1.Cluster
	VSphereCluster *vmwarev1.VSphereCluster
	PatchHelper    *patch.Helper
}

// String returns ClusterAPIVersion/ClusterNamespace/ClusterName.
func (c *ClusterContext) String() string {
	return fmt.Sprintf("%s %s/%s", c.VSphereCluster.GroupVersionKind(), c.VSphereCluster.Namespace, c.VSphereCluster.Name)
}

// Patch updates the object and its status on the API server.
func (c *ClusterContext) Patch(ctx context.Context) error {
	// always update the readyCondition.
	conditions.SetSummary(c.VSphereCluster,
		conditions.WithConditions(
			vmwarev1.ResourcePolicyReadyCondition,
			vmwarev1.ClusterNetworkReadyCondition,
			vmwarev1.LoadBalancerReadyCondition,
		),
	)

	if err := v1beta2conditions.SetSummaryCondition(c.VSphereCluster, c.VSphereCluster, vmwarev1.VSphereClusterReadyV1Beta2Condition,
		v1beta2conditions.ForConditionTypes{
			vmwarev1.VSphereClusterResourcePolicyReadyV1Beta2Condition,
			vmwarev1.VSphereClusterNetworkNotReadyV1Beta2Reason,
			vmwarev1.VSphereClusterLoadBalancerReadyV1Beta2Condition,
		},
		// Using a custom merge strategy to override reasons applied during merge.
		v1beta2conditions.CustomMergeStrategy{
			MergeStrategy: v1beta2conditions.DefaultMergeStrategy(
				// Use custom reasons.
				v1beta2conditions.ComputeReasonFunc(v1beta2conditions.GetDefaultComputeMergeReasonFunc(
					vmwarev1.VSphereClusterNotReadyV1Beta2Reason,
					vmwarev1.VSphereClusterReadyUnknownV1Beta2Reason,
					vmwarev1.VSphereClusterReadyV1Beta2Reason,
				)),
			),
		},
	); err != nil {
		return errors.Wrapf(err, "failed to set %s condition", vmwarev1.VSphereClusterReadyV1Beta2Condition)
	}

	return c.PatchHelper.Patch(ctx, c.VSphereCluster,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			vmwarev1.ResourcePolicyReadyCondition,
			vmwarev1.ClusterNetworkReadyCondition,
			vmwarev1.LoadBalancerReadyCondition,
		}},
		patch.WithOwnedV1Beta2Conditions{Conditions: []string{
			vmwarev1.VSphereClusterReadyV1Beta2Condition,
			vmwarev1.VSphereClusterResourcePolicyReadyV1Beta2Condition,
			vmwarev1.VSphereClusterNetworkNotReadyV1Beta2Reason,
			vmwarev1.VSphereClusterLoadBalancerReadyV1Beta2Condition,
		}},
	)
}
