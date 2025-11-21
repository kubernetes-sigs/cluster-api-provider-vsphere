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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryclient "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime/client"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type VirtualMachineReconciler struct {
	Client          client.Client
	InMemoryManager inmemoryruntime.Manager
	InMemoryClient  inmemoryclient.Client
	APIServerMux    *inmemoryserver.WorkloadClustersMux

	IsVMWaitingforIP  func() bool
	GetVCenterSession func(ctx context.Context) (*session.Session, error)
	GetVMPath         func() string

	IsVMReady     func() bool
	GetProviderID func() string
}

func (r *VirtualMachineReconciler) ReconcileNormal(ctx context.Context, cluster *clusterv1beta1.Cluster, machine *clusterv1beta1.Machine, virtualMachine client.Object) (_ ctrl.Result, reterr error) {
	resourceGroup := klog.KObj(cluster).String()
	r.InMemoryManager.AddResourceGroup(resourceGroup)
	r.InMemoryClient = r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	if result, err := r.reconcileNamespacesAndRegisterResourceGroup(ctx, cluster, resourceGroup); err != nil || !result.IsZero() {
		return result, err
	}

	conditionsTracker, err := r.getConditionTracker(ctx, virtualMachine)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the conditionsTracker object and status after each reconciliation.
	defer func() {
		// NOTE: Patch on conditionsTracker will only track of provisioning process of the fake node, etcd, api server, etc.
		if err := r.InMemoryClient.Update(ctx, conditionsTracker); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	ipReconciler := r.getVMIpReconciler()
	if ret, err := ipReconciler.ReconcileIP(ctx); !ret.IsZero() || err != nil {
		return ret, err
	}

	bootstrapReconciler := r.getVMBootstrapReconciler()
	if ret, err := bootstrapReconciler.reconcileBoostrap(ctx, cluster, machine, conditionsTracker); !ret.IsZero() || err != nil {
		return ret, err
	}

	return ctrl.Result{}, nil
}

func (r *VirtualMachineReconciler) ReconcileDelete(ctx context.Context, cluster *clusterv1beta1.Cluster, machine *clusterv1beta1.Machine, virtualMachine client.Object) (_ ctrl.Result, reterr error) {
	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	r.InMemoryManager.AddResourceGroup(resourceGroup)
	r.InMemoryClient = r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	conditionsTracker, err := r.getConditionTracker(ctx, virtualMachine)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the conditionsTracker object and status after each reconciliation.
	defer func() {
		// NOTE: Patch on conditionsTracker will only track of provisioning process of the fake node, etcd, api server, etc.
		if err := r.InMemoryClient.Update(ctx, conditionsTracker); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	bootstrapReconciler := r.getVMBootstrapReconciler()
	if ret, err := bootstrapReconciler.reconcileDelete(ctx, cluster, machine, conditionsTracker); !ret.IsZero() || err != nil {
		return ret, err
	}

	controllerutil.RemoveFinalizer(virtualMachine, vcsimv1.VMFinalizer)
	return ctrl.Result{}, nil
}

func (r *VirtualMachineReconciler) reconcileNamespacesAndRegisterResourceGroup(ctx context.Context, cluster *clusterv1beta1.Cluster, resourceGroup string) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Create default Namespaces.
	for _, nsName := range []string{metav1.NamespaceDefault, metav1.NamespacePublic, metav1.NamespaceSystem} {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Labels: map[string]string{
					"kubernetes.io/metadata.name": nsName,
				},
			},
		}

		if err := r.InMemoryClient.Get(ctx, client.ObjectKeyFromObject(ns), ns); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errors.Wrapf(err, "failed to get %s Namespace", nsName)
			}

			if err := r.InMemoryClient.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
				return ctrl.Result{}, errors.Wrapf(err, "failed to create %s Namespace", nsName)
			}
		}
	}

	// Registering ResourceGroup for ControlPlaneEndpoint
	if _, err := r.APIServerMux.WorkloadClusterByResourceGroup(resourceGroup); err != nil {
		l := &vcsimv1.ControlPlaneEndpointList{}
		if err := r.Client.List(ctx, l); err != nil {
			return ctrl.Result{}, err
		}
		found := false
		for _, c := range l.Items {
			c := c
			if c.Status.Host != cluster.Spec.ControlPlaneEndpoint.Host || c.Status.Port != cluster.Spec.ControlPlaneEndpoint.Port {
				continue
			}

			listenerName := klog.KObj(&c).String()
			log.Info("Registering ResourceGroup for ControlPlaneEndpoint", "ResourceGroup", resourceGroup, "ControlPlaneEndpoint", listenerName)
			err := r.APIServerMux.RegisterResourceGroup(listenerName, resourceGroup)
			if err != nil {
				return ctrl.Result{}, err
			}
			found = true
			break
		}
		if !found {
			return ctrl.Result{}, errors.Errorf("unable to find a ControlPlaneEndpoint for host %s, port %d", cluster.Spec.ControlPlaneEndpoint.Host, cluster.Spec.ControlPlaneEndpoint.Port)
		}
	}
	return ctrl.Result{}, nil
}

func (r *VirtualMachineReconciler) getVMIpReconciler() *virtualMachineIPReconciler {
	return &virtualMachineIPReconciler{
		Client: r.Client,

		// Type specific functions; those functions wraps the differences between govmomi and supervisor types,
		// thus allowing to use the same virtualMachineIPReconciler in both scenarios.
		GetVCenterSession: r.GetVCenterSession,
		IsVMWaitingforIP:  r.IsVMWaitingforIP,
		GetVMPath:         r.GetVMPath,
	}
}

func (r *VirtualMachineReconciler) getVMBootstrapReconciler() *inMemoryMachineBootstrapReconciler {
	return &inMemoryMachineBootstrapReconciler{
		Client:          r.Client,
		InMemoryManager: r.InMemoryManager,
		APIServerMux:    r.APIServerMux,

		// Type specific functions; those functions wraps the differences between govmomi and supervisor types,
		// thus allowing to use the same inMemoryMachineBootstrapReconciler in both scenarios.
		IsVMReady:     r.IsVMReady,
		GetProviderID: r.GetProviderID,
	}
}

func (r *VirtualMachineReconciler) getConditionTracker(ctx context.Context, virtualMachine client.Object) (*infrav1.VSphereVM, error) {
	// Check if there is a conditionsTracker in the resource group.
	// The conditionsTracker is an object stored in memory with the scope of storing conditions used for keeping
	// track of the provisioning process of the fake node, etcd, api server, etc for this specific virtualMachine.
	// (the process managed by this controller).
	// NOTE: The type of the in memory conditionsTracker object doesn't matter as soon as it implements Cluster API's conditions interfaces.
	// Unfortunately vmoprv1.VirtualMachine isn't a condition getter, so we fallback on using a infrav1.VSphereVM.
	conditionsTracker := &infrav1.VSphereVM{}
	if err := r.InMemoryClient.Get(ctx, client.ObjectKeyFromObject(virtualMachine), conditionsTracker); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "failed to get conditionsTracker")
		}

		conditionsTracker = &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      virtualMachine.GetName(),
				Namespace: virtualMachine.GetNamespace(),
			},
		}
		if err := r.InMemoryClient.Create(ctx, conditionsTracker); err != nil {
			return nil, errors.Wrap(err, "failed to create conditionsTracker")
		}
	}
	return conditionsTracker, nil
}
