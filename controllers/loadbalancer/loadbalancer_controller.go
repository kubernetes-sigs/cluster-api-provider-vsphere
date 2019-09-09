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

package loadbalancer

import (
	goctx "context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/config"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/loadbalancer/aws"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/util"
)

type providerFunc func(configRef corev1.ObjectReference, client client.Client, logger logr.Logger) (services.LoadBalancerService, error)

var providerFactory = map[string]providerFunc{"AWSLoadBalancerConfig": aws.NewProvider}

// Reconciler reconciles load balancers
type Reconciler struct {
	client.Client
	Recorder record.EventRecorder
	Log      logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=loadbalancers,verbs=get;list;watch;
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=loadbalancers;loadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsloadbalancerconfigs,verbs=get;list;watch;
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsloadbalancerconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch;
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch

// Reconcile ensures the back-end state reflects the Kubernetes state intent for a LoadBalancer resource.
func (r *Reconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	r.Log = r.Log.WithName("loadBalancer").
		WithName(fmt.Sprintf("namespace=%s", req.Namespace)).
		WithName(fmt.Sprintf("LoadBalancerr=%s", req.Name))

	parentCtx := goctx.Background()
	// Get the load balancer object
	loadBalancer := &infrav1.LoadBalancer{}
	if err := r.Client.Get(parentCtx, req.NamespacedName, loadBalancer); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	ctx, err := context.NewLoadBalancerContext(&context.LoadBalancerContextParams{
		Client:       r.Client,
		Context:      goctx.Background(),
		LoadBalancer: loadBalancer,
		Logger:       r.Log,
	})
	if err != nil {
		return reconcile.Result{}, err
	}
	// Always close the context when exiting this function so we can persist any LoadBalancer changes.
	defer func() {
		if err := ctx.Patch(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !loadBalancer.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(loadBalancer, ctx)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(loadBalancer, ctx)
}

func (r *Reconciler) reconcileDelete(loadBalancer *infrav1.LoadBalancer, ctx *context.LoadBalancerContext) (ctrl.Result, error) {
	providerKind := loadBalancer.Spec.ConfigRef.Kind
	newProvider, ok := providerFactory[providerKind]
	if !ok {
		err := errors.Errorf("load balancer factory missing for Kind %q", providerKind)
		ctx.Logger.Error(err, "unable to initialize the load balancer provider")
		return ctrl.Result{}, err
	}
	loadBalancerProvider, err := newProvider(loadBalancer.Spec.ConfigRef, r.Client, r.Log)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err = loadBalancerProvider.Delete(loadBalancer); err != nil {
		return reconcile.Result{RequeueAfter: config.DefaultRequeue}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcileNormal(loadBalancer *infrav1.LoadBalancer, ctx *context.LoadBalancerContext) (ctrl.Result, error) {
	providerKind := loadBalancer.Spec.ConfigRef.Kind
	newProvider, ok := providerFactory[providerKind]
	if !ok {
		err := errors.Errorf("load balancer factory missing for kind %q", providerKind)
		ctx.Logger.Error(err, "unable to initialize the load balancer provider")
		return ctrl.Result{}, err
	}
	loadBalancerProvider, err := newProvider(loadBalancer.Spec.ConfigRef, ctx.Client, r.Log)
	if err != nil {
		return ctrl.Result{}, err
	}
	ctx.Logger.V(4).Info("reconciling loadbalancer")
	selector := loadBalancer.Spec.Selector
	machines, err := infrautilv1.GetMachinesWithSelector(ctx, ctx.Client, loadBalancer.Namespace, selector)
	if err != nil {
		return ctrl.Result{}, err
	}
	ctx.Logger.V(6).Info("got machines for cluster while reconciling load balancer")

	vsphereMachines, err := infrautilv1.GetVSphereMachinesWithSelector(ctx, ctx.Client, loadBalancer.Namespace, selector)
	if err != nil {
		return ctrl.Result{}, err
	}
	ctx.Logger.V(6).Info("got vsphere machines for cluster while reconciling load balancer")

	controlPlaneIPs := []string{}
	for _, controlPlane := range machines {
		vsphereMachine, ok := vsphereMachines[controlPlane.Name]
		if !ok {
			ctx.Logger.V(6).Info("machine not yet linked to the cluster", "machine-name", controlPlane.Name)
			continue
		}
		ip, err := infrautilv1.GetMachinePreferredIPAddress(vsphereMachine)
		if err == infrautilv1.ErrParseCIDR {
			return ctrl.Result{}, err
		}
		if err != infrautilv1.ErrNoMachineIPAddr {
			controlPlaneIPs = append(controlPlaneIPs, ip)
		}
	}
	ctx.Logger.V(4).Info("gathered controlplane IPs", "controlplane-ips", controlPlaneIPs)

	apiEndpoint, err := loadBalancerProvider.Reconcile(loadBalancer, controlPlaneIPs)
	if err != nil {
		return ctrl.Result{}, err
	}
	loadBalancer.Status.APIEndpoint = infrav1.APIEndpoint{
		Host: apiEndpoint.Host,
		Port: apiEndpoint.Port,
	}
	return ctrl.Result{}, nil
}

// SetupWithManager adds this controller to the provided manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.LoadBalancer{}).
		Complete(r)
}
