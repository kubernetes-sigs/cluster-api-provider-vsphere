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

package context_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

func TestMachineRole(t *testing.T) {
	testCases := []struct {
		name   string
		ctx    *context.MachineContext
		expVal context.MachineRole
	}{
		{
			name:   "set=master should parse as 'controlplane'",
			ctx:    &context.MachineContext{Machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"set": "master"}}}},
			expVal: context.ControlPlaneRole,
		},
		{
			name:   "set=ControlPlane should parse as 'controlplane'",
			ctx:    &context.MachineContext{Machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"set": "ControlPlane"}}}},
			expVal: context.ControlPlaneRole,
		},
		{
			name:   "set=node should parse as 'node'",
			ctx:    &context.MachineContext{Machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"set": "node"}}}},
			expVal: context.NodeRole,
		},
		{
			name:   "set=Worker-node should parse as 'node'",
			ctx:    &context.MachineContext{Machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"set": "Worker-node"}}}},
			expVal: context.NodeRole,
		},
		{
			name:   "node-type=controlPlane should parse as 'controlplane'",
			ctx:    &context.MachineContext{Machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-type": "controlPlane"}}}},
			expVal: context.ControlPlaneRole,
		},
		{
			name:   "node-type=node should parse as 'node'",
			ctx:    &context.MachineContext{Machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-type": "Worker-node"}}}},
			expVal: context.NodeRole,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if role := tc.ctx.Role(); role != tc.expVal {
				t.Errorf("exp=%q act=%q", tc.expVal, role)
				t.Fail()
			}
		})
	}
}
