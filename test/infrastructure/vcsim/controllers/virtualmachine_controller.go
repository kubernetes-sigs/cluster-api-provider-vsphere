/*
Copyright 2024 The Kubernetes Authors.

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
	"context"
	"fmt"
	"path"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/finalizers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	vcsimhelpers "sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/framework/vmoperator"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/controllers/backends"
	inmemorybackend "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/controllers/backends/inmemory"
)

type VirtualMachineReconciler struct {
	Client          client.Client
	InMemoryManager inmemoryruntime.Manager
	APIServerMux    *inmemoryserver.WorkloadClustersMux

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachines,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=vsphereclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *VirtualMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the VirtualMachine instance
	virtualMachine := &vmoprv1.VirtualMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, virtualMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Add finalizer first if not set to avoid the race condition between init and delete.
	if finalizerAdded, err := finalizers.EnsureFinalizer(ctx, r.Client, virtualMachine, vcsimv1.VMFinalizer); err != nil || finalizerAdded {
		return ctrl.Result{}, err
	}

	// Fetch the owner VSphereMachine.
	vSphereMachine, err := util.GetOwnerVMWareMachine(ctx, r.Client, virtualMachine.ObjectMeta)
	// vsphereMachine can be nil in cases where custom mover other than clusterctl
	// moves the resources without ownerreferences set
	// in that case nil vsphereMachine can cause panic and CrashLoopBackOff the pod
	// preventing vspheremachine_controller from setting the ownerref
	if err != nil || vSphereMachine == nil {
		log.Info("Owner VSphereMachine not found, won't reconcile")
		return reconcile.Result{}, err
	}
	log = log.WithValues("VSphereMachine", klog.KObj(vSphereMachine))
	ctx = ctrl.LoggerInto(ctx, log)

	// Fetch the Machine.
	machine, err := getOwnerMachineV1Beta1(ctx, r.Client, vSphereMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Waiting for Machine Controller to set OwnerRef on VSphereMachine")
		return ctrl.Result{}, nil
	}
	log = log.WithValues("Machine", klog.KObj(machine))
	ctx = ctrl.LoggerInto(ctx, log)

	// Fetch the Cluster.
	cluster, err := getClusterV1Beta1FromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("VSphereMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1beta1.ClusterNameLabel))
		return ctrl.Result{}, nil
	}
	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Return early if the object or Cluster is paused.
	if cluster.Spec.Paused || annotations.HasPaused(virtualMachine) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// Fetch the VSphereCluster.
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	vsphereCluster := &vmwarev1.VSphereCluster{}
	if err := r.Client.Get(ctx, key, vsphereCluster); err != nil {
		log.Info("VSphereCluster can't be retrieved")
		return ctrl.Result{}, err
	}
	log = log.WithValues("VSphereCluster", klog.KObj(vsphereCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(virtualMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the VSphereVM + conditionsTracker object and status after each reconciliation.
	defer func() {
		// NOTE: Patch on VSphereVM will only add/remove a finalizer.
		if err := patchHelper.Patch(ctx, virtualMachine); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	backendReconciler := r.backendReconcilerFactory(ctx, cluster, machine, virtualMachine)

	// Handle deleted machines
	if !vSphereMachine.DeletionTimestamp.IsZero() {
		return backendReconciler.ReconcileDelete(ctx, cluster, machine, virtualMachine)
	}

	// Handle non-deleted machines
	return backendReconciler.ReconcileNormal(ctx, cluster, machine, virtualMachine)
}

func (r *VirtualMachineReconciler) backendReconcilerFactory(_ context.Context, cluster *clusterv1beta1.Cluster, machine *clusterv1beta1.Machine, virtualMachine *vmoprv1.VirtualMachine) backends.VirtualMachineReconciler {
	return &inmemorybackend.VirtualMachineReconciler{
		Client:          r.Client,
		InMemoryManager: r.InMemoryManager,
		APIServerMux:    r.APIServerMux,
		GetProviderID: func() string {
			// Computes the ProviderID for the node hosted on the virtualMachine
			return util.ConvertUUIDToProviderID(virtualMachine.Status.BiosUUID)
		},
		GetVCenterSession: func(ctx context.Context) (*session.Session, error) {
			// Return a connection to the vCenter where the virtualMachine is hosted
			return vmoperator.GetVCenterSession(ctx, r.Client)
		},
		GetVMPath: func() string {
			// The vm operator always create VMs under a sub-folder with named like the cluster.
			datacenter := 0
			return vcsimhelpers.VMPath(datacenter, path.Join(cluster.Name, virtualMachine.Name))
		},
		IsVMReady: func() bool {
			// A virtualMachine is ready to provision fake objects hosted on it when PoweredOn, with a primary Ip assigned and BiosUUID is set (bios id is required when provisioning the node to compute the Provider ID).
			return virtualMachine.Status.PowerState == vmoprv1.VirtualMachinePowerStateOn && virtualMachine.Status.Network != nil && (virtualMachine.Status.Network.PrimaryIP4 != "" || virtualMachine.Status.Network.PrimaryIP6 != "") && virtualMachine.Status.BiosUUID != ""
		},
		IsVMWaitingforIP: func() bool {
			// A virtualMachine is waiting for an IP when PoweredOn but without an Ip.
			return virtualMachine.Status.PowerState == vmoprv1.VirtualMachinePowerStateOn && (virtualMachine.Status.Network == nil || (virtualMachine.Status.Network.PrimaryIP4 == "" && virtualMachine.Status.Network.PrimaryIP6 == ""))
		},
	}
}

// SetupWithManager will add watches for this controller.
func (r *VirtualMachineReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&vmoprv1.VirtualMachine{}).
		WithOptions(options).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}
