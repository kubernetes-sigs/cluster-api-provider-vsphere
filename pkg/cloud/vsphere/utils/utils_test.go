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

package utils_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
)

func TestGetMachineRole(t *testing.T) {
	testCases := []struct {
		name    string
		machine *clusterv1.Machine
		expVal  string
	}{
		{
			name:    "set=master should parse as 'controlplane'",
			machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"set": "master"}}},
			expVal:  "controlplane",
		},
		{
			name:    "set=ControlPlane should parse as 'controlplane'",
			machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"set": "ControlPlane"}}},
			expVal:  "controlplane",
		},
		{
			name:    "set=node should parse as 'node'",
			machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"set": "node"}}},
			expVal:  "node",
		},
		{
			name:    "set=Worker-node should parse as 'node'",
			machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"set": "Worker-node"}}},
			expVal:  "node",
		},
		{
			name:    "node-type=controlPlane should parse as 'controlplane'",
			machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-type": "controlPlane"}}},
			expVal:  "controlplane",
		},
		{
			name:    "node-type=node should parse as 'node'",
			machine: &clusterv1.Machine{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-type": "Worker-node"}}},
			expVal:  "node",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if role := utils.GetMachineRole(tc.machine); role != tc.expVal {
				t.Errorf("exp=%q act=%q", tc.expVal, role)
				t.Fail()
			}
		})
	}
}
