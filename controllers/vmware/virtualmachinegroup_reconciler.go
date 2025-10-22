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

// Package vmware contains the VirtualMachineGroup Reconciler.
package vmware

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	reconciliationDelay = 10 * time.Second
)

// VirtualMachineGroupReconciler reconciles VirtualMachineGroup.
type VirtualMachineGroupReconciler struct {
	Client   client.Client
	Recorder record.EventRecorder
}

func (r *VirtualMachineGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the Cluster instance.
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))
	// If Cluster is deleted, just return as VirtualMachineGroup will be GCed and no extra processing needed.
	if !cluster.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	// If ControlPlane haven't initialized, requeue it since VSphereMachines of MachineDeployment will only be created after
	// ControlPlane is initialized.
	if !conditions.IsTrue(cluster, clusterv1.ClusterControlPlaneInitializedCondition) {
		log.Info("Waiting for Cluster ControlPlaneInitialized")
		return reconcile.Result{RequeueAfter: reconciliationDelay}, nil
	}

	// Continue with the main logic.
	return r.createOrUpdateVMG(ctx, cluster)

}

// createOrUpdateVMG Create or Update VirtualMachineGroup
func (r *VirtualMachineGroupReconciler) createOrUpdateVMG(ctx context.Context, cluster *clusterv1.Cluster) (_ reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Calculate current Machines of all MachineDeployments.
	current, err := getCurrentVSphereMachines(ctx, r.Client, cluster.Namespace, cluster.Name)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get current VSphereMachine of cluster %s/%s",
			cluster.Name, cluster.Namespace)
	}

	desiredVMG := &vmoprv1.VirtualMachineGroup{}
	key := &client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}

	if err := r.Client.Get(ctx, *key, desiredVMG); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to get VirtualMachineGroup")
			return ctrl.Result{}, err
		}

		// Calculate expected Machines of all MachineDeployments.
		expected := getExpectedVSphereMachines(cluster)
		if expected == 0 {
			log.Info("none of MachineDeployments specifies replica and node auto replacement doesn't support this scenario")
			return reconcile.Result{}, nil
		}

		// Wait for all intended VSphereMachines corresponding to MachineDeployment to exist only during initial Cluster creation.
		current := int32(len(current))
		if current < expected {
			log.Info("current VSphereMachines do not match expected", "Expected:", expected,
				"Current:", current, "ClusterName", cluster.Name, "Namespace", cluster.Namespace)
			return reconcile.Result{RequeueAfter: reconciliationDelay}, nil
		}

		desiredVMG = &vmoprv1.VirtualMachineGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
	}

	// Generate VM names according to the naming strategy set on the VSphereMachine.
	vmNames := make([]string, 0, len(current))
	for _, machine := range current {
		name, err := GenerateVirtualMachineName(machine.Name, machine.Spec.NamingStrategy)
		if err != nil {
			return reconcile.Result{}, err
		}
		vmNames = append(vmNames, name)
	}
	// Sort the VM names alphabetically for consistent ordering
	sort.Slice(vmNames, func(i, j int) bool {
		return vmNames[i] < vmNames[j]
	})

	members := make([]vmoprv1.GroupMember, 0, len(current))
	for _, name := range vmNames {
		members = append(members, vmoprv1.GroupMember{
			Name: name,
			Kind: "VirtualMachine",
		})
	}

	// Get all the names of MachineDeployments of the Cluster.
	if !cluster.Spec.Topology.IsDefined() {
		return reconcile.Result{}, errors.Errorf("Cluster Topology is not defined %s/%s",
			cluster.Namespace, cluster.Name)
	}
	machineDeployments := &clusterv1.MachineDeploymentList{}
	if err := r.Client.List(ctx, machineDeployments,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name}); err != nil {
		return reconcile.Result{}, err
	}
	mdNames := []string{}
	for _, md := range machineDeployments.Items {
		mdNames = append(mdNames, md.Name)
	}

	// Use CreateOrPatch to create or update the VirtualMachineGroup.
	_, err = controllerutil.CreateOrPatch(ctx, r.Client, desiredVMG, func() error {
		// Set the desired labels
		if desiredVMG.Labels == nil {
			desiredVMG.Labels = make(map[string]string)
			// Set Cluster name label
			desiredVMG.Labels[clusterv1.ClusterNameLabel] = cluster.Name
		}

		// Add per-md-zone label for day-2 operations once placement of a VM belongs to MachineDeployment is done
		// Do not update per-md-zone label once set, as placement decision should not change without user explicitly
		// ask.
		placementDecisionLabels, err := GenerateVMGPlacementAnnotations(ctx, desiredVMG, mdNames)
		if err != nil {
			return err
		}
		if len(placementDecisionLabels) > 0 {
			for k, v := range placementDecisionLabels {
				if _, exists := desiredVMG.Labels[k]; exists {
					// Skip if the label already exists
					continue
				}
				desiredVMG.Labels[k] = v
			}
		}

		// Compose bootOrder.
		desiredVMG.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{
			{
				Members: members,
			},
		}

		// Make sure the Cluster owns the VM Operator VirtualMachineGroup.
		if err = controllerutil.SetControllerReference(cluster, desiredVMG, r.Client.Scheme()); err != nil {
			return errors.Wrapf(err, "failed to mark %s %s/%s as owner of %s %s/%s",
				cluster.GroupVersionKind(),
				cluster.Namespace,
				cluster.Name,
				desiredVMG.GroupVersionKind(),
				desiredVMG.Namespace,
				desiredVMG.Name)
		}

		return nil
	})

	return reconcile.Result{}, err
}

// isMDDefined checks if there are any MachineDeployments for the given cluster
// by listing objects with the cluster.x-k8s.io/cluster-name label.
func (r *VirtualMachineGroupReconciler) isMDDefined(ctx context.Context, cluster *clusterv1.Cluster) (bool, error) {
	mdList := &clusterv1.MachineDeploymentList{}
	if err := r.Client.List(ctx, mdList, client.InNamespace(cluster.Namespace), client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name}); err != nil {
		return false, errors.Wrapf(err, "failed to list MachineDeployments for cluster %s/%s",
			cluster.Namespace, cluster.Name)
	}

	if len(mdList.Items) == 0 {
		return false, errors.Errorf("no MachineDeployments found for cluster %s/%s",
			cluster.Namespace, cluster.Name)
	}

	return true, nil
}

// isExplicitPlacement checks if any MachineDeployment has an explicit failure domain set.
func (r *VirtualMachineGroupReconciler) isExplicitPlacement(cluster *clusterv1.Cluster) (bool, error) {
	// First, ensure MachineDeployments are defined
	mdDefined, err := r.isMDDefined(context.Background(), cluster)
	if !mdDefined {
		return false, err
	}

	// Iterate through MachineDeployments to find if an explicit failure domain is set.
	mds := cluster.Spec.Topology.Workers.MachineDeployments
	for _, md := range mds {
		// If a failure domain is specified for any MachineDeployment, it indicates
		// explicit placement is configured, so return true.
		if md.FailureDomain != "" {
			return true, nil
		}
	}

	return false, nil
}

// getExpectedVSphereMachines returns the total number of replicas across all
// MachineDeployments in the Cluster's Topology.Workers.
func getExpectedVSphereMachines(cluster *clusterv1.Cluster) int32 {
	if !cluster.Spec.Topology.IsDefined() {
		return 0
	}

	var total int32
	for _, md := range cluster.Spec.Topology.Workers.MachineDeployments {
		if md.Replicas != nil {
			total += *md.Replicas
		}
	}
	return total
}

func getCurrentVSphereMachines(ctx context.Context, kubeClient client.Client, clusterNamespace, clusterName string) ([]vmwarev1.VSphereMachine, error) {
	log := ctrl.LoggerFrom(ctx)

	// List VSphereMachine objects
	var vsMachineList vmwarev1.VSphereMachineList
	if err := kubeClient.List(ctx, &vsMachineList,
		client.InNamespace(clusterNamespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName},
		client.HasLabels{clusterv1.MachineDeploymentNameLabel},
	); err != nil {
		return nil, errors.Wrapf(err, "failed to list VSphereMachines in namespace %s", clusterNamespace)
	}

	var result []vmwarev1.VSphereMachine
	for _, vs := range vsMachineList.Items {
		if vs.DeletionTimestamp.IsZero() {
			result = append(result, vs)
		}
	}
	log.V(4).Info("Final list of VSphereMachines for VMG member generation", "count", len(result))

	return result, nil
}

// GenerateVMGPlacementAnnotations returns annotations per MachineDeployment which contains zone info for placed VMs for day-2 operations.
func GenerateVMGPlacementAnnotations(ctx context.Context, vmg *vmoprv1.VirtualMachineGroup, machineDeployments []string) (map[string]string, error) {
	log := ctrl.LoggerFrom(ctx)
	annotations := make(map[string]string)

	// For each member in status
	for _, member := range vmg.Status.Members {
		// Skip if not a VM or not placement ready,
		if member.Kind != "VirtualMachine" {
			return nil, errors.Errorf("VirtualMachineGroup %s/%s contains none VirtualMachine member, member.Kind %s", vmg.Namespace, vmg.Name, member.Kind)
		}

		// Once member VM is placed, VirtualMachineGroupMemberConditionPlacementReady will be set to true.
		if !conditions.IsTrue(&member, vmoprv1.VirtualMachineGroupMemberConditionPlacementReady) {
			continue
		}

		// Check if this VM belongs to any of our target Machine Deployments
		// Use machine deployment name as the annotation key prefix.
		for _, md := range machineDeployments {
			// Check if we already found placement for this Machine Deployments
			if _, found := annotations[fmt.Sprintf("zone.cluster.x-k8s.io/%s", md)]; found {
				log.Info(fmt.Sprintf("Skipping Machine Deployment %s, placement already found in annotations", md))
				continue
			}

			// Check if VM belongs to a Machine Deployment by name (e.g. cluster-1-np-1-vm-xxx contains np-1)
			// TODO: Establish membership via the machine deployment name label
			if strings.Contains(member.Name, md) {
				// Get the VM placement information by member status.
				// VMs that have undergone placement do not have Placement info set, skip.
				if member.Placement == nil {
					log.V(4).Info("VM in VMG has no placement info. Placement is nil", "VM", member.Name, "VMG", vmg.Name, "Namespace", vmg.Namespace)
					continue
				}

				// Skip to next member if Zone is empty.
				zone := member.Placement.Zone
				if zone == "" {
					log.V(4).Info("VM in VMG has no placement info. Zone is empty", "VM", member.Name, "VMG", vmg.Name, "Namespace", vmg.Namespace)
					continue
				}

				log.Info(fmt.Sprintf("VM %s in VMG %s/%s has been placed in zone %s", member.Name, vmg.Namespace, vmg.Name, zone))
				annotations[fmt.Sprintf("zone.cluster.x-k8s.io/%s", md)] = zone
			}
		}
	}

	return annotations, nil
}

// TODO: de-dup this logic with vmopmachine.go
// GenerateVirtualMachineName generates the name of a VirtualMachine based on the naming strategy.
func GenerateVirtualMachineName(machineName string, namingStrategy *vmwarev1.VirtualMachineNamingStrategy) (string, error) {
	// Per default the name of the VirtualMachine should be equal to the Machine name (this is the same as "{{ .machine.name }}")
	if namingStrategy == nil || namingStrategy.Template == nil {
		// Note: No need to trim to max length in this case as valid Machine names will also be valid VirtualMachine names.
		return machineName, nil
	}

	name, err := infrautilv1.GenerateMachineNameFromTemplate(machineName, namingStrategy.Template)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate name for VirtualMachine")
	}

	return name, nil
}
