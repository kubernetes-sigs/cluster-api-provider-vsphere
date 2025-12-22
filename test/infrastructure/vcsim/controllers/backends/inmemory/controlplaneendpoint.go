/*
Copyright 2025 The Kubernetes Authors.

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

package inmemory

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type ControlPlaneEndpointReconciler struct {
	InMemoryManager inmemoryruntime.Manager
	APIServerMux    *inmemoryserver.WorkloadClustersMux
	PodIP           string
}

func (r *ControlPlaneEndpointReconciler) ReconcileNormal(ctx context.Context, controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling VCSim ControlPlaneEndpoint")

	// Initialize a listener for the workload cluster.
	// IMPORTANT: The fact that both the listener and the resourceGroup for a workload cluster have
	// the same name is used as assumptions in other part of the implementation.
	listenerName := klog.KObj(controlPlaneEndpoint).String()
	listener, err := r.APIServerMux.InitWorkloadClusterListener(listenerName)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to init the listener for the control plane endpoint")
	}

	controlPlaneEndpoint.Status.Host = r.PodIP // NOTE: we are replacing the listener ip with the pod ip so it will be accessible from other pods as well
	controlPlaneEndpoint.Status.Port = listener.Port()

	return ctrl.Result{}, nil
}

func (r *ControlPlaneEndpointReconciler) ReconcileDelete(ctx context.Context, controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling delete VCSim ControlPlaneEndpoint")
	listenerName := klog.KObj(controlPlaneEndpoint).String()

	// Delete the resource group hosting all the cloud resources belonging the workload cluster;
	if resourceGroup, err := r.APIServerMux.ResourceGroupByWorkloadCluster(listenerName); err == nil {
		r.InMemoryManager.DeleteResourceGroup(resourceGroup)
	}

	// Delete the listener for the workload cluster;
	if err := r.APIServerMux.DeleteWorkloadClusterListener(listenerName); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to delete the listener for the control plane endpoint")
	}

	controllerutil.RemoveFinalizer(controlPlaneEndpoint, vcsimv1.ControlPlaneEndpointFinalizer)

	return ctrl.Result{}, nil
}
