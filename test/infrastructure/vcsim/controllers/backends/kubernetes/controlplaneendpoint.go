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

package kubernetes

import (
	"context"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type ControlPlaneEndpointReconciler struct {
	Client client.Client
}

func (r *ControlPlaneEndpointReconciler) ReconcileNormal(ctx context.Context, controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// If the controlPlaneEndpoint is already set then we are done.
	// Note: the controlPlaneEndpoint doesn't have the capability to recover from the manual deletion of the service,
	// but this is considered acceptable for vcsim testing.
	if controlPlaneEndpoint.Status.Host != "" {
		return ctrl.Result{}, nil
	}

	// Get the load balancer service.
	log.Info("Creating the Kubernetes Service acting as a cluster load balancer")
	s := lbServiceHandler{
		client:               r.Client,
		controlPlaneEndpoint: controlPlaneEndpoint,
	}

	svc, err := s.LookupOrGenerate(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Wait for the cluster IP to show up.
	if svc.Spec.ClusterIP == "" {
		return ctrl.Result{Requeue: true}, err
	}

	// If service ports are not as expected, delete the service at best effort
	// Note: this should never happen (it happens if someone change the service while being created or immediately after).
	if len(svc.Spec.Ports) != 1 {
		_ = s.Delete(ctx)
		return ctrl.Result{}, errors.Errorf("service doesn't have the expected port")
	}

	controlPlaneEndpoint.Status.Host = svc.Spec.ClusterIP
	controlPlaneEndpoint.Status.Port = int32(svc.Spec.Ports[0].Port)
	return ctrl.Result{}, nil
}

func (r *ControlPlaneEndpointReconciler) ReconcileDelete(ctx context.Context, controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Deleting the Kubernetes Service acting as a load balancer in front of all the control plane instances")
	s := lbServiceHandler{
		client:               r.Client,
		controlPlaneEndpoint: controlPlaneEndpoint,
	}

	if err := s.Delete(ctx); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Cluster infrastructure deleted")

	controllerutil.RemoveFinalizer(controlPlaneEndpoint, vcsimv1.ControlPlaneEndpointFinalizer)

	return ctrl.Result{}, nil
}
