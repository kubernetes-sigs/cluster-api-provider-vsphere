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
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/clustermodule"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachinetemplates,verbs=get;list;watch

type Reconciler struct {
	*context.ControllerContext

	ClusterModuleService clustermodule.Service
}

func NewReconciler(ctx *context.ControllerContext) Reconciler {
	return Reconciler{
		ControllerContext:    ctx,
		ClusterModuleService: clustermodule.NewService(),
	}
}

func (r Reconciler) Reconcile(ctx *context.ClusterContext) (reconcile.Result, error) {
	ctx.Logger.Info("reconcile anti affinity setup")
	if !clustermodule.IsClusterCompatible(ctx) {
		conditions.MarkFalse(ctx.VSphereCluster, infrav1.ClusterModulesAvailableCondition, infrav1.VCenterVersionIncompatibleReason, clusterv1.ConditionSeverityInfo,
			"vCenter API version %s is not compatible with cluster modules", ctx.VSphereCluster.Status.VCenterVersion)
		ctx.Logger.Info("cluster is not compatible for anti affinity",
			"api version", ctx.VSphereCluster.Status.VCenterVersion)
		return reconcile.Result{}, nil
	}

	objectMap, err := r.fetchMachineOwnerObjects(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	var errList []error
	clusterModuleSpecs := []infrav1.ClusterModule{}

	for _, mod := range ctx.VSphereCluster.Spec.ClusterModules {
		curr := mod.TargetObjectName
		if mod.ControlPlane {
			curr = appendKCPKey(curr)
		}
		if obj, ok := objectMap[curr]; !ok {
			// delete the cluster module as the object is marked for deletion
			// or already deleted.
			if err := r.ClusterModuleService.Remove(ctx, mod.ModuleUUID); err != nil {
				ctx.Logger.Error(err, "failed to delete cluster module for object",
					"name", mod.TargetObjectName, "moduleUUID", mod.ModuleUUID)
				errList = append(errList, err)
			}
			delete(objectMap, curr)
		} else {
			// verify the cluster module
			exists, err := r.ClusterModuleService.DoesExist(ctx, obj, mod.ModuleUUID)
			if err != nil {
				errList = append(errList, err)
			}
			// append the module and object info to the VSphereCluster object
			// and remove it from the object map since no new cluster module
			// needs to be created.
			if exists {
				clusterModuleSpecs = append(clusterModuleSpecs, infrav1.ClusterModule{
					ControlPlane:     obj.IsControlPlane(),
					TargetObjectName: obj.GetName(),
					ModuleUUID:       mod.ModuleUUID,
				})
				delete(objectMap, curr)
			} else {
				ctx.Logger.Info("module for object not found",
					"moduleUUID", mod.ModuleUUID,
					"object", mod.TargetObjectName)
			}
		}
	}
	if len(errList) > 0 {
		ctx.Logger.Error(kerrors.NewAggregate(errList), "errors reconciling cluster modules for cluster",
			"namespace", ctx.VSphereCluster.Namespace, "name", ctx.VSphereCluster.Name)
	}

	errList = []error{}
	for _, obj := range objectMap {
		moduleUUID, err := r.ClusterModuleService.Create(ctx, obj)
		if err != nil {
			ctx.Logger.Error(err, "failed to create cluster module for target object", "name", obj.GetName())
			errList = append(errList, err)
		}
		clusterModuleSpecs = append(clusterModuleSpecs, infrav1.ClusterModule{
			ControlPlane:     obj.IsControlPlane(),
			TargetObjectName: obj.GetName(),
			ModuleUUID:       moduleUUID,
		})
	}
	ctx.VSphereCluster.Spec.ClusterModules = clusterModuleSpecs

	if len(errList) > 0 {
		err = kerrors.NewAggregate(errList)
		ctx.Logger.Error(err, "errors reconciling cluster modules for cluster",
			"namespace", ctx.VSphereCluster.Namespace, "name", ctx.VSphereCluster.Name)
	}

	switch {
	case err != nil:
		conditions.MarkFalse(ctx.VSphereCluster, infrav1.ClusterModulesAvailableCondition, infrav1.ClusterModuleSetupFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
	case err == nil && len(clusterModuleSpecs) > 0:
		conditions.MarkTrue(ctx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)
	default:
		conditions.Delete(ctx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)
	}
	return reconcile.Result{}, err
}

func (r Reconciler) toAffinityInput(obj client.Object) []reconcile.Request {
	cluster, err := util.GetClusterFromMetadata(r, r.Client, metav1.ObjectMeta{
		Namespace:       obj.GetNamespace(),
		Labels:          obj.GetLabels(),
		OwnerReferences: obj.GetOwnerReferences(),
	})
	if err != nil {
		r.Logger.Error(err, "failed to get owner cluster")
		return nil
	}

	vsphereCluster := &infrav1.VSphereCluster{}
	if err := r.Client.Get(r, client.ObjectKey{
		Name:      cluster.Spec.InfrastructureRef.Name,
		Namespace: cluster.Namespace,
	}, vsphereCluster); err != nil {
		r.Logger.Error(err, "failed to get vSphereCluster object",
			"namespace", cluster.Namespace, "name", cluster.Spec.InfrastructureRef.Name)
		return nil
	}

	return []reconcile.Request{
		{NamespacedName: client.ObjectKeyFromObject(vsphereCluster)},
	}
}

func (r Reconciler) PopulateWatchesOnController(controller controller.Controller) error {
	if err := controller.Watch(
		&source.Kind{Type: &controlplanev1.KubeadmControlPlane{}},
		handler.EnqueueRequestsFromMapFunc(r.toAffinityInput),
		predicate.Funcs{
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return false
			},
		},
	); err != nil {
		return err
	}

	if err := controller.Watch(
		&source.Kind{Type: &clusterv1.MachineDeployment{}},
		handler.EnqueueRequestsFromMapFunc(r.toAffinityInput),
		predicate.Funcs{
			GenericFunc: func(genericEvent event.GenericEvent) bool {
				return false
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				return false
			},
		},
	); err != nil {
		return err
	}
	return nil
}

func (r Reconciler) fetchMachineOwnerObjects(ctx *context.ClusterContext) (map[string]clustermodule.Wrapper, error) {
	objects := map[string]clustermodule.Wrapper{}

	name, ok := ctx.VSphereCluster.GetLabels()[clusterv1.ClusterLabelName]
	if !ok {
		return nil, errors.Errorf("missing CAPI cluster label")
	}

	labels := map[string]string{clusterv1.ClusterLabelName: name}
	kcpList := &controlplanev1.KubeadmControlPlaneList{}
	if err := r.Client.List(
		ctx, kcpList,
		client.InNamespace(ctx.VSphereCluster.GetNamespace()),
		client.MatchingLabels(labels)); err != nil {
		return nil, errors.Wrapf(err, "failed to list control plane objects")
	}
	if len(kcpList.Items) > 1 {
		return nil, errors.Errorf("multiple control plane objects found, expected 1, found %d", len(kcpList.Items))
	}

	if len(kcpList.Items) != 0 {
		if kcp := &kcpList.Items[0]; kcp.GetDeletionTimestamp().IsZero() {
			objects[appendKCPKey(kcp.GetName())] = clustermodule.NewWrapper(kcp)
		}
	}

	mdList := &clusterv1.MachineDeploymentList{}
	if err := r.Client.List(
		ctx, mdList,
		client.InNamespace(ctx.VSphereCluster.GetNamespace()),
		client.MatchingLabels(labels)); err != nil {
		return nil, errors.Wrapf(err, "failed to list machine deployment objects")
	}
	for _, md := range mdList.Items {
		if md.DeletionTimestamp.IsZero() {
			objects[md.GetName()] = clustermodule.NewWrapper(md.DeepCopy())
		}
	}
	return objects, nil
}

// appendKCPKey adds the prefix "kcp" to the name of the object
// This is used to separate a single KCP object from the Machine Deployment objects
// having the same name.
func appendKCPKey(name string) string {
	return "kcp" + name
}
