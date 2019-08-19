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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/pkg/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

const waitForClusterInfrastructureReadyDuration = 15 * time.Second //nolint

// VSphereMachineReconciler reconciles a VSphereMachine object
type VSphereMachineReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines/status,verbs=get;update;patch

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *VSphereMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	parentContext := goctx.Background()

	logger := r.Log.
		WithName(controllerName).
		WithName(fmt.Sprintf("namespace=%s", req.Namespace)).
		WithName(fmt.Sprintf("vsphereMachine=%s", req.Name))

	// Fetch the VSphereMachine instance.
	vsphereMachine := &infrav1.VSphereMachine{}
	err := r.Get(parentContext, req.NamespacedName, vsphereMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	logger = logger.WithName(vsphereMachine.APIVersion)

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(parentContext, r.Client, vsphereMachine.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machine == nil {
		logger.Info("Waiting for Machine Controller to set OwnerRef on VSphereMachine")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	logger = logger.WithName(fmt.Sprintf("machine=%s", machine.Name))

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(parentContext, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithName(fmt.Sprintf("cluster=%s", cluster.Name))

	vsphereCluster := &infrav1.VSphereCluster{}

	vsphereClusterName := client.ObjectKey{
		Namespace: vsphereMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(parentContext, vsphereClusterName, vsphereCluster); err != nil {
		logger.Info("Waiting for VSphereCluster")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	logger = logger.WithName(fmt.Sprintf("vsphereCluster=%s", vsphereCluster.Name))

	// Create the cluster context.
	clusterContext, err := context.NewClusterContext(&context.ClusterContextParams{
		Context:        parentContext,
		Cluster:        cluster,
		VSphereCluster: vsphereCluster,
		Client:         r.Client,
		Logger:         logger,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create context: %+v", err)
	}

	// Create the machine context
	machineContext, err := context.NewMachineContextFromClusterContext(
		clusterContext,
		machine,
		vsphereMachine)
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create machine context: %+v", err)
	}

	// Always close the context when exiting this function so we can persist any VSphereMachine changes.
	defer func() {
		if err := machineContext.Patch(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !vsphereMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machineContext)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(machineContext)
}

// SetupWithManager adds this controller to the provided manager.
func (r *VSphereMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.VSphereMachine{}).Watches(
		&source.Kind{Type: &clusterv1.Machine{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: util.MachineToInfrastructureMapFunc(schema.GroupVersionKind{
				Group:   infrav1.SchemeBuilder.GroupVersion.Group,
				Version: infrav1.SchemeBuilder.GroupVersion.Version,
				Kind:    "VSphereMachine",
			}),
		},
	).Complete(r)
}

func (r *VSphereMachineReconciler) reconcileDelete(ctx *context.MachineContext) (reconcile.Result, error) {
	ctx.Logger.Info("Handling deleted VSphereMachine")

	// The VM is deleted so remove the finalizer.
	ctx.VSphereMachine.Finalizers = util.Filter(ctx.VSphereMachine.Finalizers, infrav1.MachineFinalizer)

	return reconcile.Result{}, nil
}

func (r *VSphereMachineReconciler) reconcileNormal(ctx *context.MachineContext) (reconcile.Result, error) {
	// If the VSphereMachine is in an error state, return early.
	if ctx.VSphereMachine.Status.ErrorReason != nil || ctx.VSphereMachine.Status.ErrorMessage != nil {
		ctx.Logger.Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the VSphereMachine doesn't have our finalizer, add it.
	if !util.Contains(ctx.VSphereMachine.Finalizers, infrav1.MachineFinalizer) {
		ctx.VSphereMachine.Finalizers = append(ctx.VSphereMachine.Finalizers, infrav1.MachineFinalizer)
	}

	if !ctx.Cluster.Status.InfrastructureReady {
		ctx.Logger.Info("Cluster infrastructure is not ready yet, requeuing machine")
		return reconcile.Result{RequeueAfter: waitForClusterInfrastructureReadyDuration}, nil
	}

	// Make sure bootstrap data is available and populated.
	if ctx.Machine.Spec.Bootstrap.Data == nil {
		ctx.Logger.Info("Waiting for bootstrap data to be available")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return reconcile.Result{}, nil
}
