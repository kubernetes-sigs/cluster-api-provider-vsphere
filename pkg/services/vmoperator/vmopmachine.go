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
	"bytes"
	goctx "context"
	"encoding/base64"
	"fmt"
	"text/template"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
	vmwareutil "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util/vmware"
)

type VmopMachineService struct {
	deleteFunc func(vm *vmoprv1.VirtualMachine) error
}

func (v *VmopMachineService) FetchVSphereMachine(client client.Client, name types.NamespacedName) (context.MachineContext, error) {
	vsphereMachine := &vmwarev1.VSphereMachine{}
	err := client.Get(goctx.Background(), name, vsphereMachine)
	return &vmware.SupervisorMachineContext{VSphereMachine: vsphereMachine}, err
}

func (v *VmopMachineService) FetchVSphereCluster(c client.Client, cluster *clusterv1.Cluster, machineContext context.MachineContext) (context.MachineContext, error) {
	ctx, ok := machineContext.(*vmware.SupervisorMachineContext)
	if !ok {
		return nil, errors.New("received unexpected SupervisorMachineContext type")
	}

	vsphereCluster := &vmwarev1.VSphereCluster{}
	key := client.ObjectKey{
		Namespace: machineContext.GetObjectMeta().Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	err := c.Get(goctx.Background(), key, vsphereCluster)

	ctx.VSphereCluster = vsphereCluster
	return ctx, err
}

func (v *VmopMachineService) ReconcileDelete(c context.MachineContext) error {
	ctx, ok := c.(*vmware.SupervisorMachineContext)
	if !ok {
		return errors.New("received unexpected SupervisorMachineContext type")
	}
	ctx.Logger.V(2).Info("Destroying VM")

	// If debug logging is enabled, report the number of vms in the cluster before and after the reconcile
	if ctx.Logger.V(5).Enabled() {
		vms, err := getVirtualMachinesInCluster(ctx)
		ctx.Logger.Info("Trace Destroy PRE: VirtualMachines", "vmcount", len(vms), "error", err)
		defer func() {
			vms, err := getVirtualMachinesInCluster(ctx)
			ctx.Logger.Info("Trace Destroy POST: VirtualMachines", "vmcount", len(vms), "error", err)
		}()
	}

	// Allow deleteFunc to be swappable for unit testing
	if v.deleteFunc == nil {
		v.deleteFunc = func(vm *vmoprv1.VirtualMachine) error {
			return ctx.Client.Delete(ctx, vm)
		}
	}

	// First, check to see if it's already deleted
	vmopVM := vmoprv1.VirtualMachine{}
	if err := ctx.Client.Get(ctx, types.NamespacedName{Namespace: ctx.Machine.Namespace, Name: ctx.Machine.Name}, &vmopVM); err != nil {
		if apierrors.IsNotFound(err) {
			ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStateNotFound
			return err
		}
		ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStateError
		return err
	}

	// Next, check to see if it's in the process of being deleted
	if vmopVM.GetDeletionTimestamp() != nil {
		ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStateDeleting
		return nil
	}

	// If none of the above are true, Delete the VM
	if err := v.deleteFunc(&vmopVM); err != nil {
		if apierrors.IsNotFound(err) {
			ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStateNotFound
			return err
		}
		ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStateError
		return err
	}

	ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStateDeleting
	return nil
}

func (v *VmopMachineService) SyncFailureReason(c context.MachineContext) (bool, error) {
	ctx, ok := c.(*vmware.SupervisorMachineContext)
	if !ok {
		return false, errors.New("received unexpected SupervisorMachineContext type")
	}

	return ctx.VSphereMachine.Status.FailureReason != nil || ctx.VSphereMachine.Status.FailureMessage != nil, nil
}

func (v *VmopMachineService) ReconcileNormal(c context.MachineContext) (bool, error) {
	ctx, ok := c.(*vmware.SupervisorMachineContext)
	if !ok {
		return false, errors.New("received unexpected SupervisorMachineContext type")
	}

	ctx.VSphereMachine.Spec.FailureDomain = ctx.Machine.Spec.FailureDomain

	ctx.Logger.V(2).Info("Reconciling VM")

	// If debug logging is enabled, report the number of vms in the cluster before and after the reconcile
	if ctx.Logger.V(5).Enabled() {
		vms, err := getVirtualMachinesInCluster(ctx)
		ctx.Logger.Info("Trace ReconcileVM PRE: VirtualMachines", "vmcount", len(vms), "error", err)
		defer func() {
			vms, err := getVirtualMachinesInCluster(ctx)
			ctx.Logger.Info("Trace ReconcileVM POST: VirtualMachines", "vmcount", len(vms), "error", err)
		}()
	}

	// Set the VM state. Will get reset throughout the reconcile
	ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStatePending

	// Define the VM Operator VirtualMachine resource to reconcile.
	vmOperatorVM := v.newVMOperatorVM(ctx)

	// Reconcile the VM Operator VirtualMachine.
	if err := v.reconcileVMOperatorVM(ctx, vmOperatorVM); err != nil {
		conditions.MarkFalse(ctx.VSphereMachine, infrav1.VMProvisionedCondition, vmwarev1.VMCreationFailedReason, clusterv1.ConditionSeverityWarning,
			fmt.Sprintf("failed to create or update VirtualMachine: %v", err))
		// TODO: what to do if AlreadyExists error
		return false, err
	}

	// Define the bootstrap data ConfigMap resource to reconcile.
	bootstrapDataConfigMap := v.newBootstrapDataConfigMap(ctx)

	// Reconcile the bootstrap data ConfigMap.
	if err := v.reconcileBootstrapDataConfigMap(ctx, bootstrapDataConfigMap); err != nil {
		conditions.MarkFalse(ctx.VSphereMachine, infrav1.VMProvisionedCondition, vmwarev1.VMCreationFailedReason, clusterv1.ConditionSeverityWarning,
			fmt.Sprintf("failed to create or update bootstrap configmap: %v", err))
		return false, err
	}

	// Update the VM's state to Pending
	ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStatePending

	// Since vm operator only has one condition for now, we can't set vspheremachine's condition fully based on virtualmachine's
	// condition. Once vm operator surfaces enough conditions in virtualmachine, we could simply mirror the conditions in vspheremachine.
	// For now, we set conditions based on the whole virtualmachine status.
	// TODO: vm-operator does not use the cluster-api condition type. so can't use cluster-api util functions to fetch the condition
	for _, cond := range vmOperatorVM.Status.Conditions {
		if cond.Type == vmoprv1.VirtualMachinePrereqReadyCondition && cond.Severity == vmoprv1.ConditionSeverityError {
			conditions.MarkFalse(ctx.VSphereMachine, infrav1.VMProvisionedCondition, cond.Reason, clusterv1.ConditionSeverityError, cond.Message)
			return false, errors.Errorf("vm prerequisites check fails: %s", ctx)
		}
	}

	// Requeue until the VM Operator VirtualMachine has:
	// * Been created
	// * Been powered on
	// * An IP address
	// * A BIOS UUID
	if vmOperatorVM.Status.Phase != vmoprv1.Created {
		conditions.MarkFalse(ctx.VSphereMachine, infrav1.VMProvisionedCondition, vmwarev1.VMProvisionStartedReason, clusterv1.ConditionSeverityInfo, "")
		ctx.Logger.Info(fmt.Sprintf("vm is not yet created: %s", ctx))
		return true, nil
	}
	// Mark the VM as created
	ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStateCreated

	if vmOperatorVM.Status.PowerState != vmoprv1.VirtualMachinePoweredOn {
		conditions.MarkFalse(ctx.VSphereMachine, infrav1.VMProvisionedCondition, vmwarev1.PoweringOnReason, clusterv1.ConditionSeverityInfo, "")
		ctx.Logger.Info(fmt.Sprintf("vm is not yet powered on: %s", ctx))
		return true, nil
	}
	// Mark the VM as poweredOn
	ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStatePoweredOn

	if vmOperatorVM.Status.VmIp == "" {
		conditions.MarkFalse(ctx.VSphereMachine, infrav1.VMProvisionedCondition, vmwarev1.WaitingForNetworkAddressReason, clusterv1.ConditionSeverityInfo, "")
		ctx.Logger.Info(fmt.Sprintf("vm does not have an IP address: %s", ctx))
		return true, nil
	}

	if vmOperatorVM.Status.BiosUUID == "" {
		conditions.MarkFalse(ctx.VSphereMachine, infrav1.VMProvisionedCondition, vmwarev1.WaitingForBIOSUUIDReason, clusterv1.ConditionSeverityInfo, "")
		ctx.Logger.Info(fmt.Sprintf("vm does not have a BIOS UUID: %s", ctx))
		return true, nil
	}

	// Mark the VM as ready
	ctx.VSphereMachine.Status.VMStatus = vmwarev1.VirtualMachineStateReady

	if ok := v.reconcileNetwork(ctx, vmOperatorVM); !ok {
		ctx.Logger.Info("ip not yet assigned")
		return true, nil
	}

	v.reconcileProviderID(ctx, vmOperatorVM)

	// Mark the VSphereMachine as Ready
	ctx.VSphereMachine.Status.Ready = true
	conditions.MarkTrue(ctx.VSphereMachine, infrav1.VMProvisionedCondition)
	return false, nil
}

func (v VmopMachineService) newVMOperatorVM(ctx *vmware.SupervisorMachineContext) *vmoprv1.VirtualMachine {
	return &vmoprv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ctx.Machine.Name,
			Namespace: ctx.Machine.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: vmoprv1.SchemeGroupVersion.String(),
			Kind:       "VirtualMachine",
		},
	}
}

func (v VmopMachineService) reconcileVMOperatorVM(ctx *vmware.SupervisorMachineContext, vmOperatorVM *vmoprv1.VirtualMachine) error {
	// All Machine resources should define the version of Kubernetes to use.
	if ctx.Machine.Spec.Version == nil || *ctx.Machine.Spec.Version == "" {
		return errors.Errorf(
			"missing kubernetes version for %s %s/%s",
			ctx.Machine.GroupVersionKind(),
			ctx.Machine.Namespace,
			ctx.Machine.Name)
	}

	_, err := ctrlutil.CreateOrUpdate(ctx, ctx.Client, vmOperatorVM, func() error {
		// Define a new VM Operator virtual machine.
		// NOTE: Set field-by-field in order to preserve changes made directly
		//  to the VirtualMachine spec by other sources (e.g. the cloud provider)
		vmOperatorVM.Spec.ImageName = ctx.VSphereMachine.Spec.ImageName
		vmOperatorVM.Spec.ClassName = ctx.VSphereMachine.Spec.ClassName
		vmOperatorVM.Spec.StorageClass = ctx.VSphereMachine.Spec.StorageClass
		vmOperatorVM.Spec.PowerState = vmoprv1.VirtualMachinePoweredOn
		vmOperatorVM.Spec.ResourcePolicyName = ctx.VSphereCluster.Status.ResourcePolicyName
		vmOperatorVM.Spec.VmMetadata = &vmoprv1.VirtualMachineMetadata{
			ConfigMapName: vmwareutil.GetBootstrapConfigMapName(ctx.VSphereMachine.Name),
			Transport:     "ExtraConfig",
		}

		// VMOperator supports readiness probe and will add/remove endpoints to a
		// VirtualMachineService based on the outcome of the readiness check.
		// When creating the initial control plane node, we do not declare a probe
		// in order to reduce the likelihood of a race between the VirtualMachineService
		// endpoint additions and the kubeadm commands run on the VM itself.
		// Once the initial control plane node is ready, we can re-add the probe so
		// that subsequent machines do not attempt to speak to a kube-apiserver
		// that is not yet ready.
		if infrautilv1.IsControlPlaneMachine(ctx.Machine) && ctx.Cluster.Status.ControlPlaneReady {
			vmOperatorVM.Spec.ReadinessProbe = &vmoprv1.Probe{
				TCPSocket: &vmoprv1.TCPSocketAction{
					Port: intstr.FromInt(defaultAPIBindPort),
				},
			}
		}

		// Assign the VM's labels.
		vmOperatorVM.Labels = getVMLabels(ctx, vmOperatorVM.Labels)

		addResourcePolicyAnnotations(ctx, vmOperatorVM)

		if err := addVolumes(ctx, vmOperatorVM); err != nil {
			return err
		}

		// Apply hooks to modify the VM spec
		// The hooks are loosely typed so as to allow for different VirtualMachine backends
		for _, vmModifier := range ctx.VMModifiers {
			modified, err := vmModifier(vmOperatorVM)
			if err != nil {
				return err
			}
			typedModified, ok := modified.(*vmoprv1.VirtualMachine)
			if !ok {
				return fmt.Errorf("VM modifier returned result of the wrong type: %T", typedModified)
			}
			vmOperatorVM = typedModified
		}

		// Make sure the VSphereMachine owns the VM Operator VirtualMachine.
		if err := ctrlutil.SetControllerReference(ctx.VSphereMachine, vmOperatorVM, ctx.Scheme); err != nil {
			return errors.Wrapf(err, "failed to mark %s %s/%s as owner of %s %s/%s",
				ctx.VSphereMachine.GroupVersionKind(),
				ctx.VSphereMachine.Namespace,
				ctx.VSphereMachine.Name,
				vmOperatorVM.GroupVersionKind(),
				vmOperatorVM.Namespace,
				vmOperatorVM.Name)
		}

		return nil
	})
	return err
}

func (v VmopMachineService) newBootstrapDataConfigMap(ctx *vmware.SupervisorMachineContext) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.VSphereMachine.Namespace,
			Name:      vmwareutil.GetBootstrapConfigMapName(ctx.VSphereMachine.Name),
		},
	}
}

func (v VmopMachineService) reconcileBootstrapDataConfigMap(ctx *vmware.SupervisorMachineContext, configMap *corev1.ConfigMap) error {
	bootstrapData, err := vmwareutil.GetBootstrapData(ctx, ctx.Client, ctx.Machine)
	if err != nil {
		return err
	}

	_, err = ctrlutil.CreateOrUpdate(ctx, ctx.Client, configMap, func() error {
		// Make sure the VSphereMachine owns the bootstrap data ConfigMap.
		if err := ctrlutil.SetControllerReference(ctx.VSphereMachine, configMap, ctx.Scheme); err != nil {
			return errors.Wrapf(err, "failed to mark %s %s/%s as owner of %s %s/%s",
				ctx.VSphereMachine.GroupVersionKind(),
				ctx.VSphereMachine.Namespace,
				ctx.VSphereMachine.Name,
				configMap.GroupVersionKind(),
				configMap.Namespace,
				configMap.Name)
		}

		metadata, err := v.getGuestInfoMetadata(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get guest info metadata for machine %s", ctx.Machine.Name)
		}

		// The CAPI contract states that the string assigned to the field
		// Machine.Spec.Bootstrap.Data will be base64 encoded.
		configMap.Data = map[string]string{
			"guestinfo.userdata":          bootstrapData,
			"guestinfo.userdata.encoding": "base64",
			"guestinfo.metadata":          metadata,
			"guestinfo.metadata.encoding": "base64",
		}
		return nil
	})
	return err
}

func (v VmopMachineService) getGuestInfoMetadata(ctx *vmware.SupervisorMachineContext) (string, error) {
	tpl := template.Must(template.New("t").Parse(metadataFormat))

	// We can configure this for control plane machines - only the init node will
	// likely leverage it for controlPlaneEndpoint
	controlPlaneEndpoint := ""
	if util.IsControlPlaneMachine(ctx.Machine) && !ctx.Cluster.Spec.ControlPlaneEndpoint.IsZero() {
		apiEndpoint := ctx.Cluster.Spec.ControlPlaneEndpoint
		controlPlaneEndpoint = fmt.Sprintf("%s:%d", apiEndpoint.Host, apiEndpoint.Port)
	}

	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, struct {
		Hostname             string
		ControlPlaneEndpoint string
	}{
		Hostname:             ctx.Machine.Name,
		ControlPlaneEndpoint: controlPlaneEndpoint,
	}); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func (v *VmopMachineService) reconcileNetwork(ctx *vmware.SupervisorMachineContext, vm *vmoprv1.VirtualMachine) bool {
	if vm.Status.VmIp == "" {
		return false
	}

	ctx.VSphereMachine.Status.IPAddr = vm.Status.VmIp

	return true
}

func (v *VmopMachineService) reconcileProviderID(ctx *vmware.SupervisorMachineContext, vm *vmoprv1.VirtualMachine) {
	providerID := fmt.Sprintf("vsphere://%s", vm.Status.BiosUUID)

	if ctx.VSphereMachine.Spec.ProviderID == nil || *ctx.VSphereMachine.Spec.ProviderID != providerID {
		ctx.VSphereMachine.Spec.ProviderID = &providerID
		ctx.Logger.Info("Updated provider ID for machine", "machine", ctx.VSphereMachine.Name, "provider-id", providerID)
	}

	if ctx.VSphereMachine.Status.ID == nil || *ctx.VSphereMachine.Status.ID != vm.Status.BiosUUID {
		ctx.VSphereMachine.Status.ID = &vm.Status.BiosUUID
		ctx.Logger.Info("Updated VM ID for machine", "machine", ctx.VSphereMachine.Name, "vm-id", vm.Status.BiosUUID)
	}
}

// getVirtualMachinesInCluster returns all VMOperator VirtualMachine objects in the current cluster.
// First filter by clusterSelectorKey. If the result is empty, they fall back to legacyClusterSelectorKey.
func getVirtualMachinesInCluster(ctx *vmware.SupervisorMachineContext) ([]*vmoprv1.VirtualMachine, error) {
	labels := map[string]string{clusterSelectorKey: ctx.Cluster.Name}
	vmList := &vmoprv1.VirtualMachineList{}

	if err := ctx.Client.List(
		ctx, vmList,
		client.InNamespace(ctx.Cluster.Namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, errors.Wrapf(
			err, "error getting virtualmachines in cluster %s/%s",
			ctx.Cluster.Namespace, ctx.Cluster.Name)
	}

	// If the list is empty, fall back to usse legacy labels for filtering
	if len(vmList.Items) == 0 {
		legacyLabels := map[string]string{legacyClusterSelectorKey: ctx.Cluster.Name}
		if err := ctx.Client.List(
			ctx, vmList,
			client.InNamespace(ctx.Cluster.Namespace),
			client.MatchingLabels(legacyLabels)); err != nil {
			return nil, errors.Wrapf(
				err, "error getting virtualmachines in cluster %s/%s using legacy labels",
				ctx.Cluster.Namespace, ctx.Cluster.Name)
		}
	}

	vms := make([]*vmoprv1.VirtualMachine, len(vmList.Items))
	for i := range vmList.Items {
		vms[i] = &vmList.Items[i]
	}

	return vms, nil
}

// Helper function to add annotations to indicate which tag vm-operator should add as well as which clusterModule VM
// should be associated.
func addResourcePolicyAnnotations(ctx *vmware.SupervisorMachineContext, vm *vmoprv1.VirtualMachine) {
	annotations := vm.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if infrautilv1.IsControlPlaneMachine(ctx.Machine) {
		annotations[ProviderTagsAnnotationKey] = ControlPlaneVMVMAntiAffinityTagValue
		annotations[ClusterModuleNameAnnotationKey] = ControlPlaneVMClusterModuleGroupName
	} else {
		annotations[ProviderTagsAnnotationKey] = WorkerVMVMAntiAffinityTagValue
		annotations[ClusterModuleNameAnnotationKey] = vmwareutil.GetMachineDeploymentNameForCluster(ctx.Cluster)
	}

	vm.ObjectMeta.SetAnnotations(annotations)
}

func volumeName(machine *vmwarev1.VSphereMachine, volume vmwarev1.VSphereMachineVolume) string {
	return machine.Name + "-" + volume.Name
}

// addVolume ensures volume is included in vm.Spec.Volumes.
func addVolume(vm *vmoprv1.VirtualMachine, name string) {
	for _, volume := range vm.Spec.Volumes {
		claim := volume.PersistentVolumeClaim
		if claim != nil && claim.ClaimName == name {
			return // volume already present in the spec
		}
	}

	vm.Spec.Volumes = append(vm.Spec.Volumes, vmoprv1.VirtualMachineVolume{
		Name: name,
		PersistentVolumeClaim: &vmoprv1.PersistentVolumeClaimVolumeSource{
			PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: name,
				ReadOnly:  false,
			},
		},
	})
}

func addVolumes(ctx *vmware.SupervisorMachineContext, vm *vmoprv1.VirtualMachine) error {
	nvolumes := len(ctx.VSphereMachine.Spec.Volumes)
	if nvolumes == 0 {
		return nil
	}

	for _, volume := range ctx.VSphereMachine.Spec.Volumes {
		storageClassName := volume.StorageClass
		if volume.StorageClass == "" {
			storageClassName = ctx.VSphereMachine.Spec.StorageClass
		}

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      volumeName(ctx.VSphereMachine, volume),
				Namespace: ctx.VSphereMachine.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: volume.Capacity,
				},
				StorageClassName: &storageClassName,
			},
		}

		_, err := ctrlutil.CreateOrUpdate(ctx, ctx.Client, pvc, func() error {
			return ctrlutil.SetOwnerReference(ctx.VSphereMachine, pvc, ctx.Scheme)
		})
		if err != nil {
			return errors.Wrapf(err, "failed to create volume %s/%s", ctx.VSphereMachine.Namespace, pvc.Name)
		}

		addVolume(vm, pvc.Name)
	}

	return nil
}

// getVMLabels returns the labels applied to a VirtualMachine.
func getVMLabels(ctx *vmware.SupervisorMachineContext, vmLabels map[string]string) map[string]string {
	if vmLabels == nil {
		vmLabels = map[string]string{}
	}

	// Get the labels for the VM that differ based on the cluster role of
	// the Kubernetes node hosted on this VM.
	clusterRoleLabels := clusterRoleVMLabels(ctx.GetClusterContext(), infrautilv1.IsControlPlaneMachine(ctx.Machine))
	for k, v := range clusterRoleLabels {
		vmLabels[k] = v
	}

	// Get the labels that determine the VM's placement inside of a stretched
	// cluster.
	topologyLabels := getTopologyLabels(ctx)
	for k, v := range topologyLabels {
		vmLabels[k] = v
	}

	return vmLabels
}

// getTopologyLabels returns the labels related to a VM's topology.
//
// TODO(akutz): Currently this function just returns the availability zone,
//              and thus the code is optimized as such. However, in the future
//              this function may return a more diverse topology.
func getTopologyLabels(ctx *vmware.SupervisorMachineContext) map[string]string {
	if fd := ctx.VSphereMachine.Spec.FailureDomain; fd != nil && *fd != "" {
		return map[string]string{
			kubeTopologyZoneLabelKey: *fd,
		}
	}
	return nil
}
