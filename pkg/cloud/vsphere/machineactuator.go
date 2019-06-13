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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
	capierr "sigs.k8s.io/cluster-api/pkg/controller/error"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/provisioner/govmomi"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/tokens"
)

const (
	defaultTokenTTL = 10 * time.Minute
)

type VsphereClient struct {
	clusterV1alpha1  clusterv1alpha1.ClusterV1alpha1Interface
	lister           v1alpha1.Interface
	controllerClient client.Client
	provisioner      *govmomi.Provisioner
}

//TODO: remove 2nd arguments
func NewGovmomiMachineActuator(m manager.Manager, clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface, k8sClient kubernetes.Interface, lister v1alpha1.Interface, eventRecorder record.EventRecorder) (*VsphereClient, error) {
	clusterClient, err := clientset.NewForConfig(m.GetConfig())
	if err != nil {
		klog.Fatalf("Invalid API configuration for kubeconfig-control: %v", err)
	}

	provisioner, err := govmomi.New(clusterClient.ClusterV1alpha1(), k8sClient, lister, eventRecorder)
	if err != nil {
		return nil, err
	}

	return &VsphereClient{
		clusterV1alpha1:  clusterV1alpha1,
		lister:           lister,
		controllerClient: m.GetClient(),
		provisioner:      provisioner,
	}, nil
}

func (vc *VsphereClient) Create(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine) error {

	if vc.provisioner == nil {
		return errors.New("No provisioner available")
	}

	if cluster == nil {
		return errors.Errorf(
			"missing cluster for machine %s/%s",
			machine.Namespace, machine.Name)
	}

	machineRole := vsphereutils.GetMachineRole(machine)
	if machineRole == "" {
		return errors.Errorf(
			"Unable to get machine role while creating machine "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	klog.V(2).Infof("Creating machine in cluster %s=%s %s=%s %s=%s %s=%s %s=%s",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Namespace,
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name,
		"machine-role", machineRole)

	controlPlaneMachines, err := vsphereutils.GetControlPlaneMachinesForCluster(cluster, vc.lister)
	if err != nil {
		return errors.Wrapf(
			err,
			"unable to get control plane machines while creating machine "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	// Init the control plane by creating this machine.
	if machineRole == "controlplane" && len(controlPlaneMachines) <= 1 {
		if err := vc.provisioner.Create(ctx, cluster, machine, ""); err != nil {
			return errors.Wrapf(err,
				"failed to create machine as initial member of the control plane "+
					"%s=%s %s=%s %s=%s %s=%s",
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Namespace,
				"machine-namespace", machine.Namespace,
				"machine-name", machine.Name)
		}
		return nil
	}

	// Join the existing cluster.
	online, _, err := vsphereutils.GetControlPlaneStatus(cluster, vc.lister)
	if !online {
		msg := fmt.Sprintf("unable to join machine to control plane until it is online "+
			"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
		if err != nil {
			klog.V(2).Info(errors.Wrap(err, msg))
			return errors.Cause(err)
		}
		klog.V(2).Info(msg)
		return &capierr.RequeueAfterError{RequeueAfter: time.Minute * 1}
	}

	// Get a Kubernetes client for the cluster.
	kubeClient, _ := vsphereutils.GetKubeClientForCluster(cluster, vc.lister)

	// Get a new bootstrap token used to join this machine to the cluster.
	token, err := tokens.NewBootstrap(kubeClient, defaultTokenTTL)
	if err != nil {
		return errors.Wrapf(
			err,
			"unable to generate boostrap token for joining machine to cluster "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	// Create the machine and join it to the cluster.
	if err := vc.provisioner.Create(ctx, cluster, machine, token); err != nil {
		return errors.Wrapf(err,
			"failed to create machine and join it to the cluster "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Namespace,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	return nil
}

func (vc *VsphereClient) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if vc.provisioner != nil {
		return vc.provisioner.Delete(ctx, cluster, machine)
	}

	return fmt.Errorf("No provisioner available")
}

func (vc *VsphereClient) Update(ctx context.Context, cluster *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	if vc.provisioner != nil {
		return vc.provisioner.Update(ctx, cluster, goalMachine)
	}

	return fmt.Errorf("No provisioner available")
}

func (vc *VsphereClient) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	if vc.provisioner != nil {
		return vc.provisioner.Exists(ctx, cluster, machine)
	}

	return false, fmt.Errorf("No provisioner available")
}

// GetControlPlaneMachines retrieves all control plane nodes from a MachineList
func GetControlPlaneMachines(machineList *clusterv1.MachineList) []*clusterv1.Machine {
	var cpm []*clusterv1.Machine
	for _, m := range machineList.Items {
		if m.Spec.Versions.ControlPlane != "" {
			cpm = append(cpm, m.DeepCopy())
		}
	}
	return cpm
}

// defining equality as name and namespace are equivalent and not checking any other fields.
func machinesEqual(m1 *clusterv1.Machine, m2 *clusterv1.Machine) bool {
	return m1.Name == m2.Name && m1.Namespace == m2.Namespace
}
