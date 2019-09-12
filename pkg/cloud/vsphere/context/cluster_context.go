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
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
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
	Username string // NetApp
	Password string // NetApp

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

	// NetApp - get credentials from secret
	username, password, err := getVSphereCredentials(logr, params.Client, params.Cluster)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get vsphere credentials for cluster %s", params.Cluster.Name)
	}

	return &ClusterContext{
		Context:             parentContext,
		Cluster:             params.Cluster,
		VSphereCluster:      params.VSphereCluster,
		Client:              params.Client,
		Logger:              logr,
		vsphereClusterPatch: client.MergeFrom(params.VSphereCluster.DeepCopyObject()),
		Username: username, // Netapp
		Password: password, // NetApp
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
	return c.Username
}

// Pass returns the password used to access the vSphere endpoint.
func (c *ClusterContext) Pass() string {
	return c.Password
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

// NetApp
func getVSphereCredentials(logger logr.Logger, c client.Client, cluster *clusterv1.Cluster) (string, string, error) {

	const credentialSecretNameAnnotationKey = "cluster-api-vsphere-credentials-secret-name"
	secretName, ok := cluster.Annotations[credentialSecretNameAnnotationKey]
	if !ok {
		return "", "", fmt.Errorf("vSphere credential secret name annotation missing")
	}
	if secretName == "" {
		return "", "", fmt.Errorf("vSphere credential secret name missing")
	}

	secretNamespace := cluster.ObjectMeta.Namespace

	logger.V(4).Info("Fetching vSphere credentials from secret", "secret-namespace", secretNamespace, "secret-name", secretName)

	credentialSecret := &apiv1.Secret{}
	credentialSecretKey := client.ObjectKey{
		Namespace: secretNamespace,
		Name:      secretName,
	}
	if err := c.Get(context.TODO(), credentialSecretKey, credentialSecret); err != nil {
		return "", "", errors.Wrapf(err, "error getting credentials secret %s in namespace %s", secretName, secretNamespace)
	}

	userBuf, userOk := credentialSecret.Data[constants.VSphereCredentialSecretUserKey]
	passBuf, passOk := credentialSecret.Data[constants.VSphereCredentialSecretPassKey]
	if !userOk || !passOk {
		return "", "", fmt.Errorf("improperly formatted credentials secret %q in namespace %s", secretName, secretNamespace)
	}
	username, password := string(userBuf), string(passBuf)

	logger.V(4).Info("Found vSphere credentials in secret", "secret-namespace", secretNamespace, "secret-name", secretName)

	return username, password, nil
}
