/*
Copyright 2020 The Kubernetes Authors.

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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/nsxt"
)

// NSXTLoadBalancerContext is a Go context used with an NSXTLoadBalancer.
type NSXTLoadBalancerContext struct {
	*ControllerContext
	Cluster          *clusterv1.Cluster
	NSXTLoadBalancer *infrav1.NSXTLoadBalancer
	Logger           logr.Logger
	PatchHelper      *patch.Helper
	NsxtService      *nsxt.NsxtLB

}

// String returns NSXTLoadBalancerGroupVersionKind NSXTLoadBalancerNamespace/NSXTLoadBalancerName.
func (c *NSXTLoadBalancerContext) String() string {
	return fmt.Sprintf("%s %s/%s", c.NSXTLoadBalancer.GroupVersionKind(), c.NSXTLoadBalancer.Namespace, c.NSXTLoadBalancer.Name)
}

// Patch updates the object and its status on the API server.
func (c *NSXTLoadBalancerContext) Patch() error {
	return c.PatchHelper.Patch(c, c.NSXTLoadBalancer)
}
