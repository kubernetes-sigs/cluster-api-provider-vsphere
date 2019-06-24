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

	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clientv1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	clustererr "sigs.k8s.io/cluster-api/pkg/controller/error"

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

// Actuator is responsible for maintaining the Machine objects.
type Actuator struct {
	client     clientv1.ClusterV1alpha1Interface
	coreClient corev1.CoreV1Interface
}

// NewActuator returns a new instance of Actuator.
func NewActuator(
	client clientv1.ClusterV1alpha1Interface,
	coreClient corev1.CoreV1Interface) *Actuator {

	return &Actuator{
		client:     client,
		coreClient: coreClient,
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

	ctx.Logger.V(4).Info("updating machine")

	if err := govmomi.Update(ctx); err != nil {
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

	defer func() {
		opErr = actuators.PatchAndHandleError(ctx, "Exists", opErr)
	}()

	return govmomi.Exists(ctx)
}

func (a *Actuator) reconcilePKI(ctx *context.MachineContext) error {
	if !ctx.ClusterConfig.CAKeyPair.HasCertAndKey() {
		ctx.Logger.V(6).Info("cluster config is missing pki toolchain, requeue machine")
		return &clustererr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue}
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
		return &clustererr.RequeueAfterError{RequeueAfter: time.Minute * 1}
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

// reconcileReadyState sets an annotation on the machine, marking it as ready,
// once the machine has IP addresses.
func (a *Actuator) reconcileReadyState(ctx *context.MachineContext) error {
	if len(ctx.Machine.Status.Addresses) > 0 {
		if ctx.Machine.Annotations[constants.ReadyAnnotationLabel] == "" {
			if ctx.Machine.Annotations == nil {
				ctx.Machine.Annotations = map[string]string{}
			}
			ctx.Machine.Annotations[constants.ReadyAnnotationLabel] = "true"
			ctx.Logger.V(6).Info("machine is ready")
		}
	}
	return nil
}
