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

package vmware

import (
	"context"

	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apitypes "k8s.io/apimachinery/pkg/types"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbldr "sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinegroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinegroups/status,verbs=get
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch

// AddVirtualMachineGroupControllerToManager adds the VirtualMachineGroup controller to the provided
// manager.
func AddVirtualMachineGroupControllerToManager(ctx context.Context, controllerManagerCtx *capvcontext.ControllerManagerContext, mgr manager.Manager, options controller.Options) error {
	predicateLog := ctrl.LoggerFrom(ctx).WithValues("controller", "virtualmachinegroup")

	reconciler := &VirtualMachineGroupReconciler{
		Client:   controllerManagerCtx.Client,
		Recorder: mgr.GetEventRecorderFor("virtualmachinegroup-controller"),
	}

	// Predicate: only allow VMG with the cluster-name label. Ensures the controller only works on VMG objects created by CAPV.
	hasClusterNameLabel := predicate.NewPredicateFuncs(func(obj ctrlclient.Object) bool {
		labels := obj.GetLabels()
		if labels == nil {
			return false
		}
		_, ok := labels[clusterv1.ClusterNameLabel]
		return ok
	})

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&vmoprv1.VirtualMachineGroup{}).
		WithOptions(options).
		WithEventFilter(hasClusterNameLabel).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(reconciler.ClusterToVirtualMachineGroup),
		).
		Watches(
			&vmwarev1.VSphereMachine{},
			handler.EnqueueRequestsFromMapFunc(reconciler.VSphereMachineToVirtualMachineGroup),
			ctrlbldr.WithPredicates(
				predicate.Funcs{
					UpdateFunc:  func(e event.UpdateEvent) bool { return false },
					CreateFunc:  func(event.CreateEvent) bool { return true },
					DeleteFunc:  func(event.DeleteEvent) bool { return true },
					GenericFunc: func(event.GenericEvent) bool { return false },
				}),
		).
		WithEventFilter(predicates.ResourceHasFilterLabel(mgr.GetScheme(), predicateLog, controllerManagerCtx.WatchFilterValue))

	return builder.Complete(reconciler)
}

// ClusterToVirtualMachineGroup maps Cluster events to VirtualMachineGroup reconcile requests.
func (r *VirtualMachineGroupReconciler) ClusterToVirtualMachineGroup(ctx context.Context, a ctrlclient.Object) []reconcile.Request {
	cluster, ok := a.(*clusterv1.Cluster)
	if !ok {
		return nil
	}

	// Always enqueue a request for the "would-be VMG"
	return []reconcile.Request{{
		NamespacedName: apitypes.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		},
	}}
}

// VSphereMachineToVirtualMachineGroup maps VSphereMachine events to VirtualMachineGroup reconcile requests.
// This handler only processes VSphereMachine objects for Day-2 operations when VMG could be found, ensuring
// VMG member list in sync with VSphereMachines. If no corresponding VMG is found, this is a no-op.
func (r *VirtualMachineGroupReconciler) VSphereMachineToVirtualMachineGroup(ctx context.Context, a ctrlclient.Object) []reconcile.Request {
	vSphereMachine, ok := a.(*vmwarev1.VSphereMachine)
	if !ok {
		return nil
	}

	clusterName, ok := vSphereMachine.Labels[clusterv1.ClusterNameLabel]
	if !ok || clusterName == "" {
		return nil
	}

	vmg := &vmoprv1.VirtualMachineGroup{}
	err := r.Client.Get(ctx, apitypes.NamespacedName{
		Namespace: vSphereMachine.Namespace,
		Name:      clusterName,
	}, vmg)

	if err != nil {
		return nil
	}

	return []reconcile.Request{{
		NamespacedName: apitypes.NamespacedName{
			Namespace: vmg.Namespace,
			Name:      vmg.Name,
		},
	}}
}
