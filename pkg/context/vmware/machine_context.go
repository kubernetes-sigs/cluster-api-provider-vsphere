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

package vmware

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/govmomi/v1beta2"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
)

// VMModifier allows a function to be passed to VM creation to modify its spec
// The hook is loosely typed so as to allow for different VirtualMachine backends.
type VMModifier func(runtime.Object) (runtime.Object, error)

// SupervisorMachineContext is a Go capvcontext used with a VSphereMachine.
type SupervisorMachineContext struct {
	*capvcontext.BaseMachineContext
	VSphereCluster *vmwarev1.VSphereCluster
	VSphereMachine *vmwarev1.VSphereMachine
	VMOperatorVM   *vmoprvhub.VirtualMachine
	VMModifiers    []VMModifier

	// ConfigureControlPlaneVMReadinessProbe indicates whether a TCP readiness probe should be
	// configured on control plane VMs. It is resolved per-cluster from the network provider's
	// SupportsVMReadinessProbe().
	ConfigureControlPlaneVMReadinessProbe bool
}

// String returns VSphereMachineGroupVersionKind VSphereMachineNamespace/VSphereMachineName.
func (c *SupervisorMachineContext) String() string {
	return fmt.Sprintf("%s %s/%s", c.VSphereMachine.GroupVersionKind(), c.VSphereMachine.Namespace, c.VSphereMachine.Name)
}

// Patch updates the object and its status on the API server.
func (c *SupervisorMachineContext) Patch(ctx context.Context) error {
	return c.PatchHelper.Patch(ctx, c.VSphereMachine,
		patch.WithOwnedV1Beta1Conditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyV1Beta1Condition,
			infrav1.VMProvisionedV1Beta1Condition,
		}},
		patch.WithOwnedConditions{Conditions: []string{
			infrav1.VSphereMachineReadyCondition,
			infrav1.VSphereMachineVirtualMachineProvisionedCondition,
			clusterv1.PausedCondition,
		}})
}

// GetVSphereMachine returns the VSphereMachine from the SupervisorMachineContext.
func (c *SupervisorMachineContext) GetVSphereMachine() capvcontext.VSphereMachine {
	return c.VSphereMachine
}

// GetReady return when the VSphereMachine is ready.
func (c *SupervisorMachineContext) GetReady() bool {
	return ptr.Deref(c.VSphereMachine.Status.Initialization.Provisioned, false)
}

// IsHostInMaintenance reports whether VM Operator has blocked powering on the VM because its host is in maintenance.
func (c *SupervisorMachineContext) IsHostInMaintenance() bool {
	if c == nil || c.VMOperatorVM == nil {
		return false
	}
	if !feature.Gates.Enabled(feature.HostMaintenanceMode) {
		return false
	}

	condition := meta.FindStatusCondition(c.VMOperatorVM.Status.Conditions, vmoprvhub.VirtualMachinePowerStateSynced)
	return condition != nil && condition.Status == metav1.ConditionFalse &&
		condition.Reason == vmoprvhub.VirtualMachineInfraInMaintenanceReason
}

// GetObjectMeta returns the metadata for the VSphereMachine from the SupervisorMachineContext.
func (c *SupervisorMachineContext) GetObjectMeta() metav1.ObjectMeta {
	return c.VSphereMachine.ObjectMeta
}

// GetClusterContext returns the Cluster and VSphereCluster from the SupervisorMachineContext.
func (c *SupervisorMachineContext) GetClusterContext() *ClusterContext {
	return &ClusterContext{
		Cluster:        c.Cluster,
		VSphereCluster: c.VSphereCluster,
	}
}

// SetBaseMachineContext sets the BaseMachineContext for the SupervisorMachineContext.
func (c *SupervisorMachineContext) SetBaseMachineContext(base *capvcontext.BaseMachineContext) {
	c.BaseMachineContext = base
}
