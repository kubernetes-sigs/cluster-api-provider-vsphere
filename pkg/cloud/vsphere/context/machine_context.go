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
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
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

// Strings returns ClusterNamespace/ClusterName/MachineName
func (c *MachineContext) String() string {
	if c.Machine == nil {
		return c.ClusterContext.String()
	}
	return fmt.Sprintf("%s/%s/%s", c.Cluster.Namespace, c.Cluster.Name, c.Machine.Name)
}

// Role returns the machine's role.
func (c *MachineContext) Role() MachineRole {
	if c.Machine == nil {
		return ""
	}
	return GetMachineRole(c.Machine)
}

// IPAddr returns the machine's IP address.
func (c *MachineContext) IPAddr() string {
	if c.Machine == nil {
		return ""
	}
	return c.Machine.Annotations[constants.VmIpAnnotationKey]
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

// IsControlPlaneMember indicates whether a machine has the ControlPlaneRole.
func (c *MachineContext) IsControlPlaneMember() bool {
	return c.Role() == ControlPlaneRole
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
	if ipAddr == "" || !c.IsControlPlaneMember() {
		return "", errors.New("unable to get control plane endpoint")
	}
	controlPlaneEndpoint := net.JoinHostPort(ipAddr, strconv.Itoa(int(c.BindPort())))
	c.Logger.V(2).Info("got control plane endpoint from machine", "control-plane-endpoint", controlPlaneEndpoint)
	return controlPlaneEndpoint, nil
}

// Patch updates the machine on the API server.
func (c *MachineContext) Patch() error {

	ext, err := v1alpha1.EncodeMachineSpec(c.MachineConfig)
	if err != nil {
		return errors.Wrapf(err, "failed encoding machine spec for machine %q", c)
	}
	newStatus, err := v1alpha1.EncodeMachineStatus(c.MachineStatus)
	if err != nil {
		return errors.Wrapf(err, "failed encoding machine status for machine %q", c)
	}
	ext.Object = nil
	newStatus.Object = nil

	c.Machine.Spec.ProviderSpec.Value = ext

	// Build a patch and marshal that patch to something the client will
	// understand.
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

		result, err := c.MachineClient.Patch(c.Machine.Name, types.JSONPatchType, pb)
		//result, err := c.MachineClient.Update(c.Machine)
		if err != nil {
			record.Warnf(c.Machine, updateFailure, "failed to update machine config %q: %v", c, err)
			return errors.Wrapf(err, "failed to patch machine %q", c)
		}

		record.Eventf(c.Machine, updateSuccess, "updated machine config %q", c)

		// Keep the resource version updated so the status update can succeed
		c.Machine.ResourceVersion = result.ResourceVersion
	}

	c.Machine.Status.ProviderStatus = newStatus

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
