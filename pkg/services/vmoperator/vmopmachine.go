/*
Copyright 2021 The Kubernetes Authors.

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

package vmoperator

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/govmomi/v1beta2"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
	conversionclient "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/client"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

const (
	// ZoneAnnotationPrefix is the prefix used for placement decision annotations which will be set on VirtualMachineGroup.
	ZoneAnnotationPrefix = "zone.vmware.infrastructure.cluster.x-k8s.io"
)

// VmopMachineService reconciles VM Operator VM.
type VmopMachineService struct {
	Client client.Client
}

// GetMachinesInCluster returns a list of VSphereMachine objects belonging to the cluster.
func (v *VmopMachineService) GetMachinesInCluster(
	ctx context.Context,
	namespace, clusterName string) ([]client.Object, error) {
	labels := map[string]string{clusterv1.ClusterNameLabel: clusterName}
	machineList := &vmwarev1.VSphereMachineList{}

	if err := v.Client.List(
		ctx, machineList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	objects := []client.Object{}
	for _, machine := range machineList.Items {
		m := machine
		objects = append(objects, &m)
	}
	return objects, nil
}

// FetchVSphereMachine returns a MachineContext with a VSphereMachine for the passed NamespacedName.
func (v *VmopMachineService) FetchVSphereMachine(ctx context.Context, name apitypes.NamespacedName) (capvcontext.MachineContext, error) {
	vsphereMachine := &vmwarev1.VSphereMachine{}
	err := v.Client.Get(ctx, name, vsphereMachine)
	return &vmware.SupervisorMachineContext{VSphereMachine: vsphereMachine}, err
}

// FetchVSphereCluster adds the VSphereCluster for the cluster to the MachineContext.
func (v *VmopMachineService) FetchVSphereCluster(ctx context.Context, cluster *clusterv1.Cluster, machineContext capvcontext.MachineContext) (capvcontext.MachineContext, error) {
	machineCtx, ok := machineContext.(*vmware.SupervisorMachineContext)
	if !ok {
		return nil, pkgerrors.New("received unexpected SupervisorMachineContext type")
	}

	vsphereCluster := &vmwarev1.VSphereCluster{}
	key := client.ObjectKey{
		Namespace: machineContext.GetObjectMeta().Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	err := v.Client.Get(ctx, key, vsphereCluster)

	machineCtx.VSphereCluster = vsphereCluster
	return machineCtx, err
}

// ReconcileDelete reconciles delete events for VM Operator VM.
func (v *VmopMachineService) ReconcileDelete(ctx context.Context, machineCtx capvcontext.MachineContext) error {
	log := ctrl.LoggerFrom(ctx)
	supervisorMachineCtx, ok := machineCtx.(*vmware.SupervisorMachineContext)
	if !ok {
		return pkgerrors.New("received unexpected SupervisorMachineContext type")
	}
	log.Info("Deleting VirtualMachine")

	// If debug logging is enabled, report the number of vms in the cluster before and after the reconcile
	if log.V(5).Enabled() {
		vms, err := v.getVirtualMachinesInCluster(ctx, supervisorMachineCtx)
		log.V(5).Info("Trace Destroy PRE: VirtualMachines", "vmcount", len(vms), "err", err)
		defer func() {
			vms, err := v.getVirtualMachinesInCluster(ctx, supervisorMachineCtx)
			log.V(5).Info("Trace Destroy POST: VirtualMachines", "vmcount", len(vms), "err", err)
		}()
	}

	// First, check to see if it's already deleted
	vmOperatorVM := &vmoprvhub.VirtualMachine{}
	key, err := virtualMachineObjectKey(supervisorMachineCtx.Machine.Name, supervisorMachineCtx.Machine.Namespace, supervisorMachineCtx.VSphereMachine.Spec.Naming)
	if err != nil {
		return err
	}
	if err := v.Client.Get(ctx, *key, vmOperatorVM); err != nil {
		// If debug logging is enabled, report the number of vms in the cluster before and after the reconcile
		if apierrors.IsNotFound(err) {
			supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhaseNotFound
			return err
		}
		supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhaseError
		return err
	}

	// Next, check to see if it's in the process of being deleted
	if vmOperatorVM.GetDeletionTimestamp() != nil {
		supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhaseDeleting
		return nil
	}

	// If none of the above are true, Delete the VM
	if err := v.Client.Delete(ctx, vmOperatorVM); err != nil {
		if apierrors.IsNotFound(err) {
			supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhaseNotFound
			return err
		}
		supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhaseError
		return err
	}
	supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhaseDeleting
	return nil
}

// SyncFailureReason returns true if there is a Failure on the VM Operator VM.
func (v *VmopMachineService) SyncFailureReason(_ context.Context, _ capvcontext.MachineContext) error {
	return nil
}

// affinityInfo is an internal struct used to store information about VM affinity.
type affinityInfo struct {
	affinitySpec  vmoprvhub.AffinitySpec
	vmGroupName   string
	failureDomain string
}

// ReconcileNormal reconciles create and update events for VM Operator VMs.
func (v *VmopMachineService) ReconcileNormal(ctx context.Context, machineCtx capvcontext.MachineContext) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	supervisorMachineCtx, ok := machineCtx.(*vmware.SupervisorMachineContext)
	if !ok {
		return false, pkgerrors.New("received unexpected SupervisorMachineContext type")
	}

	// If debug logging is enabled, report the number of vms in the cluster before and after the reconcile
	if log.V(5).Enabled() {
		vms, err := v.getVirtualMachinesInCluster(ctx, supervisorMachineCtx)
		log.V(5).Info("Trace ReconcileVM PRE: VirtualMachines", "vmcount", len(vms), "err", err)
		defer func() {
			vms, err = v.getVirtualMachinesInCluster(ctx, supervisorMachineCtx)
			log.V(5).Info("Trace ReconcileVM POST: VirtualMachines", "vmcount", len(vms), "err", err)
		}()
	}

	// Set the VM state. Will get reset throughout the reconcile
	supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhasePending

	// Get the VirtualMachine object Key
	vmOperatorVM := &vmoprvhub.VirtualMachine{}
	vmKey, err := virtualMachineObjectKey(supervisorMachineCtx.Machine.Name, supervisorMachineCtx.Machine.Namespace, supervisorMachineCtx.VSphereMachine.Spec.Naming)
	if err != nil {
		return false, err
	}

	log = log.WithValues("VirtualMachine", klog.KRef(vmKey.Namespace, vmKey.Name))
	ctx = ctrl.LoggerInto(ctx, log)

	// The following placement rules apply when the NodeAutoPlacement feature gate is enabled.
	//
	// When creating a new cluster and the user doesn't provide info about placement of VMs in a specific failure domain,
	// CAPV will define affinity rules to ensure proper placement of the machine.
	//
	// - All the machines belonging to the same MachineDeployment should be placed in the same failure domain - required.
	// - All the machines belonging to the same MachineDeployment should be spread across esxi hosts in the same failure domain - best-efforts.
	// - Different MachineDeployments and corresponding VMs should be spread across failure domains - best-efforts.
	//
	// Note: Control plane VM placement doesn't follow the above rules, and the assumption
	// is that failureDomain is always set for control plane VMs.
	//
	// When WorkerStrictAntiAffinity is also enabled, user-configured anti-affinity terms
	// apply at the host level and replace the default host-level anti-affinity rule.
	// Zone-level affinity and anti-affinity rules still apply.
	var affInfo *affinityInfo
	if feature.Gates.Enabled(feature.NodeAutoPlacement) &&
		!infrautilv1.IsControlPlaneMachine(machineCtx.GetVSphereMachine()) {
		vmGroup := &vmoprvhub.VirtualMachineGroup{}
		key := client.ObjectKey{
			Namespace: supervisorMachineCtx.Cluster.Namespace,
			Name:      supervisorMachineCtx.Cluster.Name,
		}
		err := v.Client.Get(ctx, key, vmGroup)

		// The VirtualMachineGroup controller is going to create the vmg only when all the VSphereMachines required for the placement
		// decision exist. If the vmg does not exist yet, requeue.
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return false, err
			}

			conditions.Set(supervisorMachineCtx.VSphereMachine, metav1.Condition{
				Type:    infrav1.VSphereMachineVirtualMachineProvisionedCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.VSphereMachineVirtualMachineWaitingForVirtualMachineGroupReason,
				Message: fmt.Sprintf("Waiting for VSphereMachine's VirtualMachineGroup %s to exist", key),
			})
			log.V(4).Info(fmt.Sprintf("Waiting for VirtualMachineGroup %s to exist, requeueing", key.Name), "VirtualMachineGroup", klog.KRef(key.Namespace, key.Name))
			return true, nil
		}

		// The VirtualMachineGroup controller is going to add a VM in the vmg only when the creation of this
		// VM does not impact the placement decision. If the VM is not yet included in the member list, requeue.
		isMember := v.checkVirtualMachineGroupMembership(vmGroup, vmKey.Name)
		if !isMember {
			conditions.Set(supervisorMachineCtx.VSphereMachine, metav1.Condition{
				Type:    infrav1.VSphereMachineVirtualMachineProvisionedCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.VSphereMachineVirtualMachineWaitingForVirtualMachineGroupReason,
				Message: fmt.Sprintf("Waiting for VirtualMachineGroup %s to have %s as a member", klog.KObj(vmGroup), vmKey.Name),
			})
			log.V(4).Info(fmt.Sprintf("Waiting for VirtualMachineGroup %s to have the vm as a member, requeueing", key.Name), "VirtualMachineGroup", klog.KObj(vmGroup))
			return true, nil
		}

		affInfo = &affinityInfo{
			vmGroupName: vmGroup.Name,
		}

		// Set the zone label using the annotation of the per-md zone mapping from VirtualMachineGroup.
		// This is for new VMs created after initial placement decision/with a failureDomain defined by the user.
		mdName := supervisorMachineCtx.Machine.Labels[clusterv1.MachineDeploymentNameLabel]
		if fd, ok := vmGroup.Annotations[fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName)]; ok && fd != "" {
			affInfo.failureDomain = fd
		}

		// VM in a MachineDeployment ideally should be placed in a different failure domain than VMs
		// in other MachineDeployments.
		// In order to do so, collect names of all the MachineDeployments except the one the VM belongs to.
		machineDeployments := &clusterv1.MachineDeploymentList{}
		if err := v.Client.List(ctx, machineDeployments,
			client.InNamespace(supervisorMachineCtx.Cluster.Namespace),
			client.MatchingLabels{clusterv1.ClusterNameLabel: supervisorMachineCtx.Cluster.Name}); err != nil {
			return false, err
		}
		otherMDNames := []string{}
		for _, machineDeployment := range machineDeployments.Items {
			if machineDeployment.Spec.Template.Spec.FailureDomain == "" && machineDeployment.Name != mdName {
				otherMDNames = append(otherMDNames, machineDeployment.Name)
			}
		}
		sort.Strings(otherMDNames)

		defaultHostAntiAffinity := []vmoprvhub.VMAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						clusterv1.MachineDeploymentNameLabel: mdName,
					},
				},
				TopologyKey: corev1.LabelHostname,
			},
		}

		affInfo.affinitySpec = vmoprvhub.AffinitySpec{
			VMAffinity: &vmoprvhub.VMAffinitySpec{
				// All the machines belonging to the same MachineDeployment should be placed in the same failure domain - required.
				RequiredDuringSchedulingPreferredDuringExecution: []vmoprvhub.VMAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								clusterv1.MachineDeploymentNameLabel: mdName,
							},
						},
						TopologyKey: corev1.LabelTopologyZone,
					},
				},
			},
			VMAntiAffinity: &vmoprvhub.VMAntiAffinitySpec{},
		}

		var userConfiguredAntiAffinity bool
		if feature.Gates.Enabled(feature.WorkerStrictAntiAffinity) &&
			supervisorMachineCtx.VSphereMachine.Spec.Affinity != nil &&
			supervisorMachineCtx.VSphereMachine.Spec.Affinity.VMAntiAffinity != nil {
			userConfiguredAntiAffinity = true
			antiAff := supervisorMachineCtx.VSphereMachine.Spec.Affinity.VMAntiAffinity

			if len(antiAff.RequiredDuringSchedulingRequiredDuringExecution) > 0 {
				terms := make([]vmoprvhub.VMAffinityTerm, 0, len(antiAff.RequiredDuringSchedulingRequiredDuringExecution))
				for _, term := range antiAff.RequiredDuringSchedulingRequiredDuringExecution {
					terms = append(terms, vmoprvhub.VMAffinityTerm{
						LabelSelector: term.LabelSelector.DeepCopy(),
						TopologyKey:   term.TopologyKey,
					})
				}
				affInfo.affinitySpec.VMAntiAffinity.RequiredDuringSchedulingRequiredDuringExecution = terms
			}

			if len(antiAff.RequiredDuringSchedulingPreferredDuringExecution) > 0 {
				terms := make([]vmoprvhub.VMAffinityTerm, 0, len(antiAff.RequiredDuringSchedulingPreferredDuringExecution))
				for _, term := range antiAff.RequiredDuringSchedulingPreferredDuringExecution {
					terms = append(terms, vmoprvhub.VMAffinityTerm{
						LabelSelector: term.LabelSelector.DeepCopy(),
						TopologyKey:   term.TopologyKey,
					})
				}
				affInfo.affinitySpec.VMAntiAffinity.RequiredDuringSchedulingPreferredDuringExecution = terms
			}

			if len(antiAff.PreferredDuringSchedulingPreferredDuringExecution) > 0 {
				prefTerms := make([]vmoprvhub.VMAffinityTerm, 0, len(antiAff.PreferredDuringSchedulingPreferredDuringExecution))
				for _, term := range antiAff.PreferredDuringSchedulingPreferredDuringExecution {
					prefTerms = append(prefTerms, vmoprvhub.VMAffinityTerm{
						LabelSelector: term.LabelSelector.DeepCopy(),
						TopologyKey:   term.TopologyKey,
					})
				}
				affInfo.affinitySpec.VMAntiAffinity.PreferredDuringSchedulingPreferredDuringExecution = prefTerms
			}
		}

		if !userConfiguredAntiAffinity {
			// All the machines belonging to the same MachineDeployment should be spread across esxi hosts in the same failure domain - best-efforts.
			affInfo.affinitySpec.VMAntiAffinity.PreferredDuringSchedulingPreferredDuringExecution = defaultHostAntiAffinity
		}
		if len(otherMDNames) > 0 {
			// Different MachineDeployments and corresponding VMs should be spread across failure domains - best-efforts.
			affInfo.affinitySpec.VMAntiAffinity.PreferredDuringSchedulingPreferredDuringExecution = append(
				affInfo.affinitySpec.VMAntiAffinity.PreferredDuringSchedulingPreferredDuringExecution,
				vmoprvhub.VMAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      clusterv1.MachineDeploymentNameLabel,
								Operator: metav1.LabelSelectorOpIn,
								Values:   otherMDNames,
							},
						},
					},
					TopologyKey: corev1.LabelTopologyZone,
				},
			)
		}
	}

	// Check for the presence of an existing object
	if err := v.Client.Get(ctx, *vmKey, vmOperatorVM); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		// Define the VM Operator VirtualMachine resource to reconcile.
		vmOperatorVM = &vmoprvhub.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vmKey.Name,
				Namespace: vmKey.Namespace,
			},
		}
	}

	// Reconcile the VM Operator VirtualMachine.
	if err := v.reconcileVMOperatorVM(ctx, supervisorMachineCtx, vmOperatorVM, affInfo); err != nil {
		deprecatedv1beta1conditions.MarkFalse(supervisorMachineCtx.VSphereMachine, infrav1.VMProvisionedV1Beta1Condition, vmwarev1.VMCreationFailedV1Beta1Reason, clusterv1.ConditionSeverityWarning,
			"failed to create or update VirtualMachine: %v", err)
		conditions.Set(supervisorMachineCtx.VSphereMachine, metav1.Condition{
			Type:    infrav1.VSphereMachineVirtualMachineProvisionedCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.VSphereMachineVirtualMachineNotProvisionedReason,
			Message: fmt.Sprintf("failed to create or update VirtualMachine: %v", err),
		})
		// TODO: what to do if AlreadyExists error
		return false, err
	}

	// Update the VM's state to Pending
	supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhasePending

	// Requeue until the VM Operator VirtualMachine has:
	// * Been created
	// * Been powered on
	// * An IP address
	// * A BIOS UUID

	if !meta.IsStatusConditionTrue(vmOperatorVM.Status.Conditions, vmoprvhub.VirtualMachineConditionCreated) {
		// VM operator has conditions which indicate pre-requirements for creation are done.
		// If one of them is set to false then it hit an error case and the information must bubble up
		// to the VSphereMachineVirtualMachineProvisionedCondition in CAPV.
		// NOTE: Following conditions do not get surfaced in any capacity unless they are relevant; if they show up at all,
		// they become pre-reqs and must be true to proceed with VirtualMachine creation.
		for _, condition := range []string{
			vmoprvhub.VirtualMachineConditionClassReady,
			vmoprvhub.VirtualMachineConditionImageReady,
			vmoprvhub.VirtualMachineConditionVMSetResourcePolicyReady,
			vmoprvhub.VirtualMachineConditionBootstrapReady,
			vmoprvhub.VirtualMachineConditionStorageReady,
			vmoprvhub.VirtualMachineConditionNetworkReady,
			vmoprvhub.VirtualMachineConditionPlacementReady,
		} {
			c := meta.FindStatusCondition(vmOperatorVM.Status.Conditions, condition)
			// If the condition is not set to false then VM is still getting provisioned and the condition gets added at a later stage.
			if c == nil || c.Status != metav1.ConditionFalse {
				continue
			}
			deprecatedv1beta1conditions.MarkFalse(supervisorMachineCtx.VSphereMachine, infrav1.VMProvisionedV1Beta1Condition, c.Reason, clusterv1.ConditionSeverityError, "%s", c.Message)
			conditions.Set(supervisorMachineCtx.VSphereMachine, metav1.Condition{
				Type:    infrav1.VSphereMachineVirtualMachineProvisionedCondition,
				Status:  metav1.ConditionFalse,
				Reason:  c.Reason,
				Message: c.Message,
			})
			return false, pkgerrors.Errorf("vm prerequisites check failed for condition %s: %s", condition, klog.KObj(vmOperatorVM))
		}

		// All the pre-requisites are in place but the machines is not yet created, report it.
		deprecatedv1beta1conditions.MarkFalse(supervisorMachineCtx.VSphereMachine, infrav1.VMProvisionedV1Beta1Condition, vmwarev1.VMProvisionStartedV1Beta1Reason, clusterv1.ConditionSeverityInfo, "")
		conditions.Set(supervisorMachineCtx.VSphereMachine, metav1.Condition{
			Type:   infrav1.VSphereMachineVirtualMachineProvisionedCondition,
			Status: metav1.ConditionFalse,
			Reason: infrav1.VSphereMachineVirtualMachineProvisioningReason,
		})
		log.Info(fmt.Sprintf("VM is not yet created: %s", supervisorMachineCtx))
		return true, nil
	}
	// Mark the VM as created
	supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhaseCreated

	if vmOperatorVM.Status.PowerState != vmoprvhub.VirtualMachinePowerStateOn {
		deprecatedv1beta1conditions.MarkFalse(supervisorMachineCtx.VSphereMachine, infrav1.VMProvisionedV1Beta1Condition, vmwarev1.PoweringOnV1Beta1Reason, clusterv1.ConditionSeverityInfo, "")
		conditions.Set(supervisorMachineCtx.VSphereMachine, metav1.Condition{
			Type:   infrav1.VSphereMachineVirtualMachineProvisionedCondition,
			Status: metav1.ConditionFalse,
			Reason: infrav1.VSphereMachineVirtualMachinePoweringOnReason,
		})
		log.Info(fmt.Sprintf("VM is not yet powered on: %s", supervisorMachineCtx))
		return true, nil
	}
	// Mark the VM as poweredOn
	supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhasePoweredOn

	if vmOperatorVM.Status.Network == nil || (vmOperatorVM.Status.Network.PrimaryIP4 == "" && vmOperatorVM.Status.Network.PrimaryIP6 == "") {
		deprecatedv1beta1conditions.MarkFalse(supervisorMachineCtx.VSphereMachine, infrav1.VMProvisionedV1Beta1Condition, vmwarev1.WaitingForNetworkAddressV1Beta1Reason, clusterv1.ConditionSeverityInfo, "")
		conditions.Set(supervisorMachineCtx.VSphereMachine, metav1.Condition{
			Type:   infrav1.VSphereMachineVirtualMachineProvisionedCondition,
			Status: metav1.ConditionFalse,
			Reason: infrav1.VSphereMachineVirtualMachineWaitingForNetworkAddressReason,
		})
		log.Info("VM does not have an IP address")
		return true, nil
	}

	if vmOperatorVM.Status.BiosUUID == "" {
		deprecatedv1beta1conditions.MarkFalse(supervisorMachineCtx.VSphereMachine, infrav1.VMProvisionedV1Beta1Condition, vmwarev1.WaitingForBIOSUUIDV1Beta1Reason, clusterv1.ConditionSeverityInfo, "")
		conditions.Set(supervisorMachineCtx.VSphereMachine, metav1.Condition{
			Type:   infrav1.VSphereMachineVirtualMachineProvisionedCondition,
			Status: metav1.ConditionFalse,
			Reason: infrav1.VSphereMachineVirtualMachineWaitingForBIOSUUIDReason,
		})
		log.Info("VM does not have a BIOS UUID")
		return true, nil
	}

	// Mark the VM as ready
	supervisorMachineCtx.VSphereMachine.Status.Phase = vmwarev1.VSphereMachinePhaseReady

	if ok := v.reconcileNetwork(supervisorMachineCtx, vmOperatorVM); !ok {
		log.Info("IP not yet assigned")
		return true, nil
	}

	// Surface BiosUUID and ProviderID.
	providerID := fmt.Sprintf("vsphere://%s", vmOperatorVM.Status.BiosUUID)
	if supervisorMachineCtx.VSphereMachine.Spec.ProviderID != providerID {
		supervisorMachineCtx.VSphereMachine.Spec.ProviderID = providerID
	}

	if supervisorMachineCtx.VSphereMachine.Status.BiosUUID != vmOperatorVM.Status.BiosUUID {
		supervisorMachineCtx.VSphereMachine.Status.BiosUUID = vmOperatorVM.Status.BiosUUID
	}

	// Surface placement.
	supervisorMachineCtx.VSphereMachine.Status.FailureDomain = vmOperatorVM.Status.Zone

	// Mark the VSphereMachine as Ready
	if !ptr.Deref(supervisorMachineCtx.VSphereMachine.Status.Initialization.Provisioned, false) {
		log.Info("VSphereMachine provisioning complete")
	}
	supervisorMachineCtx.VSphereMachine.Status.Initialization.Provisioned = ptr.To(true)
	deprecatedv1beta1conditions.MarkTrue(supervisorMachineCtx.VSphereMachine, infrav1.VMProvisionedV1Beta1Condition)
	conditions.Set(supervisorMachineCtx.VSphereMachine, metav1.Condition{
		Type:   infrav1.VSphereMachineVirtualMachineProvisionedCondition,
		Status: metav1.ConditionTrue,
		Reason: infrav1.VSphereMachineVirtualMachineProvisionedReason,
	})
	return false, nil
}

// virtualMachineObjectKey returns the object key of the VirtualMachine.
// Part of this is generating the name of the VirtualMachine based on the naming strategy.
func virtualMachineObjectKey(machineName, machineNamespace string, namingStrategy vmwarev1.VirtualMachineNamingSpec) (*client.ObjectKey, error) {
	name, err := GenerateVirtualMachineName(machineName, namingStrategy)
	if err != nil {
		return nil, err
	}

	return &client.ObjectKey{
		Namespace: machineNamespace,
		Name:      name,
	}, nil
}

// GenerateVirtualMachineName generates the name of a VirtualMachine based on the naming strategy.
func GenerateVirtualMachineName(machineName string, namingStrategy vmwarev1.VirtualMachineNamingSpec) (string, error) {
	// Per default the name of the VirtualMachine should be equal to the Machine name (this is the same as "{{ .machine.name }}")
	if namingStrategy.Template == "" {
		// Note: No need to trim to max length in this case as valid Machine names will also be valid VirtualMachine names.
		return machineName, nil
	}

	name, err := infrautilv1.GenerateMachineNameFromTemplate(machineName, namingStrategy.Template)
	if err != nil {
		return "", pkgerrors.Wrap(err, "failed to generate name for VirtualMachine")
	}

	return name, nil
}

// GetHostInfo returns the hostname or IP address of the infrastructure host for the VM Operator VM.
func (v *VmopMachineService) GetHostInfo(ctx context.Context, machineCtx capvcontext.MachineContext) (string, error) {
	supervisorMachineCtx, ok := machineCtx.(*vmware.SupervisorMachineContext)
	if !ok {
		return "", pkgerrors.New("received unexpected SupervisorMachineContext type")
	}

	vmOperatorVM := &vmoprvhub.VirtualMachine{}
	key, err := virtualMachineObjectKey(supervisorMachineCtx.Machine.Name, supervisorMachineCtx.Machine.Namespace, supervisorMachineCtx.VSphereMachine.Spec.Naming)
	if err != nil {
		return "", err
	}
	if err := v.Client.Get(ctx, *key, vmOperatorVM); err != nil {
		return "", err
	}

	// Note: this was status.Host in v1alpha2 API version.
	return vmOperatorVM.Status.NodeName, nil
}

func (v *VmopMachineService) reconcileVMOperatorVM(ctx context.Context, supervisorMachineCtx *vmware.SupervisorMachineContext, vmOperatorVM *vmoprvhub.VirtualMachine, affinityInfo *affinityInfo) error {
	// All Machine resources should define the version of Kubernetes to use.
	if supervisorMachineCtx.Machine.Spec.Version == "" {
		return pkgerrors.Errorf(
			"missing kubernetes version for %s %s/%s",
			supervisorMachineCtx.Machine.GroupVersionKind(),
			supervisorMachineCtx.Machine.Namespace,
			supervisorMachineCtx.Machine.Name)
	}

	var dataSecretName string
	if dsn := supervisorMachineCtx.Machine.Spec.Bootstrap.DataSecretName; dsn != nil {
		dataSecretName = *dsn
	}

	var minHardwareVersion int32
	if version := supervisorMachineCtx.VSphereMachine.Spec.MinHardwareVersion; version != "" {
		hwVersion, err := infrautilv1.ParseHardwareVersion(version)
		if err != nil {
			return err
		}
		minHardwareVersion = int32(hwVersion)
	}

	vmExists := true
	if err := v.Client.Get(ctx, client.ObjectKeyFromObject(vmOperatorVM), vmOperatorVM); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		vmExists = false
	}
	originalVM := vmOperatorVM.DeepCopy()

	// Define a new VM Operator virtual machine.
	// NOTE: Set field-by-field in order to preserve changes made directly
	//  to the VirtualMachine spec by other sources (e.g. the cloud provider)
	if vmOperatorVM.Spec.ImageName == "" {
		vmOperatorVM.Spec.ImageName = supervisorMachineCtx.VSphereMachine.Spec.ImageName
	}
	if vmOperatorVM.Spec.ClassName == "" {
		vmOperatorVM.Spec.ClassName = supervisorMachineCtx.VSphereMachine.Spec.ClassName
	}
	if vmOperatorVM.Spec.StorageClass == "" {
		vmOperatorVM.Spec.StorageClass = supervisorMachineCtx.VSphereMachine.Spec.StorageClass
	}
	vmOperatorVM.Spec.PowerState = vmoprvhub.VirtualMachinePowerStateOn
	if vmOperatorVM.Spec.Reserved == nil {
		vmOperatorVM.Spec.Reserved = &vmoprvhub.VirtualMachineReservedSpec{}
	}
	if vmOperatorVM.Spec.Reserved.ResourcePolicyName == "" {
		vmOperatorVM.Spec.Reserved.ResourcePolicyName = supervisorMachineCtx.Cluster.Name
	}
	if vmOperatorVM.Spec.Bootstrap == nil {
		vmOperatorVM.Spec.Bootstrap = &vmoprvhub.VirtualMachineBootstrapSpec{}
	}
	vmOperatorVM.Spec.Bootstrap.CloudInit = &vmoprvhub.VirtualMachineBootstrapCloudInitSpec{
		RawCloudConfig: &vmoprvhub.SecretKeySelector{
			Name: dataSecretName,
			Key:  "user-data",
		},
	}

	var powerOffMode vmoprvhub.VirtualMachinePowerOpMode
	switch supervisorMachineCtx.VSphereMachine.Spec.PowerOffMode {
	case vmwarev1.VirtualMachinePowerOpModeHard, "": // hard is default
		powerOffMode = vmoprvhub.VirtualMachinePowerOpModeHard
	case vmwarev1.VirtualMachinePowerOpModeSoft:
		powerOffMode = vmoprvhub.VirtualMachinePowerOpModeSoft
	case vmwarev1.VirtualMachinePowerOpModeTrySoft:
		powerOffMode = vmoprvhub.VirtualMachinePowerOpModeTrySoft
	default:
		return fmt.Errorf("unable to map PowerOffMode %q to vm-operator equivalent", supervisorMachineCtx.VSphereMachine.Spec.PowerOffMode)
	}
	vmOperatorVM.Spec.PowerOffMode = powerOffMode

	if vmOperatorVM.Spec.MinHardwareVersion == 0 {
		vmOperatorVM.Spec.MinHardwareVersion = minHardwareVersion
	}

	// VMOperator supports readiness probe and will add/remove endpoints to a
	// VirtualMachineService based on the outcome of the readiness check.
	// When creating the initial control plane node, we do not declare a probe
	// in order to reduce the likelihood of a race between the VirtualMachineService
	// endpoint additions and the kubeadm commands run on the VM itself.
	// Once the initial control plane node is ready, we can re-add the probe so
	// that subsequent machines do not attempt to speak to a kube-apiserver
	// that is not yet ready.
	// Not all network providers (for example, NSX-VPC) provide support for VM
	// readiness probes. The flag PerformsVMReadinessProbe is used to determine
	// whether a VM readiness probe should be conducted.
	if supervisorMachineCtx.ConfigureControlPlaneVMReadinessProbe && infrautilv1.IsControlPlaneMachine(supervisorMachineCtx.Machine) && ptr.Deref(supervisorMachineCtx.Cluster.Status.Initialization.ControlPlaneInitialized, false) {
		vmOperatorVM.Spec.ReadinessProbe = &vmoprvhub.VirtualMachineReadinessProbeSpec{
			TCPSocket: &vmoprvhub.TCPSocketAction{
				Port: intstr.FromInt(defaultAPIBindPort),
			},
		}
	}

	// Assign the VM's labels.
	vmOperatorVM.Labels = getVMLabels(supervisorMachineCtx, vmOperatorVM.Labels, affinityInfo)

	if err := v.reconcileResourcePolicyAnnotations(ctx, supervisorMachineCtx, vmOperatorVM, vmExists); err != nil {
		return err
	}

	if err := v.addVolumes(ctx, supervisorMachineCtx, vmOperatorVM); err != nil {
		return err
	}

	// Only set spec.policies on create as the field is immutable.
	if vmOperatorVM.CreationTimestamp.IsZero() {
		vmOperatorVM.Spec.Policies = getPolicies(supervisorMachineCtx)
	}

	// Apply hooks to modify the VM spec
	// The hooks are loosely typed to allow for different VirtualMachine backends
	for _, vmModifier := range supervisorMachineCtx.VMModifiers {
		modified, err := vmModifier(vmOperatorVM)
		if err != nil {
			return err
		}
		typedModified, ok := modified.(*vmoprvhub.VirtualMachine)
		if !ok {
			return fmt.Errorf("VM modifier returned result of the wrong type: %T", typedModified)
		}
		vmOperatorVM = typedModified
	}

	// Set VM Affinity rules and GroupName.
	// The Affinity rules set in Spec.Affinity primarily take effect only during the
	// initial placement.
	// These rules DO NOT impact new VMs created after initial placement, such as scaling up,
	// because placement relies on information derived from
	// VirtualMachineGroup annotations. This ensures all the VMs
	// for a MachineDeployment are placed in the same failureDomain.
	// Note: no matter of the different placement behaviour, we are setting affinity rules on all machines for consistency.
	if affinityInfo != nil {
		// Only set spec.affinity on create as the field is immutable.
		if !vmExists && vmOperatorVM.CreationTimestamp.IsZero() {
			vmOperatorVM.Spec.Affinity = &affinityInfo.affinitySpec
		}
		if vmOperatorVM.Spec.GroupName == "" {
			vmOperatorVM.Spec.GroupName = affinityInfo.vmGroupName
		}
	}

	// Make sure the VSphereMachine owns the VM Operator VirtualMachine.
	if err := ctrlutil.SetControllerReference(supervisorMachineCtx.VSphereMachine, vmOperatorVM, v.Client.Scheme()); err != nil {
		return pkgerrors.Wrapf(err, "failed to mark %s %s/%s as owner of %s %s/%s",
			supervisorMachineCtx.VSphereMachine.GroupVersionKind(),
			supervisorMachineCtx.VSphereMachine.Namespace,
			supervisorMachineCtx.VSphereMachine.Name,
			vmOperatorVM.GroupVersionKind(),
			vmOperatorVM.Namespace,
			vmOperatorVM.Name)
	}

	if !vmExists {
		if err := v.Client.Create(ctx, vmOperatorVM); err != nil {
			return err
		}
	} else if !reflect.DeepEqual(originalVM, vmOperatorVM) {
		patch, err := conversionclient.MergeFrom(ctx, v.Client, originalVM)
		if err != nil {
			return pkgerrors.Wrapf(err, "failed to create patch for VirtualMachine object")
		}
		if err := v.Client.Patch(ctx, vmOperatorVM, patch); err != nil {
			return pkgerrors.Wrapf(err, "failed to patch VirtualMachine object")
		}
	}

	return nil
}

func convertKeyValueSlice(pairs []vmoprvhub.KeyValuePair) []vmwarev1.KeyValuePair {
	converted := make([]vmwarev1.KeyValuePair, 0, len(pairs))
	for _, pair := range pairs {
		converted = append(converted, vmwarev1.KeyValuePair{
			Key:   pair.Key,
			Value: pair.Value,
		})
	}
	return converted
}

func (v *VmopMachineService) reconcileNetwork(supervisorMachineCtx *vmware.SupervisorMachineContext, vm *vmoprvhub.VirtualMachine) bool {
	// Propagate VM status.network.interfaces to VSphereMachine.Status.NetworkInterfaces
	if vm.Status.Network != nil {
		var interfaces []vmwarev1.VSphereMachineNetworkInterfaceStatus //nolint:prealloc // Intentionally not pre-allocating here as this lead to sending empty patches to the kube-apiserver in the past.
		for _, vmIface := range vm.Status.Network.Interfaces {
			iface := vmwarev1.VSphereMachineNetworkInterfaceStatus{
				Name:      vmIface.Name,
				DeviceKey: vmIface.DeviceKey,
			}
			// set IP
			if vmIface.IP != nil {
				var dhcp vmwarev1.VSphereMachineNetworkDHCPStatus
				if vmIface.IP.DHCP != nil {
					dhcp = vmwarev1.VSphereMachineNetworkDHCPStatus{
						IP4: vmwarev1.VSphereMachineNetworkDHCPOptionsStatus{
							Enabled: ptr.To(vmIface.IP.DHCP.IP4.Enabled),
							Config:  convertKeyValueSlice(vmIface.IP.DHCP.IP4.Config),
						},
						IP6: vmwarev1.VSphereMachineNetworkDHCPOptionsStatus{
							Enabled: ptr.To(vmIface.IP.DHCP.IP6.Enabled),
							Config:  convertKeyValueSlice(vmIface.IP.DHCP.IP6.Config),
						},
					}
				}
				var addresses []vmwarev1.VSphereMachineNetworkInterfaceIPAddrStatus
				for _, addr := range vmIface.IP.Addresses {
					addresses = append(addresses, vmwarev1.VSphereMachineNetworkInterfaceIPAddrStatus{
						Address:  addr.Address,
						Lifetime: addr.Lifetime,
						Origin:   addr.Origin,
						State:    addr.State,
					})
				}
				iface.IP = vmwarev1.VSphereMachineNetworkInterfaceIPStatus{
					AutoConfigurationEnabled: vmIface.IP.AutoConfigurationEnabled,
					MACAddr:                  vmIface.IP.MACAddr,
					DHCP:                     dhcp,
					Addresses:                addresses,
				}
			}
			// set DNS
			if vmIface.DNS != nil {
				iface.DNS = vmwarev1.VSphereMachineNetworkDNSStatus{
					DHCP:          ptr.To(vmIface.DNS.DHCP),
					DomainName:    vmIface.DNS.DomainName,
					HostName:      vmIface.DNS.HostName,
					Nameservers:   vmIface.DNS.Nameservers,
					SearchDomains: vmIface.DNS.SearchDomains,
				}
			}
			interfaces = append(interfaces, iface)
		}
		supervisorMachineCtx.VSphereMachine.Status.Network = vmwarev1.VSphereMachineNetworkStatus{
			Interfaces: interfaces,
		}
	}

	if vm.Status.Network.PrimaryIP4 == "" && vm.Status.Network.PrimaryIP6 == "" {
		return false
	}

	ipAddr := vm.Status.Network.PrimaryIP4
	if ipAddr == "" {
		ipAddr = vm.Status.Network.PrimaryIP6
	}

	// Cluster API requires InfrastructureMachineStatus.Addresses to be set
	if ipAddr != "" {
		supervisorMachineCtx.VSphereMachine.Status.Addresses = []clusterv1.MachineAddress{
			{
				Type:    clusterv1.MachineInternalIP,
				Address: ipAddr,
			},
		}
	}

	return true
}

// getVirtualMachinesInCluster returns all VMOperator VirtualMachine objects in the current cluster.
// First filter by ClusterSelectorKey. If the result is empty, they fall back to legacyClusterSelectorKey.
func (v *VmopMachineService) getVirtualMachinesInCluster(ctx context.Context, supervisorMachineCtx *vmware.SupervisorMachineContext) ([]*vmoprvhub.VirtualMachine, error) {
	if supervisorMachineCtx.Cluster == nil {
		return []*vmoprvhub.VirtualMachine{}, pkgerrors.Errorf("No cluster is set for machine %s in namespace %s", supervisorMachineCtx.GetVSphereMachine().GetName(), supervisorMachineCtx.GetVSphereMachine().GetNamespace())
	}
	labels := map[string]string{ClusterSelectorKey: supervisorMachineCtx.Cluster.Name}
	vmList := &vmoprvhub.VirtualMachineList{}

	if err := v.Client.List(
		ctx, vmList,
		client.InNamespace(supervisorMachineCtx.Cluster.Namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, pkgerrors.Wrapf(
			err, "error getting virtualmachines in cluster %s/%s",
			supervisorMachineCtx.Cluster.Namespace, supervisorMachineCtx.Cluster.Name)
	}

	// If the list is empty, fall back to use legacy labels for filtering
	if len(vmList.Items) == 0 {
		legacyLabels := map[string]string{legacyClusterSelectorKey: supervisorMachineCtx.Cluster.Name}
		if err := v.Client.List(
			ctx, vmList,
			client.InNamespace(supervisorMachineCtx.Cluster.Namespace),
			client.MatchingLabels(legacyLabels)); err != nil {
			return nil, pkgerrors.Wrapf(
				err, "error getting virtualmachines in cluster %s/%s using legacy labels",
				supervisorMachineCtx.Cluster.Namespace, supervisorMachineCtx.Cluster.Name)
		}
	}

	vms := make([]*vmoprvhub.VirtualMachine, len(vmList.Items))
	for i := range vmList.Items {
		vms[i] = &vmList.Items[i]
	}

	return vms, nil
}

// reconcileResourcePolicyAnnotations reconciles anti-affinity and Cluster Module Group (CMG) annotations.
//
// Control Plane: Currently uses legacy CP anti-affinity tag and "control-plane" CMG.
//
// Workers:
// - If WorkerStrictAntiAffinity is disabled: Always sets legacy worker tag and CMG ("<cluster>-workers-0").
// - If WorkerStrictAntiAffinity is enabled (which implies NodeAutoPlacement is enabled):
//   - Existing VM: Preserves annotations as-is; spec.affinity is also preserved during reconciliation.
//   - Greenfield (VMSetRP has no worker CMG): Skips legacy annotations; uses spec.affinity only
//     (the new policy-based AF/AAF).
//   - Brownfield (VMSetRP has worker CMG): Inspects non-terminating peer VMs in the same
//     cluster and MachineDeployment. If any peer lacks spec.affinity, sets both legacy worker
//     annotations; otherwise omits legacy annotations, including when there are no eligible peers.
//     The pool typically transitions over two rollouts: the first replaces VMs lacking
//     spec.affinity with VMs using both spec.affinity and legacy annotations while legacy
//     peers remain. Once all eligible peers have spec.affinity, subsequent replacements
//     omit legacy annotations, completing the transition during the second rollout.
func (v *VmopMachineService) reconcileResourcePolicyAnnotations(ctx context.Context, supervisorMachineCtx *vmware.SupervisorMachineContext, vm *vmoprvhub.VirtualMachine, vmExists bool) error {
	annotations := vm.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if infrautilv1.IsControlPlaneMachine(supervisorMachineCtx.Machine) {
		annotations[ProviderTagsAnnotationKey] = ControlPlaneVMVMAntiAffinityTagValue
		annotations[ClusterModuleNameAnnotationKey] = ControlPlaneVMClusterModuleGroupName
		vm.ObjectMeta.SetAnnotations(annotations)
		return nil
	}

	if !feature.Gates.Enabled(feature.WorkerStrictAntiAffinity) {
		annotations[ProviderTagsAnnotationKey] = WorkerVMVMAntiAffinityTagValue
		annotations[ClusterModuleNameAnnotationKey] = getMachineDeploymentNameForCluster(supervisorMachineCtx.Cluster)
		vm.ObjectMeta.SetAnnotations(annotations)
		return nil
	}

	// On VM Update: Retain the current annotation and spec.affinity if they exist.
	// CAPV never removes them because we cannot programmatically verify if the annotation's
	// removal or the spec.affinity (ComputePolicy) tags have been successfully applied and enforced in VC.
	if vmExists {
		return nil
	}

	// On VM Creation:
	// Step 1 (Check VMSetRP): Inspect the associated VMSetRP's spec.clusterModuleGroups.
	vmSetRP := &vmoprvhub.VirtualMachineSetResourcePolicy{}
	key := client.ObjectKey{
		Namespace: supervisorMachineCtx.Cluster.Namespace,
		Name:      supervisorMachineCtx.Cluster.Name,
	}
	if err := v.Client.Get(ctx, key, vmSetRP); err != nil {
		return pkgerrors.Wrapf(err, "error getting VirtualMachineSetResourcePolicy %s/%s", key.Namespace, key.Name)
	}

	hasWorkerCMG := false
	legacyWorkerCMG := getMachineDeploymentNameForCluster(supervisorMachineCtx.Cluster)
	for _, cmg := range vmSetRP.Spec.ClusterModuleGroups {
		if cmg == legacyWorkerCMG {
			hasWorkerCMG = true
			break
		}
	}

	// If it does not contain the legacy worker CMG (indicative of a greenfield cluster),
	// CAPV skips legacy annotations completely and applies spec.affinity only.
	if !hasWorkerCMG {
		return nil
	}

	// Step 2 (Stateless VM Affinity Check): If the worker CMG is present (indicative of a brownfield cluster):
	mdName := supervisorMachineCtx.Machine.Labels[clusterv1.MachineDeploymentNameLabel]
	if mdName == "" {
		annotations[ProviderTagsAnnotationKey] = WorkerVMVMAntiAffinityTagValue
		annotations[ClusterModuleNameAnnotationKey] = legacyWorkerCMG
		vm.ObjectMeta.SetAnnotations(annotations)
		return nil
	}

	vmList := &vmoprvhub.VirtualMachineList{}
	if err := v.Client.List(ctx, vmList,
		client.InNamespace(supervisorMachineCtx.Cluster.Namespace),
		client.MatchingLabels{clusterv1.MachineDeploymentNameLabel: mdName},
	); err != nil {
		return pkgerrors.Wrapf(err, "error listing virtual machines for MachineDeployment %s", mdName)
	}

	// Check if all observed non-terminating peer VMs in this MachineDeployment pool have spec.affinity configured:
	// - Skip terminating VMs during rolling upgrades, remediation, or scale-down. A deletion
	//   timestamp excludes a peer from this decision; it does not confirm removal from vCenter.
	// - Skip the current VM itself (item.Name == vm.Name).
	// - Verify cluster ownership: although ClusterClass-managed topologies generate unique MD names with random
	//   hashes, non-ClusterClass or custom-templated clusters may share identical MD names (e.g. "workers-0")
	//   within the same supervisor namespace. Checking the cluster labels prevents cross-cluster
	//   interference in multi-tenant namespaces.
	// - If any eligible peer VM lacks spec.affinity, the pool still contains legacy VMs, and new VMs
	//   must retain the legacy ClusterModuleGroup annotation to maintain anti-affinity with existing VMs.
	allHaveAffinity := true
	clusterName := supervisorMachineCtx.Cluster.Name
	for _, item := range vmList.Items {
		if !item.DeletionTimestamp.IsZero() {
			continue
		}
		if item.Name == vm.Name {
			continue
		}
		if item.Labels[clusterv1.ClusterNameLabel] != clusterName && item.Labels[ClusterSelectorKey] != clusterName {
			continue
		}
		if item.Spec.Affinity == nil {
			allHaveAffinity = false
			break
		}
	}

	// If all eligible peers have spec.affinity configured, or there are no eligible peers:
	// Start creating new VMs with spec.affinity only (without the legacy vsphere-cluster-module-group annotation).
	// Else: Keep creating VMs with both spec.affinity and the legacy vsphere-cluster-module-group annotation.
	if !allHaveAffinity {
		annotations[ProviderTagsAnnotationKey] = WorkerVMVMAntiAffinityTagValue
		annotations[ClusterModuleNameAnnotationKey] = legacyWorkerCMG
		vm.ObjectMeta.SetAnnotations(annotations)
	}

	return nil
}

func volumeName(machine *vmwarev1.VSphereMachine, volume vmwarev1.VSphereMachineVolume) string {
	return machine.Name + "-" + volume.Name
}

// addVolume ensures volume is included in vm.Spec.Volumes.
func addVolume(vm *vmoprvhub.VirtualMachine, name string) {
	for _, volume := range vm.Spec.Volumes {
		claim := volume.PersistentVolumeClaim
		if claim != nil && claim.ClaimName == name {
			return // volume already present in the spec
		}
	}

	vm.Spec.Volumes = append(vm.Spec.Volumes, vmoprvhub.VirtualMachineVolume{
		Name: name,
		VirtualMachineVolumeSource: vmoprvhub.VirtualMachineVolumeSource{
			PersistentVolumeClaim: &vmoprvhub.PersistentVolumeClaimVolumeSource{
				PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: name,
					ReadOnly:  false,
				},
			},
		},
	})
}

func (v *VmopMachineService) addVolumes(ctx context.Context, supervisorMachineCtx *vmware.SupervisorMachineContext, vm *vmoprvhub.VirtualMachine) error {
	nvolumes := len(supervisorMachineCtx.VSphereMachine.Spec.Volumes)
	if nvolumes == 0 {
		return nil
	}

	for _, volume := range supervisorMachineCtx.VSphereMachine.Spec.Volumes {
		storageClassName := volume.StorageClass
		if volume.StorageClass == "" {
			storageClassName = supervisorMachineCtx.VSphereMachine.Spec.StorageClass
		}

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      volumeName(supervisorMachineCtx.VSphereMachine, volume),
				Namespace: supervisorMachineCtx.VSphereMachine.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: volume.Capacity,
				},
				StorageClassName: &storageClassName,
			},
		}

		// The CSI zone annotation must be set when using a zonal storage class,
		// which is required when the cluster has multiple (3) zones.
		// Single zone clusters (legacy/default) do not support zonal storage and must not
		// have the zone annotation set.
		//
		// However, with Node Auto Placement enabled, failureDomain is optional and CAPV no longer
		// sets PVC annotations when creating worker VMs. PVC placement now follows the StorageClass behavior (Immediate or WaitForFirstConsumer).
		// Control Plane VMs will still have failureDomain set, and we will set PVC annotation.
		zonal := len(supervisorMachineCtx.VSphereCluster.Status.FailureDomains) > 1

		if zone := supervisorMachineCtx.Machine.Spec.FailureDomain; zonal && zone != "" {
			topology := []map[string]string{
				{corev1.LabelTopologyZone: zone},
			}
			b, err := json.Marshal(topology)
			if err != nil {
				return pkgerrors.Errorf("failed to marshal zone topology %q: %s", zone, err)
			}
			pvc.Annotations = map[string]string{
				"csi.vsphere.volume-requested-topology": string(b),
			}
		}

		if _, err := ctrlutil.CreateOrPatch(ctx, v.Client, pvc, func() error {
			if err := ctrlutil.SetOwnerReference(
				supervisorMachineCtx.VSphereMachine,
				pvc,
				v.Client.Scheme(),
			); err != nil {
				return pkgerrors.Wrapf(
					err,
					"error setting %s/%s as owner of %s/%s",
					supervisorMachineCtx.VSphereMachine.Namespace,
					supervisorMachineCtx.VSphereMachine.Name,
					pvc.Namespace,
					pvc.Name,
				)
			}
			return nil
		}); err != nil {
			return pkgerrors.Wrapf(
				err,
				"failed to create volume %s/%s",
				pvc.Namespace,
				pvc.Name)
		}

		addVolume(vm, pvc.Name)
	}

	return nil
}

// getVMLabels returns the labels applied to a VirtualMachine.
func getVMLabels(supervisorMachineCtx *vmware.SupervisorMachineContext, vmLabels map[string]string, affinityInfo *affinityInfo) map[string]string {
	if vmLabels == nil {
		vmLabels = map[string]string{}
	}

	// Get the labels for the VM that differ based on the cluster role of
	// the Kubernetes node hosted on this VM.
	clusterRoleLabels := clusterRoleVMLabels(supervisorMachineCtx.GetClusterContext(), infrautilv1.IsControlPlaneMachine(supervisorMachineCtx.Machine))
	for k, v := range clusterRoleLabels {
		vmLabels[k] = v
	}

	// Set the labels that determine the VM's placement.
	// Note: if the failureDomain is not set, auto placement will happen according to affinity rules on VM during initial Cluster creation.
	// For VM created during day-2 operation like scaling up, we should expect the failureDomain to be always set.
	// Note: It is important that the value zone label is set on a vm must never change once it is set,
	// because the zone in the VirtualMachineGroup might change in case this info is derived from spec.template.spec.failureDomain.
	var failureDomain string
	if affinityInfo != nil && affinityInfo.failureDomain != "" {
		failureDomain = affinityInfo.failureDomain
	}
	topologyLabels := getTopologyLabels(supervisorMachineCtx, failureDomain)
	for k, v := range topologyLabels {
		vmLabels[k] = v
	}

	// Ensure the VM has a label that can be used when searching for
	// resources associated with the target cluster.
	vmLabels[clusterv1.ClusterNameLabel] = supervisorMachineCtx.GetClusterContext().Cluster.Name

	// Ensure the VM has the machine deployment name label
	if !infrautilv1.IsControlPlaneMachine(supervisorMachineCtx.Machine) {
		vmLabels[clusterv1.MachineDeploymentNameLabel] = supervisorMachineCtx.Machine.Labels[clusterv1.MachineDeploymentNameLabel]
	}

	return vmLabels
}

// getTopologyLabels returns the labels related to a VM's topology.
//
// TODO(akutz): Currently this function just returns the availability zone,
//
//	and thus the code is optimized as such. However, in the future
//	this function may return a more diverse topology.
func getTopologyLabels(supervisorMachineCtx *vmware.SupervisorMachineContext, failureDomain string) map[string]string {
	// This is for explicit placement.
	if fd := supervisorMachineCtx.Machine.Spec.FailureDomain; fd != "" {
		return map[string]string{
			corev1.LabelTopologyZone: fd,
		}
	}
	// This is for automatic placement.
	if failureDomain != "" {
		return map[string]string{
			corev1.LabelTopologyZone: failureDomain,
		}
	}
	return nil
}

func getPolicies(supervisorMachineCtx *vmware.SupervisorMachineContext) []vmoprvhub.PolicySpec {
	// Assign policies when the feature gate is enabled.
	if !feature.Gates.Enabled(feature.InfrastructurePolicies) {
		return nil
	}

	refs := supervisorMachineCtx.VSphereMachine.Spec.Policies
	if len(refs) == 0 {
		return nil
	}
	result := make([]vmoprvhub.PolicySpec, 0, len(refs))
	for _, ref := range refs {
		result = append(result, vmoprvhub.PolicySpec{
			Name: ref.Name,
			Kind: ref.Kind,
		})
	}
	return result
}

// getMachineDeploymentName returns the MachineDeployment name for a Cluster.
// This is also the name used by VSphereMachineTemplate and KubeadmConfigTemplate.
func getMachineDeploymentNameForCluster(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-workers-0", cluster.Name)
}

// checkVirtualMachineGroupMembership checks if the machine is in the first boot order group
// and performs logic if a match is found, as first boot order contains all the worker VMs.
func (v *VmopMachineService) checkVirtualMachineGroupMembership(vmOperatorVMGroup *vmoprvhub.VirtualMachineGroup, virtualMachineName string) bool {
	if len(vmOperatorVMGroup.Spec.BootOrder) > 0 {
		for _, member := range vmOperatorVMGroup.Spec.BootOrder[0].Members {
			if member.Name == virtualMachineName {
				return true
			}
		}
	}
	return false
}
