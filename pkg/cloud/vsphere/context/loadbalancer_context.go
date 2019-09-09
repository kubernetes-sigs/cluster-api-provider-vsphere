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
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

// LoadBalancerContextParams are the parameters needed to create a LoadBalancerContext.
type LoadBalancerContextParams struct {
	Context      context.Context
	LoadBalancer *v1alpha2.LoadBalancer
	Client       client.Client
	Logger       logr.Logger
}

// LoadBalancerContext is a Go context used with a CAPI cluster.
type LoadBalancerContext struct {
	context.Context
	LoadBalancer *v1alpha2.LoadBalancer
	Client       client.Client
	Logger       logr.Logger

	LoadBalancerPatch client.Patch
}

// NewLoadBalancerContext returns a new LoadBalancerContext.
func NewLoadBalancerContext(params *LoadBalancerContextParams) (*LoadBalancerContext, error) {
	parentContext := params.Context
	if parentContext == nil {
		parentContext = context.Background()
	}

	logr := params.Logger
	if logr == nil {
		logr = klogr.New().WithName("default-logger")
	}
	logr = logr.WithName(params.LoadBalancer.APIVersion).WithName(params.LoadBalancer.Namespace).WithName(params.LoadBalancer.Name)

	return &LoadBalancerContext{
		Context:           parentContext,
		LoadBalancer:      params.LoadBalancer,
		Client:            params.Client,
		Logger:            logr,
		LoadBalancerPatch: client.MergeFrom(params.LoadBalancer.DeepCopyObject()),
	}, nil
}

// Patch updates the object and its status on the API server.
func (c *LoadBalancerContext) Patch() error {

	// Patch LoadBalancer object.
	if err := c.Client.Patch(c, c.LoadBalancer, c.LoadBalancerPatch); err != nil {
		return errors.Wrapf(err, "error patching LoadBalancer %s/%s", c.LoadBalancer.Namespace, c.LoadBalancer.Name)
	}

	// Patch LoadBalancer status.
	if err := c.Client.Status().Patch(c, c.LoadBalancer, c.LoadBalancerPatch); err != nil {
		return errors.Wrapf(err, "error patching VSphereCluster %s/%s status", c.LoadBalancer.Namespace, c.LoadBalancer.Name)
	}

	return nil
}
