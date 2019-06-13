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
	"regexp"

	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// MachineRole is the role of a Machine
type MachineRole string

const (
	// ControlPlaneRole indicates a machine is a member of the control plane.
	ControlPlaneRole MachineRole = "controlplane"

	// NodeRole indicates a machine is a member of the cluster.
	NodeRole MachineRole = "node"
)

// GetMachineRole returns the Machine's role.
func GetMachineRole(machine *clusterv1.Machine) MachineRole {
	role := machine.Labels["set"]
	if ok, _ := regexp.MatchString(`(?i)controlplane|master`, role); ok {
		return ControlPlaneRole
	}
	if ok, _ := regexp.MatchString(`(?i)(?:worker-)?node`, role); ok {
		return NodeRole
	}
	role = machine.Labels["node-type"]
	if ok, _ := regexp.MatchString(`(?i)controlplane|master`, role); ok {
		return ControlPlaneRole
	}
	if ok, _ := regexp.MatchString(`(?i)(?:worker-)?node`, role); ok {
		return NodeRole
	}
	return ""
}
