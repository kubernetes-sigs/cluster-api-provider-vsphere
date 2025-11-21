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

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// AddVirtualMachineGroupControllerToManager adds the VirtualMachineGroup controller to the provided manager.
func AddVirtualMachineGroupControllerToManager(ctx context.Context, controllerManagerCtx *capvcontext.ControllerManagerContext, mgr manager.Manager, options controller.Options) error {
	predicateLog := ctrl.LoggerFrom(ctx).WithValues("controller", "virtualmachinegroup")

	reconciler := &VirtualMachineGroupReconciler{
		Client:   controllerManagerCtx.Client,
		Recorder: mgr.GetEventRecorderFor("virtualmachinegroup-controller"),
	}

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		WithOptions(options).
		// Set the controller's name explicitly to virtualmachinegroup.
		Named("virtualmachinegroup").
		Watches(
			&vmoprv1.VirtualMachineGroup{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), reconciler.Client.RESTMapper(), &clusterv1.Cluster{}),
			ctrlbldr.WithPredicates(predicates.ResourceIsChanged(mgr.GetScheme(), predicateLog)),
		).
		Watches(
			&vmwarev1.VSphereMachine{},
			handler.EnqueueRequestsFromMapFunc(reconciler.VSphereMachineToCluster),
			ctrlbldr.WithPredicates(
				predicate.Funcs{
					UpdateFunc: func(event.UpdateEvent) bool { return false },
					CreateFunc: func(e event.CreateEvent) bool {
						// Only handle VSphereMachine which belongs to a MachineDeployment
						_, found := e.Object.GetLabels()[clusterv1.MachineDeploymentNameLabel]
						return found
					},
					DeleteFunc: func(e event.DeleteEvent) bool {
						// Only handle VSphereMachine which belongs to a MachineDeployment
						_, found := e.Object.GetLabels()[clusterv1.MachineDeploymentNameLabel]
						return found
					},
					GenericFunc: func(event.GenericEvent) bool { return false },
				}),
		).
		WithEventFilter(predicates.ResourceHasFilterLabel(mgr.GetScheme(), predicateLog, controllerManagerCtx.WatchFilterValue))

	return builder.Complete(reconciler)
}

// VSphereMachineToCluster maps VSphereMachine events to Cluster reconcile requests.
func (r *VirtualMachineGroupReconciler) VSphereMachineToCluster(_ context.Context, a ctrlclient.Object) []reconcile.Request {
	vSphereMachine, ok := a.(*vmwarev1.VSphereMachine)
	if !ok {
		return nil
	}

	clusterName, ok := vSphereMachine.Labels[clusterv1.ClusterNameLabel]
	if !ok || clusterName == "" {
		return nil
	}

	return []reconcile.Request{{
		NamespacedName: apitypes.NamespacedName{
			Namespace: vSphereMachine.Namespace,
			Name:      clusterName,
		},
	}}
}
