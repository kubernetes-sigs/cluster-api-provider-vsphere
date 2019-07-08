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

package machine

import (
	goctx "context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clientv1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	clusterErr "sigs.k8s.io/cluster-api/pkg/controller/error"
	remotev1 "sigs.k8s.io/cluster-api/pkg/controller/remote"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/actuators"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeclient"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/kubeconfig"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/tokens"
)

const (
	defaultTokenTTL = 10 * time.Minute
)

//+kubebuilder:rbac:groups=vsphere.cluster.k8s.io,resources=vspheremachineproviderspecs;vspheremachineproviderstatuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=machines;machines/status;machinedeployments;machinedeployments/status;machinesets;machinesets/status;machineclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes;events;configmaps,verbs=get;list;watch;create;update;patch

// Actuator is responsible for maintaining the Machine objects.
type Actuator struct {
	client           clientv1.ClusterV1alpha1Interface
	coreClient       corev1.CoreV1Interface
	controllerClient controllerClient.Client
}

// NewActuator returns a new instance of Actuator.
func NewActuator(
	client clientv1.ClusterV1alpha1Interface,
	coreClient corev1.CoreV1Interface,
	controllerClient controllerClient.Client) *Actuator {

	return &Actuator{
		client:           client,
		coreClient:       coreClient,
		controllerClient: controllerClient,
	}
}

// Create creates a new machine.
func (a *Actuator) Create(
	parentCtx goctx.Context,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine) (opErr error) {

	ctx, err := context.NewMachineContext(
		&context.MachineContextParams{
			ClusterContextParams: context.ClusterContextParams{
				Context:          parentCtx,
				Cluster:          cluster,
				Client:           a.client,
				CoreClient:       a.coreClient,
				ControllerClient: a.controllerClient,
				Logger:           klogr.New().WithName("[machine-actuator]"),
			},
			Machine: machine,
		})
	if err != nil {
		return err
	}

	if _, ok := machine.Annotations[constants.MaintenanceAnnotationLabel]; ok {
		ctx.Logger.V(4).Info("skipping operations on machine", "reason", "annotation", "annotation-key", constants.MaintenanceAnnotationLabel)
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Create", opErr)
	}()

	if err := a.reconcilePKI(ctx); err != nil {
		return err
	}

	return a.doInitOrJoin(ctx)
}

// Delete removes a machine.
func (a *Actuator) Delete(
	parentCtx goctx.Context,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine) (opErr error) {

	ctx, err := context.NewMachineContext(
		&context.MachineContextParams{
			ClusterContextParams: context.ClusterContextParams{
				Context:          parentCtx,
				Cluster:          cluster,
				Client:           a.client,
				CoreClient:       a.coreClient,
				ControllerClient: a.controllerClient,
			},
			Machine: machine,
		})
	if err != nil {
		return err
	}

	if _, ok := machine.Annotations[constants.MaintenanceAnnotationLabel]; ok {
		ctx.Logger.V(4).Info("skipping operations on machine", "reason", "annotation", "annotation-key", constants.MaintenanceAnnotationLabel)
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Delete", opErr)
	}()

	ctx.Logger.V(2).Info("deleting machine")

	return govmomi.Delete(ctx)
}

// Update updates a machine from the backend platform's information.
func (a *Actuator) Update(
	parentCtx goctx.Context,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine) (opErr error) {

	ctx, err := context.NewMachineContext(
		&context.MachineContextParams{
			ClusterContextParams: context.ClusterContextParams{
				Context:          parentCtx,
				Cluster:          cluster,
				Client:           a.client,
				CoreClient:       a.coreClient,
				ControllerClient: a.controllerClient,
			},
			Machine: machine,
		})
	if err != nil {
		return err
	}

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Update", opErr)
	}()

	ctx.Logger.V(6).Info("updating machine")

	if err := govmomi.Update(ctx); err != nil {
		return err
	}

	if err := a.reconcileKubeConfig(ctx); err != nil {
		return err
	}

	if err := a.reconcileReadyState(ctx); err != nil {
		return err
	}

	return nil
}

// Exists returns a flag indicating whether or not a machine exists.
func (a *Actuator) Exists(
	parentCtx goctx.Context,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine) (ok bool, opErr error) {

	ctx, err := context.NewMachineContext(
		&context.MachineContextParams{
			ClusterContextParams: context.ClusterContextParams{
				Context:          parentCtx,
				Cluster:          cluster,
				Client:           a.client,
				CoreClient:       a.coreClient,
				ControllerClient: a.controllerClient,
			},
			Machine: machine,
		})
	if err != nil {
		return false, err
	}

	if _, ok := machine.Annotations[constants.MaintenanceAnnotationLabel]; ok {
		ctx.Logger.V(4).Info("skipping operations on machine", "reason", "annotation", "annotation-key", constants.MaintenanceAnnotationLabel)
		return false, &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Exists", opErr)
	}()

	return govmomi.Exists(ctx)
}

func (a *Actuator) reconcilePKI(ctx *context.MachineContext) error {
	if !ctx.ClusterConfig.CAKeyPair.HasCertAndKey() {
		ctx.Logger.V(6).Info("cluster config is missing pki toolchain, requeue machine")
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}
	return nil
}

func (a *Actuator) doInitOrJoin(ctx *context.MachineContext) error {

	// Determine whether or not to initialize the control plane, join an
	// existing control plane, or join the cluster as a worker node.
	initControlPlane, err := a.shouldInitControlPlane(ctx)
	if err != nil {
		return err
	}
	if initControlPlane {
		ctx.Logger.V(6).Info("initializing control plane")
		return govmomi.Create(ctx, "")
	}

	// Get a client for the target cluster.
	client, err := kubeclient.New(ctx)
	if err != nil {
		ctx.Logger.Error(err, "target cluster is not ready")
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	// Get a new bootstrap token used to join this machine to the cluster.
	token, err := tokens.NewBootstrap(client, defaultTokenTTL)
	if err != nil {
		return errors.Wrapf(err, "unable to generate boostrap token for joining machine to cluster %q", ctx)
	}

	// Create the machine and join it to the cluster.
	if ctx.HasControlPlaneRole() {
		ctx.Logger.V(6).Info("joining control plane")
	} else {
		ctx.Logger.V(6).Info("joining cluster")
	}
	return govmomi.Create(ctx, token)
}

// reconcileKubeConfig creates a secret on the management cluster with
// the kubeconfig for target cluster.
func (a *Actuator) reconcileKubeConfig(ctx *context.MachineContext) error {
	if !ctx.HasControlPlaneRole() {
		return nil
	}

	// Get the control plane endpoint.
	controlPlaneEndpoint, err := ctx.ControlPlaneEndpoint()
	if err != nil {
		ctx.Logger.Error(err, "requeueing until control plane endpoint is available")
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	// Create a new kubeconfig for the target cluster.
	ctx.Logger.V(6).Info("generating kubeconfig secret")
	kubeConfig, err := kubeconfig.New(ctx.Cluster.Name, controlPlaneEndpoint, ctx.ClusterConfig.CAKeyPair)
	if err != nil {
		return errors.Wrapf(err, "error generating kubeconfig for %q", ctx)
	}

	// Define the kubeconfig secret for the target cluster.
	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.Cluster.Namespace,
			Name:      remotev1.KubeConfigSecretName(ctx.Cluster.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: ctx.Cluster.APIVersion,
					Kind:       ctx.Cluster.Kind,
					Name:       ctx.Cluster.Name,
					UID:        ctx.Cluster.UID,
				},
			},
		},
		StringData: map[string]string{
			"value": kubeConfig,
		},
	}

	// Create the kubeconfig secret.
	if _, err := a.coreClient.Secrets(ctx.Cluster.Namespace).Create(secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			ctx.Logger.Error(err, "error creating kubeconfig secret")
			return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
		}
		ctx.Logger.V(6).Info("kubeconfig secret already exists")
	} else {
		ctx.Logger.V(4).Info("created kubeconfig secret")
	}

	return nil
}

// reconcileReadyState returns a requeue error until the machine appears
// in the target cluster's list of nodes.
func (a *Actuator) reconcileReadyState(ctx *context.MachineContext) error {

	// Normally the following code would delete the ready annotation to
	// allow it to be recalculated. However, because a machine's annotations
	// are the only thing guaranteed to survive the pivot (from a set that
	// also includes the cluster annotations and the machine's NodeRef), this
	// annotation cannot be deleted. Else this would create a race condition
	// where, post-pivot, a second machine may attempt to initialize the
	// control plane.
	//delete(ctx.Machine.Annotations, constants.MachineReadyAnnotationLabel)

	if ctx.Machine.Status.NodeRef == nil {
		ctx.Logger.V(6).Info("requeuing until noderef is set")
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	if ctx.Machine.Annotations == nil {
		ctx.Machine.Annotations = map[string]string{}
	}

	ctx.Machine.Annotations[constants.MachineReadyAnnotationLabel] = ""
	ctx.Logger.V(6).Info("machine is ready")

	return nil
}

// shouldInitControlPlane returns a flag indicating whether or not this machine
// should initialize the control plane. If false is returned then this machine
// should join the existing cluster.
func (a *Actuator) shouldInitControlPlane(ctx *context.MachineContext) (bool, error) {

	// Check to see if the control plane is already initialized.
	if machines, err := ctx.GetMachines(); err == nil {
		for _, m := range machines {
			if m.Status.NodeRef != nil {
				ctx.Logger.V(6).Info("control plane is already initialized: noderef exists", "node-ref", m.Status.NodeRef.String())
				return false, nil
			}
			if _, ok := m.Annotations[constants.MachineReadyAnnotationLabel]; ok {
				ctx.Logger.V(6).Info("control plane is already initialized: ready annotation", "machine", fmt.Sprintf("%s/%s", m.Namespace, m.Name))
				return false, nil
			}
		}
	}

	// If the control plane is *not* ready, then can this machine initialize
	// the control plane? First, does it even have the control plane role?
	// If not then all this worker node can do is requeue until the control
	// plane is ready.
	if !ctx.HasControlPlaneRole() {
		ctx.Logger.V(6).Info("machine is worker; requeue until control plane is ready")
		return false, &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	// The machine has the control plane role, but can it acquire the config
	// map that gates access to initializing the control plane?
	controlPlaneConfigMap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.Cluster.Namespace,
			Name:      actuators.GetNameOfControlPlaneConfigMap(ctx.Cluster.UID),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: ctx.Cluster.APIVersion,
					Kind:       ctx.Cluster.Kind,
					Name:       ctx.Cluster.Name,
					UID:        ctx.Cluster.UID,
				},
			},
		},
		Data: map[string]string{
			"firstMachine": ctx.Machine.Name,
		},
	}

	// Create the control plane config map. If this fails because such a config
	// map already exists, then it's because the control plane is currently
	// being initialized by another control plane machine, and this machine
	// should requeue until the control plane is ready. If successful, then it
	// means this is the first control plane machine.
	if _, err := ctx.CoreClient.ConfigMaps(ctx.Cluster.Namespace).Create(controlPlaneConfigMap); err != nil {
		if apierrors.IsAlreadyExists(err) {
			ctx.Logger.V(6).Info("control plane is already being initialized; requeue until ready")
		} else {
			ctx.Logger.Error(err, "error creating control plane config map")
		}
		return false, &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	// The control plane is not ready, and this machine should initialize it.
	return true, nil
}
