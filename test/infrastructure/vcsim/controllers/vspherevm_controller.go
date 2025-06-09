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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/finalizers"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

// TODO: implement support for CAPV deployed in arbitrary ns (TBD if we need this).
const capvNamespace = "capv-system"

type VSphereVMReconciler struct {
	Client          client.Client
	InMemoryManager inmemoryruntime.Manager
	APIServerMux    *inmemoryserver.WorkloadClustersMux

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspherevms,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vsphereclusteridentities,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *VSphereVMReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the VSphereVM instance
	vSphereVM := &infrav1.VSphereVM{}
	if err := r.Client.Get(ctx, req.NamespacedName, vSphereVM); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Add finalizer first if not set to avoid the race condition between init and delete.
	if finalizerAdded, err := finalizers.EnsureFinalizer(ctx, r.Client, vSphereVM, vcsimv1.VMFinalizer); err != nil || finalizerAdded {
		return ctrl.Result{}, err
	}

	// Fetch the owner VSphereMachine.
	vSphereMachine, err := util.GetOwnerVSphereMachine(ctx, r.Client, vSphereVM.ObjectMeta)
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
		log.Info("VSphereVM owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1beta1.ClusterNameLabel))
		return ctrl.Result{}, nil
	}
	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Return early if the object or Cluster is paused.
	if cluster.Spec.Paused || annotations.HasPaused(vSphereVM) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// Fetch the VSphereCluster.
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	vSphereCluster := &infrav1.VSphereCluster{}
	if err := r.Client.Get(ctx, key, vSphereCluster); err != nil {
		log.Info("VSphereCluster can't be retrieved")
		return ctrl.Result{}, err
	}
	log = log.WithValues("VSphereCluster", klog.KObj(vSphereCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	r.InMemoryManager.AddResourceGroup(resourceGroup)

	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

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

		if err := inmemoryClient.Get(ctx, client.ObjectKeyFromObject(ns), ns); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errors.Wrapf(err, "failed to get %s Namespace", nsName)
			}

			if err := inmemoryClient.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
				return ctrl.Result{}, errors.Wrapf(err, "failed to create %s Namespace", nsName)
			}
		}
	}

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
	// track of the provisioning process of the fake node, etcd, api server, etc for this specific vSphereVM.
	// (the process managed by this controller).
	// NOTE: The type of the in memory conditionsTracker object doesn't matter as soon as it implements Cluster API's conditions interfaces.
	conditionsTracker := &infrav1.VSphereVM{}
	if err := inmemoryClient.Get(ctx, client.ObjectKeyFromObject(vSphereVM), conditionsTracker); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrap(err, "failed to get conditionsTracker")
		}

		conditionsTracker = &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vSphereVM.Name,
				Namespace: vSphereVM.Namespace,
			},
		}
		if err := inmemoryClient.Create(ctx, conditionsTracker); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to create conditionsTracker")
		}
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(vSphereVM, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the VSphereVM + conditionsTracker object and status after each reconciliation.
	defer func() {
		// NOTE: Patch on VSphereVM will only add/remove a finalizer.
		if err := patchHelper.Patch(ctx, vSphereVM); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}

		// NOTE: Patch on conditionsTracker will only track of provisioning process of the fake node, etcd, api server, etc.
		if err := inmemoryClient.Update(ctx, conditionsTracker); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deleted machines
	if !vSphereMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster, vSphereCluster, machine, vSphereVM, conditionsTracker)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, cluster, vSphereCluster, machine, vSphereVM, conditionsTracker)
}

func (r *VSphereVMReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1beta1.Cluster, vSphereCluster *infrav1.VSphereCluster, machine *clusterv1beta1.Machine, vSphereVM *infrav1.VSphereVM, conditionsTracker *infrav1.VSphereVM) (ctrl.Result, error) {
	ipReconciler := r.getVMIpReconciler(vSphereCluster, vSphereVM)
	if ret, err := ipReconciler.ReconcileIP(ctx); !ret.IsZero() || err != nil {
		return ret, err
	}

	bootstrapReconciler := r.getVMBootstrapReconciler(vSphereVM)
	if ret, err := bootstrapReconciler.reconcileBoostrap(ctx, cluster, machine, conditionsTracker); !ret.IsZero() || err != nil {
		return ret, err
	}

	return ctrl.Result{}, nil
}

func (r *VSphereVMReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1beta1.Cluster, _ *infrav1.VSphereCluster, machine *clusterv1beta1.Machine, vSphereVM *infrav1.VSphereVM, conditionsTracker *infrav1.VSphereVM) (ctrl.Result, error) {
	bootstrapReconciler := r.getVMBootstrapReconciler(vSphereVM)
	if ret, err := bootstrapReconciler.reconcileDelete(ctx, cluster, machine, conditionsTracker); !ret.IsZero() || err != nil {
		return ret, err
	}

	controllerutil.RemoveFinalizer(vSphereVM, vcsimv1.VMFinalizer)
	return ctrl.Result{}, nil
}

func (r *VSphereVMReconciler) getVMIpReconciler(vSphereCluster *infrav1.VSphereCluster, vSphereVM *infrav1.VSphereVM) *vmIPReconciler {
	return &vmIPReconciler{
		Client: r.Client,

		// Type specific functions; those functions wraps the differences between govmomi and supervisor types,
		// thus allowing to use the same vmIPReconciler in both scenarios.
		GetVCenterSession: func(ctx context.Context) (*session.Session, error) {
			// Return a connection to the vCenter where the vSphereVM is hosted
			return r.getVCenterSession(ctx, vSphereCluster, vSphereVM)
		},
		IsVMWaitingforIP: func() bool {
			// A vSphereVM is waiting for an IP when not ready VMProvisioned condition is false with reason WaitingForIPAllocation
			return !vSphereVM.Status.Ready && v1beta1conditions.IsFalse(vSphereVM, infrav1.VMProvisionedCondition) && v1beta1conditions.GetReason(vSphereVM, infrav1.VMProvisionedCondition) == infrav1.WaitingForIPAllocationReason
		},
		GetVMPath: func() string {
			// Return vmref of the VM as it is populated already by CAPV
			return vSphereVM.Status.VMRef
		},
	}
}

func (r *VSphereVMReconciler) getVMBootstrapReconciler(vSphereVM *infrav1.VSphereVM) *vmBootstrapReconciler {
	return &vmBootstrapReconciler{
		Client:          r.Client,
		InMemoryManager: r.InMemoryManager,
		APIServerMux:    r.APIServerMux,

		// Type specific functions; those functions wraps the differences between govmomi and supervisor types,
		// thus allowing to use the same vmBootstrapReconciler in both scenarios.
		IsVMReady: func() bool {
			// A vSphereVM is ready to provision fake objects hosted on it when both ready and BiosUUID is set (bios id is required when provisioning the node to compute the Provider ID)
			return vSphereVM.Status.Ready && vSphereVM.Spec.BiosUUID != ""
		},
		GetProviderID: func() string {
			// Computes the ProviderID for the node hosted on the vSphereVM
			return util.ConvertUUIDToProviderID(vSphereVM.Spec.BiosUUID)
		},
	}
}

func (r *VSphereVMReconciler) getVCenterSession(ctx context.Context, vSphereCluster *infrav1.VSphereCluster, vSphereVM *infrav1.VSphereVM) (*session.Session, error) {
	if vSphereCluster.Spec.IdentityRef == nil {
		return nil, errors.New("vcsim do not support using credentials provided to the manager")
	}

	creds, err := identity.GetCredentials(ctx, r.Client, vSphereCluster, capvNamespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve credentials from IdentityRef")
	}

	params := session.NewParams().
		WithServer(vSphereVM.Spec.Server).
		WithDatacenter(vSphereVM.Spec.Datacenter).
		WithUserInfo(creds.Username, creds.Password).
		WithThumbprint(vSphereVM.Spec.Thumbprint)

	return session.GetOrCreate(ctx, params)
}

// SetupWithManager will add watches for this controller.
func (r *VSphereVMReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	predicateLog := ctrl.LoggerFrom(ctx).WithValues("controller", "vspherevm")

	err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.VSphereVM{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), predicateLog, r.WatchFilterValue)).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}

// Reimplementation of some functions at "sigs.k8s.io/cluster-api/util" to be compatible to v1beta1.

// getClusterV1Beta1FromMetadata returns the Cluster object (if present) using the object metadata.
func getClusterV1Beta1FromMetadata(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1beta1.Cluster, error) {
	if obj.Labels[clusterv1beta1.ClusterNameLabel] == "" {
		return nil, errors.WithStack(capiutil.ErrNoCluster)
	}
	return getClusterV1Beta1ByName(ctx, c, obj.Namespace, obj.Labels[clusterv1beta1.ClusterNameLabel])
}

// getClusterV1Beta1ByName finds and return a Cluster object using the specified params.
func getClusterV1Beta1ByName(ctx context.Context, c client.Client, namespace, name string) (*clusterv1beta1.Cluster, error) {
	cluster := &clusterv1beta1.Cluster{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := c.Get(ctx, key, cluster); err != nil {
		return nil, errors.Wrapf(err, "failed to get Cluster/%s", name)
	}

	return cluster, nil
}

// getOwnerMachineV1Beta1 returns the Machine object owning the current resource.
func getOwnerMachineV1Beta1(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1beta1.Machine, error) {
	for _, ref := range obj.GetOwnerReferences() {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}
		if ref.Kind == "Machine" && gv.Group == clusterv1beta1.GroupVersion.Group {
			return getMachineByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// getMachineByName finds and return a Machine object using the specified params.
func getMachineByName(ctx context.Context, c client.Client, namespace, name string) (*clusterv1beta1.Machine, error) {
	m := &clusterv1beta1.Machine{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}
