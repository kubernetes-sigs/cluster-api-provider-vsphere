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

	vmg := &vmoprv1.VirtualMachineGroup{}
	key := &client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}

	if err := r.Client.Get(ctx, *key, vmg); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to get VirtualMachineGroup")
			return ctrl.Result{}, err
		}
		vmg = &vmoprv1.VirtualMachineGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
	}

	// // Proceed only if multiple zones are available.
	// // If there is only one zone(default), node automatic placement is unnecessary
	// // because all Machine Deployments will be scheduled into that single zone.
	// // The VSphereCluster resource discovers the underlying zones,
	// // which we treat as the source of truth.
	// vsphereClusterList := &vmwarev1.VSphereClusterList{}
	// labelKey := clusterv1.ClusterNameLabel
	// if err := r.Client.List(ctx, vsphereClusterList,
	// 	client.InNamespace(cluster.Namespace),
	// 	client.MatchingLabels(map[string]string{labelKey: cluster.Name}),
	// ); err != nil {
	// 	return reconcile.Result{}, fmt.Errorf("failed to list VSphereClusters in namespace %s: %w", cluster.Namespace, err)
	// }

	// vsphereCluster := &vmwarev1.VSphereCluster{}
	// switch len(vsphereClusterList.Items) {
	// case 0:
	// 	return reconcile.Result{}, fmt.Errorf("no VSphereCluster found with label %s=%s in namespace %s", labelKey, cluster.Name, cluster.Namespace)
	// case 1:
	// 	vsphereCluster = &vsphereClusterList.Items[0]
	// default:
	// 	return reconcile.Result{}, fmt.Errorf("found %d VSphereClusters with label %s=%s in namespace %s; expected exactly 1", len(vsphereClusterList.Items), labelKey, cluster.Name, cluster.Namespace)
	// }

	// // Fetch the VSphereCluster instance.
	// if vsphereCluster.Status.Ready != true {
	// 	log.Info("Waiting for VSphereCluster to be ready with failure domain discovered")
	// 	return reconcile.Result{RequeueAfter: reconciliationDelay}, nil

	// }

	// if len(vsphereCluster.Status.FailureDomains) <= 1 {
	// 	log.Info("Single or no zone detected; skipping node automatic placement")
	// 	return reconcile.Result{}, nil
	// }

	// If ControlPlane haven't initialized, requeue it since VSphereMachines of MachineDeployment will only be created after
	// ControlPlane is initialized.
	if !conditions.IsTrue(cluster, clusterv1.ClusterControlPlaneInitializedCondition) {
		log.Info("Waiting for Cluster ControlPlaneInitialized")
		return reconcile.Result{RequeueAfter: reconciliationDelay}, nil
	}

	// Continue with the main logic.
	return r.createOrUpdateVMG(ctx, cluster, vmg)

}

// createOrUpdateVMG Create or Update VirtualMachineGroup
func (r *VirtualMachineGroupReconciler) createOrUpdateVMG(ctx context.Context, cluster *clusterv1.Cluster, desiredVMG *vmoprv1.VirtualMachineGroup) (_ reconcile.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Calculate expected Machines of all MachineDeployments.
	expectedMachines := getExpectedMachines(cluster)
	if expectedMachines == 0 {
		log.Info("none of MachineDeployments specifies replica and node auto replacement doesn't support this scenario")
		return reconcile.Result{}, nil
	}

	// Calculate current Machines of all MachineDeployments.
	currentVSphereMachines, err := getCurrentVSphereMachines(ctx, r.Client, cluster.Namespace, cluster.Name)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get current VSphereMachine of cluster %s/%s",
			cluster.Name, cluster.Namespace)
	}

	// Wait until all VSphereMachines are create, this could happen during initial deployment or day-2 like cluster update.
	current := int32(len(currentVSphereMachines))
	if current < expectedMachines {
		// Only check timeout if VMG doesn't exist.
		// if desiredVMG.CreationTimestamp.IsZero() {
		// 	if _, err := r.isMDDefined(ctx, cluster); err != nil {
		// 		log.Error(err, "cluster MachineDeployments are not defined")
		// 		return reconcile.Result{}, nil
		// 	}

		// 	mdList := &clusterv1.MachineDeploymentList{}
		// 	if err := r.Client.List(ctx, mdList,
		// 		client.InNamespace(cluster.Namespace),
		// 		client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name},
		// 	); err != nil {
		// 		return reconcile.Result{}, errors.Errorf("failed to list MachineDeployments: %w", err)
		// 	}

		// 	// If no deployments exist, report error
		// 	if len(mdList.Items) == 0 {
		// 		return reconcile.Result{}, errors.Errorf("no MachineDeployments found for cluster %s/%s", cluster.Namespace, cluster.Name)
		// 	}

		// 	// Check one MachineDeployment's creation timestamp
		// 	firstMD := mdList.Items[0]
		// 	if time.Since(firstMD.CreationTimestamp.Time) > 1*time.Minute {
		// 		log.Error(errors.New("timeout waiting for VSphereMachines"), "1 minute timeout after MachineDeployment creation",
		// 			"MachineDeployment", firstMD.Name, "Cluster", cluster.Namespace+"/"+cluster.Name)

		// 		return reconcile.Result{}, nil
		// 	}
		// }

		log.Info("current VSphereMachines do not match expected", "Expected:", expectedMachines,
			"Current:", current, "ClusterName", cluster.Name, "Namespace", cluster.Namespace)
		return reconcile.Result{RequeueAfter: reconciliationDelay}, nil
	}

	// Generate all the members of the VirtualMachineGroup.
	members := make([]vmoprv1.GroupMember, 0, len(currentVSphereMachines))
	// Sort the VSphereMachines by name for consistent ordering
	sort.Slice(currentVSphereMachines, func(i, j int) bool {
		return currentVSphereMachines[i].Name < currentVSphereMachines[j].Name
	})

	for _, vm := range currentVSphereMachines {
		members = append(members, vmoprv1.GroupMember{
			Name: vm.Name,
			Kind: "VirtualMachine",
		})
	}

	// Get all the names of MachineDeployments of the Cluster.
	if !cluster.Spec.Topology.IsDefined() {
		return reconcile.Result{}, errors.Errorf("Cluster Topology is not defined %s/%s",
			cluster.Namespace, cluster.Name)
	}
	mds := cluster.Spec.Topology.Workers.MachineDeployments
	mdNames := make([]string, 0, len(mds))
	for _, md := range mds {
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
		placementDecisionLabels, err := GenerateVMGPlacementLabels(ctx, desiredVMG, mdNames)
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

// getExpectedMachines returns the total number of replicas across all
// MachineDeployments in the Cluster's Topology.Workers.
func getExpectedMachines(cluster *clusterv1.Cluster) int32 {
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

	// // List MachineDeployments for the cluster.
	// var mdList clusterv1.MachineDeploymentList
	// if err := kubeClient.List(ctx, &mdList,
	// 	client.InNamespace(clusterNamespace),
	// 	client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName},
	// ); err != nil {
	// 	return nil, errors.Wrapf(err, "failed to list MachineDeployments for cluster %s/%s", clusterNamespace, clusterName)
	// }
	// validMDs := make(map[string]struct{})
	// for _, md := range mdList.Items {
	// 	validMDs[md.Name] = struct{}{}
	// }
	// log.V(6).Info("Identified active MachineDeployments", "count", len(validMDs))

	// // List MachineSets and filter those owned by a valid MachineDeployment.
	// var msList clusterv1.MachineSetList
	// if err := kubeClient.List(ctx, &msList,
	// 	client.InNamespace(clusterNamespace),
	// 	client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName},
	// ); err != nil {
	// 	return nil, errors.Wrapf(err, "failed to list MachineSets for cluster %s/%s", clusterNamespace, clusterName)
	// }
	// validMS := make(map[string]struct{})
	// for _, ms := range msList.Items {
	// 	for _, owner := range ms.OwnerReferences {
	// 		if owner.Kind == "MachineDeployment" && owner.APIVersion == clusterv1.GroupVersion.String() {
	// 			if _, ok := validMDs[owner.Name]; ok {
	// 				validMS[ms.Name] = struct{}{}
	// 				break
	// 			}
	// 		}
	// 	}
	// }
	// log.V(6).Info("Filtered MachineSets owned by valid MachineDeployments", "count", len(validMS))

	// // List Machines and filter those owned by valid MachineSets (skip control plane).
	// var machineList clusterv1.MachineList
	// if err := kubeClient.List(ctx, &machineList,
	// 	client.InNamespace(clusterNamespace),
	// 	client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName},
	// ); err != nil {
	// 	return nil, errors.Wrapf(err, "failed to list Machines for cluster %s/%s", clusterNamespace, clusterName)
	// }

	// workerMachines := make(map[string]struct{})
	// for _, m := range machineList.Items {
	// 	if _, isControlPlane := m.Labels[clusterv1.MachineControlPlaneLabel]; isControlPlane {
	// 		continue
	// 	}
	// 	for _, owner := range m.OwnerReferences {
	// 		if owner.Kind == "MachineSet" && owner.APIVersion == clusterv1.GroupVersion.String() {
	// 			if _, ok := validMS[owner.Name]; ok {
	// 				workerMachines[m.Name] = struct{}{}
	// 				break
	// 			}
	// 		}
	// 	}
	// }
	// log.V(5).Info("Identified worker Machines linked to MachineSets", "count", len(workerMachines))

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

// GenerateVMGPlacementLabels returns labels per MachineDeployment which contain zone info for placed VMs for day-2 operationss
func GenerateVMGPlacementLabels(ctx context.Context, vmg *vmoprv1.VirtualMachineGroup, machineDeployments []string) (map[string]string, error) {
	log := ctrl.LoggerFrom(ctx)
	labels := make(map[string]string)

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
		// Use machine deployment name as the label key.
		for _, md := range machineDeployments {
			// Check if we already found placement for this Machine Deployments
			if _, found := labels[md]; found {
				log.Info(fmt.Sprintf("Skipping Machine Deployment %s, placement already found", md))
				continue
			}

			// Check if VM belongs to a Machine Deployment by name (e.g. cluster-1-np-1-vm-xxx contains np-1)
			if strings.Contains(member.Name, md) {
				// Get the VM placement information by member status.
				if member.Placement == nil {
					return nil, errors.Errorf("VM %s in VMG %s/%s has no placement info. Placement is nil)", member.Name, vmg.Namespace, vmg.Name)
				}

				// Get the VM placement information by member status.
				zone := member.Placement.Zone
				if zone == "" {
					return nil, errors.Errorf("VM %s in VMG %s/%s has no placement info. Zone is empty", member.Name, vmg.Namespace, vmg.Name)
				}

				log.Info(fmt.Sprintf("VM %s in VMG %s/%s has been placed in zone %s", member.Name, vmg.Namespace, vmg.Name, zone))
				labels[fmt.Sprintf("zone.cluster.x-k8s.io/%s", md)] = zone
			}
		}
	}

	return labels, nil
}
