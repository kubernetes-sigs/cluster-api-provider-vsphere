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
	"maps"
	"slices"
	"strings"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	return r.reconcileNormal(ctx, cluster)
}

// createOrUpdateVirtualMachineGroup Create or Update VirtualMachineGroup.
func (r *VirtualMachineGroupReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster) (reconcile.Result, error) {
	// Get all the data required for computing the desired VMG.
	currentVMG, err := r.getVirtualMachineGroup(ctx, cluster)
	if err != nil {
		return reconcile.Result{}, err
	}
	vSphereMachines, err := r.getVSphereMachines(ctx, cluster)
	if err != nil {
		return reconcile.Result{}, err
	}
	machineDeployments, err := r.getMachineDeployments(ctx, cluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Before initial placement VirtualMachineGroup does not exist yet.
	if currentVMG == nil {
		// VirtualMachineGroup creation starts the initial placement process that should take care
		// of spreading VSphereMachines across failure domains in an ideal way / according to user intent.
		// The initial placement can be performed only when all the VSphereMachines to be considered for the
		// placement decision exists; if this condition is not met, return (watches will trigger new
		// reconcile whenever new VSphereMachines are created).
		// Note: In case there are no MachineDeployments, or all the MachineDeployments have zero replicas,
		// no placement decision is required, and thus no VirtualMachineGroup will be created.
		if !shouldCreateVirtualMachineGroup(ctx, machineDeployments, vSphereMachines) {
			return reconcile.Result{}, nil
		}

		// Computes the new VirtualMachineGroup including all the VSphereMachines to be considered
		// for the initial placement decision.
		newVMG, err := computeVirtualMachineGroup(ctx, cluster, machineDeployments, vSphereMachines, nil)
		if err != nil {
			return reconcile.Result{}, err
		}

		// FIXME: Log. Details?
		if err := r.Client.Create(ctx, newVMG); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create new VMG")
		}
		return reconcile.Result{}, nil
	}

	// If the VirtualMachineGroup exist, either the placement decision is being performed, or
	// the placement decision has been already completed. In both cases, the VirtualMachineGroup
	// must be keep up to date with the changes that happens to MachineDeployments and vSphereMachines.
	//
	// However, while the initial placement decision is being performed, the addition of new
	// vSphereMachines to the VirtualMachineGroup must be deferred to prevent race conditions.
	//
	// After initial placement, new vSphereMachines will be added to the VirtualMachineGroup for
	// sake of consistency, but those machines will be placed in the same failureDomain
	// already used for the other vSphereMachines in the same MachineDeployment (new vSphereMachines
	// will align to the initial placement decision).

	// Computes the updated VirtualMachineGroup including reflecting changes in the cluster.
	updatedVMG, err := computeVirtualMachineGroup(ctx, cluster, machineDeployments, vSphereMachines, currentVMG)
	if err != nil {
		return reconcile.Result{}, err
	}

	// FIXME: Log. Diff? Details?
	if err := r.Client.Patch(ctx, updatedVMG, client.MergeFromWithOptions(currentVMG, client.MergeFromWithOptimisticLock{})); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to patch VMG")
	}
	return reconcile.Result{}, nil
}

// computeVirtualMachineGroup gets the desired VirtualMachineGroup.
func computeVirtualMachineGroup(ctx context.Context, cluster *clusterv1.Cluster, mds []clusterv1.MachineDeployment, vSphereMachines []vmwarev1.VSphereMachine, existingVMG *vmoprv1.VirtualMachineGroup) (*vmoprv1.VirtualMachineGroup, error) {
	// Create an empty VirtualMachineGroup
	vmg := &vmoprv1.VirtualMachineGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name,
			Namespace:   cluster.Namespace,
			Annotations: map[string]string{},
		},
	}

	// If there is an VirtualMachineGroup, clone it into the desired VirtualMachineGroup
	// and clean up all the info that must be re-computed during this reconcile.
	if existingVMG != nil {
		vmg = existingVMG.DeepCopy()
		vmg.Annotations = make(map[string]string)
		for key, value := range existingVMG.Annotations {
			if !strings.HasPrefix(key, ZoneAnnotationPrefix+"/") {
				vmg.Annotations[key] = value
			}
		}
	}
	vmg.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{{}}

	// Compute the info required to compute the VirtualMachineGroup.

	// Get the mapping between the virtualMachine name that will be generated by a vSphereMachine
	// and the MachineDeployment that controls the vSphereMachine.
	virtualMachineNameToMachineDeployment, err := getVirtualMachineNameToMachineDeploymentMapping(ctx, vSphereMachines)
	if err != nil {
		return nil, err
	}

	// Sort virtualMachine names to ensure VirtualMachineGroup is generated in a consistent way across reconcile.
	sortedVirtualMachineNames := slices.Sorted(maps.Keys(virtualMachineNameToMachineDeployment))

	// Get the mapping between the MachineDeployment and failure domain, which is one of:
	// - the failureDomain explicitly assigned by the user to a MachineDeployment (by setting spec.template.spec.failureDomain).
	// - the failureDomain selected by the placement decision for a MachineDeployment
	// Note: if a MachineDeployment is not included in this mapping, the MachineDeployment is still pending a placement decision.
	machineDeploymentToFailureDomain := getMachineDeploymentToFailureDomainMapping(ctx, mds, existingVMG, virtualMachineNameToMachineDeployment)

	// Set the annotations on the VirtualMachineGroup surfacing the failure domain selected during the
	// placement decision for each MachineDeployment.
	// Note: when a MachineDeployment will be deleted, the corresponding annotation will be removed (not added anymore by this func).
	for md, failureDomain := range machineDeploymentToFailureDomain {
		vmg.Annotations[fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, md)] = failureDomain
	}

	// Compute the list of Members for the VirtualMachineGroup.

	// If the VirtualMachineGroup is being created, ensure that all the existing VirtualMachines are
	// included in the VirtualMachineGroup for the initial placement decision.
	if existingVMG == nil {
		for _, virtualMachineName := range sortedVirtualMachineNames {
			vmg.Spec.BootOrder[0].Members = append(vmg.Spec.BootOrder[0].Members, vmoprv1.GroupMember{
				Name: virtualMachineName,
				Kind: "VirtualMachine",
			})
		}
		return vmg, nil
	}

	// If the VirtualMachineGroup exists, keep this list of VirtualMachine up to date.
	// Note: while the initial placement decision is being performed, the addition of new
	// VirtualMachine to the VirtualMachineGroup must be deferred to prevent race conditions.
	//
	// After initial placement, new VirtualMachine will be added to the VirtualMachineGroup for
	// sake of consistency, but those machines will be placed in the same failureDomain
	// already used for the other VirtualMachine in the same MachineDeployment (new VirtualMachine
	// will align to the initial placement decision).

	existingVirtualMachineNames := sets.New[string]()
	if len(existingVMG.Spec.BootOrder) > 0 {
		for _, member := range existingVMG.Spec.BootOrder[0].Members {
			existingVirtualMachineNames.Insert(member.Name)
		}
	}

	for _, virtualMachineName := range sortedVirtualMachineNames {
		// If a VirtualMachine is already part of the VirtualMachineGroup, keep it in the VirtualMachineGroup
		// Note: when a VirtualMachine will be deleted, the corresponding member will be removed (not added anymore by this func)
		if existingVirtualMachineNames.Has(virtualMachineName) {
			vmg.Spec.BootOrder[0].Members = append(vmg.Spec.BootOrder[0].Members, vmoprv1.GroupMember{
				Name: virtualMachineName,
				Kind: "VirtualMachine",
			})
			continue
		}

		// If a VirtualMachine is not yet in the VirtualMachineGroup, it should be added only if
		// the VirtualMachine is controlled by a MachineDeployment for which the placement decision is already
		// completed.
		// Note: If the placement decision for the MachineDeployment controlling a VirtualMachine is still pending,
		// this logic defers adding the VirtualMachine in the VirtualMachineGroup to prevent race conditions.
		md := virtualMachineNameToMachineDeployment[virtualMachineName]
		if _, isPlaced := machineDeploymentToFailureDomain[md]; isPlaced {
			vmg.Spec.BootOrder[0].Members = append(vmg.Spec.BootOrder[0].Members, vmoprv1.GroupMember{
				Name: virtualMachineName,
				Kind: "VirtualMachine",
			})
		}
	}

	return vmg, nil
}

// getMachineDeploymentToFailureDomainMapping returns the mapping between MachineDeployment and failure domain.
// The mapping is computed according to following rules:
//   - If the MachineDeployment is explicitly assigned to a failure domain by setting spec.template.spec.failureDomain,
//     use this value for the mapping.
//   - If the annotations on the VirtualMachineGroup already has the failure domain selected during the
//     initial placement decision for a MachineDeployment, use it.
//   - If annotations on the VirtualMachineGroup are not yet set, try to get the failure domain selected
//     during the initial placement decision from VirtualMachineGroup status (placement decision just completed).
//   - If none of the above rules are satisfied, the MachineDeployment is still pending a placement decision.
//
// Note: In case the failure domain is explicitly assigned by setting spec.template.spec.failureDomain, the mapping always
// report the latest value for this field (even if there might still be Machines yet to be rolled out to the new failure domain).
func getMachineDeploymentToFailureDomainMapping(_ context.Context, mds []clusterv1.MachineDeployment, existingVMG *vmoprv1.VirtualMachineGroup, virtualMachineNameToMachineDeployment map[string]string) map[string]string {
	machineDeploymentToFailureDomainMapping := map[string]string{}
	for _, md := range mds {
		if !md.DeletionTimestamp.IsZero() {
			continue
		}

		// If the MachineDeployment is explicitly assigned to a failure domain by setting spec.template.spec.failureDomain, use this value for the mapping.
		if md.Spec.Template.Spec.FailureDomain != "" {
			machineDeploymentToFailureDomainMapping[md.Name] = md.Spec.Template.Spec.FailureDomain
			continue
		}

		// If the MachineDeployment is not explicitly assigned to a failure domain (spec.template.spec.failureDomain is empty),
		// and VirtualMachineGroup does not exist yet, the MachineDeployment is still pending a placement decision.
		if existingVMG == nil {
			continue
		}

		// If the VirtualMachineGroup exist, check if the placement decision for the MachineDeployment
		// has been already surfaced into the VirtualMachineGroup annotations.
		if failureDomain := existingVMG.Annotations[fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, md.Name)]; failureDomain != "" {
			machineDeploymentToFailureDomainMapping[md.Name] = failureDomain
			continue
		}

		// If the placement decision for the MachineDeployment, try to get the failure domain selected
		// during the initial placement decision from VirtualMachineGroup status (placement decision just completed).
		// Note: this info will surface in VirtualMachineGroup annotations at the end of the current reconcile.
		for _, member := range existingVMG.Status.Members {
			// Ignore members controller by other MachineDeployments
			if memberMD := virtualMachineNameToMachineDeployment[member.Name]; memberMD != md.Name {
				continue
			}

			// Consider only VirtualMachineGroup members for which the placement decision has been completed.
			// Note: given that all the VirtualMachines in a MachineDeployment must be placed in the
			// same failure domain / zone, the mapping can be inferred as soon as one member is placed.
			if !conditions.IsTrue(&member, vmoprv1.VirtualMachineGroupMemberConditionPlacementReady) {
				continue
			}
			if member.Placement != nil && member.Placement.Zone != "" {
				// FIXME: log
				machineDeploymentToFailureDomainMapping[md.Name] = member.Placement.Zone
				break
			}
		}
	}
	return machineDeploymentToFailureDomainMapping
}

// getVirtualMachineNameToMachineDeploymentMapping returns the mapping between VirtualMachine name and corresponding MachineDeployment.
// The mapping is inferred from vSphereMachines; please note:
// - The name of the VirtualMachine generated by a vSphereMachines can be computed in a deterministic way (it is not required to wait for the VirtualMachine to exist)
// - The name of the MachineDeployment corresponding to a vSphereMachine can be derived from the annotation that is propagated by CAPI.
func getVirtualMachineNameToMachineDeploymentMapping(_ context.Context, vSphereMachines []vmwarev1.VSphereMachine) (map[string]string, error) {
	virtualMachineNameToMachineDeployment := map[string]string{}
	for _, vsphereMachine := range vSphereMachines {
		if !vsphereMachine.DeletionTimestamp.IsZero() {
			continue
		}

		virtualMachineName, err := vmoperator.GenerateVirtualMachineName(vsphereMachine.Name, vsphereMachine.Spec.NamingStrategy)
		if err != nil {
			return nil, err
		}
		if md := vsphereMachine.Labels[clusterv1.MachineDeploymentNameLabel]; md != "" {
			virtualMachineNameToMachineDeployment[virtualMachineName] = md
		}
	}
	return virtualMachineNameToMachineDeployment, nil
}

// shouldCreateVirtualMachineGroup should return true when the conditions to create a VirtualMachineGroup are met.
func shouldCreateVirtualMachineGroup(ctx context.Context, mds []clusterv1.MachineDeployment, vSphereMachines []vmwarev1.VSphereMachine) bool {
	log := ctrl.LoggerFrom(ctx)

	// Gets the total number or worker machines that should exist in the cluster at a given time.
	// Note. Deleting MachineDeployment are ignored.
	var expectedVSphereMachineCount int32
	for _, md := range mds {
		if md.DeletionTimestamp.IsZero() {
			expectedVSphereMachineCount += ptr.Deref(md.Spec.Replicas, 0)
		}
	}

	// In case there are no MachineDeployments or all the MachineDeployments have zero replicas, there is
	// no need to create a VirtualMachineGroup.
	if expectedVSphereMachineCount == 0 {
		return false
	}

	// If the number of workers VSphereMachines matches the number of expected replicas in the MachineDeployments,
	// then all the VSphereMachines required for the initial placement decision do exist, then it is possible to create
	// the VirtualMachineGroup.
	// FIXME: we should probably include in the count only machines for MD included above (otherwise machines from deleting MS might lead to false positives / negatives
	currentVSphereMachineCount := int32(len(vSphereMachines))
	if currentVSphereMachineCount != expectedVSphereMachineCount {
		log.Info(fmt.Sprintf("Waiting for VSphereMachines required for the initial placement (expected %d, current %d)", expectedVSphereMachineCount, currentVSphereMachineCount))
		return false
	}
	return true
}

func (r *VirtualMachineGroupReconciler) getVirtualMachineGroup(ctx context.Context, cluster *clusterv1.Cluster) (*vmoprv1.VirtualMachineGroup, error) {
	vmg := &vmoprv1.VirtualMachineGroup{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(cluster), vmg); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to get VirtualMachineGroup %s", klog.KObj(vmg))
		}
		return nil, nil
	}
	return vmg, nil
}

func (r *VirtualMachineGroupReconciler) getVSphereMachines(ctx context.Context, cluster *clusterv1.Cluster) ([]vmwarev1.VSphereMachine, error) {
	var vsMachineList vmwarev1.VSphereMachineList
	if err := r.Client.List(ctx, &vsMachineList,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name},
		client.HasLabels{clusterv1.MachineDeploymentNameLabel},
	); err != nil {
		return nil, errors.Wrap(err, "failed to get VSphereMachines")
	}
	return vsMachineList.Items, nil
}

func (r *VirtualMachineGroupReconciler) getMachineDeployments(ctx context.Context, cluster *clusterv1.Cluster) ([]clusterv1.MachineDeployment, error) {
	machineDeployments := &clusterv1.MachineDeploymentList{}
	if err := r.Client.List(ctx, machineDeployments,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name},
	); err != nil {
		return nil, errors.Wrap(err, "failed to list MachineDeployments")
	}
	return machineDeployments.Items, nil
}
