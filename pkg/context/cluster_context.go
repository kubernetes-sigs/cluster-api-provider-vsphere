/*
Copyright 2019 The Kubernetes Authors.

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

package context

import (
	"fmt"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util/patch"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

// ClusterContext is a Go context used with a CAPI cluster.
type ClusterContext struct {
	*ControllerContext
	Cluster        *clusterv1.Cluster
	VSphereCluster *v1alpha2.VSphereCluster
	PatchHelper    *patch.Helper
	Logger         logr.Logger
}

// String returns ClusterGroupVersionKind ClusterNamespace/ClusterName.
func (c *ClusterContext) String() string {
	return fmt.Sprintf("%s %s/%s", c.VSphereCluster.GroupVersionKind(), c.VSphereCluster.Namespace, c.VSphereCluster.Name)
}

// Patch updates the object and its status on the API server.
func (c *ClusterContext) Patch() error {
	return c.PatchHelper.Patch(c, c.VSphereCluster)
}
