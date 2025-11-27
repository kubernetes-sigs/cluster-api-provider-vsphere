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
	"strings"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/vmoperator"
)

const (
	// ZoneAnnotationPrefix is the prefix used for placement decision annotations which will be set on VirtualMachineGroup.
	ZoneAnnotationPrefix = "zone.vmware.infrastructure.cluster.x-k8s.io"
)

// VirtualMachineGroupReconciler reconciles VirtualMachineGroup.
type VirtualMachineGroupReconciler struct {
	Client   client.Client
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinegroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinegroups/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vmware.infrastructure.cluster.x-k8s.io,resources=vspheremachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch

// This controller is introduced to coordinate the creation and maintenance of
// the VirtualMachineGroup (VMG) object with respect to the worker VSphereMachines in the Cluster.
//
// - Batch Coordination: Gating the initial creation of the VMG until for the first time all the
// MachineDeployment replicas will have a corresponding VSphereMachine.
// Once this condition is met, the VirtualMachineGroup is created considering
// the initial set of machines for the initial placement decision.
// When the VirtualMachineGroup reports the placement decision, then finally
// creation of VirtualMachines is unblocked.
//
// - Placement Persistence: Persisting the MachineDeployment-to-Zone mapping (placement decision) as a
// metadata annotation on the VMG object. The same decision must be respected also for placement
// of machines created after initial placement.
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

	// Note: VirtualMachineGroup is going to have same name and namespace of the cluster.
	// Using cluster here, because VirtualMachineGroup is created only after initial placement completes.
	log = log.WithValues("VirtualMachineGroup", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// If Cluster is deleted, just return as VirtualMachineGroup will be GCed and no extra processing needed.
	if !cluster.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	// If ControlPlane haven't initialized, requeue it since CAPV will only start to reconcile VSphereMachines of
	// MachineDeployment after ControlPlane is initialized.
	if !conditions.IsTrue(cluster, clusterv1.ClusterControlPlaneInitializedCondition) {
		return reconcile.Result{}, nil
	}

	return r.createOrUpdateVirtualMachineGroup(ctx, cluster)
}

// createOrUpdateVirtualMachineGroup Create or Update VirtualMachineGroup.
func (r *VirtualMachineGroupReconciler) createOrUpdateVirtualMachineGroup(ctx context.Context, cluster *clusterv1.Cluster) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Get current VSphereMachines of all MachineDeployments.
	currentVSphereMachines, err := getCurrentVSphereMachines(ctx, r.Client, cluster.Namespace, cluster.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	vmg := &vmoprv1.VirtualMachineGroup{}
	key := &client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}

	if err := r.Client.Get(ctx, *key, vmg); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, errors.Wrapf(err, "failed to get VirtualMachineGroup %s", klog.KObj(vmg))
		}

		// If the VirtualMachineGroup does not exist yet,
		// calculate expected VSphereMachine count of all MachineDeployments.
		expectedVSphereMachineCount, err := getExpectedVSphereMachineCount(ctx, r.Client, cluster)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to get expected Machines of all MachineDeployment, Cluster %s", klog.KObj(cluster))
		}

		// Since CAPV retrieves placement decisions from the VirtualMachineGroup to guide
		// day-2 worker VM placement. At least one VM is expected for each MachineDeployment.
		// If no worker of MachineDeployment is defined,the controller
		// interprets this as an intentional configuration, just logs the observation and no-op.
		if expectedVSphereMachineCount == 0 {
			log.Info("Found 0 desired VSphereMachine of MachineDeployment, stop reconcile")
			return reconcile.Result{}, nil
		}

		// Wait for all intended VSphereMachines corresponding to MachineDeployment to exist only during initial Cluster creation.
		// For day-2, VirtualMachineGroup exists and should not run into here  wait for VSphereMachines.
		currentVSphereMachineCount := int32(len(currentVSphereMachines))
		if currentVSphereMachineCount != expectedVSphereMachineCount {
			log.Info("Waiting for expected VSphereMachines required for the initial placement call", "Expected:", expectedVSphereMachineCount,
				"Current:", currentVSphereMachineCount, "Cluster", klog.KObj(cluster))
			return reconcile.Result{}, nil
		}

		vmg = &vmoprv1.VirtualMachineGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
		}
	}

	// Generate VM names according to the naming strategy set on the VSphereMachine.
	vmNames := make([]string, 0, len(currentVSphereMachines))
	for _, machine := range currentVSphereMachines {
		name, err := vmoperator.GenerateVirtualMachineName(machine.Name, machine.Spec.NamingStrategy)
		if err != nil {
			return reconcile.Result{}, err
		}
		vmNames = append(vmNames, name)
	}
	// Sort the VM names alphabetically for consistent ordering
	slices.Sort(vmNames)

	members := make([]vmoprv1.GroupMember, 0, len(currentVSphereMachines))
	for _, name := range vmNames {
		members = append(members, vmoprv1.GroupMember{
			Name: name,
			Kind: "VirtualMachine",
		})
	}

	// The core purpose of isCreateOrPatchAllowed is to prevent the VirtualMachineGroup from being updated with new members
	// that require placement, unless the VirtualMachineGroup
	// has successfully completed its initial placement and added the required
	// placement annotations. This stabilizes placement decisions before allowing new VMs
	// to be added under the group.
	//
	// The CreateOrPatch is allowed if:
	// 1. The VirtualMachineGroup is being initially created.
	// 2. The update won't add new member:
	//    1) scale-down operation
	//    2) no member change.
	// 3. When the VirtualMachineGroup is placement Ready, continue to check following.
	//    1) The new member's underlying CAPI Machine has a FailureDomain set (will skip placement process).
	//    2) The new member requires placement annotation AND the VirtualMachineGroup has the corresponding
	//       placement annotation for the member's MachineDeployment.
	//
	// This prevents member updates that could lead to new VMs being created
	// without necessary zone labels, resulting in undesired placement.
	err = isCreateOrPatchAllowed(ctx, r.Client, members, vmg)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Use CreateOrPatch to create or update the VirtualMachineGroup.
	_, err = controllerutil.CreateOrPatch(ctx, r.Client, vmg, func() error {
		return r.reconcileVirtualMachineGroup(ctx, vmg, cluster, members)
	})

	return reconcile.Result{}, err
}

// reconcileVirtualMachineGroup mutates the VirtualMachineGroup object to reflect the necessary spec and metadata changes.
func (r *VirtualMachineGroupReconciler) reconcileVirtualMachineGroup(ctx context.Context, vmg *vmoprv1.VirtualMachineGroup, cluster *clusterv1.Cluster, members []vmoprv1.GroupMember) error {
	// Set the desired labels
	if vmg.Labels == nil {
		vmg.Labels = make(map[string]string)
	}
	// Always ensure cluster name label is set
	vmg.Labels[clusterv1.ClusterNameLabel] = cluster.Name

	if vmg.Annotations == nil {
		vmg.Annotations = make(map[string]string)
	}

	// Get all the names of MachineDeployments of the Cluster.
	machineDeployments := &clusterv1.MachineDeploymentList{}
	if err := r.Client.List(ctx, machineDeployments,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name}); err != nil {
		return err
	}
	mdNames := []string{}
	for _, md := range machineDeployments.Items {
		// Skip MachineDeployment marked for removal.
		if !md.DeletionTimestamp.IsZero() {
			mdNames = append(mdNames, md.Name)
		}
	}

	// Add per-md-zone label for day-2 operations once placement of a VM belongs to MachineDeployment is done.
	// Do not update per-md-zone label once set, as placement decision should not change without user explicitly
	// set failureDomain.
	if err := generateVirtualMachineGroupAnnotations(ctx, r.Client, vmg, mdNames); err != nil {
		return err
	}

	vmg.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{
		{
			Members: members,
		},
	}

	// Set the owner reference
	if err := controllerutil.SetControllerReference(cluster, vmg, r.Client.Scheme()); err != nil {
		return errors.Wrapf(err, "failed to mark Cluster %s as owner of VirtualMachineGroup %s", klog.KObj(cluster), klog.KObj(vmg))
	}

	return nil
}

// isCreateOrPatchAllowed checks if a VirtualMachineGroup is allowd to create or patch by check if BootOrder.Members update is allowed.
func isCreateOrPatchAllowed(ctx context.Context, kubeClient client.Client, targetMember []vmoprv1.GroupMember, vmg *vmoprv1.VirtualMachineGroup) error {
	logger := log.FromContext(ctx)
	key := client.ObjectKey{
		Namespace: vmg.Namespace,
		Name:      vmg.Name,
	}

	// Retrieve the current VirtualMachineGroup state
	currentVMG := &vmoprv1.VirtualMachineGroup{}
	if err := kubeClient.Get(ctx, key, currentVMG); err != nil {
		if apierrors.IsNotFound(err) {
			// 1. If VirtualMachineGroup is not found, allow CreateOrPatch as it should be in initial creation phase.
			logger.V(6).Info("VirtualMachineGroup not created yet, allowing create")
			return nil
		}
		return errors.Wrapf(err, "failed to get VirtualMachineGroup %s/%s, blocking patch", vmg.Namespace, vmg.Name)
	}
	// Copy retrieved data back to the input pointer for consistency.
	*vmg = *currentVMG

	// Get current member names from VirtualMachineGroup Spec.BootOrder.
	currentMemberNames := make(map[string]struct{})
	if len(vmg.Spec.BootOrder) > 0 {
		for _, m := range vmg.Spec.BootOrder[0].Members {
			currentMemberNames[m.Name] = struct{}{}
		}
	}

	// 2. If removing members, allow immediately since it doesn't impact placement or placement annotation set.
	if len(targetMember) < len(currentMemberNames) {
		logger.V(6).Info("Scaling down detected (fewer target members), allowing patch.")
		return nil
	}

	var newMembers []vmoprv1.GroupMember
	for _, m := range targetMember {
		if _, exists := currentMemberNames[m.Name]; !exists {
			newMembers = append(newMembers, m)
		}
	}

	// If no new member added, allow patch.
	if len(newMembers) == 0 {
		logger.V(6).Info("No new member detected, allowing patch.")
		return nil
	}

	// 3. If initial placement is still in progress, block adding new member.
	if !conditions.IsTrue(vmg, vmoprv1.ReadyConditionType) {
		return fmt.Errorf("waiting for VirtualMachineGroup %s to get condition %s to true, temporarily blocking patch", klog.KObj(vmg), vmoprv1.ReadyConditionType)
	}

	// 4. Check newly added members for Machine.Spec.FailureDomain via VSphereMachine.If a member belongs to a Machine
	// which has failureDomain specified, allow it since it will skip the placement
	// process. If not, continue to check if the belonging MachineDeployment has got placement annotation.
	for _, newMember := range newMembers {
		vsphereMachineKey := types.NamespacedName{
			Namespace: vmg.Namespace,
			Name:      newMember.Name, // Member Name is the VSphereMachine Name.
		}
		vsphereMachine := &vmwarev1.VSphereMachine{}
		if err := kubeClient.Get(ctx, vsphereMachineKey, vsphereMachine); err != nil {
			if apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "VSphereMachine for new member %s not found, temporarily blocking patch", newMember.Name)
			}
			return errors.Wrapf(err, "failed to get VSphereMachine %s", klog.KRef(newMember.Name, vmg.Namespace))
		}

		var machineOwnerName string
		for _, owner := range vsphereMachine.OwnerReferences {
			if owner.Kind == "Machine" {
				machineOwnerName = owner.Name
				break
			}
		}

		if machineOwnerName == "" {
			// VSphereMachine found but owner Machine reference is missing
			return fmt.Errorf("VSphereMachine %s found but owner Machine reference is missing, temporarily blocking patch", newMember.Name)
		}

		machineKey := types.NamespacedName{
			Namespace: vmg.Namespace,
			Name:      machineOwnerName,
		}
		machine := &clusterv1.Machine{}

		if err := kubeClient.Get(ctx, machineKey, machine); err != nil {
			if apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "Machine %s not found via owner reference, temporarily blocking patch", klog.KRef(machineOwnerName, vmg.Namespace))
			}
			return errors.Wrapf(err, "failed to get CAPI Machine %s", klog.KRef(machineOwnerName, vmg.Namespace))
		}

		// If FailureDomain is set on CAPI Machine, placement process will be skipped. Allow update for this member.
		fd := machine.Spec.FailureDomain
		if fd != "" {
			logger.V(6).Info("New member's Machine has FailureDomain specified. Allowing patch", "Member", newMember.Name)
			continue
		}

		// 5. If FailureDomain is NOT set. Requires placement or placement Annotation. Fall through to Annotation check.
		// If no Placement Annotations, block member update and wait for it.
		annotations := vmg.GetAnnotations()
		if len(annotations) == 0 {
			return fmt.Errorf("waiting for placement annotation to add VMG member %s, temporarily blocking patch", newMember.Name)
		}

		mdLabelName := vsphereMachine.Labels[clusterv1.MachineDeploymentNameLabel]
		if mdLabelName == "" {
			return fmt.Errorf("VSphereMachine doesn't have MachineDeployment name label %s, blocking patch", klog.KObj(vsphereMachine))
		}

		annotationKey := fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdLabelName)
		if _, found := annotations[annotationKey]; !found {
			return fmt.Errorf("waiting for placement annotation %s to add VMG member %s, temporarily blocking patch", annotationKey, newMember.Name)
		}
	}

	logger.V(6).Info("All newly added members either existed or have satisfied placement requirements, allowing patch")
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
		// Skip MachineDeployment marked for removal
		if md.DeletionTimestamp.IsZero() && md.Spec.Replicas != nil {
			total += *md.Spec.Replicas
		}
	}

	return total, nil
}

// getCurrentVSphereMachines returns the list of VSphereMachines belonging to the Cluster’s MachineDeployments.
// VSphereMachines marked for removal are excluded from the result.
func getCurrentVSphereMachines(ctx context.Context, kubeClient client.Client, clusterNamespace, clusterName string) ([]vmwarev1.VSphereMachine, error) {
	// List VSphereMachine objects
	var vsMachineList vmwarev1.VSphereMachineList
	if err := kubeClient.List(ctx, &vsMachineList,
		client.InNamespace(clusterNamespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName},
		client.HasLabels{clusterv1.MachineDeploymentNameLabel},
	); err != nil {
		return nil, errors.Wrapf(err, "failed to list VSphereMachines of Cluster %s", klog.KRef(clusterNamespace, clusterName))
	}

	var result []vmwarev1.VSphereMachine
	for _, vs := range vsMachineList.Items {
		if vs.DeletionTimestamp.IsZero() {
			result = append(result, vs)
		}
	}
	return result, nil
}

// generateVirtualMachineGroupAnnotations checks the VMG status for placed members, verifies their ownership
// by fetching the corresponding VSphereMachine, and extracts the zone information to persist it
// as an annotation on the VMG object for Day-2 operations. It will also clean up
// any existing placement annotations that correspond to MachineDeployments that no longer exist.
//
// The function attempts to find at least one successfully placed VM (VirtualMachineGroupMemberConditionPlacementReady==True)
// for each MachineDeployment and records its zone. Once a Zone is recorded for an MD, subsequent VMs
// belonging to that same MD are skipped.
func generateVirtualMachineGroupAnnotations(ctx context.Context, kubeClient client.Client, vmg *vmoprv1.VirtualMachineGroup, machineDeployments []string) error {
	log := ctrl.LoggerFrom(ctx)
	log.V(5).Info(fmt.Sprintf("Generating annotations for VirtualMachineGroup %s/%s", vmg.Name, vmg.Namespace))

	if vmg.Annotations == nil {
		vmg.Annotations = make(map[string]string)
	}
	annotations := vmg.Annotations

	// If a MachineDeployment has been deleted, its corresponding placement annotation
	// on the VirtualMachineGroup should also be removed to avoid configuration drift.
	activeMDs := sets.New(machineDeployments...)

	// Iterate over existing VirtualMachineGroup annotations and delete those that are stale.
	for key := range annotations {
		if !strings.HasPrefix(key, ZoneAnnotationPrefix+"/") {
			// Skip non-placement annotations
			continue
		}

		mdName := strings.TrimPrefix(key, ZoneAnnotationPrefix+"/")

		// If the MD name is NOT in the list of currently active MDs, delete the annotation.
		if !activeMDs.Has(mdName) {
			log.Info(fmt.Sprintf("Cleaning up stale placement annotation for none-existed MachineDeployment %s", mdName))
			delete(annotations, key)
		}
	}

	// Iterate through the VirtualMachineGroup's members in Status.
	for _, member := range vmg.Status.Members {
		ns := vmg.Namespace

		// Skip it if member's VirtualMachineGroupMemberConditionPlacementReady is still not true.
		if !conditions.IsTrue(&member, vmoprv1.VirtualMachineGroupMemberConditionPlacementReady) {
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
			return errors.Wrapf(err, "failed to get VSphereMachine %s/%s", member.Name, ns)
		}

		mdName, found := vsm.Labels[clusterv1.MachineDeploymentNameLabel]
		if !found {
			log.Info(fmt.Sprintf("Failed to get MachineDeployment label from VSphereMachine %s/%s, skipping it", member.Name, ns))
			continue
		}

		// If we already found placement for this MachineDeployment, continue and move to next member.
		if _, found := annotations[fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName)]; found {
			continue
		}

		// Check if this VM belongs to any of our target MachineDeployments.
		if !activeMDs.Has(mdName) {
			log.V(5).Info("Skipping member as its MachineDeployment name is not in the known list.",
				"VMName", member.Name, "MDName", mdName)
			continue
		}

		// Get the VM placement information by member status.
		// VMs that have undergone placement do not have Placement info set, skip.
		// VMs of Machine with failureDomain specified do not have Placement info set, skip.
		if member.Placement == nil {
			log.V(5).Info(fmt.Sprintf("VM %s in VMG %s/%s has no placement info. Placement is nil", member.Name, vmg.Name, ns))
			continue
		}

		// Skip to next member if Zone is empty.
		zone := member.Placement.Zone
		if zone == "" {
			log.V(5).Info(fmt.Sprintf("VM %s in VMG %s/%s has no placement info. Zone is empty", member.Name, "VMG", ns))
			continue
		}

		log.V(5).Info(fmt.Sprintf("VM %s in VMG %s/%s has been placed in zone %s", member.Name, ns, vmg.Name, zone))
		annotations[fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName)] = zone
	}

	return nil
}
