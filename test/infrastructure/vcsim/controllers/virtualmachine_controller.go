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
	"time"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	vcsimhelpers "sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/framework/vmoperator"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type VirtualMachineReconciler struct {
	Client            client.Client
	InMemoryManager   inmemoryruntime.Manager
	APIServerMux      *inmemoryserver.WorkloadClustersMux
	EnableKeepAlive   bool
	KeepAliveDuration time.Duration

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
	machine, err := capiutil.GetOwnerMachine(ctx, r.Client, vSphereMachine.ObjectMeta)
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
	cluster, err := capiutil.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("VSphereMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.ClusterNameLabel))
		return ctrl.Result{}, nil
	}
	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, virtualMachine) {
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

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	r.InMemoryManager.AddResourceGroup(resourceGroup)

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

	// Check if there is a conditionsTracker in the resource group.
	// The conditionsTracker is an object stored in memory with the scope of storing conditions used for keeping
	// track of the provisioning process of the fake node, etcd, api server, etc for this specific virtualMachine.
	// (the process managed by this controller).
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()
	// NOTE: The type of the in memory conditionsTracker object doesn't matter as soon as it implements Cluster API's conditions interfaces.
	// Unfortunately vmoprv1.VirtualMachine isn't a condition getter, so we fallback on using a infrav1.VSphereVM.
	conditionsTracker := &infrav1.VSphereVM{}
	if err := inmemoryClient.Get(ctx, client.ObjectKeyFromObject(virtualMachine), conditionsTracker); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrap(err, "failed to get conditionsTracker")
		}

		conditionsTracker = &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      virtualMachine.Name,
				Namespace: virtualMachine.Namespace,
			},
		}
		if err := inmemoryClient.Create(ctx, conditionsTracker); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to create conditionsTracker")
		}
	}

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

		// NOTE: Patch on conditionsTracker will only track of provisioning process of the fake node, etcd, api server, etc.
		if err := inmemoryClient.Update(ctx, conditionsTracker); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deleted machines
	if !vSphereMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster, machine, virtualMachine, conditionsTracker)
	}

	// Add finalizer first if not set to avoid the race condition between init and delete.
	// Note: Finalizers in general can only be added when the deletionTimestamp is not set.
	if !controllerutil.ContainsFinalizer(virtualMachine, VMFinalizer) {
		controllerutil.AddFinalizer(virtualMachine, VMFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, cluster, machine, virtualMachine, conditionsTracker)
}

func (r *VirtualMachineReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, virtualMachine *vmoprv1.VirtualMachine, conditionsTracker *infrav1.VSphereVM) (ctrl.Result, error) {
	ipReconciler := r.getVMIpReconciler(cluster, virtualMachine)
	if ret, err := ipReconciler.ReconcileIP(ctx); !ret.IsZero() || err != nil {
		return ret, err
	}

	bootstrapReconciler := r.getVMBootstrapReconciler(virtualMachine)
	if ret, err := bootstrapReconciler.reconcileBoostrap(ctx, cluster, machine, conditionsTracker); !ret.IsZero() || err != nil {
		return ret, err
	}

	return ctrl.Result{}, nil
}

func (r *VirtualMachineReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, virtualMachine *vmoprv1.VirtualMachine, conditionsTracker *infrav1.VSphereVM) (ctrl.Result, error) {
	bootstrapReconciler := r.getVMBootstrapReconciler(virtualMachine)
	if ret, err := bootstrapReconciler.reconcileDelete(ctx, cluster, machine, conditionsTracker); !ret.IsZero() || err != nil {
		return ret, err
	}

	controllerutil.RemoveFinalizer(virtualMachine, VMFinalizer)
	return ctrl.Result{}, nil
}

func (r *VirtualMachineReconciler) getVMIpReconciler(cluster *clusterv1.Cluster, virtualMachine *vmoprv1.VirtualMachine) *vmIPReconciler {
	return &vmIPReconciler{
		Client:            r.Client,
		EnableKeepAlive:   r.EnableKeepAlive,
		KeepAliveDuration: r.KeepAliveDuration,

		// Type specific functions; those functions wraps the differences between govmomi and supervisor types,
		// thus allowing to use the same vmIPReconciler in both scenarios.
		GetVCenterSession: func(ctx context.Context) (*session.Session, error) {
			// Return a connection to the vCenter where the virtualMachine is hosted
			return r.getVCenterSession(ctx)
		},
		IsVMWaitingforIP: func() bool {
			// A virtualMachine is waiting for an IP when PoweredOn but without an Ip.
			return virtualMachine.Status.PowerState == vmoprv1.VirtualMachinePoweredOn && virtualMachine.Status.VmIp == ""
		},
		GetVMPath: func() string {
			// The vm operator always create VMs under a sub-folder with named like the cluster.
			datacenter := 0
			return vcsimhelpers.VMPath(datacenter, path.Join(cluster.Name, virtualMachine.Name))
		},
	}
}

func (r *VirtualMachineReconciler) getVMBootstrapReconciler(virtualMachine *vmoprv1.VirtualMachine) *vmBootstrapReconciler {
	return &vmBootstrapReconciler{
		Client:          r.Client,
		InMemoryManager: r.InMemoryManager,
		APIServerMux:    r.APIServerMux,

		// Type specific functions; those functions wraps the differences between govmomi and supervisor types,
		// thus allowing to use the same vmBootstrapReconciler in both scenarios.
		IsVMReady: func() bool {
			// A virtualMachine is ready to provision fake objects hosted on it when PoweredOn, with a primary Ip assigned and BiosUUID is set (bios id is required when provisioning the node to compute the Provider ID).
			return virtualMachine.Status.PowerState == vmoprv1.VirtualMachinePoweredOn && virtualMachine.Status.VmIp != "" && virtualMachine.Status.BiosUUID != ""
		},
		GetProviderID: func() string {
			// Computes the ProviderID for the node hosted on the virtualMachine
			return util.ConvertUUIDToProviderID(virtualMachine.Status.BiosUUID)
		},
	}
}

func (r *VirtualMachineReconciler) getVCenterSession(ctx context.Context) (*session.Session, error) {
	return vmoperator.GetVCenterSession(ctx, r.Client, r.EnableKeepAlive, r.KeepAliveDuration)
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
