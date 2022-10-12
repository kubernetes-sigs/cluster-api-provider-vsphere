/*
Copyright 2022 The Kubernetes Authors.

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
	"strings"

	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
)

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;clusters,verbs=get;watch;list

const (
	nodeLabelControllerNameShort = "node-label-controller"
)

// AddNodeLabelControllerToManager adds the VM controller to the provided manager.
func AddNodeLabelControllerToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {
	var (
		controllerNameLong = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, nodeLabelControllerNameShort)
	)

	controllerContext := &context.ControllerContext{
		ControllerManagerContext: ctx,
		Name:                     nodeLabelControllerNameShort,
		Recorder:                 record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		Logger:                   ctx.Logger.WithName(nodeLabelControllerNameShort),
	}
	r := nodeLabelReconciler{
		ControllerContext:  controllerContext,
		remoteClientGetter: remote.NewClusterClient,
	}
	if _, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Machine{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles}).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(ctx))).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return false
			},
		}).
		Build(r); err != nil {
		return err
	}
	return nil
}

type nodeLabelReconciler struct {
	*context.ControllerContext

	remoteClientGetter remote.ClusterClientGetter
}

type nodeContext struct {
	Cluster *clusterv1.Cluster
	Machine *clusterv1.Machine
}

func (r nodeLabelReconciler) Reconcile(ctx goctx.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	logger := r.Logger.WithName(req.Namespace).WithName(req.Name)

	machine := &clusterv1.Machine{}
	key := client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}
	if err := r.Client.Get(ctx, key, machine); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Machine not found, won't reconcile", "machine", key)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := clusterutilv1.GetClusterFromMetadata(r.ControllerContext, r.Client, machine.ObjectMeta)
	if err == nil {
		if annotations.IsPaused(cluster, machine) {
			logger.V(4).Info("Machine linked to a cluster that is paused")
			return reconcile.Result{}, nil
		}
	}

	if !machine.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	nodeCtx := &nodeContext{
		Cluster: cluster,
		Machine: machine,
	}
	return r.reconcileNormal(nodeCtx)
}

func (r nodeLabelReconciler) reconcileNormal(ctx *nodeContext) (reconcile.Result, error) {
	logger := r.Logger.WithName(ctx.Machine.Namespace).WithName(ctx.Machine.Name)
	logger = logger.WithValues("cluster", ctx.Cluster.Name, "machine", ctx.Machine.Name)

	// Check the current labels on the machine
	labels := ctx.Machine.GetLabels()
	nodePrefixLabels := map[string]string{}
	for key, value := range labels {
		if strings.HasPrefix(key, constants.NodeLabelPrefix) {
			nodePrefixLabels[key] = value
		}
	}

	if len(nodePrefixLabels) == 0 {
		return reconcile.Result{}, nil
	}

	clusterClient, err := r.remoteClientGetter(r, nodeLabelControllerNameShort, r.Client, client.ObjectKeyFromObject(ctx.Cluster))
	if err != nil {
		logger.Info("The control plane is not ready yet", "err", err)
		return reconcile.Result{RequeueAfter: clusterNotReadyRequeueTime}, nil
	}

	node := &apiv1.Node{}
	if err := clusterClient.Get(r, client.ObjectKey{Name: ctx.Machine.GetName()}, node); err != nil {
		logger.Error(err, "unable to get node object", "node", ctx.Machine.GetName())
		return reconcile.Result{}, err
	}

	patchHelper, err := patch.NewHelper(node, clusterClient)
	if err != nil {
		return reconcile.Result{}, err
	}

	nodeLabels := node.GetLabels()
	for k, v := range nodePrefixLabels {
		nodeLabels[k] = v
	}
	node.Labels = nodeLabels
	if err := patchHelper.Patch(r, node); err != nil {
		logger.Error(err, "unable to patch node object", "node", node.Name)
		return reconcile.Result{}, err
	}

	logger.V(4).Info("Marked node with prefixed labels", "node", node.Name, "number-of-labels", len(nodePrefixLabels))
	return reconcile.Result{}, nil
}
