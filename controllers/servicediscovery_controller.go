/*
Copyright 2021 The Kubernetes Authors.

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
	"net"
	"net/url"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	vmwarecontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
)

const (
	clusterNotReadyRequeueTime     = time.Minute * 2
	ServiceDiscoveryControllerName = "servicediscovery-controller"

	supervisorLoadBalancerSvcNamespace = "kube-system"
	supervisorLoadBalancerSvcName      = "kube-apiserver-lb-svc"
	supervisorAPIServerPort            = 6443

	supervisorHeadlessSvcNamespace = "default"
	supervisorHeadlessSvcName      = "supervisor"
)

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get

// AddServiceDiscoveryControllerToManager adds the ServiceDiscovery controller to the provided manager.
func AddServiceDiscoveryControllerToManager(ctx context.Context, controllerManagerCtx *capvcontext.ControllerManagerContext, mgr manager.Manager, tracker *remote.ClusterCacheTracker, options controller.Options) error {
	var (
		controllerNameShort = ServiceDiscoveryControllerName
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", controllerManagerCtx.Namespace, controllerManagerCtx.Name, controllerNameShort)
	)

	r := &serviceDiscoveryReconciler{
		Client:                    controllerManagerCtx.Client,
		Recorder:                  record.New(mgr.GetEventRecorderFor(controllerNameLong)),
		remoteClusterCacheTracker: tracker,
	}

	configMapCache, err := cache.New(mgr.GetConfig(), cache.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
		// TODO: Reintroduce the cache sync period
		// Resync:    ctx.SyncPeriod,
		Namespaces: []string{metav1.NamespacePublic},
	})
	if err != nil {
		return errors.Wrapf(err, "failed to create configmap cache")
	}
	if err := mgr.Add(configMapCache); err != nil {
		return errors.Wrapf(err, "failed to start configmap cache")
	}
	src := source.Kind(configMapCache, &corev1.ConfigMap{})

	return ctrl.NewControllerManagedBy(mgr).For(&vmwarev1.VSphereCluster{}).
		Named(ServiceDiscoveryControllerName).
		WithOptions(options).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.serviceToClusters),
		).
		WatchesRawSource(
			src,
			handler.EnqueueRequestsFromMapFunc(r.configMapToClusters),
		).
		// watch the CAPI cluster
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestForOwner(
				mgr.GetScheme(), mgr.GetRESTMapper(),
				&vmwarev1.VSphereCluster{},
				handler.OnlyControllerOwner(),
			)).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), controllerManagerCtx.WatchFilterValue)).
		Complete(r)
}

type serviceDiscoveryReconciler struct {
	Client   client.Client
	Recorder record.Recorder

	remoteClusterCacheTracker *remote.ClusterCacheTracker
}

func (r *serviceDiscoveryReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Starting Reconcile")

	// Get the vspherecluster for this request.
	vsphereCluster := &vmwarev1.VSphereCluster{}
	// Note: VSphereCluster doesn't have to be added to the logger as controller-runtime
	// already adds the reconciled object (which is VSphereCluster).
	if err := r.Client.Get(ctx, req.NamespacedName, vsphereCluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("VSphereCluster not found, won't reconcile")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(vsphereCluster, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(
			err,
			"failed to init patch helper for %s %s/%s",
			vsphereCluster.GroupVersionKind(),
			vsphereCluster.Namespace,
			vsphereCluster.Name)
	}

	// Create the cluster context for this request.
	clusterContext := &vmwarecontext.ClusterContext{
		VSphereCluster: vsphereCluster,
		PatchHelper:    patchHelper,
	}

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		if err := clusterContext.Patch(ctx); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// This type of controller doesn't care about delete events.
	if !vsphereCluster.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	cluster, err := clusterutilv1.GetClusterFromMetadata(ctx, r.Client, vsphereCluster.ObjectMeta)
	if err != nil {
		log.Error(err, "unable to get Cluster from VSphereCluster")
		return reconcile.Result{RequeueAfter: clusterNotReadyRequeueTime}, nil
	}

	// We cannot proceed until we are able to access the target cluster. Until
	// then just return a no-op and wait for the next sync.
	guestClient, err := r.remoteClusterCacheTracker.GetClient(ctx, client.ObjectKeyFromObject(cluster))
	if err != nil {
		if errors.Is(err, remote.ErrClusterLocked) {
			log.V(5).Info("Requeuing because another worker has the lock on the ClusterCacheTracker")
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "The control plane is not ready yet")
		return reconcile.Result{RequeueAfter: clusterNotReadyRequeueTime}, nil
	}

	// Defer to the Reconciler for reconciling a non-delete event.
	return r.ReconcileNormal(ctx, &vmwarecontext.GuestClusterContext{
		ClusterContext: clusterContext,
		GuestClient:    guestClient,
	})
}

func (r *serviceDiscoveryReconciler) ReconcileNormal(ctx context.Context, guestClusterCtx *vmwarecontext.GuestClusterContext) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Reconciling Service Discovery")

	if err := r.reconcileSupervisorHeadlessService(ctx, guestClusterCtx); err != nil {
		conditions.MarkFalse(guestClusterCtx.VSphereCluster, vmwarev1.ServiceDiscoveryReadyCondition, vmwarev1.SupervisorHeadlessServiceSetupFailedReason,
			clusterv1.ConditionSeverityWarning, err.Error())
		return reconcile.Result{}, errors.Wrapf(err, "failed to configure supervisor headless service for %v", guestClusterCtx.VSphereCluster)
	}

	return reconcile.Result{}, nil
}

// Setup a local k8s service in the target cluster that proxies to the Supervisor Cluster API Server. The add-ons are
// dependent on this local service to connect to the Supervisor Cluster.
func (r *serviceDiscoveryReconciler) reconcileSupervisorHeadlessService(ctx context.Context, guestClusterCtx *vmwarecontext.GuestClusterContext) error {
	log := ctrl.LoggerFrom(ctx)

	// Create the headless service to the supervisor api server on the target cluster.
	supervisorPort := vmwarev1.SupervisorAPIServerPort
	svc := NewSupervisorHeadlessService(vmwarev1.SupervisorHeadlessSvcPort, supervisorPort)
	if err := guestClusterCtx.GuestClient.Create(ctx, svc); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "cannot create k8s service %s/%s in ", svc.Namespace, svc.Name)
	}

	supervisorHost, err := r.getSupervisorAPIServerAddress(ctx)
	if err != nil {
		// Note: We have watches on the LB Svc (VIP) & the cluster-info configmap (FIP). There is no need to return an error to keep
		// re-trying.
		conditions.MarkFalse(guestClusterCtx.VSphereCluster, vmwarev1.ServiceDiscoveryReadyCondition, vmwarev1.SupervisorHeadlessServiceSetupFailedReason,
			clusterv1.ConditionSeverityWarning, err.Error())
		return nil
	}

	log.Info("Discovered supervisor apiserver address", "host", supervisorHost, "port", supervisorPort)
	// CreateOrPatch the newEndpoints with the discovered supervisor api server address
	newEndpoints := NewSupervisorHeadlessServiceEndpoints(
		supervisorHost,
		supervisorPort,
	)
	endpointsKey := types.NamespacedName{
		Namespace: newEndpoints.Namespace,
		Name:      newEndpoints.Name,
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: newEndpoints.Namespace,
			Name:      newEndpoints.Name,
		},
	}
	result, err := controllerutil.CreateOrPatch(
		ctx,
		guestClusterCtx.GuestClient,
		endpoints,
		func() error {
			endpoints.Subsets = newEndpoints.Subsets
			return nil
		})
	if err != nil {
		return errors.Wrapf(
			err,
			"cannot create k8s service endpoints %s",
			endpointsKey,
		)
	}

	endpointsSubsetsStr := fmt.Sprintf("%+v", endpoints.Subsets)

	switch result {
	case controllerutil.OperationResultNone:
		log.Info(
			"no update required for k8s service endpoints",
			"endpointsKey",
			endpointsKey,
			"endpointsSubsets",
			endpointsSubsetsStr,
		)
	case controllerutil.OperationResultCreated:
		log.Info(
			"created k8s service endpoints",
			"endpointsKey",
			endpointsKey,
			"endpointsSubsets",
			endpointsSubsetsStr,
		)
	case controllerutil.OperationResultUpdated:
		log.Info(
			"updated k8s service endpoints",
			"endpointsKey",
			endpointsKey,
			"endpointsSubsets",
			endpointsSubsetsStr,
		)
	default:
		log.Error(
			nil,
			"unexpected result during createOrPatch k8s service endpoints",
			"endpointsKey",
			endpointsKey,
			"endpointsSubsets",
			endpointsSubsetsStr,
			"result",
			result,
		)
	}

	conditions.MarkTrue(guestClusterCtx.VSphereCluster, vmwarev1.ServiceDiscoveryReadyCondition)
	return nil
}

func (r *serviceDiscoveryReconciler) getSupervisorAPIServerAddress(ctx context.Context) (string, error) {
	log := ctrl.LoggerFrom(ctx)

	// Discover the supervisor api server address
	// 1. Check if a k8s service "kube-system/kube-apiserver-lb-svc" is available, if so, fetch the loadbalancer IP.
	// 2. If not, get the Supervisor Cluster Management Network Floating IP (FIP) from the cluster-info configmap. This is
	// to support non-NSX-T development usecases only. If we are unable to find the cluster-info configmap for some reason,
	// we log the error.
	supervisorHost, err := GetSupervisorAPIServerVIP(r.Client)
	if err != nil {
		log.Error(err, "Unable to discover supervisor apiserver virtual ip, fallback to floating ip")
		supervisorHost, err = GetSupervisorAPIServerFIP(r.Client)
		if err != nil {
			return "", errors.Wrapf(err, "Unable to discover supervisor apiserver address")
		}
	}
	return supervisorHost, nil
}

func NewSupervisorHeadlessService(port, targetPort int) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmwarev1.SupervisorHeadlessSvcName,
			Namespace: vmwarev1.SupervisorHeadlessSvcNamespace,
		},
		Spec: corev1.ServiceSpec{
			// Note: This will be a headless service with no selectors. The endpoints will be manually created.
			ClusterIP: corev1.ClusterIPNone,
			Ports: []corev1.ServicePort{
				{
					Port:       int32(port),
					TargetPort: intstr.FromInt(targetPort),
				},
			},
		},
	}
}

func NewSupervisorHeadlessServiceEndpoints(targetHost string, targetPort int) *corev1.Endpoints {
	var endpointAddr corev1.EndpointAddress
	if ip := net.ParseIP(targetHost); ip != nil {
		endpointAddr.IP = ip.String()
	} else {
		endpointAddr.Hostname = targetHost
	}
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmwarev1.SupervisorHeadlessSvcName,
			Namespace: vmwarev1.SupervisorHeadlessSvcNamespace,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					endpointAddr,
				},
				Ports: []corev1.EndpointPort{
					{
						Port: int32(targetPort),
					},
				},
			},
		},
	}
}

func GetSupervisorAPIServerVIP(client client.Client) (string, error) {
	svc := &corev1.Service{}
	svcKey := types.NamespacedName{Name: vmwarev1.SupervisorLoadBalancerSvcName, Namespace: vmwarev1.SupervisorLoadBalancerSvcNamespace}
	if err := client.Get(context.Background(), svcKey, svc); err != nil {
		return "", errors.Wrapf(err, "unable to get supervisor loadbalancer svc %s", svcKey)
	}
	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		ingress := svc.Status.LoadBalancer.Ingress[0]
		if ipAddr := ingress.IP; ipAddr != "" {
			return ipAddr, nil
		}
		return ingress.Hostname, nil
	}
	return "", errors.Errorf("no VIP found in the supervisor loadbalancer svc %s", svcKey)
}

func GetSupervisorAPIServerFIP(client client.Client) (string, error) {
	urlString, err := getSupervisorAPIServerURLWithFIP(client)
	if err != nil {
		return "", errors.Wrap(err, "unable to get supervisor url")
	}
	urlVal, err := url.Parse(urlString)
	if err != nil {
		return "", errors.Wrapf(err, "unable to parse supervisor url from %s", urlString)
	}
	host := urlVal.Hostname()
	if host == "" {
		return "", errors.Errorf("unable to get supervisor host from url %s", urlVal)
	}
	return host, nil
}

func getSupervisorAPIServerURLWithFIP(client client.Client) (string, error) {
	cm := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{Name: bootstrapapi.ConfigMapClusterInfo, Namespace: metav1.NamespacePublic}
	if err := client.Get(context.Background(), cmKey, cm); err != nil {
		return "", err
	}
	kubeconfig, err := tryParseClusterInfoFromConfigMap(cm)
	if err != nil {
		return "", err
	}
	clusterConfig := getClusterFromKubeConfig(kubeconfig)
	if clusterConfig != nil {
		return clusterConfig.Server, nil
	}
	return "", errors.Errorf("unable to get cluster from kubeconfig in ConfigMap %s/%s", cm.Namespace, cm.Name)
}

// tryParseClusterInfoFromConfigMap tries to parse a kubeconfig file from a ConfigMap key.
func tryParseClusterInfoFromConfigMap(cm *corev1.ConfigMap) (*clientcmdapi.Config, error) {
	kubeConfigString, ok := cm.Data[bootstrapapi.KubeConfigKey]
	if !ok || kubeConfigString == "" {
		return nil, errors.Errorf("no %s key in ConfigMap %s/%s", bootstrapapi.KubeConfigKey, cm.Namespace, cm.Name)
	}
	parsedKubeConfig, err := clientcmd.Load([]byte(kubeConfigString))
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't parse the kubeconfig file in the ConfigMap %s/%s", cm.Namespace, cm.Name)
	}
	return parsedKubeConfig, nil
}

// GetClusterFromKubeConfig returns the default Cluster of the specified KubeConfig.
func getClusterFromKubeConfig(config *clientcmdapi.Config) *clientcmdapi.Cluster {
	// If there is an unnamed cluster object, use it
	if config.Clusters[""] != nil {
		return config.Clusters[""]
	}
	if config.Contexts[config.CurrentContext] != nil {
		return config.Clusters[config.Contexts[config.CurrentContext].Cluster]
	}
	return nil
}

// serviceToClusters is a mapper function used to enqueue reconcile.Requests
// It watches for Service objects of type LoadBalancer for the supervisor api-server.
func (r *serviceDiscoveryReconciler) serviceToClusters(ctx context.Context, o client.Object) []reconcile.Request {
	if o.GetNamespace() != vmwarev1.SupervisorLoadBalancerSvcNamespace || o.GetName() != vmwarev1.SupervisorLoadBalancerSvcName {
		return nil
	}
	return allClustersRequests(ctx, r.Client)
}

// configMapToClusters is a mapper function used to enqueue reconcile.Requests
// It watches for cluster-info configmaps for the supervisor api-server.
func (r *serviceDiscoveryReconciler) configMapToClusters(ctx context.Context, o client.Object) []reconcile.Request {
	if o.GetNamespace() != metav1.NamespacePublic || o.GetName() != bootstrapapi.ConfigMapClusterInfo {
		return nil
	}
	return allClustersRequests(ctx, r.Client)
}

func allClustersRequests(ctx context.Context, c client.Client) []reconcile.Request {
	vsphereClusterList := &vmwarev1.VSphereClusterList{}
	if err := c.List(ctx, vsphereClusterList, &client.ListOptions{}); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(vsphereClusterList.Items))
	for _, vSphereCluster := range vsphereClusterList.Items {
		key := client.ObjectKey{
			Namespace: vSphereCluster.GetNamespace(),
			Name:      vSphereCluster.GetName(),
		}
		requests = append(requests, reconcile.Request{NamespacedName: key})
	}
	return requests
}
