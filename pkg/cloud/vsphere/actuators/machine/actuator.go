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
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/tokens"
)

const (
	defaultTokenTTL = 10 * time.Minute
)

//+kubebuilder:rbac:groups=vsphereproviderconfig.sigs.k8s.io,resources=vspheremachineproviderconfigs;vspheremachineproviderstatuses,verbs=get;list;watch;create;update;patch;delete
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
				Context:    parentCtx,
				Cluster:    cluster,
				Client:     a.client,
				CoreClient: a.coreClient,
				Logger:     klogr.New().WithName("[machine-actuator]"),
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

	if err := a.reconcileInitOrJoin(ctx); err != nil {
		return err
	}

	return nil
}

// Delete removes a machine.
func (a *Actuator) Delete(
	parentCtx goctx.Context,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine) (opErr error) {

	ctx, err := context.NewMachineContext(
		&context.MachineContextParams{
			ClusterContextParams: context.ClusterContextParams{
				Context:    parentCtx,
				Cluster:    cluster,
				Client:     a.client,
				CoreClient: a.coreClient,
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
				Context:    parentCtx,
				Cluster:    cluster,
				Client:     a.client,
				CoreClient: a.coreClient,
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
				Context:    parentCtx,
				Cluster:    cluster,
				Client:     a.client,
				CoreClient: a.coreClient,
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

// TODO(akutz) Implement distributed locking to support multiple control
//             plane members.
func (a *Actuator) reconcileInitOrJoin(ctx *context.MachineContext) error {

	// If this is the control plane node then initialize the cluster.
	if ctx.HasControlPlaneRole() {
		return govmomi.Create(ctx, "")
	}

	// Otherwise wait for the cluster to come online.
	if online, _, _ := kubeclient.GetControlPlaneStatus(ctx); !online {
		ctx.Logger.V(6).Info("unable to join machine to control plane until it is online")
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	// Get a Kubernetes client for the cluster.
	kubeClient, err := kubeclient.GetKubeClientForCluster(ctx.ClusterContext)
	if err != nil {
		return errors.Wrapf(err, "failed to get kubeclient while creating machine %q", ctx)
	}

	// Get a new bootstrap token used to join this machine to the cluster.
	token, err := tokens.NewBootstrap(kubeClient, defaultTokenTTL)
	if err != nil {
		return errors.Wrapf(err, "unable to generate boostrap token for joining machine to cluster %q", ctx)
	}

	// Create the machine and join it to the cluster.
	return govmomi.Create(ctx, token)
}

// reconcileKubeConfig creates a secret on the management cluster with
// the kubeconfig for target cluster.
func (a *Actuator) reconcileKubeConfig(ctx *context.MachineContext) error {
	if !ctx.HasControlPlaneRole() {
		return nil
	}
	if ctx.IPAddr() == "" {
		ctx.Logger.V(6).Info("requeueing reconcileKubeConfig until IP addr is present")
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}

	// Get the name of the secret that stores the kubeconfig.
	secretName := remotev1.KubeConfigSecretName(ctx.Cluster.Name)

	// Create a new kubeconfig for the target cluster.
	ctx.Logger.V(6).Info("generating kubeconfig secret")
	kubeConfig, err := kubeclient.GetKubeConfig(ctx)
	if err != nil {
		return errors.Wrapf(err, "error generating kubeconfig for %q", ctx)
	}

	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		StringData: map[string]string{
			"value": kubeConfig,
		},
	}

	// Create the kubeconfig secret.
	if _, err := a.coreClient.Secrets(ctx.Cluster.Namespace).Create(secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "error creating kubeconfig secret for %q", ctx)
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
	if ctx.Machine.Status.NodeRef == nil {
		ctx.Logger.V(6).Info("requeuing until noderef is set")
		return &clusterErr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
	}
	return nil
}
