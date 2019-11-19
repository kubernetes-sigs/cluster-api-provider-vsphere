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
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

// ClusterContextParams are the parameters needed to create a ClusterContext.
type ClusterContextParams struct {
	Context        context.Context
	Cluster        *clusterv1.Cluster
	VSphereCluster *v1alpha2.VSphereCluster
	Client         client.Client
	Logger         logr.Logger
}

// ClusterContext is a Go context used with a CAPI cluster.
type ClusterContext struct {
	context.Context
	Cluster        *clusterv1.Cluster
	VSphereCluster *v1alpha2.VSphereCluster
	Client         client.Client
	Logger         logr.Logger

	vsphereClusterPatch client.Patch
}

// NewClusterContext returns a new ClusterContext.
func NewClusterContext(params *ClusterContextParams) (*ClusterContext, error) {
	parentContext := params.Context
	if parentContext == nil {
		parentContext = context.Background()
	}

	logr := params.Logger
	if logr == nil {
		logr = klogr.New().WithName("default-logger")
	}
	logr = logr.WithName(params.Cluster.APIVersion).WithName(params.Cluster.Namespace).WithName(params.Cluster.Name)

	return &ClusterContext{
		Context:             parentContext,
		Cluster:             params.Cluster,
		VSphereCluster:      params.VSphereCluster,
		Client:              params.Client,
		Logger:              logr,
		vsphereClusterPatch: client.MergeFrom(params.VSphereCluster.DeepCopyObject()),
	}, nil
}

// NewClusterLoggerContext creates a new ClusterContext with the given logger context.
func NewClusterLoggerContext(parentContext *ClusterContext, loggerContext string) *ClusterContext {
	ctx := &ClusterContext{
		Context:             parentContext.Context,
		Cluster:             parentContext.Cluster,
		VSphereCluster:      parentContext.VSphereCluster,
		Client:              parentContext.Client,
		vsphereClusterPatch: parentContext.vsphereClusterPatch,
	}
	ctx.Logger = parentContext.Logger.WithName(loggerContext)
	return ctx
}

// Strings returns ClusterNamespace/ClusterName
func (c *ClusterContext) String() string {
	return fmt.Sprintf("%s/%s", c.Cluster.Namespace, c.Cluster.Name)
}

// GetCluster returns the Cluster object.
func (c *ClusterContext) GetCluster() *clusterv1.Cluster {
	return c.Cluster
}

// GetClient returns the controller client.
func (c *ClusterContext) GetClient() client.Client {
	return c.Client
}

// GetObject returns the Cluster object.
func (c *ClusterContext) GetObject() runtime.Object {
	return c.Cluster
}

// GetLogger returns the Logger.
func (c *ClusterContext) GetLogger() logr.Logger {
	return c.Logger
}

// ClusterName returns the name of the cluster.
func (c *ClusterContext) ClusterName() string {
	return c.Cluster.Name
}

// User returns the username used to access the vSphere endpoint.
func (c *ClusterContext) User() string {
	return os.Getenv("VSPHERE_USERNAME")
}

// Pass returns the password used to access the vSphere endpoint.
func (c *ClusterContext) Pass() string {
	return os.Getenv("VSPHERE_PASSWORD")
}

// CanLogin returns a flag indicating whether the cluster config has
// enough information to login to the vSphere endpoint.
func (c *ClusterContext) CanLogin() bool {
	return c.VSphereCluster.Spec.Server != "" && c.User() != ""
}

// Patch updates the object and its status on the API server.
func (c *ClusterContext) Patch() error {

	// Patch Cluster object.
	if err := c.Client.Patch(c, c.VSphereCluster, c.vsphereClusterPatch); err != nil {
		return errors.Wrapf(err, "error patching VSphereCluster %s/%s", c.Cluster.Namespace, c.Cluster.Name)
	}

	// Patch Cluster status.
	if err := c.Client.Status().Patch(c, c.VSphereCluster, c.vsphereClusterPatch); err != nil {
		return errors.Wrapf(err, "error patching VSphereCluster %s/%s status", c.Cluster.Namespace, c.Cluster.Name)
	}

	return nil
}
