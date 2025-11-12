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
	"time"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

const (
	// reconciliationDelay is the delay time for requeueAfter.
	reconciliationDelay = 10 * time.Second
	// ZoneAnnotationPrefix is the prefix used for placement decision annotations which will be set on VirtualMachineGroup.
	ZoneAnnotationPrefix = "zone.cluster.x-k8s.io"
)

// VirtualMachineGroupReconciler reconciles VirtualMachineGroup.
type VirtualMachineGroupReconciler struct {
	Client   client.Client
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinegroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinegroups/status,verbs=get
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch

// This controller is introduced by CAPV to coordinate the creation and maintenance of
// the VirtualMachineGroup (VMG) object with respect to the worker VSphereMachines in the Cluster.
//
// - Batch Coordination: Gating the initial creation of the VMG until all expected worker
// VSphereMachines are present. This ensures the complete VM member list is sent to the VM
// Service in a single batch operation due to a limitation of underlying service.
//
// - Placement Persistence: Persisting the MachineDeployment-to-Zone mapping (placement decision) as a
// metadata annotation on the VMG object. This decision is crucial for guiding newer VMs created
// during Day-2 operations such as scaling, upgrades, and remediations, ensuring consistency. This is also due to
// a known limitation of underlying services.
//
// - Membership Maintenance: Dynamically updating the VMG's member list to reflect the current
// state of VMs belonging to MachineDeployments (handling scale-up/down events).

func (r *VirtualMachineGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	return r.createOrUpdateVirtualMachineGroup(ctx, cluster)
}

// createOrUpdateVirtualMachineGroup Create or Update VirtualMachineGroup.
func (r *VirtualMachineGroupReconciler) createOrUpdateVirtualMachineGroup(ctx context.Context, cluster *clusterv1.Cluster) (reconcile.Result, error) {
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
			return reconcile.Result{}, err
		}

		// Calculate expected Machines of all MachineDeployments.
		// CAPV retrieves placement decisions from the VirtualMachineGroup to guide
		// day-2 VM placement. At least one VM is required for each MachineDeployment.
		expected, err := getExpectedVSphereMachineCount(ctx, r.Client, cluster)
		if err != nil {
			log.Error(err, "failed to get expected Machines of all MachineDeployment")
			return reconcile.Result{}, err
		}

		if expected == 0 {
			errMsg := fmt.Sprintf("Found 0 desired VSphereMachine for Cluster %s/%s", cluster.Name, cluster.Namespace)
			log.Error(nil, errMsg)
			return reconcile.Result{}, errors.New(errMsg)
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
		return r.reconcileVirtualMachineState(ctx, desiredVMG, cluster, members, mdNames)
	})

	return reconcile.Result{}, err
}

// reconcileVirtualMachineState mutates the desiredVMG object to reflect the necessary spec and metadata changes.
func (r *VirtualMachineGroupReconciler) reconcileVirtualMachineState(ctx context.Context, desiredVMG *vmoprv1.VirtualMachineGroup, cluster *clusterv1.Cluster, members []vmoprv1.GroupMember, mdNames []string) error {
	// Set the desired labels
	if desiredVMG.Labels == nil {
		desiredVMG.Labels = make(map[string]string)
		desiredVMG.Labels[clusterv1.ClusterNameLabel] = cluster.Name
	}

	if desiredVMG.Annotations == nil {
		desiredVMG.Annotations = make(map[string]string)
	}

	// Add per-md-zone label for day-2 operations once placement of a VM belongs to MachineDeployment is done.
	// Do not update per-md-zone label once set, as placement decision should not change without user explicitly
	// set failureDomain.
	placementAnnotations, err := GenerateVirtualMachineGroupAnnotations(ctx, r.Client, desiredVMG, mdNames)
	if err != nil {
		return err
	}
	if len(placementAnnotations) > 0 {
		for k, v := range placementAnnotations {
			if _, exists := desiredVMG.Annotations[k]; !exists {
				desiredVMG.Annotations[k] = v
			}
		}
	}

	// Set the BootOrder spec as the
	desiredVMG.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{
		{
			Members: members,
		},
	}

	// Set the owner reference
	if err := controllerutil.SetControllerReference(cluster, desiredVMG, r.Client.Scheme()); err != nil {
		return errors.Wrapf(err, "failed to mark %s as owner of %s", klog.KObj(cluster), klog.KObj(desiredVMG))
	}

	return nil
}

// getExpectedVSphereMachineCount get expected total count of Machines belonging to the Cluster.
func getExpectedVSphereMachineCount(ctx context.Context, kubeClient client.Client, cluster *clusterv1.Cluster) (int32, error) {
	var mdList clusterv1.MachineDeploymentList
	if err := kubeClient.List(
		ctx,
		&mdList,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name},
	); err != nil {
		return 0, errors.Wrap(err, "failed to list MachineDeployments")
	}

	var total int32
	for _, md := range mdList.Items {
		if md.Spec.Replicas != nil {
			total += *md.Spec.Replicas
		}
	}

	return total, nil
}

// getCurrentVSphereMachines returns the list of VSphereMachines belonging to the Cluster’s MachineDeployments.
// VSphereMachines marked for removal are excluded from the result.
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

// GenerateVirtualMachineGroupAnnotations checks the VMG status for placed members, verifies their ownership
// by fetching the corresponding VSphereMachine, and extracts the zone information to persist it
// as an annotation on the VMG object for Day-2 operations.
//
// The function attempts to find at least one successfully placed VM (VirtualMachineGroupMemberConditionPlacementReady==True)
// for each MachineDeployment and records its zone. Once a zone is recorded for an MD, subsequent VMs
// belonging to that same MD are skipped.
func GenerateVirtualMachineGroupAnnotations(ctx context.Context, kubeClient client.Client, vmg *vmoprv1.VirtualMachineGroup, machineDeployments []string) (map[string]string, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info(fmt.Sprintf("Generating annotations for VirtualMachineGroup %s/%s", vmg.Name, vmg.Namespace))

	annotations := vmg.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Iterate through the VMG's members in Status.
	for _, member := range vmg.Status.Members {
		ns := vmg.Namespace
		// Only VirtualMachines contribute to placement decisions.
		if member.Kind != "VirtualMachine" {
			log.Info(fmt.Sprintf("Member %s of %s/%s is not VirtualMachine type, skipping it", member.Name, vmg.Name, vmg.Namespace))
			continue
		}

		// Skip it if member's VirtualMachineGroupMemberConditionPlacementReady is still not true.
		if !conditions.IsTrue(&member, vmoprv1.VirtualMachineGroupMemberConditionPlacementReady) {
			log.Info(fmt.Sprintf("Member %s of %s/%s is not PlacementReady, skipping it", member.Name, vmg.Name, vmg.Namespace))
			continue
		}

		// Get VSphereMachine which share the same Name of the member Name and get the MachineDeployment Name it belonged to.
		vsmKey := types.NamespacedName{
			Name:      member.Name,
			Namespace: vmg.Namespace,
		}
		vsm := &vmwarev1.VSphereMachine{}
		if err := kubeClient.Get(ctx, vsmKey, vsm); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info(fmt.Sprintf("VSphereMachine %s/%s by member Name %s is not found, skipping it", member.Name, ns, member.Name))
				continue
			}
			log.Error(err, "failed to get VSphereMachine %s/%s", member.Name, ns)
			return nil, err
		}

		mdNameFromLabel, found := vsm.Labels[clusterv1.MachineDeploymentNameLabel]
		if !found {
			log.Info(fmt.Sprintf("Failed to get MachineDeployment label from VSphereMachine %s/%s, skipping it", member.Name, ns))
			continue
		}

		// If we already found placement for this MachineDeployment, continue and move to next member.
		if v, found := annotations[fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdNameFromLabel)]; found {
			log.V(4).Info(fmt.Sprintf("Skipping MachineDeployment %s/%s, placement annotation %s already found", mdNameFromLabel, vsm.Namespace, v))
			continue
		}

		// Check if this VM belongs to any of our target MachineDeployments.
		// Annotation format is "zone.cluster.x-k8s.io/{machine-deployment-name}".
		for _, md := range machineDeployments {
			if mdNameFromLabel != md {
				continue
			}

			// Get the VM placement information by member status.
			// VMs that have undergone placement do not have Placement info set, skip.
			if member.Placement == nil {
				log.V(4).Info(fmt.Sprintf("VM %s in VMG %s/%s has no placement info. Placement is nil", member.Name, vmg.Name, ns))
				continue
			}

			// Skip to next member if Zone is empty.
			zone := member.Placement.Zone
			if zone == "" {
				log.V(4).Info(fmt.Sprintf("VM %s in VMG %s/%s has no placement info. Zone is empty", member.Name, "VMG", ns))
				continue
			}

			log.Info(fmt.Sprintf("VM %s in VMG %s/%s has been placed in zone %s", member.Name, ns, vmg.Name, zone))
			annotations[fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, md)] = zone
			// Break from the inner loop as placement for this MachineDeployment is found.
			break
		}
	}

	return annotations, nil
}

// GenerateVirtualMachineName generates the name of a VirtualMachine based on the naming strategy.
// Duplicated this logic from pkg/services/vmoperator/vmopmachine.go.
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
