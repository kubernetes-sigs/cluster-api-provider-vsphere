/*
Copyright 2017 The Kubernetes Authors.

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

package vsphere

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
	capierr "sigs.k8s.io/cluster-api/pkg/controller/error"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/patch"

	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/provisioner/govmomi"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/tokens"
)

const (
	defaultTokenTTL = 10 * time.Minute
)

type MachineActuator struct {
	clusterV1alpha1  clusterv1alpha1.ClusterV1alpha1Interface
	lister           v1alpha1.Interface
	controllerClient client.Client
	provisioner      *govmomi.Provisioner
	eventRecorder    record.EventRecorder
}

func NewMachineActuator(
	m manager.Manager,
	clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface,
	k8sClient kubernetes.Interface,
	lister v1alpha1.Interface,
	eventRecorder record.EventRecorder) (*MachineActuator, error) {

	clusterClient, err := clientset.NewForConfig(m.GetConfig())
	if err != nil {
		klog.Fatalf("Invalid API configuration for kubeconfig-control: %v", err)
	}

	provisioner, err := govmomi.New(clusterClient.ClusterV1alpha1(), k8sClient, lister, eventRecorder)
	if err != nil {
		return nil, err
	}

	return &MachineActuator{
		clusterV1alpha1:  clusterV1alpha1,
		lister:           lister,
		controllerClient: m.GetClient(),
		provisioner:      provisioner,
		eventRecorder:    eventRecorder,
	}, nil
}

func (a *MachineActuator) Create(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine) (result error) {

	if cluster == nil {
		return errors.Errorf(
			"missing cluster for machine %s/%s",
			machine.Namespace, machine.Name)
	}

	machineRole := vsphereutils.GetMachineRole(machine)
	if machineRole == "" {
		return errors.Errorf(
			"unable to get machine role while creating machine %s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	klog.V(2).Infof("Creating machine in cluster %s=%s %s=%s %s=%s %s=%s %s=%s",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Name,
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name,
		"machine-role", machineRole)

	clusterConfig, err := vsphereconfigv1.ClusterConfigFromCluster(cluster)
	if err != nil {
		return errors.Wrapf(err,
			"unable to get cluster config while creating machine %s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	if !clusterConfig.CAKeyPair.HasCertAndKey() {
		klog.V(4).Infof("cluster config is missing pki toolchain, requeue machine %s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
		return &capierr.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds}
	}

	machineCopy := machine.DeepCopy()
	defer func() {
		if err := a.patchMachine(machine, machineCopy); err != nil {
			if result == nil {
				result = err
			} else {
				result = errors.Wrap(result, err.Error())
			}
		}
	}()

	controlPlaneMachines, err := vsphereutils.GetControlPlaneMachinesForCluster(cluster, a.lister)
	if err != nil {
		return errors.Wrapf(err,
			"unable to get control plane machines while creating machine %s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	// Init the control plane by creating this machine.
	if machineRole == "controlplane" && len(controlPlaneMachines) == 1 {
		if err := a.provisioner.Create(ctx, cluster, machine, ""); err != nil {
			return errors.Wrapf(err,
				"failed to create machine as initial member of the control plane %s=%s %s=%s %s=%s %s=%s",
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Name,
				"machine-namespace", machine.Namespace,
				"machine-name", machine.Name)
		}
		return nil
	}

	// Join the existing cluster.
	online, _, err := vsphereutils.GetControlPlaneStatus(cluster, a.lister)
	if !online {
		msg := fmt.Sprintf("unable to join machine to control plane until it is online %s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
		if err != nil {
			// Log the error wrapped with the message since we don't return
			// the full error, only the cause (which may be a Requeue error)
			klog.V(2).Info(errors.Wrap(err, msg))
			return errors.Cause(err)
		}
		klog.V(2).Info(msg)
		return &capierr.RequeueAfterError{RequeueAfter: time.Minute * 1}
	}

	// Get a Kubernetes client for the cluster.
	kubeClient, err := vsphereutils.GetKubeClientForCluster(cluster, a.lister)
	if err != nil {
		return errors.Wrapf(err,
			"failed to get kubeclient while creating machine %s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	// Get a new bootstrap token used to join this machine to the cluster.
	token, err := tokens.NewBootstrap(kubeClient, defaultTokenTTL)
	if err != nil {
		return errors.Wrapf(err,
			"unable to generate boostrap token for joining machine to cluster %s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	// Create the machine and join it to the cluster.
	if err := a.provisioner.Create(ctx, cluster, machine, token); err != nil {
		return errors.Wrapf(err,
			"failed to create machine and join it to the cluster %s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	return nil
}

func (a *MachineActuator) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (result error) {
	klog.V(2).Infof("Deleting machine in cluster %s=%s %s=%s %s=%s %s=%s",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Name,
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name)

	machineCopy := machine.DeepCopy()
	defer func() {
		if err := a.patchMachine(machine, machineCopy); err != nil {
			if result == nil {
				result = err
			} else {
				result = errors.Wrap(result, err.Error())
			}
		}
	}()

	return a.provisioner.Delete(ctx, cluster, machine)
}

func (a *MachineActuator) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (result error) {
	klog.V(2).Infof("Updating machine in cluster %s=%s %s=%s %s=%s %s=%s",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Name,
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name)

	machineCopy := machine.DeepCopy()
	defer func() {
		if err := a.patchMachine(machine, machineCopy); err != nil {
			if result == nil {
				result = err
			} else {
				result = errors.Wrap(result, err.Error())
			}
		}
	}()

	return a.provisioner.Update(ctx, cluster, machine)
}

func (a *MachineActuator) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	return a.provisioner.Exists(ctx, cluster, machine)
}

func (a *MachineActuator) patchMachine(
	machine, machineCopy *clusterv1.Machine) error {

	machineClient := a.clusterV1alpha1.Machines(machine.Namespace)

	machineConfig, err := vsphereconfigv1.MachineConfigFromMachine(machine)
	if err != nil {
		return errors.Wrapf(err,
			"unable to get machine provider spec for machine while patching machine %s=%s %s=%s",
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	machineStatus, err := vsphereconfigv1.MachineStatusFromMachine(machine)
	if err != nil {
		return errors.Wrapf(err,
			"unable to get machine provider status for machine while patching machine %s=%s %s=%s",
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	ext, err := vsphereconfigv1.EncodeMachineSpec(machineConfig)
	if err != nil {
		return errors.Wrap(err, "failed encoding machine spec")
	}
	newStatus, err := vsphereconfigv1.EncodeMachineStatus(machineStatus)
	if err != nil {
		return errors.Wrap(err, "failed encoding machine status")
	}

	machine.Spec.ProviderSpec.Value = ext

	// Build a patch and marshal that patch to something the client will understand.
	p, err := patch.NewJSONPatch(machineCopy, machine)
	if err != nil {
		return errors.Wrap(err, "failed to create new JSONPatch")
	}

	// Do not update Machine if nothing has changed
	if len(p) != 0 {
		pb, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return errors.Wrap(err, "failed to json marshal patch")
		}

		klog.V(1).Infof(
			"patching machine %s=%s %s=%s",
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)

		result, err := machineClient.Patch(machine.Name, types.JSONPatchType, pb)
		if err != nil {
			a.eventRecorder.Eventf(
				machine, corev1.EventTypeWarning,
				"UpdateFailure",
				"failed to update machine config %s=%s %s=%s %s=%v",
				"machine-namespace", machine.Namespace,
				"machine-name", machine.Name,
				"error", err)
			return errors.Wrap(err, "failed to patch machine")
		}

		a.eventRecorder.Eventf(
			machine, corev1.EventTypeNormal,
			"UpdateSuccess",
			"updated machine config %s=%s %s=%s",
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)

		// Keep the resource version updated so the status update can succeed
		machine.ResourceVersion = result.ResourceVersion
	}

	machine.Status.ProviderStatus = newStatus

	if !reflect.DeepEqual(machine.Status, machineCopy.Status) {
		klog.V(1).Infof(
			"updating machine status %s=%s %s=%s",
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
		if _, err := machineClient.UpdateStatus(machine); err != nil {
			a.eventRecorder.Eventf(
				machine, corev1.EventTypeWarning,
				"UpdateFailure",
				"failed to update machine status %s=%s %s=%s %s=%v",
				"machine-namespace", machine.Namespace,
				"machine-name", machine.Name,
				"error", err)
			return errors.Wrap(err, "failed to update machine status")
		}
	}

	a.eventRecorder.Eventf(
		machine, corev1.EventTypeNormal,
		"UpdateSuccess",
		"updated machine status %s=%s %s=%s",
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name)

	return nil
}
