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

package containers

import (
	"context"
	"time"

	dockerclient "github.com/docker/docker/client"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type ContainerReconciler struct {
	Client       client.Client
	dockerClient *dockerclient.Client
}

func (r *ContainerReconciler) ReconcileNormal(ctx context.Context, cluster *clusterv1beta1.Cluster, machine *clusterv1beta1.Machine, virtualMachine client.Object) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Make sure bootstrap data is available and populated.
	// NOTE: we are not using bootstrap data, but we wait for it in order to simulate a real machine provisioning workflow.
	if machine.Spec.Bootstrap.DataSecretName == nil {
		if !util.IsControlPlaneMachine(machine) && !v1beta1conditions.IsTrue(cluster, clusterv1beta1.ControlPlaneInitializedCondition) {
			log.Info("Waiting for the control plane to be initialized")
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil // keep requeueing since we don't have a watch on machines // TODO: check if we can avoid this
		}

		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil // keep requeueing since we don't have a watch on machines // TODO: check if we can avoid this
	}

	return ctrl.Result{}, nil
}

func (r *ContainerReconciler) ReconcileDelete(ctx context.Context, cluster *clusterv1beta1.Cluster, machine *clusterv1beta1.Machine, virtualMachine client.Object) (_ ctrl.Result, reterr error) {

	controllerutil.RemoveFinalizer(virtualMachine, vcsimv1.VMFinalizer)
	return ctrl.Result{}, nil
}
