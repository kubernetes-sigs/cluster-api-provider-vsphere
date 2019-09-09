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

package controllers

import (
	goctx "context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/config"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

// ErrNoOwnerCluster is returned when a vsphereCluster has no ownerRef to a cluster
var ErrNoOwnerCluster = errors.New("No owner found for vsphereCluster")

// ErrVsphereClusterNotFound is returned when no vsphereCluster is found
var ErrVsphereClusterNotFound = errors.New("vsphereCluster not found")

// GetClusterContext creates clusterContext wrapped with logs
func GetClusterContext(req ctrl.Request, logger logr.Logger, client client.Client) (ctx *context.ClusterContext, _ ctrl.Result, reterr error) {
	parentContext := goctx.Background()

	logger = logger.WithName(fmt.Sprintf("namespace=%s", req.Namespace)).
		WithName(fmt.Sprintf("vsphereCluster=%s", req.Name))

	// Fetch the VSphereCluster instance
	vsphereCluster := &infrav1.VSphereCluster{}
	reterr = client.Get(parentContext, req.NamespacedName, vsphereCluster)
	if reterr != nil {
		return nil, reconcile.Result{}, ErrVsphereClusterNotFound
	}

	logger = logger.WithName(vsphereCluster.APIVersion)

	// Fetch the Cluster.
	cluster, reterr := clusterutilv1.GetOwnerCluster(parentContext, client, vsphereCluster.ObjectMeta)
	if reterr != nil {
		return nil, reconcile.Result{}, reterr
	}
	if cluster == nil {
		logger.Info("Waiting for Cluster Controller to set OwnerRef on VSphereCluster")
		return nil, reconcile.Result{RequeueAfter: config.DefaultRequeue}, ErrNoOwnerCluster
	}

	logger = logger.WithName(fmt.Sprintf("cluster=%s", cluster.Name))

	// Create the context.
	ctx, reterr = context.NewClusterContext(&context.ClusterContextParams{
		Context:        parentContext,
		Cluster:        cluster,
		VSphereCluster: vsphereCluster,
		Client:         client,
		Logger:         logger,
	})
	if reterr != nil {
		return nil, reconcile.Result{}, errors.Errorf("failed to create cluster context: %+v", reterr)
	}
	return ctx, reconcile.Result{}, nil
}
