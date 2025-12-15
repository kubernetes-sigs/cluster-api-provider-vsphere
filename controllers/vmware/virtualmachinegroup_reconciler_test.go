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
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/vmoperator"
)

func Test_shouldCreateVirtualMachineGroup(t *testing.T) {
	tests := []struct {
		name            string
		mds             []clusterv1.MachineDeployment
		vSphereMachines []vmwarev1.VSphereMachine
		want            bool
	}{
		{
			name: "Should not create a VMG if the expected VSphereMachines do not exist yet",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "", 2),
				*createMD("md2", "test-cluster", "", 1),
				*createMD("md3", "test-cluster", "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", "test-cluster", "md1", ""),
				*createVSphereMachine("m2", "test-cluster", "md1", "", func(vm *vmwarev1.VSphereMachine) {
					vm.DeletionTimestamp = ptr.To(metav1.Now())
				}),
				*createVSphereMachine("m3", "test-cluster", "md2", ""),
				*createVSphereMachine("m4", "test-cluster", "md3", "zone1"),
			},
			want: false, // tot replicas = 4, 3 VSphereMachine exist, 1 VSphereMachine in deleting.
		},
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := shouldCreateVirtualMachineGroup(ctx, tt.mds, tt.vSphereMachines)
			g.Expect(got).To(Equal(tt.want))
		})
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
			name: "MachineDeployment mapping should use spec.FailureDomain",
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
			name: "MachineDeployment mapping should use spec.FailureDomain (latest value must be used)",
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", "test-cluster", "zone2", 1), // failure domain explicitly set
			},
			existingVMG: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone1", // Previously md1 was assigned to zone1
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone1", // Placement decision for md1 already reported into annotation
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
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: cluster.Name,
					},
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       cluster.Name,
							UID:        cluster.UID,
							Controller: ptr.To(true),
						},
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
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: cluster.Name,
						"other-label":              "foo-bar"},
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
						"other-annotation": "foo-bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       cluster.Name,
							UID:        cluster.UID,
							Controller: ptr.To(true),
						},
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
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: cluster.Name,
						"other-label":              "foo-bar",
					},
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md4"): "zone2", // failureDomain for md4 is explicitly set by the user
						"other-annotation": "foo-bar", // Other annotation without ZoneAnnotationPrefix should be preserved
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       cluster.Name,
							UID:        cluster.UID,
							Controller: ptr.To(true),
						},
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
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: cluster.Name,
					},
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone5", // failureDomain for md2 set by initial placement
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       cluster.Name,
							UID:        cluster.UID,
							Controller: ptr.To(true),
						},
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
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: cluster.Name,
					},
					Annotations: map[string]string{
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						// annotation for md2 deleted, md2 does not exist anymore
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md3"): "zone1", // failureDomain for md3 is explicitly set by the user
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md4"): "zone2", // failureDomain for md4 is explicitly set by the user
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       cluster.Name,
							UID:        cluster.UID,
							Controller: ptr.To(true),
						},
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
		cluster         *clusterv1.Cluster
		mds             []clusterv1.MachineDeployment
		vSphereMachines []vmwarev1.VSphereMachine
		existingVMG     *vmoprv1.VirtualMachineGroup
		wantResult      ctrl.Result
		wantVMG         *vmoprv1.VirtualMachineGroup
	}{
		// Before initial placement
		{
			name:            "VirtualMachineGroup should not be created when the cluster is not yet initialized",
			cluster:         clusterNotYetInitialized,
			mds:             nil,
			vSphereMachines: nil,
			existingVMG:     nil,
			wantResult:      ctrl.Result{},
			wantVMG:         nil,
		},
		{
			name:    "VirtualMachineGroup should not be created when waiting for vSphereMachines to exist",
			cluster: clusterInitialized,
			mds: []clusterv1.MachineDeployment{
				*createMD("md1", clusterInitialized.Name, "", 1),
				*createMD("md2", clusterInitialized.Name, "zone1", 1),
			},
			vSphereMachines: []vmwarev1.VSphereMachine{
				*createVSphereMachine("m1", clusterInitialized.Name, "md1", "", withCustomNamingStrategy()),
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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

		// During initial placement
		{
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md4"): "zone2", // failureDomain for md4 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md4"): "zone2", // failureDomain for md4 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						// md4 deleted
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md3"): "zone2", // failureDomain for md3 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
			name:    "VSphereMachines are removed after initial placement",
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md3"): "zone2", // failureDomain for md3 is explicitly set by the user
					},
					// Not setting labels and ownerReferences for sake of simplicity
				},
				Spec: vmoprv1.VirtualMachineGroupSpec{
					BootOrder: []vmoprv1.VirtualMachineGroupBootOrderGroup{
						{
							Members: []vmoprv1.GroupMember{
								{Name: "m1-vm", Kind: "VirtualMachine"},
								{Name: "m2-vm", Kind: "VirtualMachine"},
								{Name: "m3", Kind: "VirtualMachine"},
								{Name: "m4-vm", Kind: "VirtualMachine"},
								{Name: "m5", Kind: "VirtualMachine"},
								{Name: "m6", Kind: "VirtualMachine"},
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
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md1"): "zone4", // failureDomain for md1 set by initial placement
						fmt.Sprintf("%s/%s", vmoperator.ZoneAnnotationPrefix, "md2"): "zone1", // failureDomain for md2 is explicitly set by the user
						// md3 deleted
					},
					// Not setting labels and ownerReferences for sake of simplicity
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
			g.Expect(vmg.Labels).To(HaveKeyWithValue(clusterv1.ClusterNameLabel, tt.cluster.Name))
			g.Expect(vmg.OwnerReferences).To(ContainElement(metav1.OwnerReference{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
				Name:       tt.cluster.Name,
				UID:        tt.cluster.UID,
				Controller: ptr.To(true),
			}))
			g.Expect(vmg.Annotations).To(Equal(tt.wantVMG.Annotations))
			g.Expect(vmg.Spec.BootOrder).To(Equal(tt.wantVMG.Spec.BootOrder))
		})
	}
}

type machineDeploymentOption func(md *clusterv1.MachineDeployment)

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
