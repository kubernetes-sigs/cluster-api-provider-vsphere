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
	"crypto/rsa"
	"fmt"
	"math/rand"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// TODO: investigate if we can share this code with the CAPI in memory provider.

const (
	// VMFinalizer allows this reconciler to cleanup resources before removing the
	// VSphereVM from the API Server.
	VMFinalizer = "vcsim.fake.infrastructure.cluster.x-k8s.io"
)

const (
	// VMProvisionedCondition documents the status of VM provisioning,
	// which includes the VM being provisioned and with a boostrap secret available.
	VMProvisionedCondition clusterv1.ConditionType = "VMProvisioned"

	// WaitingForVMInfrastructureReason (Severity=Info) documents provisioning waiting for the VM
	// infrastructure to be ready.
	WaitingForVMInfrastructureReason = "WaitingForVMInfrastructure"

	// WaitingControlPlaneInitializedReason (Severity=Info) documents provisioning waiting
	// for the control plane to be initialized.
	WaitingControlPlaneInitializedReason = "WaitingControlPlaneInitialized"

	// WaitingForBootstrapDataReason (Severity=Info) documents provisioning waiting for the bootstrap
	// data to be ready before starting to create the CloudMachine/VM.
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"
)

var ( // TODO: make this configurable
	nodeStartupDuration = 10 * time.Second
	nodeStartupJitter   = 0.3
)

const (
	// NodeProvisionedCondition documents the status of the provisioning of the Kubernetes node.
	NodeProvisionedCondition clusterv1.ConditionType = "NodeProvisioned"

	// NodeWaitingForStartupTimeoutReason (Severity=Info) documents the Kubernetes Node provisioning.
	NodeWaitingForStartupTimeoutReason = "WaitingForStartupTimeout"
)

var ( // TODO: make this configurable
	etcdStartupDuration = 10 * time.Second
	etcdStartupJitter   = 0.3
)

const (
	// EtcdProvisionedCondition documents the status of the provisioning of the etcd member.
	EtcdProvisionedCondition clusterv1.ConditionType = "EtcdProvisioned"

	// EtcdWaitingForStartupTimeoutReason (Severity=Info) documents the etcd pod provisioning.
	EtcdWaitingForStartupTimeoutReason = "WaitingForStartupTimeout"
)

var ( // TODO: make this configurable
	apiServerStartupDuration = 10 * time.Second
	apiServerStartupJitter   = 0.3
)

const (
	// APIServerProvisionedCondition documents the status of the provisioning of the APIServer instance.
	APIServerProvisionedCondition clusterv1.ConditionType = "APIServerProvisioned"

	// APIServerWaitingForStartupTimeoutReason (Severity=Info) documents the API server pod provisioning.
	APIServerWaitingForStartupTimeoutReason = "WaitingForStartupTimeout"
)

// defines annotations to be applied to in memory etcd pods in order to track etcd cluster
// info belonging to the etcd member each pod represent.
const (
	// EtcdClusterIDAnnotationName defines the name of the annotation applied to in memory etcd
	// pods to track the cluster ID of the etcd member each pod represent.
	EtcdClusterIDAnnotationName = "etcd.inmemory.infrastructure.cluster.x-k8s.io/cluster-id"

	// EtcdMemberIDAnnotationName defines the name of the annotation applied to in memory etcd
	// pods to track the member ID of the etcd member each pod represent.
	EtcdMemberIDAnnotationName = "etcd.inmemory.infrastructure.cluster.x-k8s.io/member-id"

	// EtcdLeaderFromAnnotationName defines the name of the annotation applied to in memory etcd
	// pods to track leadership status of the etcd member each pod represent.
	// Note: We are tracking the time from an etcd member is leader; if more than one pod has this
	// annotation, the last etcd member that became leader is the current leader.
	// By using this mechanism leadership can be forwarded to another pod with an atomic operation
	// (add/update of the annotation to the pod/etcd member we are forwarding leadership to).
	EtcdLeaderFromAnnotationName = "etcd.inmemory.infrastructure.cluster.x-k8s.io/leader-from"

	// EtcdMemberRemoved is added to etcd pods which have been removed from the etcd cluster.
	EtcdMemberRemoved = "etcd.inmemory.infrastructure.cluster.x-k8s.io/member-removed"
)

type ConditionsTracker interface {
	client.Object
	conditions.Getter
	conditions.Setter
}

type vmBootstrapReconciler struct {
	Client          client.Client
	InMemoryManager inmemoryruntime.Manager
	APIServerMux    *inmemoryserver.WorkloadClustersMux

	IsVMReady     func() bool
	GetProviderID func() string
}

func (r *vmBootstrapReconciler) reconcileBoostrap(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if !conditions.Has(conditionsTracker, VMProvisionedCondition) {
		conditions.MarkFalse(conditionsTracker, VMProvisionedCondition, WaitingForVMInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
	}

	// Make sure bootstrap data is available and populated.
	// NOTE: we are not using bootstrap data, but we wait for it in order to simulate a real machine provisioning workflow.
	if machine.Spec.Bootstrap.DataSecretName == nil {
		if !util.IsControlPlaneMachine(machine) && !conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition) {
			conditions.MarkFalse(conditionsTracker, VMProvisionedCondition, WaitingControlPlaneInitializedReason, clusterv1.ConditionSeverityInfo, "")
			log.Info("Waiting for the control plane to be initialized")
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil // keep requeueing since we don't have a watch on machines // TODO: check if we can avoid this
		}

		conditions.MarkFalse(conditionsTracker, VMProvisionedCondition, WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil // keep requeueing since we don't have a watch on machines // TODO: check if we can avoid this
	}

	// Check if the infrastructure is ready and the Bios UUID to be set (required for computing the Provide ID), otherwise return and wait for the vsphereVM object to be updated
	if !r.IsVMReady() {
		log.Info("Waiting for machine infrastructure to become ready")
		return reconcile.Result{}, nil // TODO: check if we can avoid this
	}
	if !conditions.IsTrue(conditionsTracker, VMProvisionedCondition) {
		conditions.MarkTrue(conditionsTracker, VMProvisionedCondition)
	}

	// Call the inner reconciliation methods.
	phases := []func(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error){
		r.reconcileBoostrapNode,
		r.reconcileBoostrapETCD,
		r.reconcileBoostrapAPIServer,
		r.reconcileBoostrapScheduler,
		r.reconcileBoostrapControllerManager,
		r.reconcileBoostrapKubeadmObjects,
		r.reconcileBoostrapKubeProxy,
		r.reconcileBoostrapCoredns,
	}

	res := ctrl.Result{}
	errs := make([]error, 0)
	for _, phase := range phases {
		phaseResult, err := phase(ctx, cluster, machine, conditionsTracker)
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

func (r *vmBootstrapReconciler) reconcileBoostrapNode(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	nodeName := conditionsTracker.GetName()

	provisioningDuration := nodeStartupDuration
	provisioningDuration += time.Duration(rand.Float64() * nodeStartupJitter * float64(provisioningDuration)) //nolint:gosec // Intentionally using a weak random number generator here.

	start := conditions.Get(conditionsTracker, VMProvisionedCondition).LastTransitionTime
	now := time.Now()
	if now.Before(start.Add(provisioningDuration)) {
		conditions.MarkFalse(conditionsTracker, NodeProvisionedCondition, NodeWaitingForStartupTimeoutReason, clusterv1.ConditionSeverityInfo, "")
		remainingTime := start.Add(provisioningDuration).Sub(now)
		log.Info("Waiting for Node to start", "Start", start, "Duration", provisioningDuration, "RemainingTime", remainingTime, "Node", nodeName)
		return ctrl.Result{RequeueAfter: remainingTime}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	// Create Node
	// TODO: consider if to handle an additional setting adding a delay in between create node and node ready/provider ID being set
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: corev1.NodeSpec{
			ProviderID: r.GetProviderID(),
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if util.IsControlPlaneMachine(machine) {
		if node.Labels == nil {
			node.Labels = map[string]string{}
		}
		node.Labels["node-role.kubernetes.io/control-plane"] = ""
	}

	if err := inmemoryClient.Get(ctx, client.ObjectKeyFromObject(node), node); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to get node")
		}

		// NOTE: for the first control plane machine we might create the node before etcd and API server pod are running
		// but this is not an issue, because it won't be visible to CAPI until the API server start serving requests.
		if err := inmemoryClient.Create(ctx, node); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to create Node")
		}
		log.Info("Node created", "Node", klog.KObj(node))
	}

	conditions.MarkTrue(conditionsTracker, NodeProvisionedCondition)
	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileBoostrapETCD(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	etcdMember := fmt.Sprintf("etcd-%s", conditionsTracker.GetName())

	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// No-op if the Node is not provisioned yet
	if !conditions.IsTrue(conditionsTracker, NodeProvisionedCondition) {
		return ctrl.Result{}, nil
	}

	// Wait for the etcd pod to start up; etcd pod start happens a configurable time after the Node is provisioned.
	provisioningDuration := etcdStartupDuration
	provisioningDuration += time.Duration(rand.Float64() * etcdStartupJitter * float64(provisioningDuration)) //nolint:gosec // Intentionally using a weak random number generator here.

	start := conditions.Get(conditionsTracker, NodeProvisionedCondition).LastTransitionTime
	now := time.Now()
	if now.Before(start.Add(provisioningDuration)) {
		conditions.MarkFalse(conditionsTracker, EtcdProvisionedCondition, EtcdWaitingForStartupTimeoutReason, clusterv1.ConditionSeverityInfo, "")
		remainingTime := start.Add(provisioningDuration).Sub(now)
		log.Info("Waiting for etcd Pod to start", "Start", start, "Duration", provisioningDuration, "RemainingTime", remainingTime, "Pod", klog.KRef(metav1.NamespaceSystem, etcdMember))
		return ctrl.Result{RequeueAfter: remainingTime}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	// Create the etcd pod
	// TODO: consider if to handle an additional setting adding a delay in between create pod and pod ready
	etcdPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      etcdMember,
			Labels: map[string]string{
				"component": "etcd",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: conditionsTracker.GetName(),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if err := inmemoryClient.Get(ctx, client.ObjectKeyFromObject(etcdPod), etcdPod); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to get etcd Pod")
		}

		// Gets info about the current etcd cluster, if any.
		info, err := r.getEtcdInfo(ctx, inmemoryClient)
		if err != nil {
			return ctrl.Result{}, err
		}

		// If this is the first etcd member in the cluster, assign a cluster ID
		if info.clusterID == "" {
			for {
				info.clusterID = fmt.Sprintf("%d", rand.Uint32()) //nolint:gosec // weak random number generator is good enough here
				if info.clusterID != "0" {
					break
				}
			}
		}

		// Computes a unique memberID.
		var memberID string
		for {
			memberID = fmt.Sprintf("%d", rand.Uint32()) //nolint:gosec // weak random number generator is good enough here
			if !info.members.Has(memberID) && memberID != "0" {
				break
			}
		}

		// Annotate the pod with the info about the etcd cluster.
		etcdPod.Annotations = map[string]string{
			EtcdClusterIDAnnotationName: info.clusterID,
			EtcdMemberIDAnnotationName:  memberID,
		}

		// If the etcd cluster is being created it doesn't have a leader yet, so set this member as a leader.
		if info.leaderID == "" {
			etcdPod.Annotations[EtcdLeaderFromAnnotationName] = time.Now().Format(time.RFC3339)
		}

		// NOTE: for the first control plane machine we might create the etcd pod before the API server pod is running
		// but this is not an issue, because it won't be visible to CAPI until the API server start serving requests.
		if err := inmemoryClient.Create(ctx, etcdPod); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to create Pod")
		}
	}

	// If there is not yet an etcd member listener for this machine, add it to the server.
	listenerName, err := r.APIServerMux.WorkloadClusterByResourceGroup(resourceGroup)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !r.APIServerMux.HasEtcdMember(listenerName, etcdMember) {
		// Getting the etcd CA
		s, err := secret.Get(ctx, r.Client, client.ObjectKeyFromObject(cluster), secret.EtcdCA)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to get etcd CA")
		}
		certData, exists := s.Data[secret.TLSCrtDataName]
		if !exists {
			return ctrl.Result{}, errors.Errorf("invalid etcd CA: missing data for %s", secret.TLSCrtDataName)
		}

		cert, err := certs.DecodeCertPEM(certData)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "invalid etcd CA: invalid %s", secret.TLSCrtDataName)
		}

		keyData, exists := s.Data[secret.TLSKeyDataName]
		if !exists {
			return ctrl.Result{}, errors.Errorf("invalid etcd CA: missing data for %s", secret.TLSKeyDataName)
		}

		key, err := certs.DecodePrivateKeyPEM(keyData)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "invalid etcd CA: invalid %s", secret.TLSKeyDataName)
		}

		if err := r.APIServerMux.AddEtcdMember(listenerName, etcdMember, cert, key.(*rsa.PrivateKey)); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to start etcd member")
		}
		log.Info("etcd Pod started", "Pod", klog.KObj(etcdPod))
	}

	conditions.MarkTrue(conditionsTracker, EtcdProvisionedCondition)
	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileBoostrapAPIServer(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	apiServer := fmt.Sprintf("kube-apiserver-%s", conditionsTracker.GetName())

	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// No-op if the Node is not provisioned yet
	if !conditions.IsTrue(conditionsTracker, NodeProvisionedCondition) {
		return ctrl.Result{}, nil
	}

	// Wait for the API server pod to start up; API server pod start happens a configurable time after the Node is provisioned.
	provisioningDuration := apiServerStartupDuration
	provisioningDuration += time.Duration(rand.Float64() * apiServerStartupJitter * float64(provisioningDuration)) //nolint:gosec // Intentionally using a weak random number generator here.

	start := conditions.Get(conditionsTracker, NodeProvisionedCondition).LastTransitionTime
	now := time.Now()
	if now.Before(start.Add(provisioningDuration)) {
		conditions.MarkFalse(conditionsTracker, APIServerProvisionedCondition, APIServerWaitingForStartupTimeoutReason, clusterv1.ConditionSeverityInfo, "")
		remainingTime := start.Add(provisioningDuration).Sub(now)
		log.Info("Waiting for API server Pod to start", "Start", start, "Duration", provisioningDuration, "RemainingTime", remainingTime, "Pod", klog.KRef(metav1.NamespaceSystem, apiServer))
		return ctrl.Result{RequeueAfter: remainingTime}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	// Create the apiserver pod
	// TODO: consider if to handle an additional setting adding a delay in between create pod and pod ready

	apiServerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      apiServer,
			Labels: map[string]string{
				"component": "kube-apiserver",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: conditionsTracker.GetName(),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if err := inmemoryClient.Get(ctx, client.ObjectKeyFromObject(apiServerPod), apiServerPod); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to get apiServer Pod")
		}

		if err := inmemoryClient.Create(ctx, apiServerPod); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to create apiServer Pod")
		}
	}

	// If there is not yet an API server listener for this machine.
	listenerName, err := r.APIServerMux.WorkloadClusterByResourceGroup(resourceGroup)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !r.APIServerMux.HasAPIServer(listenerName, apiServer) {
		// Getting the Kubernetes CA
		s, err := secret.Get(ctx, r.Client, client.ObjectKeyFromObject(cluster), secret.ClusterCA)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to get cluster CA")
		}
		certData, exists := s.Data[secret.TLSCrtDataName]
		if !exists {
			return ctrl.Result{}, errors.Errorf("invalid cluster CA: missing data for %s", secret.TLSCrtDataName)
		}

		cert, err := certs.DecodeCertPEM(certData)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "invalid cluster CA: invalid %s", secret.TLSCrtDataName)
		}

		keyData, exists := s.Data[secret.TLSKeyDataName]
		if !exists {
			return ctrl.Result{}, errors.Errorf("invalid cluster CA: missing data for %s", secret.TLSKeyDataName)
		}

		key, err := certs.DecodePrivateKeyPEM(keyData)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "invalid cluster CA: invalid %s", secret.TLSKeyDataName)
		}

		// Adding the APIServer.
		// NOTE: When the first APIServer is added, the workload cluster listener is started.
		if err := r.APIServerMux.AddAPIServer(listenerName, apiServer, cert, key.(*rsa.PrivateKey)); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to start API server")
		}
		log.Info("API server Pod started", "Pod", klog.KObj(apiServerPod))
	}

	conditions.MarkTrue(conditionsTracker, APIServerProvisionedCondition)
	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileBoostrapScheduler(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// NOTE: we are creating the scheduler pod to make KCP happy, but we are not implementing any
	// specific behaviour for this component because they are not relevant for stress tests.
	// As a current approximation, we create the scheduler as soon as the API server is provisioned;
	// also, the scheduler is immediately marked as ready.
	if !conditions.IsTrue(conditionsTracker, APIServerProvisionedCondition) {
		return ctrl.Result{}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	schedulerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      fmt.Sprintf("kube-scheduler-%s", conditionsTracker.GetName()),
			Labels: map[string]string{
				"component": "kube-scheduler",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: conditionsTracker.GetName(),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if err := inmemoryClient.Create(ctx, schedulerPod); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create scheduler Pod")
	}

	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileBoostrapControllerManager(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// NOTE: we are creating the controller manager pod to make KCP happy, but we are not implementing any
	// specific behaviour for this component because they are not relevant for stress tests.
	// As a current approximation, we create the controller manager as soon as the API server is provisioned;
	// also, the controller manager is immediately marked as ready.
	if !conditions.IsTrue(conditionsTracker, APIServerProvisionedCondition) {
		return ctrl.Result{}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	controllerManagerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      fmt.Sprintf("kube-controller-manager-%s", conditionsTracker.GetName()),
			Labels: map[string]string{
				"component": "kube-controller-manager",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: conditionsTracker.GetName(),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if err := inmemoryClient.Create(ctx, controllerManagerPod); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create controller manager Pod")
	}

	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileBoostrapKubeadmObjects(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, _ ConditionsTracker) (ctrl.Result, error) {
	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	// create kubeadm ClusterRole and ClusterRoleBinding enforced by KCP
	// NOTE: we create those objects because this is what kubeadm does, but KCP creates
	// ClusterRole and ClusterRoleBinding if not found.

	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeadm:get-nodes",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"nodes"},
			},
		},
	}
	if err := inmemoryClient.Create(ctx, role); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create kubeadm:get-nodes ClusterRole")
	}

	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeadm:get-nodes",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "kubeadm:get-nodes",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: "system:bootstrappers:kubeadm:default-node-token",
			},
		},
	}
	if err := inmemoryClient.Create(ctx, roleBinding); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create kubeadm:get-nodes ClusterRoleBinding")
	}

	// create kubeadm config map
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeadm-config",
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{
			"ClusterConfiguration": "",
		},
	}
	if err := inmemoryClient.Create(ctx, cm); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create kubeadm-config ConfigMap")
	}

	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileBoostrapKubeProxy(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, _ ConditionsTracker) (ctrl.Result, error) {
	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// TODO: Add provisioning time for KubeProxy.

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	// Create the kube-proxy-daemonset
	kubeProxyDaemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "kube-proxy",
			Labels: map[string]string{
				"component": "kube-proxy",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kube-proxy",
							Image: fmt.Sprintf("registry.k8s.io/kube-proxy:%s", *machine.Spec.Version),
						},
					},
				},
			},
		},
	}
	if err := inmemoryClient.Get(ctx, client.ObjectKeyFromObject(kubeProxyDaemonSet), kubeProxyDaemonSet); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to get kube-proxy DaemonSet")
		}

		if err := inmemoryClient.Create(ctx, kubeProxyDaemonSet); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to create kube-proxy DaemonSet")
		}
	}
	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileBoostrapCoredns(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, _ ConditionsTracker) (ctrl.Result, error) {
	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// TODO: Add provisioning time for CoreDNS.

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	// Create the coredns configMap.
	corednsConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "coredns",
		},
		Data: map[string]string{
			"Corefile": "ANG",
		},
	}
	if err := inmemoryClient.Get(ctx, client.ObjectKeyFromObject(corednsConfigMap), corednsConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to get coreDNS configMap")
		}

		if err := inmemoryClient.Create(ctx, corednsConfigMap); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to create coreDNS configMap")
		}
	}
	// Create the coredns deployment.
	corednsDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "coredns",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "coredns",
							Image: "registry.k8s.io/coredns/coredns:v1.10.1",
						},
					},
				},
			},
		},
	}

	if err := inmemoryClient.Get(ctx, client.ObjectKeyFromObject(corednsDeployment), corednsDeployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to get coreDNS deployment")
		}

		if err := inmemoryClient.Create(ctx, corednsDeployment); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, errors.Wrapf(err, "failed to create coreDNS deployment")
		}
	}
	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	// Call the inner reconciliation methods.
	phases := []func(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error){
		r.reconcileDeleteNode,
		r.reconcileDeleteETCD,
		r.reconcileDeleteAPIServer,
		r.reconcileDeleteScheduler,
		r.reconcileDeleteControllerManager,
		// Note: We are not deleting kubeadm objects because they exist in K8s, they are not related to a specific machine.
	}

	res := ctrl.Result{}
	errs := make([]error, 0)
	for _, phase := range phases {
		phaseResult, err := phase(ctx, cluster, machine, conditionsTracker)
		if err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			continue
		}
		res = capiutil.LowestNonZeroResult(res, phaseResult)
	}
	if res.IsZero() && len(errs) == 0 {
		controllerutil.RemoveFinalizer(conditionsTracker, infrav1.VMFinalizer)
	}
	return res, kerrors.NewAggregate(errs)
}

func (r *vmBootstrapReconciler) reconcileDeleteNode(ctx context.Context, cluster *clusterv1.Cluster, _ *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	// Delete Node
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: conditionsTracker.GetName(),
		},
	}

	// TODO(killianmuldoon): check if we can drop this given that the MachineController is already draining pods and deleting nodes.
	if err := inmemoryClient.Delete(ctx, node); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to delete Node")
	}

	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileDeleteETCD(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	etcdMember := fmt.Sprintf("etcd-%s", conditionsTracker.GetName())
	etcdPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      etcdMember,
		},
	}
	if err := inmemoryClient.Delete(ctx, etcdPod); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to delete etcd Pod")
	}

	listenerName, err := r.APIServerMux.WorkloadClusterByResourceGroup(resourceGroup)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.APIServerMux.DeleteEtcdMember(listenerName, etcdMember); err != nil {
		return ctrl.Result{}, err
	}

	// TODO: if all the etcd members are gone, cleanup all the k8s objects from the resource group.
	// note: it is not possible to delete the resource group, because cloud resources should be preserved.
	// given that, in order to implement this it is required to find a way to identify all the k8s resources (might be via gvk);
	// also, deletion must happen suddenly, without respecting finalizers or owner references links.

	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileDeleteAPIServer(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	apiServer := fmt.Sprintf("kube-apiserver-%s", conditionsTracker.GetName())
	apiServerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      apiServer,
		},
	}
	if err := inmemoryClient.Delete(ctx, apiServerPod); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to delete apiServer Pod")
	}

	listenerName, err := r.APIServerMux.WorkloadClusterByResourceGroup(resourceGroup)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.APIServerMux.DeleteAPIServer(listenerName, apiServer); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileDeleteScheduler(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	schedulerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      fmt.Sprintf("kube-scheduler-%s", conditionsTracker.GetName()),
		},
	}
	if err := inmemoryClient.Delete(ctx, schedulerPod); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to scheduler Pod")
	}

	return ctrl.Result{}, nil
}

func (r *vmBootstrapReconciler) reconcileDeleteControllerManager(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, conditionsTracker ConditionsTracker) (ctrl.Result, error) {
	// No-op if the machine is not a control plane machine.
	if !util.IsControlPlaneMachine(machine) {
		return ctrl.Result{}, nil
	}

	// Compute the resource group unique name.
	resourceGroup := klog.KObj(cluster).String()
	inmemoryClient := r.InMemoryManager.GetResourceGroup(resourceGroup).GetClient()

	controllerManagerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      fmt.Sprintf("kube-controller-manager-%s", conditionsTracker.GetName()),
		},
	}
	if err := inmemoryClient.Delete(ctx, controllerManagerPod); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, errors.Wrapf(err, "failed to controller manager Pod")
	}

	return ctrl.Result{}, nil
}

type etcdInfo struct {
	clusterID string
	leaderID  string
	members   sets.Set[string]
}

func (r *vmBootstrapReconciler) getEtcdInfo(ctx context.Context, inmemoryClient inmemoryruntime.Client) (etcdInfo, error) {
	etcdPods := &corev1.PodList{}
	if err := inmemoryClient.List(ctx, etcdPods,
		client.InNamespace(metav1.NamespaceSystem),
		client.MatchingLabels{
			"component": "etcd",
			"tier":      "control-plane"},
	); err != nil {
		return etcdInfo{}, errors.Wrap(err, "failed to list etcd members")
	}

	if len(etcdPods.Items) == 0 {
		return etcdInfo{}, nil
	}

	info := etcdInfo{
		members: sets.New[string](),
	}
	var leaderFrom time.Time
	for _, pod := range etcdPods.Items {
		if _, ok := pod.Annotations[EtcdMemberRemoved]; ok {
			continue
		}
		if info.clusterID == "" {
			info.clusterID = pod.Annotations[EtcdClusterIDAnnotationName]
		} else if pod.Annotations[EtcdClusterIDAnnotationName] != info.clusterID {
			return etcdInfo{}, errors.New("invalid etcd cluster, members have different cluster ID")
		}
		memberID := pod.Annotations[EtcdMemberIDAnnotationName]
		info.members.Insert(memberID)

		if t, err := time.Parse(time.RFC3339, pod.Annotations[EtcdLeaderFromAnnotationName]); err == nil {
			if t.After(leaderFrom) {
				info.leaderID = memberID
				leaderFrom = t
			}
		}
	}

	if info.leaderID == "" {
		// TODO: consider if and how to automatically recover from this case
		//  note: this can happen also when reading etcd members in the server, might be it is something we have to take case before deletion...
		//  for now it should not be an issue because KCP forward etcd leadership before deletion.
		return etcdInfo{}, errors.New("invalid etcd cluster, no leader found")
	}

	return info, nil
}
