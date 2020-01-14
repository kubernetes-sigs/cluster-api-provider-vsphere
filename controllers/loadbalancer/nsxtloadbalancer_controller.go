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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	nsxtapi "github.com/vmware/go-vmware-nsxt"
	"github.com/vmware/go-vmware-nsxt/loadbalancer"
	nsxtmanager "github.com/vmware/go-vmware-nsxt/manager"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/nsxt"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

var (
	controlledTypeNSXT     = &infrav1.NSXTLoadBalancer{}
	controlledTypeNameNSXT = reflect.TypeOf(controlledTypeNSXT).Elem().Name()
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=nsxtloadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=nsxtloadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch;delete

// AddNSXTLoadBalancerControllerToManager adds the NSX-T load balancer
// controller to the provided manager.
func AddNSXTLoadBalancerControllerToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {

	var (
		controlledTypeGVK   = infrav1.GroupVersion.WithKind(controlledTypeNameNSXT)
		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(controlledTypeNameNSXT))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, controllerNameShort)
	)

	// Build the controller context.
	controllerContext := &context.ControllerContext{
		ControllerManagerContext: ctx,
		Name:                     controllerNameShort,
		Recorder:                 record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		Logger:                   ctx.Logger.WithName(controllerNameShort),
	}

	reconciler := nsxtlbReconciler{ControllerContext: controllerContext}

	return ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledTypeNSXT).
		// Watch any VSphereVM resources owned by the controlled type.
		Watches(
			&source.Kind{Type: &infrav1.VSphereVM{}},
			&handler.EnqueueRequestForOwner{OwnerType: controlledTypeNSXT, IsController: false},
		).
		// Watch the CAPI machines that are members of the control plane which
		// this NSX-TLoadBalancer servies.
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(reconciler.controlPlaneMachineToNSXToadBalancer),
			},
		).
		// Watch a GenericEvent channel for the controlled resource.
		//
		// This is useful when there are events outside of Kubernetes that
		// should cause a resource to be synchronized, such as a goroutine
		// waiting on some asynchronous, external task to complete.
		Watches(
			&source.Channel{Source: ctx.GetGenericEventChannelFor(controlledTypeGVK)},
			&handler.EnqueueRequestForObject{},
		).
		Complete(reconciler)
}

type nsxtlbReconciler struct {
	*context.ControllerContext
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r nsxtlbReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {

	// Get the NSXTLoadBalancer resource for this request.
	nsxtlb := &infrav1.NSXTLoadBalancer{}
	if err := r.Client.Get(r, req.NamespacedName, nsxtlb); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Info("NSXTLoadBalancer not found, won't reconcile", "key", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(nsxtlb, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			nsxtlb.GroupVersionKind(),
			nsxtlb.Namespace,
			nsxtlb.Name)
	}

	nsxtCfg := &nsxtapi.Configuration{
		BasePath:  "/api/v1",
		Host:      nsxtlb.Spec.Server,
		Scheme:    "https",
		UserAgent: "kubernetes/cluster-api-provider-vsphere",
		UserName:  r.ControllerContext.NSXTUsername,
		Password:  r.ControllerContext.NSXTPassword,
		Insecure:  nsxtlb.Spec.Insecure,
	}

	nsxtClient, err := nsxtapi.NewAPIClient(nsxtCfg)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Create the NSXTLoadBalancer context for this request.
	ctx := &context.NSXTLoadBalancerContext{
		ControllerContext: r.ControllerContext,
		NSXTLoadBalancer:  nsxtlb,
		Logger:            r.Logger.WithName(req.Namespace).WithName(req.Name),
		PatchHelper:       patchHelper,
		NsxtService:       nsxt.New(nsxtClient, nsxtCfg),
	}

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		// Patch the NSXTLoadBalancer resource.
		if err := ctx.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}
			ctx.Logger.Error(err, "patch failed", "resource", ctx.String())
		}
	}()

	// Handle deleted nsxtloadbalancers
	if !nsxtlb.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx)
	}

	// Fetch the CAPI Cluster.
	cluster, err := clusterutilv1.GetOwnerCluster(r, r.Client, nsxtlb.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		r.Logger.Info("Waiting for VSphereCluster Controller to set OwnerRef on NSXTLoadBalancer")
		return reconcile.Result{}, nil
	}
	ctx.Cluster = cluster

	// Handle non-deleted nsxtloadbalancers
	return r.reconcileNormal(ctx)
}

func (r nsxtlbReconciler) reconcileDelete(ctx *context.NSXTLoadBalancerContext) (reconcile.Result, error) {
	ctx.Logger.Info("Handling deleted NSXTLoadBalancer")

	nsxtLB := ctx.NSXTLoadBalancer
	nsxtService := ctx.NsxtService
	lbName := nsxtService.GetLoadBalancerName(nsxtLB)

	lbPool, exists, err := nsxtService.GetLBPoolByName(lbName)
	if err != nil {
		return reconcile.Result{}, err
	}

	if exists {
		_, err := nsxtService.Client.ServicesApi.DeleteLoadBalancerPool(nsxtService.Client.Context, lbPool.Id)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	virtualServerName := nsxtService.GetVirtualServerName(lbName, 6443)
	virtualServer, exists, err := nsxtService.GetVirtualServerByName(virtualServerName)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !exists {
		return reconcile.Result{}, nil
	}

	_, err = nsxtService.Client.ServicesApi.DeleteLoadBalancerVirtualServer(nsxtService.Client.Context, virtualServer.Id, nil)
	if err != nil {
		return reconcile.Result{}, err
	}
	// The VM is deleted so remove the finalizer.
	ctrlutil.RemoveFinalizer(ctx.NSXTLoadBalancer, infrav1.NSXTLoadBalancerFinalizer)

	return reconcile.Result{}, nil
}

func (r nsxtlbReconciler) reconcileNormal(ctx *context.NSXTLoadBalancerContext) (reconcile.Result, error) {
	// If the NSXTLoadBalancer doesn't have our finalizer, add it.
	ctrlutil.AddFinalizer(ctx.NSXTLoadBalancer, infrav1.NSXTLoadBalancerFinalizer)

	nsxtLB := ctx.NSXTLoadBalancer
	nsxtService := ctx.NsxtService
	lbName := nsxtService.GetLoadBalancerName(nsxtLB)

	virtualServers, err := nsxtService.GetVirtualServers(nsxtLB)
	if err != nil {
		return reconcile.Result{}, err
	}

	var vip string
	releaseAllocatedVIP := true

	ips := nsxtService.GetUniqueIPsFromVirtualServers(virtualServers)
	if len(ips) == 0 {
		allocation, _, err := nsxtService.Client.PoolManagementApi.AllocateOrReleaseFromIpPool(nsxtService.Client.Context, nsxtLB.Spec.VirtualIPPoolID,
			nsxtmanager.AllocationIpAddress{}, "ALLOCATE")
		if err != nil {
			return reconcile.Result{}, err
		}

		vip := allocation.AllocationId
		defer func() {
			if !releaseAllocatedVIP {
				return
			}

			// release VIP from pool if load balancer was not created successfully
			_, _, err := nsxtService.Client.PoolManagementApi.AllocateOrReleaseFromIpPool(nsxtService.Client.Context, nsxtLB.Spec.VirtualIPPoolID,
				nsxtmanager.AllocationIpAddress{AllocationId: vip}, "RELEASE")
			if err != nil {
				klog.Errorf("error releasing VIP %s after load balancer failed to provision", vip)
			}

		}()
	} else if len(ips) == 1 {
		vip = ips[0]
	} else {
		return reconcile.Result{}, fmt.Errorf("got more than 1 VIP for service %s", nsxtLB.Name)
	}

	// Get the CAPI Machine resources for the cluster.
	machineList := &clusterv1.MachineList{}
	if err := ctx.Client.List(
		ctx, machineList,
		ctrlclient.InNamespace(ctx.Cluster.Namespace),
		ctrlclient.MatchingLabels(
			map[string]string{
				clusterv1.ClusterLabelName: ctx.Cluster.Name,
			},
		)); err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err, "failed to get machines for Cluster %s %s/%s for %s",
			ctx.Cluster.GroupVersionKind(),
			ctx.Cluster.Namespace,
			ctx.Cluster.Name,
			ctx)
	}

	// Get the control plane machines.
	controlPlaneMachines := clusterutilv1.GetControlPlaneMachinesFromList(machineList)

	lbMembers := nsxtService.MachinesToLBMembers(controlPlaneMachines)
	lbPool, err := nsxtService.CreateOrUpdateLBPool(lbName, lbMembers)
	if err != nil {
		return reconcile.Result{}, err
	}

	var newVirtualServerIDs []string
	virtualServerID := ""
	virtualServerExists := false
	for _, virtualServer := range virtualServers {
		if virtualServer.DisplayName != nsxtService.GetVirtualServerName(lbName, 6443) {
			continue
		}

		virtualServerExists = true
		virtualServerID = virtualServer.Id
		break
	}

	virtualServer := loadbalancer.LbVirtualServer{
		DisplayName:            nsxtService.GetVirtualServerName(lbName, 6443),
		Description:            fmt.Sprintf("LoadBalancer VirtualServer managed by CAPV"),
		IpProtocol:             "TCP",
		DefaultPoolMemberPorts: []string{strconv.Itoa(6443)},
		IpAddress:              vip,
		Ports:                  []string{strconv.Itoa(int(6443))},
		Enabled:                true,
		PoolId:                 lbPool.Id,
	}

	if !virtualServerExists {
		virtualServer, _, err = nsxtService.Client.ServicesApi.CreateLoadBalancerVirtualServer(nsxtService.Client.Context, virtualServer)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else {
		virtualServer, _, err = nsxtService.Client.ServicesApi.UpdateLoadBalancerVirtualServer(nsxtService.Client.Context, virtualServerID, virtualServer)
		if err != nil {
			return reconcile.Result{}, err
		}

	}

	newVirtualServerIDs = append(newVirtualServerIDs, virtualServer.Id)

	err = nsxtService.AddVirtualServersToLoadBalancer(newVirtualServerIDs, nsxtLB.Spec.LoadBalancerServiceID)
	if err != nil {
		return reconcile.Result{}, err
	}

	releaseAllocatedVIP = false

	return reconcile.Result{}, nil
}

// controlPlaneMachineToNSXTLoadBalancer is a handler.ToRequestsFunc to be
// used to trigger reconcile events for an NSXTLoadBalancer when a CAPI
// Machine is reconciled and it has IP addresses and is a member of the same
// control plane that the NSXTLoadBalancer services.
func (r nsxtlbReconciler) controlPlaneMachineToNSXToadBalancer(o handler.MapObject) []ctrl.Request {
	machine, ok := o.Object.(*clusterv1.Machine)
	if !ok {
		r.Logger.Error(errors.New("invalid type"),
			"Expected to receive a CAPI Machine resource",
			"expectedType", "Machine",
			"actualType", fmt.Sprintf("%T", o.Object))
		return nil
	}
	if !infrautilv1.IsControlPlaneMachine(machine) {
		return nil
	}
	if len(machine.Status.Addresses) == 0 {
		return nil
	}

	cluster, err := clusterutilv1.GetClusterFromMetadata(r, r.Client, machine.ObjectMeta)
	if err != nil {
		r.Logger.Error(err,
			"Machine is missing cluster label or cluster does not exist",
			"machineAPIVersion", machine.APIVersion,
			"machineKind", machine.Kind,
			"machineNamespace", machine.Namespace,
			"machineName", machine.Name)
		return nil
	}

	if cluster.Spec.InfrastructureRef == nil {
		return nil
	}

	// The infraClusterRef may not specify the namespace as it's assumed to be
	// in the same namespace as the Cluster. When the namespace is empty, set it
	// to the same namespace as the Cluster.
	infraClusterRef := cluster.Spec.InfrastructureRef
	if infraClusterRef.Namespace == "" {
		infraClusterRef.Namespace = cluster.Namespace
	}

	infraClusterKey := client.ObjectKey{
		Namespace: infraClusterRef.Namespace,
		Name:      infraClusterRef.Name,
	}
	infraCluster := &unstructured.Unstructured{Object: map[string]interface{}{}}
	infraCluster.SetAPIVersion(infraClusterRef.APIVersion)
	infraCluster.SetKind(infraClusterRef.Kind)
	if err := r.Client.Get(r, infraClusterKey, infraCluster); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Error(err,
				"Waiting on infrastructure cluster",
				"infraClusterAPIVersion", infraClusterRef.APIVersion,
				"infraClusterKind", infraClusterRef.Kind,
				"infraClusterNamespace", infraClusterRef.Namespace,
				"infraClusterName", infraClusterRef.Name)
			return nil
		}
		r.Logger.Error(err,
			"Unexpected error while waiting on infrastructure cluster",
			"infraClusterAPIVersion", infraClusterRef.APIVersion,
			"infraClusterKind", infraClusterRef.Kind,
			"infraClusterNamespace", infraClusterRef.Namespace,
			"infraClusterName", infraClusterRef.Name)
		return nil
	}

	loadBalancerRef := &corev1.ObjectReference{}
	if err := clusterutilv1.UnstructuredUnmarshalField(infraCluster, loadBalancerRef, "spec", "loadBalancerRef"); err != nil {
		if err != clusterutilv1.ErrUnstructuredFieldNotFound {
			r.Logger.Error(err,
				"Unexpected error getting infrastructure cluster's spec.loadBalancerRef",
				"infraClusterAPIVersion", infraCluster.GetAPIVersion(),
				"infraClusterKind", infraCluster.GetKind(),
				"infraClusterNamespace", infraCluster.GetNamespace(),
				"infraClusterName", infraCluster.GetName())
		}
		return nil
	}

	// The loadBalancerRef may not specify the namespace as it's assumed to be
	// in the same namespace as the Cluster. When the namespace is empty, set it
	// to the same namespace as the Cluster.
	if loadBalancerRef.Namespace == "" {
		loadBalancerRef.Namespace = cluster.Namespace
	}

	if loadBalancerRef.Name == "" {
		r.Logger.Error(err, "Infrastructure cluster's spec.loadBalancerRef.Name is empty",
			"infraClusterAPIVersion", infraCluster.GetAPIVersion(),
			"infraClusterKind", infraCluster.GetKind(),
			"infraClusterNamespace", infraCluster.GetNamespace(),
			"infraClusterName", infraCluster.GetName(),
			"loadBalancerRefAPIVersion", loadBalancerRef.APIVersion,
			"loadBalancerRefKind", loadBalancerRef.Kind,
			"loadBalancerRefNamespace", loadBalancerRef.Namespace,
			"loadBalancerRefName", loadBalancerRef.Name)
		return nil
	}

	if loadBalancerRef.Kind != controlledTypeNameNSXT {
		return nil
	}

	if loadBalancerRef.APIVersion != infrav1.GroupVersion.String() {
		return nil
	}

	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: loadBalancerRef.Namespace,
			Name:      loadBalancerRef.Name,
		},
	}}
}
