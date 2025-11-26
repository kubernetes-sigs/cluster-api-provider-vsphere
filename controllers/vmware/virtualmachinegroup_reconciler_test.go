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

package vmware

import (
	"fmt"

	"testing"

	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

func Test_shouldCreateVirtualMachineGroup(t *testing.T) {
	tests := []struct {
		name            string
		mds             []clusterv1.MachineDeployment
		vSphereMachines []vmwarev1.VSphereMachine
		want            bool
	}{
		{
			name: "Should create a VMG if all the expected VSphereMachines exists",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "", 2),
				*createMD("md2", "test-cluster", "", 1),
				*createMD("md3", "test-cluster", "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", "test-cluster", "md1", ""),
				*createVSphereMachine("m2", "test-cluster", "md1", ""),
				*createVSphereMachine("m3", "test-cluster", "md2", ""),
				*createVSphereMachine("m4", "test-cluster", "md3", "zone1"),
			},
			want: true, // tot replicas = 4, 4 VSphereMachine exist
		},
		{
			name: "Should create a VMG if all the expected VSphereMachines exists, deleting MD should be ignored",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "", 2),
				*createMD("md2", "test-cluster", "", 1, func(md *clusterv1.MachineDeployment) {
					md.DeletionTimestamp = ptr.To(metav1.Now())
				}), // Should not be included in the count
				*createMD("md3", "test-cluster", "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", "test-cluster", "md1", ""),
				*createVSphereMachine("m2", "test-cluster", "md1", ""),
				*createVSphereMachine("m4", "test-cluster", "md3", "zone1"),
			},
			want: true, // tot replicas = 3 (one md is deleting, so not included in the total), 3 VSphereMachine exist
		},
		{
			name: "Should not create a VMG if some of the expected VSphereMachines does not exist",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "", 2),
				*createMD("md2", "test-cluster", "", 1),
				*createMD("md3", "test-cluster", "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", "test-cluster", "md1", ""),
				*createVSphereMachine("m3", "test-cluster", "md2", ""),
				*createVSphereMachine("m4", "test-cluster", "md3", "zone1"),
			},
			want: false, // tot replicas = 4, 3 VSphereMachine exist
		},
		{
			name:            "Should not create a VMG there are no expected VSphereMachines",
			mds:             []clusterv1.MachineDeployment{}, // No Machine deployments
			vSphereMachines: []vmwarev1.VSphereMachine{},     // No VSphereMachine
			want:            false,
		},
	}
<<<<<<< HEAD

	member := func(name string) vmoprv1.GroupMember { return vmoprv1.GroupMember{Name: name} }

	// CAPI Machine helpers
	makeCAPIMachine := func(name, namespace string, fd *string) *clusterv1.Machine {
		m := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		}
		if fd != nil {
			m.Spec = clusterv1.MachineSpec{FailureDomain: *fd}
		}
		return m
	}
	makeCAPIMachineNoFailureDomain := func(name, namespace string) *clusterv1.Machine {
		return makeCAPIMachine(name, namespace, nil)
=======
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := shouldCreateVirtualMachineGroup(ctx, tt.mds, tt.vSphereMachines)
			g.Expect(got).To(Equal(tt.want))
		})
>>>>>>> 9409e432 (POC AAF)
	}
}

func Test_getVirtualMachineNameToMachineDeploymentMapping(t *testing.T) {
	tests := []struct {
		name            string
		vSphereMachines []vmwarev1.VSphereMachine
		want            map[string]string
	}{
		{
			name: "mapping from VirtualMachineName to MachineDeployment is inferred from vSphereMachines",
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", "test-cluster", "md1", ""),
				*createVSphereMachine("m2", "test-cluster", "md1", ""),
				*createVSphereMachine("m3", "test-cluster", "md2", ""),
				*createVSphereMachine("m4", "test-cluster", "md3", "zone1"),
			},
			want: map[string]string{
				// Note VirtualMachineName is equal to the VSphereMachine because when using the default
				"m1": "md1",
				"m2": "md1",
				"m3": "md2",
				"m4": "md3",
			},
		},
		{
			name: "mapping from VirtualMachineName to MachineDeployment is inferred from vSphereMachines",
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", "test-cluster", "md1", ""),
				*createVSphereMachine("m2", "test-cluster", "md1", ""),
				*createVSphereMachine("m3", "test-cluster", "md2", ""),
				*createVSphereMachine("m4", "test-cluster", "md3", "zone1"),
			},
			want: map[string]string{
				// Note VirtualMachineName is equal to the VSphereMachine name because when using the default naming strategy
				"m1": "md1",
				"m2": "md1",
				"m3": "md2",
				"m4": "md3",
			},
		},
		{
			name: "mapping from VirtualMachineName to MachineDeployment is inferred from vSphereMachines (custom naming strategy)",
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", "test-cluster", "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", "test-cluster", "md1", "", withCustomNamingStrategy(), func(m *vmwarev1.VSphereMachine) {
					m.DeletionTimestamp = ptr.To(metav1.Now())
				}), // Should not be included in the mapping
				*createVSphereMachine("m3", "test-cluster", "md2", "", withCustomNamingStrategy()),
				*createVSphereMachine("m4", "test-cluster", "md3", "zone1"),
			},
			want: map[string]string{
				"m1-vm": "md1",
				// "m2-vm" not be included in the count
				"m3-vm": "md2",
				"m4":    "md3",
			},
		},
		{
			name: "deleting vSphereMachines are not included in the mapping",
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", "test-cluster", "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", "test-cluster", "md1", "", withCustomNamingStrategy(), func(m *vmwarev1.VSphereMachine) {
					m.DeletionTimestamp = ptr.To(metav1.Now())
				}), // Should not be included in the mapping
				*createVSphereMachine("m3", "test-cluster", "md2", "", withCustomNamingStrategy()),
				*createVSphereMachine("m4", "test-cluster", "md3", "zone1"),
			},
			want: map[string]string{
				"m1-vm": "md1",
				// "m2-vm" not be included in the count
				"m3-vm": "md2",
				"m4":    "md3",
			},
		},
		{
			name: "vSphereMachines without the MachineDeploymentNameLabel are not included in the mapping",
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", "test-cluster", "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", "test-cluster", "md1", "", withCustomNamingStrategy(), func(m *vmwarev1.VSphereMachine) {
					delete(m.Labels, clusterv1.MachineDeploymentNameLabel)
				}), // Should not be included in the mapping
				*createVSphereMachine("m3", "test-cluster", "md2", "", withCustomNamingStrategy()),
				*createVSphereMachine("m4", "test-cluster", "md3", "zone1"),
			},
			want: map[string]string{
				"m1-vm": "md1",
				// "m2-vm" not be included in the count
				"m3-vm": "md2",
				"m4":    "md3",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, err := getVirtualMachineNameToMachineDeploymentMapping(ctx, tt.vSphereMachines)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func Test_getMachineDeploymentToFailureDomainMapping(t *testing.T) {
	tests := []struct {
		name                                  string
		mds                                   []clusterv1.MachineDeployment
		existingVMG                           *vmoprv1.VirtualMachineGroup
		virtualMachineNameToMachineDeployment map[string]string
		want                                  map[string]string
	}{
		{
			name: "MachineDeployment mapping should use spec.failure domain",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "zone1", 1), // failure domain explicitly set
			},
			existingVMG:                           nil,
			virtualMachineNameToMachineDeployment: nil,
			want: map[string]string{
				"md1": "zone1",
			},
		},
		{
			name: "MachineDeployment mapping should use spec.failure domain (latest value must be used)",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "zone2", 1), // failure domain explicitly set
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone1", // Previously md1 was assigned to zone1
					},
				},
			},
			virtualMachineNameToMachineDeployment: nil,
			want: map[string]string{
				"md1": "zone2", // latest spec.failure must be used
			},
		},
		{
			name: "MachineDeployment mapping should use placement decision from VirtualMachineGroup annotations",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "", 1), // failure domain not explicitly set
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone1", // Placement decision for md1 already reported into annotation
					},
				},
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						{
							Name: "m1-vm",
							Placement: &vmoprv1.VirtualMachinePlacementStatus{
								Zone: "zone2", // Note: this should never happen (different placement decision than what is in the annotation), but using this value to validate that the mapping used is the one from the annotation.
							},
						},
					},
				},
			},
			virtualMachineNameToMachineDeployment: map[string]string{
				"m1-vm": "md1",
			},
			want: map[string]string{
				"md1": "zone1",
			},
		},
		{
			name: "MachineDeployment mapping should use placement decision from VirtualMachineGroup status",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "", 1), // failure domain not explicitly set
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						// Placement decision for md1 not yet reported into annotation
					},
				},
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						{
							Name: "m1-vm",
							Placement: &vmoprv1.VirtualMachinePlacementStatus{
								Zone: "zone1",
							},
							Conditions: []metav1.Condition{
								{
									Type:   vmoprv1.VirtualMachineGroupMemberConditionPlacementReady,
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
			virtualMachineNameToMachineDeployment: map[string]string{
				"m1-vm": "md1",
			},
			want: map[string]string{
				"md1": "zone1",
			},
		},
		{
			name: "MachineDeployment not yet placed (VirtualMachineGroup not yet created)",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "", 1), // failure domain not explicitly set
			},
			existingVMG: nil,
			virtualMachineNameToMachineDeployment: map[string]string{
				"m1-vm": "md1",
			},
			want: map[string]string{
				// "md1" not yet placed
			},
		},
		{
			name: "MachineDeployment not yet placed (VirtualMachineGroup status not yet reporting placement for MachineDeployment's VirtualMachines)",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "", 1), // failure domain not explicitly set
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						// Placement decision for md1 not yet reported into annotation
					},
				},
				// Status empty
			},
			virtualMachineNameToMachineDeployment: nil,
			want:                                  map[string]string{}, // "md1" not yet placed
		},
		{
			name: "MachineDeployment not yet placed (VirtualMachineGroup status not yet reporting placement completed for MachineDeployment's VirtualMachines)",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "", 1), // failure domain not explicitly set
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						// Placement decision for md1 not yet reported into annotation
					},
				},
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						{
							Name: "m1-vm",
							Conditions: []metav1.Condition{
								{
									Type:   vmoprv1.VirtualMachineGroupMemberConditionPlacementReady,
									Status: metav1.ConditionFalse, // placement not completed yet
								},
							},
						},
					},
				},
			},
			virtualMachineNameToMachineDeployment: map[string]string{
				"m1-vm": "md1",
			},
			want: map[string]string{
				// "md1" not yet placed
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := getMachineDeploymentToFailureDomainMapping(ctx, tt.mds, tt.existingVMG, tt.virtualMachineNameToMachineDeployment)
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestVirtualMachineGroupReconciler_computeVirtualMachineGroup(t *testing.T) {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "test-cluster",
		},
	}
	tests := []struct {
		name            string
		mds             []clusterv1.MachineDeployment
		vSphereMachines []vmwarev1.VSphereMachine
		existingVMG     *vmoprv1.VirtualMachineGroup
		want            *vmoprv1.VirtualMachineGroup
	}{
		// Compute new VirtualMachineGroup (start initial placement)
		{
			name: "compute new VirtualMachineGroup",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", cluster.Name, "", 2),
				*createMD("md2", cluster.Name, "", 1),
				*createMD("md3", cluster.Name, "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", cluster.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", cluster.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m3", cluster.Name, "md2", "", withCustomNamingStrategy()),
				*createVSphereMachine("m4", cluster.Name, "md3", "zone1"),
			},
			existingVMG: nil,
			want: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3-vm", Kind: "VirtualMachine"},
								{Name: "m4", Kind: "VirtualMachine"},
							},
						},
					},
				},
			},
		},

		// Compute updated VirtualMachineGroup (during initial placement)
		{
			name: "compute updated VirtualMachineGroup during initial placement",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", cluster.Name, "", 2),
				*createMD("md3", cluster.Name, "zone1", 2),
				*createMD("md4", cluster.Name, "zone2", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", cluster.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m5", cluster.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m4", cluster.Name, "md3", "zone1"),
				*createVSphereMachine("m6", cluster.Name, "md3", "zone1"),
				*createVSphereMachine("m7", cluster.Name, "md4", "zone2"),
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"}, // Deleted after VMG creation
								{Name: "m3-vm", Kind: "VirtualMachine"}, // Deleted after VMG creation (the entire md2 was deleted).
								{Name: "m4", Kind: "VirtualMachine"},
								// m5-vm (md1), m6 (md3), m7 (md4) created after VMG creation.
							},
						},
					},
				},
				// Not setting status for sake of simplicity (also we are simulating when placing decision is not yet completed)
			},
			want: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md4"): "zone2", // failureDomain for md4 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"}, // existing before, still existing
								// "m2-vm" was deleted
								// "m3-vm" was deleted
								{Name: "m4", Kind: "VirtualMachine"}, // existing before, still existing
								// "m5-vm" was added, but it should not be added yet because md1 is not yet placed
								{Name: "m6", Kind: "VirtualMachine"}, // added, failureDomain for md3 is explicitly set by the user
								{Name: "m7", Kind: "VirtualMachine"}, // added, failureDomain for md4 is explicitly set by the user
							},
						},
					},
				},
			},
		},

		// Compute updated VirtualMachineGroup (after initial placement)
		{
			name: "compute updated VirtualMachineGroup after initial placement",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", cluster.Name, "", 2),
				*createMD("md3", cluster.Name, "zone1", 2),
				*createMD("md4", cluster.Name, "zone2", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", cluster.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m5", cluster.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m4", cluster.Name, "md3", "zone1"),
				*createVSphereMachine("m6", cluster.Name, "md3", "zone1"),
				*createVSphereMachine("m7", cluster.Name, "md4", "zone2"),
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone5", // failureDomain for md2 set by initial placement
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"}, // Deleted after VMG creation
								{Name: "m3-vm", Kind: "VirtualMachine"}, // Deleted after VMG creation (the entire md2 was deleted).
								{Name: "m4", Kind: "VirtualMachine"},
								// m5-vm (md1), m6 (md3), m7 (md4) created after VMG creation.
							},
						},
					},
				},
				// Not setting status for sake of simplicity (in a real VMG, after the placement decision status should have members)
			},
			want: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						// annotation for md2 deleted, md2 does not exist anymore
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md4"): "zone2", // failureDomain for md4 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"}, // existing before, still existing
								// "m2-vm" was deleted
								// "m3-vm" was deleted
								{Name: "m4", Kind: "VirtualMachine"},    // existing before, still existing
								{Name: "m5-vm", Kind: "VirtualMachine"}, // added, failureDomain for md1 set by initial placement
								{Name: "m6", Kind: "VirtualMachine"},    // added, failureDomain for md3 is explicitly set by the user
								{Name: "m7", Kind: "VirtualMachine"},    // added, failureDomain for md4 is explicitly set by the user
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, err := computeVirtualMachineGroup(ctx, cluster, tt.mds, tt.vSphereMachines, tt.existingVMG)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(got).To(BeComparableTo(tt.want))
		})
	}
}

func TestVirtualMachineGroupReconciler_ReconcileSequence(t *testing.T) {
	clusterNotYetInitialized := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "test-cluster",
		},
	}
	clusterInitialized := clusterNotYetInitialized.DeepCopy()
	clusterInitialized.Status.Conditions = []metav1.Condition{
		{
			Type:   clusterv1.ClusterControlPlaneInitializedCondition,
			Status: metav1.ConditionTrue,
		},
	}

	tests := []struct {
		name            string
<<<<<<< HEAD
		targetMember    []vmoprv1.GroupMember
		vmgInput        *vmoprv1.VirtualMachineGroup
		existingObjects []runtime.Object
		wantErr         bool
		expectedErrMsg  string
=======
		cluster         *clusterv1.Cluster
		mds             []clusterv1.MachineDeployment
		vSphereMachines []vmwarev1.VSphereMachine
		existingVMG     *vmoprv1.VirtualMachineGroup
		wantResult      ctrl.Result
		wantVMG         *vmoprv1.VirtualMachineGroup
>>>>>>> 9409e432 (POC AAF)
	}{
		// Before initial placement
		{
<<<<<<< HEAD
			name:            "Allow Create if VirtualMachineGroup not existed",
			targetMember:    []vmoprv1.GroupMember{member(memberName1)},
			vmgInput:        baseVMG.DeepCopy(),
			existingObjects: nil,
			wantErr:         false,
			expectedErrMsg:  "",
=======
			name:            "VirtualMachineGroup should not be created when the cluster is not yet initialized",
			cluster:         clusterNotYetInitialized,
			mds:             nil,
			vSphereMachines: nil,
			existingVMG:     nil,
			wantResult:      ctrl.Result{},
			wantVMG:         nil,
>>>>>>> 9409e432 (POC AAF)
		},
		{
			name:    "VirtualMachineGroup should not be created when waiting for vSphereMachines to exist",
			cluster: clusterNotYetInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterNotYetInitialized.Name, "", 1),
				*createMD("md2", clusterNotYetInitialized.Name, "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterNotYetInitialized.Name, "md1", "", withCustomNamingStrategy()),
			},
			existingVMG: nil,
			wantResult:  ctrl.Result{},
			wantVMG:     nil,
		},
		{
			name:    "VirtualMachineGroup should not be created when waiting for vSphereMachines to exist (adapt to changes)",
			cluster: clusterInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterInitialized.Name, "", 2), // Scaled up one additional machine is still missing
				*createMD("md2", clusterInitialized.Name, "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m3", clusterInitialized.Name, "md2", "zone1"),
			},
			existingVMG: nil,
			wantResult:  ctrl.Result{},
			wantVMG:     nil,
		},
		{
			name:    "VirtualMachineGroup should be created when all the vSphereMachines exist",
			cluster: clusterInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterInitialized.Name, "", 2),
				*createMD("md2", clusterInitialized.Name, "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m3", clusterInitialized.Name, "md2", "zone1"),
			},
			existingVMG: nil,
			wantResult:  ctrl.Result{},
			wantVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
<<<<<<< HEAD
							Name: memberName1,
							Kind: memberKind,
						}}}}
				return []runtime.Object{v}
			}(),
			wantErr:        false,
			expectedErrMsg: "",
		},
		{
			name:         "Allow Patch if no new member",
			targetMember: []vmoprv1.GroupMember{member(memberName1)}, // No new members
			vmgInput:     baseVMG.DeepCopy(),
			existingObjects: func() []runtime.Object {
				v := baseVMG.DeepCopy()
				// Annotation for mdName1 is missing
				v.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{{Members: []vmoprv1.GroupMember{
					{
						Name: memberName1,
						Kind: memberKind,
					},
				}}}
				return []runtime.Object{v}
			}(),
			wantErr:        false,
			expectedErrMsg: "",
=======
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
							},
						},
					},
				},
			},
>>>>>>> 9409e432 (POC AAF)
		},

		// During initial placement
		{
<<<<<<< HEAD
			name:         "Block Patch to add new member if VirtualMachineGroup is not Placement Ready",
			targetMember: []vmoprv1.GroupMember{member(memberName1), member(memberName2)},
			vmgInput:     baseVMG.DeepCopy(),
			existingObjects: func() []runtime.Object {
				v := baseVMG.DeepCopy()
				v.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{
					{Members: []vmoprv1.GroupMember{
						{
							Name: memberName1,
							Kind: memberKind,
						}}}}
				return []runtime.Object{v}
			}(),
			wantErr:        true,
			expectedErrMsg: fmt.Sprintf("waiting for VirtualMachineGroup %s to get condition Ready to true, temporarily blocking patch", vmgName),
		},
		{
			name:         "Block Patch if new member VSphereMachine Not Found",
			targetMember: []vmoprv1.GroupMember{member(memberName1), member(memberName2)}, // vm-02 is new
			vmgInput:     baseVMG.DeepCopy(),
			existingObjects: func() []runtime.Object {
				v := baseVMG.DeepCopy()
				conditions.Set(v, metav1.Condition{
					Type:   vmoprv1.ReadyConditionType,
					Status: metav1.ConditionTrue})
				v.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{{Members: []vmoprv1.GroupMember{
					{
						Name: memberName1,
						Kind: memberKind,
					},
				}}}
				// vm-02 VSphereMachine is missing
				return []runtime.Object{v, makeVSphereMachineOwned(memberName1, vmgNamespace, ownerMachineName1, mdName1), makeCAPIMachine(ownerMachineName1, vmgNamespace, ptr.To(failureDomainA))}
			}(),
			wantErr:        true,
			expectedErrMsg: fmt.Sprintf("VSphereMachine for new member %s not found, temporarily blocking patch", memberName2),
		},
		{
			name:         "Block Patch if VSphereMachine found but owner CAPI Machine missing",
			targetMember: []vmoprv1.GroupMember{member(memberName1), member(memberName2)}, // vm-02 is new
			vmgInput:     baseVMG.DeepCopy(),
			existingObjects: func() []runtime.Object {
				v := baseVMG.DeepCopy()
				conditions.Set(v, metav1.Condition{
					Type:   vmoprv1.ReadyConditionType,
					Status: metav1.ConditionTrue})
				v.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{{Members: []vmoprv1.GroupMember{
					{
						Name: memberName1,
						Kind: memberKind,
					},
				}}}
				// vm-02 VSphereMachine exists but has no owner ref
				return []runtime.Object{v, makeVSphereMachineOwned(memberName1, vmgNamespace, "ownerMachineName1", mdName1), makeCAPIMachine("ownerMachineName1", vmgNamespace, ptr.To(failureDomainA)), makeVSphereMachineNoOwner(memberName2, vmgNamespace)}
			}(),
			wantErr:        true,
			expectedErrMsg: fmt.Sprintf("VSphereMachine %s found but owner Machine reference is missing, temporarily blocking patch", memberName2),
		},
		{
			name:         "Allow Patch if all new members have Machine FailureDomain specified",
			targetMember: []vmoprv1.GroupMember{member(memberName1), member(memberName2)}, // vm-02 is new
			vmgInput:     baseVMG.DeepCopy(),
			existingObjects: func() []runtime.Object {
				v := baseVMG.DeepCopy()
				conditions.Set(v, metav1.Condition{
					Type:   vmoprv1.ReadyConditionType,
					Status: metav1.ConditionTrue})
				v.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{{Members: []vmoprv1.GroupMember{
					{
						Name: memberName1,
						Kind: memberKind,
					},
				}}}
				// m-02 (owner of ownerMachineName2) has FailureDomain set
				return []runtime.Object{
					v,
					makeVSphereMachineOwned(memberName1, vmgNamespace, "ownerMachineName1", mdName1), makeCAPIMachine("ownerMachineName1", vmgNamespace, nil),
					makeVSphereMachineOwned(memberName2, vmgNamespace, "ownerMachineName2", mdName2), makeCAPIMachine("ownerMachineName2", vmgNamespace, ptr.To(failureDomainA)),
				}
			}(),
			// Allowed because new members don't require placement
			wantErr:        false,
			expectedErrMsg: "",
		},
		{
			name:         "Block Patch if placement annotation is missing",
			targetMember: []vmoprv1.GroupMember{member(memberName1), member(memberName2)}, // vm-02 is new and requires placement
			vmgInput:     baseVMG.DeepCopy(),
			existingObjects: func() []runtime.Object {
				v := baseVMG.DeepCopy()
				conditions.Set(v, metav1.Condition{
					Type:   vmoprv1.ReadyConditionType,
					Status: metav1.ConditionTrue})
				v.Annotations = map[string]string{
					fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
				}
				v.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{{Members: []vmoprv1.GroupMember{
					{
						Name: memberName1,
						Kind: memberKind,
					},
				}}}
				// m-02 lacks FailureDomain and new Member vm-02 requires placement annotation but not exists
				return []runtime.Object{
					v,
					makeVSphereMachineOwned(memberName1, vmgNamespace, "ownerMachineName1", mdName1), makeCAPIMachine("ownerMachineName1", vmgNamespace, ptr.To(failureDomainA)),
					makeVSphereMachineOwned(memberName2, vmgNamespace, "ownerMachineName2", mdName2), makeCAPIMachineNoFailureDomain("ownerMachineName2", vmgNamespace),
				}
			}(),
			wantErr:        true,
			expectedErrMsg: fmt.Sprintf("waiting for placement annotation to add VMG member %s, temporarily blocking patch", memberName2),
		},
		{
			name:         "Allow Patch Machine since required placement annotation exists",
			targetMember: []vmoprv1.GroupMember{member(memberName1), member(memberName2)}, // vm-02 is new and requires placement
			vmgInput:     baseVMG.DeepCopy(),
			existingObjects: func() []runtime.Object {
				v := baseVMG.DeepCopy()
				conditions.Set(v, metav1.Condition{
					Type:   vmoprv1.ReadyConditionType,
					Status: metav1.ConditionTrue})
				v.Annotations = map[string]string{
					fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
					fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName2): zoneB,
				}
				v.Spec.BootOrder = []vmoprv1.VirtualMachineGroupBootOrderGroup{{Members: []vmoprv1.GroupMember{
					{
						Name: memberName1,
						Kind: memberKind,
					},
				}}}
				return []runtime.Object{
					v,
					makeVSphereMachineOwned(memberName1, vmgNamespace, "ownerMachineName1", mdName1), makeCAPIMachine("ownerMachineName1", vmgNamespace, nil),
					makeVSphereMachineOwned(memberName2, vmgNamespace, "ownerMachineName2", mdName2), makeCAPIMachineNoFailureDomain("ownerMachineName2", vmgNamespace),
				}
			}(),
			wantErr:        false,
			expectedErrMsg: "",
		},
	}

	for _, tt := range tests {
		// Looks odd, but need to reinitialize test variable
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			kubeClient := fake.NewClientBuilder().WithRuntimeObjects(tt.existingObjects...).Build()

			vmgInput := tt.vmgInput.DeepCopy()

			err := isCreateOrPatchAllowed(ctx, kubeClient, tt.targetMember, vmgInput)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedErrMsg))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestGetExpectedVSphereMachineCount(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())

	targetCluster := newTestCluster(clusterName, clusterNamespace)

	mdA := newMachineDeployment("md-a", clusterName, clusterNamespace, true, ptr.To(int32(3)))
	mdB := newMachineDeployment("md-b", clusterName, clusterNamespace, true, ptr.To(int32(5)))
	mdCNil := newMachineDeployment("md-c-nil", clusterName, clusterNamespace, false, nil)
	mdDZero := newMachineDeployment("md-d-zero", clusterName, clusterNamespace, true, ptr.To(int32(0)))
	// Create an MD for a different cluster (should be filtered)
	mdOtherCluster := newMachineDeployment("md-other", otherClusterName, clusterNamespace, true, ptr.To(int32(5)))

	tests := []struct {
		name           string
		initialObjects []client.Object
		expectedTotal  int32
		wantErr        bool
	}{
		{
			name:           "Sum of two MDs",
			initialObjects: []client.Object{mdA, mdB},
			expectedTotal:  8,
			wantErr:        false,
		},
		{
			name:           "Should get count when MDs include nil and zero replicas",
			initialObjects: []client.Object{mdA, mdB, mdCNil, mdDZero},
			expectedTotal:  8,
			wantErr:        false,
		},
		{
			name:           "Should filters out MDs from other clusters",
			initialObjects: []client.Object{mdA, mdB, mdOtherCluster},
			expectedTotal:  8,
			wantErr:        false,
		},
		{
			name:           "Should succeed when no MachineDeployments found",
			initialObjects: []client.Object{},
			expectedTotal:  0,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		// Looks odd, but need to reinitialize test variable
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.initialObjects...).Build()
			total, err := getExpectedVSphereMachineCount(ctx, fakeClient, targetCluster)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(total).To(Equal(tt.expectedTotal))
			}
		})
	}
}

func TestGetCurrentVSphereMachines(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(vmwarev1.AddToScheme(scheme)).To(Succeed())

	// VSphereMachine names are based on CAPI Machine names, but we use fake name here.
	vsmName1 := fmt.Sprintf("%s-%s", mdName1, "vsm-1")
	vsmName2 := fmt.Sprintf("%s-%s", mdName2, "vsm-2")
	vsm1 := newVSphereMachine(vsmName1, mdName1, false, false, nil)
	vsm2 := newVSphereMachine(vsmName2, mdName2, false, false, nil)
	vsmDeleting := newVSphereMachine("vsm-3", mdName1, false, true, nil) // Deleting
	vsmControlPlane := newVSphereMachine("vsm-cp", "not-md", true, false, nil)

	tests := []struct {
		name    string
		objects []client.Object
		want    int
	}{
		{
			name: "Should filtered out deleting VSphereMachines",
			objects: []client.Object{
				vsm1,
				vsm2,
				vsmDeleting,
				vsmControlPlane,
			},
			want: 2,
		},
		{
			name:    "Want no Error if no VSphereMachines found",
			objects: []client.Object{},
			want:    0,
		},
	}

	for _, tt := range tests {
		// Looks odd, but need to reinitialize test variable
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()
			got, err := getCurrentVSphereMachines(ctx, fakeClient, clusterNamespace, clusterName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(HaveLen(tt.want))

			// Check that the correct Machines are present
			if tt.want > 0 {
				names := make([]string, len(got))
				for i, vsm := range got {
					names[i] = vsm.Name
				}
				sort.Strings(names)
				g.Expect(names).To(Equal([]string{vsmName1, vsmName2}))
			}
		})
	}
}
func TestGenerateVirtualMachineGroupAnnotations(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(vmwarev1.AddToScheme(scheme)).To(Succeed())

	baseVMG := &vmoprv1.VirtualMachineGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        clusterName,
			Namespace:   clusterNamespace,
			Annotations: make(map[string]string),
		},
	}

	// VSphereMachines corresponding to the VMG members
	vsmName1 := fmt.Sprintf("%s-%s", mdName1, "vsm-1")
	vsmName2 := fmt.Sprintf("%s-%s", mdName2, "vsm-2")
	vsmNameSameMD := fmt.Sprintf("%s-%s", mdName1, "vsm-same-md")
	vsm1 := newVSphereMachine(vsmName1, mdName1, false, false, nil)
	vsm2 := newVSphereMachine(vsmName2, mdName2, false, false, nil)
	vsmSameMD := newVSphereMachine(vsmNameSameMD, mdName1, false, false, nil)
	vsmMissingLabel := newVSphereMachine("vsm-nolabel", mdName2, false, false, nil)
	vsmMissingLabel.Labels = nil // Explicitly remove labels for test case

	tests := []struct {
		name                 string
		vmg                  *vmoprv1.VirtualMachineGroup
		machineDeployments   []string
		initialClientObjects []client.Object
		expectedAnnotations  map[string]string
		wantErr              bool
	}{
		{
			name: "Deletes stale annotation for none-existed MD",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				// This MD (mdNameStale) is NOT in the machineDeployments list below.
				v.SetAnnotations(map[string]string{
					fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdNameStale): zoneA,
					"other/annotation": "keep-me",
				})
				v.Status = vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{},
				}
				return v
			}(),
			machineDeployments:   []string{mdName1},
			initialClientObjects: []client.Object{},
			expectedAnnotations: map[string]string{
				"other/annotation": "keep-me",
			},
			wantErr: false,
		},
		{
			name: "Skip if VSphereMachine Missing MachineDeployment Label",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				v.Status = vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus("vsm-nolabel", "VirtualMachine", true, true, zoneA),
					},
				}
				return v
			}(),
			machineDeployments:   []string{mdName1},
			initialClientObjects: []client.Object{vsmMissingLabel},
			expectedAnnotations:  map[string]string{},
			wantErr:              false,
		},
		{
			name: "Skip if VSphereMachine is Not Found in API",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				v.Status = vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus("non-existent-vm", "VirtualMachine", true, true, zoneA),
					},
				}
				return v
			}(),
			machineDeployments:   []string{mdName1},
			initialClientObjects: []client.Object{vsm1},
			expectedAnnotations:  map[string]string{},
			wantErr:              false,
		},
		{
			name: "Skip as placement already exists in VMG Annotations",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				v.Annotations = map[string]string{fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA}
				v.Status.Members = []vmoprv1.VirtualMachineGroupMemberStatus{
					newVMGMemberStatus(vsmName1, "VirtualMachine", true, true, zoneB),
				}
				return v
			}(),
			machineDeployments:   []string{mdName1},
			initialClientObjects: []client.Object{vsm1},
			// Should retain existing zone-a
			expectedAnnotations: map[string]string{
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
			},
			wantErr: false,
		},
		{
			name: "Skip if placement is nil",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				v.Status = vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vsmName1, "VirtualMachine", true, false, zoneA),
					},
				}
				return v
			}(),
			machineDeployments:   []string{mdName1},
			initialClientObjects: []client.Object{vsm1},
			expectedAnnotations:  map[string]string{},
			wantErr:              false,
		},
		{
			name: "Skip if Zone is empty string",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				v.Status = vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vsmName1, "VirtualMachine", true, true, ""),
					},
				}
				return v
			}(),
			machineDeployments:   []string{mdName1},
			initialClientObjects: []client.Object{vsm1},
			expectedAnnotations:  map[string]string{},
			wantErr:              false,
		},
		{
			name: "Cleans stale and adds new annotations",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				// Stale annotation to be deleted
				v.SetAnnotations(map[string]string{
					fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdNameStale): zoneB,
				})
				v.Status = vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vsmName1, "VirtualMachine", true, true, zoneA),
					},
				}
				return v
			}(),
			machineDeployments:   []string{mdName1},
			initialClientObjects: []client.Object{vsm1},
			expectedAnnotations: map[string]string{
				// Stale annotation for mdNameStale should be gone
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
			},
			wantErr: false,
		},
		{
			name: "Placement found for two distinct MDs",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				v.Status = vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vsmName1, "VirtualMachine", true, true, zoneA),
						newVMGMemberStatus(vsmName2, "VirtualMachine", true, true, zoneB),
					},
				}
				return v
			}(),
			machineDeployments:   []string{mdName1, mdName2},
			initialClientObjects: []client.Object{vsm1, vsm2},
			expectedAnnotations: map[string]string{
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName2): zoneB,
			},
			wantErr: false,
		},
		{
			name: "Placement found for MD1 but not MD2 since PlacementReady is not true",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				v.Status = vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vsmName1, "VirtualMachine", true, true, zoneA),
						newVMGMemberStatus(vsmName2, "VirtualMachine", false, false, ""),
					},
				}
				return v
			}(),
			machineDeployments:   []string{mdName1, mdName2},
			initialClientObjects: []client.Object{vsm1, vsm2},
			expectedAnnotations: map[string]string{
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
			},
			wantErr: false,
		},
		{
			name: "Keep the original annotation if VMs for the same MD placed to new zone",
			vmg: func() *vmoprv1.VirtualMachineGroup {
				v := baseVMG.DeepCopy()
				v.Status = vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vsmName1, "VirtualMachine", true, true, zoneA),
						newVMGMemberStatus(vsmNameSameMD, "VirtualMachine", true, true, zoneB),
					},
				}
				return v
			}(),
			machineDeployments:   []string{mdName1},
			initialClientObjects: []client.Object{vsm1, vsmSameMD},
			expectedAnnotations: map[string]string{
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		// Looks odd, but need to reinitialize test variable
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.initialClientObjects...).Build()
			err := generateVirtualMachineGroupAnnotations(ctx, fakeClient, tt.vmg, tt.machineDeployments)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(tt.vmg.Annotations).To(Equal(tt.expectedAnnotations))
			}
		})
	}
}

func TestVirtualMachineGroupReconciler_ReconcileFlow(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(vmwarev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(vmoprv1.AddToScheme(scheme)).To(Succeed())

	// Initial objects for the successful VMG creation path
	cluster := newCluster(clusterName, clusterNamespace, true, 1, 1)
	vsm1 := newVSphereMachine("vsm-1", mdName1, false, false, nil)
	vsm2 := newVSphereMachine("vsm-2", mdName2, false, false, nil)
	// VSM 3 is in deletion (will be filtered out)
	vsm3 := newVSphereMachine("vsm-3", mdName1, false, true, nil)
	md1 := newMachineDeployment(mdName1, clusterName, clusterNamespace, true, ptr.To(int32(1)))
	md2 := newMachineDeployment(mdName2, clusterName, clusterNamespace, true, ptr.To(int32(1)))
	machine1 := newMachine("machine-vsm-1", mdName1, "")
	machine2 := newMachine("machine-vsm-2", mdName2, "")

	// VMG Ready state for Day-2 checks
	readyVMGMembers := []vmoprv1.GroupMember{
		{Name: vsm1.Name, Kind: memberKind},
		{Name: vsm2.Name, Kind: memberKind},
	}

	// VMG Ready but haven't added placement annotation
	vmgReady := newVMG(clusterName, clusterNamespace, readyVMGMembers, true, nil)

	// VMG Ready and have placement annotation for Day-2 checks
	vmgPlaced := newVMG(clusterName, clusterNamespace, readyVMGMembers, true, map[string]string{
		fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
	})

	tests := []struct {
		name                 string
		initialObjects       []client.Object
		expectedResult       reconcile.Result
		expectVMGExists      bool
		expectedMembersCount int
		expectedAnnotations  map[string]string
		expectedErrorMsg     string
	}{
		// VMG Create
		{
			name:                 "Should Exit if Cluster Not Found",
			initialObjects:       []client.Object{},
			expectedResult:       reconcile.Result{},
			expectVMGExists:      false,
			expectedMembersCount: 0,
		},
		{
			name: "Should Exit if Cluster Deletion Timestamp Set",
			initialObjects: []client.Object{
				func() client.Object {
					c := cluster.DeepCopy()
					c.Finalizers = []string{"test.finalizer.cluster"}
					c.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					return c
				}(),
			},
			expectedResult:  reconcile.Result{},
			expectVMGExists: false,
		},
		{
			name: "Should Requeue if ControlPlane Not Initialized",
			initialObjects: []client.Object{
				newCluster(clusterName, clusterNamespace, false, 1, 0),
			},
			expectedResult:  reconcile.Result{},
			expectVMGExists: false,
		},
		{
			name:                 "Should Requeue if VMG Not Found and Machines not ready",
			initialObjects:       []client.Object{cluster.DeepCopy(), md1.DeepCopy(), md2.DeepCopy()},
			expectedResult:       reconcile.Result{},
			expectVMGExists:      false,
			expectedMembersCount: 0,
		},
		{
			name: "Should Succeed to create VMG",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				md1.DeepCopy(),
				vsm1.DeepCopy(),
				md2.DeepCopy(),
				vsm1.DeepCopy(),
			},
			expectedResult:       reconcile.Result{},
			expectVMGExists:      true,
			expectedMembersCount: 2,
		},
		// VMG Update: Member Scale Down
		{
			name: "Should Succeed to update VMG if removing member even placement is not ready",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				newMachineDeployment(mdName1, clusterName, clusterNamespace, true, ptr.To(int32(1))),
				// VSM3 is in deletion
				vsm1.DeepCopy(),
				vsm2.DeepCopy(),
				vsm3.DeepCopy(),
				// Existing VMG has vsm-1, vsm-2 and vsm-3, simulating scale-down state
				newVMG(clusterName, clusterNamespace, []vmoprv1.GroupMember{
					{Name: "vsm-1", Kind: memberKind},
					{Name: "vsm-2", Kind: memberKind},
					{Name: "vsm-3", Kind: memberKind},
				}, false, nil),
			},
			expectedResult:       reconcile.Result{},
			expectVMGExists:      true,
			expectedMembersCount: 2,
		},
		// VMG Placement Annotation
		{
			name: "Should add Placement annotation after Placement ready",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				md1.DeepCopy(),
				vsm1.DeepCopy(),
				machine1.DeepCopy(),
				md2.DeepCopy(),
				vsm2.DeepCopy(),
				machine2.DeepCopy(),
				vmgReady.DeepCopy(),
=======
			name:    "No op if nothing changes during initial placement",
			cluster: clusterInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterInitialized.Name, "", 2),
				*createMD("md2", clusterInitialized.Name, "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m3", clusterInitialized.Name, "md2", "zone1"),
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
							},
						},
					},
				},
			},
			wantResult: ctrl.Result{},
			wantVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
							},
						},
					},
				},
>>>>>>> 9409e432 (POC AAF)
			},
			expectedResult:       reconcile.Result{},
			expectVMGExists:      true,
			expectedMembersCount: 2,
			expectedAnnotations: map[string]string{
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName2): zoneB,
			},
		},
		{
			name: "Should cleanup stale VMG annotation for deleted MD",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				// MD1,MD2 is active
				md1.DeepCopy(),
				vsm1.DeepCopy(),
				machine1.DeepCopy(),
				md2.DeepCopy(),
				vsm2.DeepCopy(),
				machine2.DeepCopy(),
				// VMG has annotations and a stale one for md-old
				newVMG(clusterName, clusterNamespace, readyVMGMembers, true, map[string]string{
					fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
					fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName2): zoneB,
					fmt.Sprintf("%s/md-old", ZoneAnnotationPrefix):      "zone-c",
				}),
			},
			expectedResult:       reconcile.Result{},
			expectVMGExists:      true,
			expectedMembersCount: 1,
			expectedAnnotations: map[string]string{
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName1): zoneA,
				fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, mdName2): zoneB,
			},
		},
		{
			name: "Should block adding member if VMG not Ready (waiting for initial placement)",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				// MD1 spec is 2 (scale-up target)
				newMachineDeployment(mdName1, clusterName, clusterNamespace, true, ptr.To(int32(2))),
				// Only 1 VSM currently exists (vsm-1) for MD1
				vsm1.DeepCopy(),
				machine1.DeepCopy(),
				vsm2.DeepCopy(),
				machine2.DeepCopy(),
				newVSphereMachine("vsm-new", mdName1, false, false, nil),
				// VMG exists but is NOT Ready (simulating placement in progress)
				newVMG(clusterName, clusterNamespace, readyVMGMembers, false, nil),
			},
			expectedResult:  reconcile.Result{},
			expectVMGExists: true,
			// Expect an error because isCreateOrPatchAllowed blocks
			expectedErrorMsg:     "waiting for VirtualMachineGroup",
			expectedMembersCount: 2,
		},
		{
			name: "Should block adding member if VMG Ready but MD annotation is missing",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				newMachineDeployment(mdName1, clusterName, clusterNamespace, true, ptr.To(int32(2))),
				// Only vsm-1 currently exists for MD1
				vsm1.DeepCopy(),
				machine1.DeepCopy(),
				vsm2.DeepCopy(),
				machine2.DeepCopy(),
				// vsm-new is the new member requiring placement
				newVSphereMachine("vsm-new", mdName1, false, false, nil),
				newMachine("machine-vsm-new", mdName1, ""),
				// VMG is Ready, but has no placement annotations
				vmgReady.DeepCopy(),
			},
			expectedResult:  reconcile.Result{},
			expectVMGExists: true,
			// Expected error from isCreateOrPatchAllowed: waiting for placement annotation
			expectedErrorMsg:     fmt.Sprintf("waiting for placement annotation %s/%s", ZoneAnnotationPrefix, mdName1),
			expectedMembersCount: 2,
		},
		{
			name: "Should succeed adding member when VMG Ready AND placement annotation exists",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				newMachineDeployment(mdName1, clusterName, clusterNamespace, true, ptr.To(int32(2))),
				vsm1.DeepCopy(),
				machine1.DeepCopy(),
				vsm2.DeepCopy(),
				machine2.DeepCopy(),
				newVSphereMachine("vsm-new", mdName1, false, false, nil),
				newMachine("machine-vsm-new", mdName1, ""),
				// VMG is Placed (Ready + Annotation)
				vmgPlaced.DeepCopy(),
			},
			expectedResult:       reconcile.Result{},
			expectVMGExists:      true,
			expectedMembersCount: 2,
		},
		{
			name: "Should succeed adding member if new member has FailureDomain set",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				newMachineDeployment("md-new", clusterName, clusterNamespace, true, ptr.To(int32(2))),
				vsm1.DeepCopy(),
				machine1.DeepCopy(),
				vsm2.DeepCopy(),
				machine2.DeepCopy(),
				newVSphereMachine("vsm-new", "md-new", false, false, nil),
				// New machine has a FailureDomain set, which bypasses the VMG placement annotation check
				newMachine("machine-vsm-new", "md-new", "zone-new"),
				// VMG is Ready, but has no placement annotation for new machine deployment (this should be bypassed)
				vmgReady.DeepCopy(),
			},
			expectedResult:       reconcile.Result{},
			expectVMGExists:      true,
			expectedMembersCount: 2, // Scale-up should succeed due to FailureDomain bypass
		},
<<<<<<< HEAD
	}

	for _, tt := range tests {
		// Looks odd, but need to reinitialize test variable
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.initialObjects...).Build()
			reconciler := &VirtualMachineGroupReconciler{
				Client:   fakeClient,
				Recorder: record.NewFakeRecorder(1),
			}
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: clusterNamespace}}

			result, err := reconciler.Reconcile(ctx, req)

			if tt.expectedErrorMsg != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedErrorMsg))
				return
			}

			g.Expect(err).NotTo(HaveOccurred(), "Reconcile should not return an error")
			g.Expect(result).To(Equal(tt.expectedResult))

			vmg := &vmoprv1.VirtualMachineGroup{}
			vmgKey := types.NamespacedName{Name: clusterName, Namespace: clusterNamespace}
			err = fakeClient.Get(ctx, vmgKey, vmg)

			if tt.expectVMGExists {
				g.Expect(err).NotTo(HaveOccurred(), "VMG should exist")
				// Check that the core fields were set by the MutateFn
				g.Expect(vmg.Labels).To(HaveKeyWithValue(clusterv1.ClusterNameLabel, clusterName))
				// Check member count
				g.Expect(vmg.Spec.BootOrder).To(HaveLen(tt.expectedMembersCount), "VMG members count mismatch")
				// Check annotations
				if tt.expectedAnnotations != nil {
					g.Expect(vmg.Annotations).To(Equal(tt.expectedAnnotations))
				}
				// VMG members should match the VSphereMachine name
				g.Expect(vmg.Spec.BootOrder[0].Members[0].Name).To(Equal("vsm-1"))
			} else {
				// Check VMG does not exist if expected
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		})
	}
}

// Helper function to create a basic Cluster object.
func newCluster(name, namespace string, initialized bool, replicasMD1, replicasMD2 int32) *clusterv1.Cluster {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: name},
=======
		{
			name:    "Only new VSphereMachines with an explicit placement are added during initial placement",
			cluster: clusterInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterInitialized.Name, "", 3),      // scaled up
				*createMD("md2", clusterInitialized.Name, "zone1", 2), // scaled up
				*createMD("md3", clusterInitialized.Name, "", 1),      // new
				*createMD("md4", clusterInitialized.Name, "zone2", 1), // new
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m4", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()), // new
				*createVSphereMachine("m3", clusterInitialized.Name, "md2", "zone1"),
				*createVSphereMachine("m5", clusterInitialized.Name, "md2", "zone1"),                        // new
				*createVSphereMachine("m6", clusterInitialized.Name, "md3", "", withCustomNamingStrategy()), // new
				*createVSphereMachine("m7", clusterInitialized.Name, "md4", "zone3"),                        // new
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
							},
						},
					},
				},
			},
			wantResult: ctrl.Result{},
			wantVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md4"): "zone2", // failureDomain for md4 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								// "m4-vm" not added, placement decision for md1 not yet completed
								{Name: "m3", Kind: "VirtualMachine"},
								{Name: "m5", Kind: "VirtualMachine"}, // added, failureDomain for md2 is explicitly set by the user
								// "m6-vm" not added, placement decision for md3 not yet completed
								{Name: "m7", Kind: "VirtualMachine"}, // added, failureDomain for md4 is explicitly set by the user
							},
						},
					},
				},
			},
>>>>>>> 9409e432 (POC AAF)
		},
		{
			name:    "VSphereMachines are removed during initial placement",
			cluster: clusterInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterInitialized.Name, "", 3),      // scaled down
				*createMD("md2", clusterInitialized.Name, "zone1", 2), // scaled down
				// md3 deleted
				// md4 deleted
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				// m4 deleted
				*createVSphereMachine("m3", clusterInitialized.Name, "md2", "zone1"),
				// m5 deleted
				// m6 deleted
				// m7 deleted
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md4"): "zone2", // failureDomain for md4 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								// "m4-vm" not added, placement decision for md1 not yet completed
								{Name: "m3", Kind: "VirtualMachine"},
								{Name: "m5", Kind: "VirtualMachine"}, // added, failureDomain for md2 is explicitly set by the user
								// "m6-vm" not added, placement decision for md3 not yet completed
								{Name: "m7", Kind: "VirtualMachine"}, // added, failureDomain for md4 is explicitly set by the user
							},
						},
					},
				},
			},
			wantResult: ctrl.Result{},
			wantVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						// md4 deleted
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								// "m4-vm" deleted (it was never added)
								{Name: "m3", Kind: "VirtualMachine"},
								// "m5" deleted
								// "m6" deleted
								// "m7" deleted
							},
						},
					},
				},
			},
		},

		// After initial placement
		{
			name:    "No op if nothing changes after initial placement",
			cluster: clusterInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterInitialized.Name, "", 2),
				*createMD("md2", clusterInitialized.Name, "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m3", clusterInitialized.Name, "md2", "zone1"),
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
							},
						},
					},
				},
				// Not setting status for sake of simplicity (in a real VMG, after the placement decision status should have members)
			},
			wantResult: ctrl.Result{},
			wantVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
							},
						},
					},
				},
			},
		},
		{
			name:    "New VSphereMachines are added after initial placement",
			cluster: clusterInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterInitialized.Name, "", 3),      // scaled up
				*createMD("md2", clusterInitialized.Name, "zone1", 2), // scaled up
				*createMD("md3", clusterInitialized.Name, "zone2", 1), // new
				// Adding a new MD without explicit placement is not supported at this stage
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m4", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()), // new
				*createVSphereMachine("m3", clusterInitialized.Name, "md2", "zone1"),
				*createVSphereMachine("m5", clusterInitialized.Name, "md2", "zone1"), // new
				*createVSphereMachine("m6", clusterInitialized.Name, "md3", "zone2"), // new
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
							},
						},
					},
				},
				// Not setting status for sake of simplicity (in a real VMG, after the placement decision status should have members)
			},
			wantResult: ctrl.Result{},
			wantVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md3"): "zone2", // failureDomain for md3 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
								{Name: "m4-vm", Kind: "VirtualMachine"}, // added, failureDomain for md1 set by initial placement
								{Name: "m5", Kind: "VirtualMachine"},    // added, failureDomain for md2 is explicitly set by the user
								{Name: "m6", Kind: "VirtualMachine"},    // added, failureDomain for md3 is explicitly set by the user
							},
						},
					},
				},
			},
		},
		{
			name:    "VSphereMachines are removed during initial placement",
			cluster: clusterInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterInitialized.Name, "", 3),      // scaled down
				*createMD("md2", clusterInitialized.Name, "zone1", 2), // scaled down
				// md3 deleted
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				*createVSphereMachine("m2", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
				// m4 deleted
				*createVSphereMachine("m3", clusterInitialized.Name, "md2", "zone1"),
				// m5 deleted
				// m5 deleted
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md3"): "zone2", // failureDomain for md3 is explicitly set by the user
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
								{Name: "m4-vm", Kind: "VirtualMachine"}, // added, failureDomain for md1 set by initial placement
								{Name: "m5", Kind: "VirtualMachine"},    // added, failureDomain for md2 is explicitly set by the user
								{Name: "m6", Kind: "VirtualMachine"},    // added, failureDomain for md3 is explicitly set by the user
							},
						},
					},
				},
				// Not setting status for sake of simplicity (in a real VMG, after the placement decision status should have members)
			},
			wantResult: ctrl.Result{},
			wantVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: clusterInitialized.Namespace,
					Name:      clusterInitialized.Name,
					UID:       types.UID("uid"),
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						// md3 deleted
					},
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
								// m4-vm deleted
								// m5 deleted
								// m6 deleted
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			objects := []client.Object{tt.cluster}
			if tt.existingVMG != nil {
				objects = append(objects, tt.existingVMG)
			}
			for _, md := range tt.mds {
				objects = append(objects, &md)
			}
			for _, vSphereMachine := range tt.vSphereMachines {
				objects = append(objects, &vSphereMachine)
			}

			c := fake.NewClientBuilder().WithObjects(objects...).Build()
			r := &VirtualMachineGroupReconciler{
				Client: c,
			}
			got, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: tt.cluster.Namespace, Name: tt.cluster.Name}})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(got).To(Equal(tt.wantResult))

			vmg := &vmoprv1.VirtualMachineGroup{}
			err = r.Client.Get(ctx, client.ObjectKeyFromObject(tt.cluster), vmg)

			if tt.wantVMG == nil {
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(vmg.Annotations).To(Equal(tt.wantVMG.Annotations))
			g.Expect(vmg.Spec.BootOrder).To(Equal(tt.wantVMG.Spec.BootOrder))
		})
	}
}

<<<<<<< HEAD
// Helper function to create a VSphereMachine (worker, owned by a CAPI Machine).
func newVSphereMachine(name, mdName string, isCP, deleted bool, namingStrategy *vmwarev1.VirtualMachineNamingStrategy) *vmwarev1.VSphereMachine {
	vsm := &vmwarev1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: clusterNamespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: vmwarev1.VSphereMachineSpec{
			NamingStrategy: namingStrategy,
		},
	}
	if !isCP {
		vsm.Labels[clusterv1.MachineDeploymentNameLabel] = mdName
	} else {
		vsm.Labels[clusterv1.MachineControlPlaneLabel] = "true"
	}
	if deleted {
		vsm.Finalizers = []string{"test.finalizer.0"}
		vsm.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}

	vsm.OwnerReferences = []metav1.OwnerReference{
		{
			Kind: "Machine",
			Name: fmt.Sprintf("machine-%s", name),
		},
	}

	return vsm
}
=======
type machineDeploymentOption func(md *clusterv1.MachineDeployment)
>>>>>>> 9409e432 (POC AAF)

func createMD(name, cluster, failureDomain string, replicas int32, options ...machineDeploymentOption) *clusterv1.MachineDeployment {
	md := &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      name,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster,
			},
		},
		Spec: clusterv1.MachineDeploymentSpec{
			Template: clusterv1.MachineTemplateSpec{Spec: clusterv1.MachineSpec{FailureDomain: failureDomain}},
			Replicas: &replicas,
		},
	}
	for _, opt := range options {
		opt(md)
	}
	return md
}

type vSphereMachineOption func(m *vmwarev1.VSphereMachine)

func withCustomNamingStrategy() func(m *vmwarev1.VSphereMachine) {
	return func(m *vmwarev1.VSphereMachine) {
		m.Spec.NamingStrategy = &vmwarev1.VirtualMachineNamingStrategy{
			Template: ptr.To[string]("{{ .machine.name }}-vm"),
		}
	}
}

<<<<<<< HEAD
// Helper to create a new CAPI Machine.
func newMachine(name, mdName, fd string) *clusterv1.Machine {
	m := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: clusterNamespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:           clusterName,
				clusterv1.MachineDeploymentNameLabel: mdName,
			},
		},
		Spec: clusterv1.MachineSpec{
			FailureDomain: fd,
		},
	}
	// Machine owner reference for VSphereMachine
	m.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: vmwarev1.GroupVersion.String(),
			Kind:       "VSphereMachine",
			Name:       strings.TrimPrefix(name, "machine-"), // VSphereMachine Name matches VM Name logic
		},
	}
	return m
}

// Helper to create a new VMG with a list of members and conditions.
func newVMG(name, ns string, members []vmoprv1.GroupMember, ready bool, annotations map[string]string) *vmoprv1.VirtualMachineGroup {
	v := &vmoprv1.VirtualMachineGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Labels:      map[string]string{clusterv1.ClusterNameLabel: name},
			Annotations: annotations,
			Finalizers:  []string{"vmg.test.finalizer"},
		},
		Spec: vmoprv1.VirtualMachineGroupSpec{
			BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
				{Members: members},
			},
		},
	}
	if ready {
		conditions.Set(v, metav1.Condition{
			Type:   vmoprv1.ReadyConditionType,
			Status: metav1.ConditionTrue,
		})
		v.Status = vmoprv1.VirtualMachineGroupStatus{
			Members: []vmoprv1.VirtualMachineGroupMemberStatus{
				newVMGMemberStatus("vsm-1", "VirtualMachine", true, true, zoneA),
				newVMGMemberStatus("vsm-2", "VirtualMachine", true, true, zoneB),
			},
		}
	}
	return v
}
=======
func createVSphereMachine(name, cluster, md, failureDomain string, options ...vSphereMachineOption) *vmwarev1.VSphereMachine {
	m := &vmwarev1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      name,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:           cluster,
				clusterv1.MachineDeploymentNameLabel: md,
			},
		},
		Spec: vmwarev1.VSphereMachineSpec{
			FailureDomain: &failureDomain,
		},
	}
	for _, opt := range options {
		opt(m)
	}
	return m
}
>>>>>>> 9409e432 (POC AAF)
