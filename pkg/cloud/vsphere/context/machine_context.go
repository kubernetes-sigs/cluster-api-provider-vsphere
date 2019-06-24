/*
Copyright 2019 The Kubernetes Authors.

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

package context

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	clusterUtilv1 "sigs.k8s.io/cluster-api/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/patch"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
)

// MachineContextParams are the parameters needed to create a MachineContext.
type MachineContextParams struct {
	ClusterContextParams
	Machine *clusterv1.Machine
}

// MachineContext is a Go context used with a CAPI cluster.
type MachineContext struct {
	*ClusterContext
	Machine       *clusterv1.Machine
	MachineCopy   *clusterv1.Machine
	MachineClient client.MachineInterface
	MachineConfig *v1alpha1.VsphereMachineProviderConfig
	MachineStatus *v1alpha1.VsphereMachineProviderStatus
	Session       *Session
}

// NewMachineContextFromClusterContext creates a new MachineContext using an
// existing CluserContext.
func NewMachineContextFromClusterContext(
	clusterCtx *ClusterContext, machine *clusterv1.Machine) (*MachineContext, error) {

	var machineClient client.MachineInterface
	if clusterCtx.client != nil {
		machineClient = clusterCtx.client.Machines(machine.Namespace)
	}

	machineConfig, err := v1alpha1.MachineConfigFromMachine(machine)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load machine provider config")
	}

	if machineConfig.KubeadmConfiguration.Init.LocalAPIEndpoint.BindPort == 0 {
		machineConfig.KubeadmConfiguration.Init.LocalAPIEndpoint.BindPort = constants.DefaultBindPort
	}
	if cp := machineConfig.KubeadmConfiguration.Join.ControlPlane; cp != nil {
		if cp.LocalAPIEndpoint.BindPort == 0 {
			cp.LocalAPIEndpoint.BindPort = constants.DefaultBindPort
		}
	}

	machineStatus, err := v1alpha1.MachineStatusFromMachine(machine)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load machine provider status")
	}

	clusterCtx.Logger = clusterCtx.Logger.WithName(machine.Name)

	machineCtx := &MachineContext{
		ClusterContext: clusterCtx,
		Machine:        machine,
		MachineCopy:    machine.DeepCopy(),
		MachineClient:  machineClient,
		MachineConfig:  machineConfig,
		MachineStatus:  machineStatus,
	}

	if machineCtx.CanLogin() {
		session, err := getOrCreateCachedSession(machineCtx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create vSphere session for machine %q", machineCtx)
		}
		machineCtx.Session = session
	}

	return machineCtx, nil
}

// NewMachineContext returns a new MachineContext.
func NewMachineContext(params *MachineContextParams) (*MachineContext, error) {
	ctx, err := NewClusterContext(&params.ClusterContextParams)
	if err != nil {
		return nil, err
	}
	return NewMachineContextFromClusterContext(ctx, params.Machine)
}

// NewMachineLoggerContext creates a new MachineContext with the given logger context.
func NewMachineLoggerContext(parentContext *MachineContext, loggerContext string) *MachineContext {
	ctx := &MachineContext{
		ClusterContext: parentContext.ClusterContext,
		Machine:        parentContext.Machine,
		MachineCopy:    parentContext.MachineCopy,
		MachineClient:  parentContext.MachineClient,
		MachineConfig:  parentContext.MachineConfig,
		MachineStatus:  parentContext.MachineStatus,
		Session:        parentContext.Session,
	}
	ctx.Logger = parentContext.Logger.WithName(loggerContext)
	return ctx
}

// Strings returns ClusterNamespace/ClusterName/MachineName
func (c *MachineContext) String() string {
	if c.Machine == nil {
		return c.ClusterContext.String()
	}
	return fmt.Sprintf("%s/%s/%s", c.Cluster.Namespace, c.Cluster.Name, c.Machine.Name)
}

// GetMoRef returns a managed object reference for the VM associated with
// the machine. A nil value is returned if the MachineRef is not yet set.
func (c *MachineContext) GetMoRef() *types.ManagedObjectReference {
	if c.MachineConfig.MachineRef == "" {
		return nil
	}
	return &types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: c.MachineConfig.MachineRef,
	}
}

// GetObject returns the Machine object.
func (c *MachineContext) GetObject() runtime.Object {
	return c.Machine
}

// GetSession returns the login session for this context.
func (c *MachineContext) GetSession() *Session {
	return c.Session
}

// HasControlPlaneRole returns a flag indicating whether or not a machine has
// the control plane role.
func (c *MachineContext) HasControlPlaneRole() bool {
	if c.Machine == nil {
		return false
	}
	return clusterUtilv1.IsControlPlaneMachine(c.Machine)
}

// IPAddr returns the machine's first IP address.
func (c *MachineContext) IPAddr() string {
	if c.Machine == nil {
		return ""
	}
	for _, nodeAddr := range c.Machine.Status.Addresses {
		if nodeAddr.Type == corev1.NodeInternalIP {
			return nodeAddr.Address
		}
	}
	return ""
}

// BindPort returns the machine's API bind port.
func (c *MachineContext) BindPort() int32 {
	if c.Machine == nil {
		return constants.DefaultBindPort
	}
	bindPort := c.MachineConfig.KubeadmConfiguration.Init.LocalAPIEndpoint.BindPort
	if cp := c.MachineConfig.KubeadmConfiguration.Join.ControlPlane; cp != nil {
		if jbp := cp.LocalAPIEndpoint.BindPort; jbp != bindPort {
			bindPort = jbp
		}
	}
	if bindPort == 0 {
		bindPort = constants.DefaultBindPort
	}
	return bindPort
}

// ControlPlaneEndpoint returns the control plane endpoint for the cluster.
// This function first attempts to retrieve the control plane endpoint with
// ClusterContext.ControlPlaneEndpoint.
// If no endpoint is returned then this machine's IP address is used as the
// control plane endpoint if the machine is a control plane node.
// Otherwise an error is returned.
func (c *MachineContext) ControlPlaneEndpoint() (string, error) {

	if controlPlaneEndpoint, _ := c.ClusterContext.ControlPlaneEndpoint(); controlPlaneEndpoint != "" {
		return controlPlaneEndpoint, nil
	}

	ipAddr := c.IPAddr()
	if ipAddr == "" || !c.HasControlPlaneRole() {
		return "", errors.New("unable to get control plane endpoint")
	}
	controlPlaneEndpoint := net.JoinHostPort(ipAddr, strconv.Itoa(int(c.BindPort())))
	c.Logger.V(2).Info("got control plane endpoint from machine", "control-plane-endpoint", controlPlaneEndpoint)
	return controlPlaneEndpoint, nil
}

// Patch updates the machine on the API server.
func (c *MachineContext) Patch() error {

	// Ensure the provider spec is encoded.
	newProviderSpec, err := v1alpha1.EncodeMachineSpec(c.MachineConfig)
	if err != nil {
		return errors.Wrapf(err, "failed encoding machine spec for machine %q", c)
	}
	c.Machine.Spec.ProviderSpec.Value = newProviderSpec

	// Make sure the status isn't part of the JSON patch.
	newStatus := c.Machine.Status.DeepCopy()
	c.Machine.Status = clusterv1.MachineStatus{}
	c.MachineCopy.Status.DeepCopyInto(&c.Machine.Status)

	// Build and marshal a patch for the machine object, minus the status.
	p, err := patch.NewJSONPatch(c.MachineCopy, c.Machine)
	if err != nil {
		return errors.Wrapf(err, "failed to create new JSONPatch for machine %q", c)
	}

	// Do not update Machine if nothing has changed
	if len(p) != 0 {
		pb, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "failed to json marshal patch for machine %q", c)
		}

		c.Logger.V(1).Info("patching machine")
		c.Logger.V(6).Info("generated json patch for machine", "json-patch", string(pb))

		result, err := c.MachineClient.Patch(c.Machine.Name, apitypes.JSONPatchType, pb)
		if err != nil {
			record.Warnf(c.Machine, updateFailure, "failed to update machine config %q: %v", c, err)
			return errors.Wrapf(err, "failed to patch machine %q", c)
		}

		record.Eventf(c.Machine, updateSuccess, "updated machine config %q", c)

		// Keep the resource version updated so the status update can succeed
		c.Machine.ResourceVersion = result.ResourceVersion
	}

	// Put the status back.
	c.Machine.Status = clusterv1.MachineStatus{}
	newStatus.DeepCopyInto(&c.Machine.Status)

	if !reflect.DeepEqual(c.Machine.Status, c.MachineCopy.Status) {
		c.Logger.V(1).Info("updating machine status")
		if _, err := c.MachineClient.UpdateStatus(c.Machine); err != nil {
			record.Warnf(c.Machine, updateFailure, "failed to update machine status for machine %q: %v", c, err)
			return errors.Wrapf(err, "failed to update machine status for machine %q", c)
		}
		record.Eventf(c.Machine, updateSuccess, "updated machine status for machine %q", c)
	}
	return nil
}
