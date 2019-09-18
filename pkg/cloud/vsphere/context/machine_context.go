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
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

// MachineContextParams are the parameters needed to create a MachineContext.
type MachineContextParams struct {
	ClusterContextParams
	Machine        *clusterv1.Machine
	VSphereMachine *v1alpha2.VSphereMachine
}

// MachineContext is a Go context used with a CAPI cluster.
type MachineContext struct {
	*ClusterContext
	Machine        *clusterv1.Machine
	VSphereMachine *v1alpha2.VSphereMachine
	Session        *Session
	RestSession *RestSession // NetApp

	vsphereMachinePatch client.Patch
}

// NewMachineContextFromClusterContext creates a new MachineContext using an
// existing CluserContext.
func NewMachineContextFromClusterContext(
	clusterCtx *ClusterContext,
	machine *clusterv1.Machine,
	vsphereMachine *v1alpha2.VSphereMachine) (*MachineContext, error) {

	clusterCtx.Logger = clusterCtx.Logger.WithName(machine.Name)

	machineCtx := &MachineContext{
		ClusterContext:      clusterCtx,
		Machine:             machine,
		VSphereMachine:      vsphereMachine,
		vsphereMachinePatch: client.MergeFrom(vsphereMachine.DeepCopyObject()),
	}

	if machineCtx.CanLogin() {
		session, err := getOrCreateCachedSession(machineCtx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create vSphere session for machine %q", machineCtx)
		}
		machineCtx.Session = session
		// NetApp
		restSession, err := getOrCreateCachedRESTSession(machineCtx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create vSphere REST session for machine %q", machineCtx)
		}
		machineCtx.RestSession = restSession
	}

	return machineCtx, nil
}

// NewMachineContext returns a new MachineContext.
func NewMachineContext(params *MachineContextParams) (*MachineContext, error) {
	ctx, err := NewClusterContext(&params.ClusterContextParams)
	if err != nil {
		return nil, err
	}
	return NewMachineContextFromClusterContext(ctx, params.Machine, params.VSphereMachine)
}

// NewMachineLoggerContext creates a new MachineContext with the given logger context.
func NewMachineLoggerContext(parentContext *MachineContext, loggerContext string) *MachineContext {
	ctx := &MachineContext{
		ClusterContext: parentContext.ClusterContext,
		Machine:        parentContext.Machine,
		VSphereMachine: parentContext.VSphereMachine,
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

// GetObject returns the Machine object.
func (c *MachineContext) GetObject() runtime.Object {
	return c.Machine
}

// GetSession returns the login session for this context.
func (c *MachineContext) GetSession() *Session {
	return c.Session
}

// Patch updates the object and its status on the API server.
func (c *MachineContext) Patch() error {

	// Patch Machine object.
	if err := c.Client.Patch(c, c.VSphereMachine, c.vsphereMachinePatch); err != nil {
		return errors.Wrapf(err, "error patching VSphereMachine %s/%s", c.Machine.Namespace, c.Machine.Name)
	}

	// Patch Machine status.
	if err := c.Client.Status().Patch(c, c.VSphereMachine, c.vsphereMachinePatch); err != nil {
		return errors.Wrapf(err, "error patching VSphereMachine %s/%s status", c.Machine.Namespace, c.Machine.Name)
	}

	return nil
}

// NetApp
// GetNKSClusterInfo returns NKS information on the cluster that the machine is a part of
// Returns clusterID, workspaceID, isServiceCluster
func (c *MachineContext) GetNKSClusterInfo() (string, string, bool) {

	const ClusterIdLabel = "hci.nks.netapp.com/cluster"
	const WorkspaceIdLabel = "hci.nks.netapp.com/workspace"
	const ClusterRoleLabel = "hci.nks.netapp.com/role"
	const ServiceClusterRole = "service-cluster"

	var workspaceID = ""
	var clusterID = ""
	var isServiceCluster bool

	if val, ok := c.Cluster.Labels[WorkspaceIdLabel]; ok {
		workspaceID = val
	}
	if val, ok := c.Cluster.Labels[ClusterIdLabel]; ok {
		clusterID = val
	}
	if val, ok := c.Cluster.Labels[ClusterRoleLabel]; ok {
		if val == ServiceClusterRole {
			isServiceCluster = true
		}
	}

	return clusterID, workspaceID, isServiceCluster
}

// NetApp
func (c *MachineContext) GetMachineAnnotation() string {
	// TODO: At this point we do not know if it is a service cluster - need better communication of that
	clusterID, workspaceID, _ := c.GetNKSClusterInfo()
	return fmt.Sprintf("VM is part of NKS kubernetes cluster %s with cluster ID %s in workspace with ID %s", c.Cluster.Name, clusterID, workspaceID)
}
