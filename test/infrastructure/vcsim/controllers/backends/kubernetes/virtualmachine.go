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

package kubernetes

import (
	"context"
	"time"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type VirtualMachineReconciler struct {
	Client client.Client

	IsVMReady func() bool

	overrideGetManagerContainer func(ctx context.Context) (*corev1.Container, error)
}

func (r *VirtualMachineReconciler) ReconcileNormal(ctx context.Context, cluster *clusterv1beta1.Cluster, machine *clusterv1beta1.Machine, virtualMachine client.Object) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Make sure bootstrap data is available and populated.
	// NOTE: we are not using bootstrap data, but we wait for it in order to simulate a real machine provisioning workflow.
	if machine.Spec.Bootstrap.DataSecretName == nil {
		if !util.IsControlPlaneMachine(machine) && !v1beta1conditions.IsTrue(cluster, clusterv1beta1.ControlPlaneInitializedCondition) {
			log.Info("Waiting for the control plane to be initialized")
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil // keep requeueing since we don't have a watch on machines // TODO: check if we can avoid this
		}

		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil // keep requeueing since we don't have a watch on machines // TODO: check if we can avoid this
	}

	// Check if the infrastructure is ready and the Bios UUID to be set (required for computing the Provider ID), otherwise return and wait for the vsphereVM object to be updated
	if !r.IsVMReady() {
		log.Info("Waiting for machine infrastructure to become ready")
		return reconcile.Result{}, nil // TODO: check if we can avoid this
	}

	// Call the inner reconciliation methods.
	phases := []func(ctx context.Context, cluster *clusterv1beta1.Cluster, machine *clusterv1beta1.Machine, virtualMachine client.Object) (ctrl.Result, error){
		r.reconcileCertificates,
		r.reconcileKubeConfig,
		r.reconcilePods,
	}

	res := ctrl.Result{}
	errs := make([]error, 0)
	for _, phase := range phases {
		phaseResult, err := phase(ctx, cluster, machine, virtualMachine)
		if err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			continue
		}
		res = capiutil.LowestNonZeroResult(res, phaseResult)
	}
	return res, kerrors.NewAggregate(errs)
}

// reconcileCertificates reconcile the cluster certificates in the management cluster, as required by the CAPI contract.
func (r *VirtualMachineReconciler) reconcileCertificates(ctx context.Context, cluster *clusterv1beta1.Cluster, _ *clusterv1beta1.Machine, virtualMachine client.Object) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DEBUG: reconcileCertificates")

	secretHandler := caSecretHandler{
		client:         r.Client,
		cluster:        cluster,
		virtualMachine: virtualMachine,
	}

	if err := secretHandler.LookupOrGenerate(ctx); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to generate cluster's certificate authorities")
	}
	return ctrl.Result{}, nil
}

// reconcileKubeConfig reconcile the cluster admin kubeconfig in the management cluster, as required by the CAPI contract.
func (r *VirtualMachineReconciler) reconcileKubeConfig(ctx context.Context, cluster *clusterv1beta1.Cluster, _ *clusterv1beta1.Machine, virtualMachine client.Object) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DEBUG: reconcileKubeConfig")
	// If the secret with the CA is not yet in cache, wait fo in a bit before giving up.
	if err := wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if _, err := secret.GetFromNamespacedName(ctx, r.Client, client.ObjectKeyFromObject(cluster), secret.ClusterCA); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to read cluster CA while generating admin kubeconfig")
	}

	secretHandler := kubeConfigSecretHandler{
		client:         r.Client,
		cluster:        cluster,
		virtualMachine: virtualMachine,
	}

	// Note: the kubemarkControlPlane doesn't support implement kubeconfig client certificate renewal,
	// but this is considered acceptable for the goals of the kubemark provider.
	if err := secretHandler.LookupOrGenerate(ctx); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to generate secret with the cluster's admin kubeconfig")
	}
	return ctrl.Result{}, nil
}

// reconcilePods reconcile pods hosting a control plane replicas.
// Note: The implementation currently manage one replica without remediation support, but there is already part of
// scaffolding for implementing support for n replicas.
// TODO: implement, support for n replicas, remediation
func (r *VirtualMachineReconciler) reconcilePods(ctx context.Context, cluster *clusterv1beta1.Cluster, machine *clusterv1beta1.Machine, virtualMachine client.Object) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DEBUG: reconcilePods")

	controlPlaneEndpoint, err := r.getControPlaneEndPoint(ctx, cluster, virtualMachine)
	if err != nil {
		return ctrl.Result{}, err
	}

	podHandler := controlPlanePodHandler{
		client:                      r.Client,
		cluster:                     cluster,
		controlPlaneEndpoint:        controlPlaneEndpoint,
		virtualMachine:              virtualMachine,
		overrideGetManagerContainer: r.overrideGetManagerContainer,
	}

	// Create RBAC rules for the pod to run.
	if err := podHandler.LookupAndGenerateRBAC(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Gets the list of pods hosting a control plane replicas.
	pods, err := podHandler.GetPods(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: Change this logic, we should generate if there is not a pod for this VM.
	//   PodList must be used for join CP
	if len(pods.Items) < 1 {
		log.Info("Scaling up control plane replicas to 1")
		if err := podHandler.Generate(ctx, *machine.Spec.Version); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to generate control plane pod")
		}
		// Requeue so we can refresh the list of pods hosting a control plane replicas.
		return ctrl.Result{Requeue: true}, nil
	}

	// Wait for the pod to become running.
	// TODO: Detect pod is up, figure it out the path forward to VM readiness.
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *VirtualMachineReconciler) ReconcileDelete(ctx context.Context, cluster *clusterv1beta1.Cluster, machine *clusterv1beta1.Machine, virtualMachine client.Object) (_ ctrl.Result, reterr error) {
	controlPlaneEndpoint, err := r.getControPlaneEndPoint(ctx, cluster, virtualMachine)
	if err != nil {
		return ctrl.Result{}, err
	}

	podHandler := controlPlanePodHandler{
		client:                      r.Client,
		cluster:                     cluster,
		controlPlaneEndpoint:        controlPlaneEndpoint,
		virtualMachine:              virtualMachine,
		overrideGetManagerContainer: r.overrideGetManagerContainer,
	}

	// Delete all pods
	pods, err := podHandler.GetPods(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, pod := range pods.Items {
		if err := podHandler.Delete(ctx, pod.Name); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, errors.Wrap(err, "failed to delete control plane pod")
			}
		}
	}

	// TODO: Cleanup RBAC (might be they should be renamed by Cluster)

	// TODO: Delete kubeconfig? it should go away via garbage collector...

	// TODO: Delete all secrets? it should go away via garbage collector...

	controllerutil.RemoveFinalizer(virtualMachine, vcsimv1.VMFinalizer)
	return ctrl.Result{}, nil
}

func (r *VirtualMachineReconciler) getControPlaneEndPoint(ctx context.Context, cluster *clusterv1beta1.Cluster, virtualMachine client.Object) (*vcsimv1.ControlPlaneEndpoint, error) {
	controlPlaneEndpoint := &vcsimv1.ControlPlaneEndpoint{}
	if name, ok := virtualMachine.GetAnnotations()["control-plane-endpoint.vcsim.infrastructure.cluster.x-k8s.io"]; ok {
		if err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: virtualMachine.GetNamespace(),
			Name:      name,
		}, controlPlaneEndpoint); err != nil {
			return nil, err
		}
	} else {
		l := &vcsimv1.ControlPlaneEndpointList{}
		if err := r.Client.List(ctx, l); err != nil {
			return nil, err
		}
		found := false
		for _, cpe := range l.Items {
			if cpe.Status.Host != cluster.Spec.ControlPlaneEndpoint.Host || cpe.Status.Port != cluster.Spec.ControlPlaneEndpoint.Port {
				continue
			}

			originalVirtualMachine := virtualMachine.(*vmoprv1.VirtualMachine).DeepCopy()
			patch := client.MergeFrom(originalVirtualMachine)

			annotations := virtualMachine.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations["control-plane-endpoint.vcsim.infrastructure.cluster.x-k8s.io"] = cpe.Name

			if err := r.Client.Patch(ctx, virtualMachine, patch); err != nil {
				return nil, err
			}

			controlPlaneEndpoint = &cpe
			found = true
			break
		}
		if !found {
			return nil, errors.Errorf("unable to find a ControlPlaneEndpoint for host %s, port %d", cluster.Spec.ControlPlaneEndpoint.Host, cluster.Spec.ControlPlaneEndpoint.Port)
		}
	}
	return controlPlaneEndpoint, nil
}
