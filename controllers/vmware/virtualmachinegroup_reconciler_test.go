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
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

const (
	clusterName      = "test-cluster"
	otherClusterName = "other-cluster"
	clusterNamespace = "test-ns"
	mdName1          = "md-worker-a"
	mdName2          = "md-worker-b"
	zoneA            = "zone-a"
	zoneB            = "zone-b"
)

func TestGetExpectedVSphereMachines(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	targetCluster := newTestCluster(clusterName, clusterNamespace)

	mdA := newMachineDeployment("md-a", clusterName, clusterNamespace, ptr.To(int32(3)))
	mdB := newMachineDeployment("md-b", clusterName, clusterNamespace, ptr.To(int32(5)))
	mdCNil := newMachineDeployment("md-c-nil", clusterName, clusterNamespace, nil)
	mdDZero := newMachineDeployment("md-d-zero", clusterName, clusterNamespace, ptr.To(int32(0)))
	// Create an MD for a different cluster (should be filtered)
	mdOtherCluster := newMachineDeployment("md-other", otherClusterName, clusterNamespace, ptr.To(int32(5)))

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
			name:           "Should succeed when MDs include nil and zero replicas",
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
		t.Run(tt.name, func(_ *testing.T) {
			scheme := runtime.NewScheme()
			g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.initialObjects...).Build()
			total, err := getExpectedVSphereMachines(ctx, fakeClient, targetCluster)
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
	vsm1 := newVSphereMachine("vsm-1", mdName1, false, false, nil)
	vsm2 := newVSphereMachine("vsm-2", mdName2, false, false, nil)
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
		t.Run(tt.name, func(_ *testing.T) {
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
				g.Expect(names).To(Equal([]string{"vsm-1", "vsm-2"}))
			}
		})
	}
}

func TestGenerateVMGPlacementAnnotations(t *testing.T) {
	g := NewWithT(t)

	// Define object names for members
	vmName1 := fmt.Sprintf("%s-%s-vm-1", clusterName, mdName1)
	vmName2 := fmt.Sprintf("%s-%s-vm-2", clusterName, mdName2)
	vmNameUnplaced := fmt.Sprintf("%s-%s-vm-unplaced", clusterName, mdName1)
	vmNameWrongKind := "not-a-vm"

	tests := []struct {
		name               string
		vmg                *vmoprv1.VirtualMachineGroup
		machineDeployments []string
		wantAnnotations    map[string]string
		wantErr            bool
	}{
		{
			name: "Should get placement annotation when two placed VMs for two MDs",
			vmg: &vmoprv1.VirtualMachineGroup{
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						// Placed member for MD1 in Zone A
						newVMGMemberStatus(vmName1, "VirtualMachine", true, zoneA),
						// Placed member for MD2 in Zone B
						newVMGMemberStatus(vmName2, "VirtualMachine", true, zoneB),
					},
				},
			},
			machineDeployments: []string{clusterName + "-" + mdName1, clusterName + "-" + mdName2},
			wantAnnotations: map[string]string{
				fmt.Sprintf("zone.cluster.x-k8s.io/%s", clusterName+"-"+mdName1): zoneA,
				fmt.Sprintf("zone.cluster.x-k8s.io/%s", clusterName+"-"+mdName2): zoneB,
			},
			wantErr: false,
		},
		{
			name: "No placement annotation when VM PlacementReady is false)",
			vmg: &vmoprv1.VirtualMachineGroup{
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vmNameUnplaced, "VirtualMachine", false, ""),
					},
				},
			},
			machineDeployments: []string{clusterName + "-" + mdName1},
			wantAnnotations:    map[string]string{},
			wantErr:            false,
		},
		{
			name: "No placement annotation when PlacementReady but missing Zone info",
			vmg: &vmoprv1.VirtualMachineGroup{
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vmName1, "VirtualMachine", true, ""),
					},
				},
			},
			machineDeployments: []string{clusterName + "-" + mdName1},
			wantAnnotations:    map[string]string{},
			wantErr:            false,
		},
		{
			name: "Should keep placement annotation when first placement decision is found",
			vmg: &vmoprv1.VirtualMachineGroup{
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						// First VM sets the placement
						newVMGMemberStatus(vmName1, "VirtualMachine", true, zoneA),
						// Second VM is ignored
						newVMGMemberStatus(vmName1, "VirtualMachine", true, zoneB),
					},
				},
			},
			machineDeployments: []string{clusterName + "-" + mdName1},
			wantAnnotations: map[string]string{
				fmt.Sprintf("zone.cluster.x-k8s.io/%s", clusterName+"-"+mdName1): zoneA,
			},
			wantErr: false,
		},
		{
			name: "Should return Error if Member Kind is not VirtualMachine",
			vmg: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace},
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vmNameWrongKind, "VirtualMachineGroup", true, zoneA),
					},
				},
			},
			machineDeployments: []string{clusterName + "-" + mdName1},
			wantAnnotations:    nil,
			wantErr:            true,
		},
	}

	for _, tt := range tests {
		// Looks odd, but need to reinitialize test variable
		tt := tt
		t.Run(tt.name, func(_ *testing.T) {
			ctx := ctrl.LoggerInto(context.Background(), ctrl.LoggerFrom(context.Background()))

			got, err := GenerateVMGPlacementAnnotations(ctx, tt.vmg, tt.machineDeployments)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(got).To(Equal(tt.wantAnnotations))
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

	// Initial objects for the successful VMG creation path (Expected: 1, Current: 1)
	cluster := newCluster(clusterName, clusterNamespace, true, 1, 0)
	vsm1 := newVSphereMachine("vsm-1", mdName1, false, false, nil)
	md1 := newMachineDeployment(mdName1, clusterName, clusterNamespace, ptr.To(int32(1)))

	tests := []struct {
		name           string
		initialObjects []client.Object
		expectedResult reconcile.Result
		checkVMGExists bool
	}{
		{
			name:           "Should Exit if Cluster Not Found",
			initialObjects: []client.Object{},
			expectedResult: reconcile.Result{},
			checkVMGExists: false,
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
			expectedResult: reconcile.Result{},
			checkVMGExists: false,
		},
		{
			name: "Should Requeue if ControlPlane Not Initialized",
			initialObjects: []client.Object{
				newCluster(clusterName, clusterNamespace, false, 1, 0),
			},
			expectedResult: reconcile.Result{RequeueAfter: reconciliationDelay},
			checkVMGExists: false,
		},
		{
			name: "Should Requeue if VMG Not Found",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				md1.DeepCopy(),
			},
			expectedResult: reconcile.Result{RequeueAfter: reconciliationDelay},
			checkVMGExists: false,
		},
		{
			name: "Should Succeed if VMG is created",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				md1.DeepCopy(),
				vsm1.DeepCopy(),
			},
			expectedResult: reconcile.Result{},
			checkVMGExists: true,
		},
		{
			name: "Should Succeed if VMG is already existed",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				md1.DeepCopy(),
				vsm1.DeepCopy(),
				&vmoprv1.VirtualMachineGroup{
					ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace},
				},
			},
			expectedResult: reconcile.Result{},
			checkVMGExists: true,
		},
	}

	for _, tt := range tests {
		// Looks odd, but need to reinitialize test variable
		tt := tt
		t.Run(tt.name, func(_ *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.initialObjects...).Build()
			reconciler := &VirtualMachineGroupReconciler{
				Client:   fakeClient,
				Recorder: record.NewFakeRecorder(1),
			}
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: clusterNamespace}}

			result, err := reconciler.Reconcile(ctx, req)

			g.Expect(err).NotTo(HaveOccurred(), "Reconcile should not return an error")
			g.Expect(result).To(Equal(tt.expectedResult))

			vmg := &vmoprv1.VirtualMachineGroup{}
			vmgKey := types.NamespacedName{Name: clusterName, Namespace: clusterNamespace}
			err = fakeClient.Get(ctx, vmgKey, vmg)

			if tt.checkVMGExists {
				g.Expect(err).NotTo(HaveOccurred(), "VMG should exist")
				// Check that the core fields were set by the MutateFn
				g.Expect(vmg.Labels).To(HaveKeyWithValue(clusterv1.ClusterNameLabel, clusterName))
				g.Expect(vmg.Spec.BootOrder).To(HaveLen(1))
				expected, err := getExpectedVSphereMachines(ctx, fakeClient, tt.initialObjects[0].(*clusterv1.Cluster))
				g.Expect(err).NotTo(HaveOccurred(), "Should get expected Machines")
				g.Expect(vmg.Spec.BootOrder[0].Members).To(HaveLen(int(expected)))

				// VMG members should match the VSphereMachine name
				g.Expect(vmg.Spec.BootOrder[0].Members[0].Name).To(Equal("vsm-1"))
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
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				Workers: clusterv1.WorkersTopology{
					MachineDeployments: []clusterv1.MachineDeploymentTopology{
						{Name: mdName1, Replicas: &replicasMD1},
						{Name: mdName2, Replicas: &replicasMD2},
					},
				},
			},
		},
	}
	if initialized {
		conditions.Set(cluster, metav1.Condition{
			Type:   clusterv1.ClusterControlPlaneInitializedCondition,
			Status: metav1.ConditionTrue,
		})
	}
	return cluster
}

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
	return vsm
}

// Helper function to create a VMG member status with placement info.
func newVMGMemberStatus(name, kind string, isPlacementReady bool, zone string) vmoprv1.VirtualMachineGroupMemberStatus {
	memberStatus := vmoprv1.VirtualMachineGroupMemberStatus{
		Name: name,
		Kind: kind,
	}

	if isPlacementReady {
		conditions.Set(&memberStatus, metav1.Condition{
			Type:   vmoprv1.VirtualMachineGroupMemberConditionPlacementReady,
			Status: metav1.ConditionTrue,
		})
		memberStatus.Placement = &vmoprv1.VirtualMachinePlacementStatus{Zone: zone}
	}
	return memberStatus
}

// Helper function to create a MachineDeployment object.
func newMachineDeployment(name, clusterName, clusterNS string, replicas *int32) *clusterv1.MachineDeployment {
	return &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: clusterNS,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
		},
		Spec: clusterv1.MachineDeploymentSpec{
			Replicas: replicas,
		},
	}
}

// Helper function to create a basic Cluster object used as input.
func newTestCluster(name, namespace string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
