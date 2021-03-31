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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
)

// HAProxyLoadBalancerContext is a Go context used with an HAProxyLoadBalancer.
type HAProxyLoadBalancerContext struct {
	*ControllerContext
	Cluster             *clusterv1.Cluster
	HAProxyLoadBalancer *infrav1.HAProxyLoadBalancer
	Logger              logr.Logger
	PatchHelper         *patch.Helper
}

// String returns HAProxyLoadBalancerGroupVersionKind HAProxyLoadBalancerNamespace/HAProxyLoadBalancerName.
func (c *HAProxyLoadBalancerContext) String() string {
	return fmt.Sprintf("%s %s/%s", c.HAProxyLoadBalancer.GroupVersionKind(), c.HAProxyLoadBalancer.Namespace, c.HAProxyLoadBalancer.Name)
}

// Patch updates the object and its status on the API server.
func (c *HAProxyLoadBalancerContext) Patch() error {
	return c.PatchHelper.Patch(c, c.HAProxyLoadBalancer)
}

// GetLogger returns this context's logger.
func (c *HAProxyLoadBalancerContext) GetLogger() logr.Logger {
	return c.Logger
}
