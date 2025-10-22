package vmware

import (
	"context"
	"fmt"
	"testing"
	"time"

	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	topologyv1 "sigs.k8s.io/cluster-api-provider-vsphere/internal/apis/topology/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var s = runtime.NewScheme()

func init() {
	// Register all necessary API types for the fake client
	_ = vmoprv1.AddToScheme(s)
	_ = infrav1.AddToScheme(s)
	_ = vmwarev1.AddToScheme(s)
	_ = topologyv1.AddToScheme(s)
	_ = clusterv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
}

const (
	clusterName      = "test-cluster"
	clusterNamespace = "test-ns"
	mdName1          = "md-worker-a"
	mdName2          = "md-worker-b"
	zoneA            = "zone-a"
	zoneB            = "zone-b"
)

// Helper function to create a basic Cluster object
func newCluster(name, namespace string, initialized bool, topology bool) *clusterv1.Cluster {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: name},
		},
	}
	if initialized {
		conditions.MarkTrue(cluster, clusterv1.ClusterControlPlaneInitializedCondition)
	} else {
		conditions.MarkFalse(cluster, clusterv1.ClusterControlPlaneInitializedCondition, "Waiting", clusterv1.ConditionSeverityInfo, "")
	}

	if topology {
		cluster.Spec.Topology = &clusterv1.Topology{}
		cluster.Spec.Topology.Workers = &clusterv1.Workers{
			MachineDeployments: clusterv1.MachineDeploymentTopology{},
		}
	}
	return cluster
}

// Helper function to create a MachineDeploymentTopology for the Cluster spec
func newMDTopology(name string, replicas int32, fd string) clusterv1.MachineDeploymentTopology {
	return clusterv1.MachineDeploymentTopology{
		Class:         "test-class",
		Name:          name,
		FailureDomain: &fd, // Pointer to FailureDomain string
		Replicas:      &replicas,
	}
}

// Helper function to create a VSphereCluster
func newVSphereCluster(name, namespace string, ready bool, zones ...string) *vmwarev1.VSphereCluster {
	vsc := &vmwarev1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: name},
		},
	}
	if ready {
		conditions.MarkTrue(vsc, vmwarev1.VSphereClusterReadyCondition)
	}

	for _, zone := range zones {
		vsc.Status.FailureDomains = append(vsc.Status.FailureDomains, vmwarev1.FailureDomainStatus{Name: zone})
	}
	return vsc
}

// Helper function to create a CAPI Machine (worker or control plane)
func newMachine(name, mdName string, isControlPlane bool) *clusterv1.Machine {
	m := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: clusterNamespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:           clusterName,
				clusterv1.MachineDeploymentNameLabel: mdName,
			},
		},
	}
	if isControlPlane {
		m.Labels[clusterv1.MachineControlPlaneLabel] = "true"
	}
	return m
}

// Helper function to create a VSphereMachine (owned by a CAPI Machine)
func newVSphereMachine(name, ownerMachineName string) *infrav1.VSphereMachine {
	return &infrav1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: clusterNamespace,
			OwnerReferences: metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       ownerMachineName,
				},
			},
		},
	}
}

// Helper function to create a VMG member status with placement info
func newVMGMemberStatus(name, kind string, ready bool, zone string) vmoprv1.GroupMember {
	member := vmoprv1.GroupMember{
		Name: name,
		Kind: kind,
	}

	if ready {
		conditions.MarkTrue(&member, vmoprv1.VirtualMachineGroupMemberConditionPlacementReady)
		member.Placement = &vmoprv1.Placement{
			Zone: zone,
		}
	}
	return member
}

// Helper function to create a mock MachineDeployment
func newMachineDeployment(name string) *clusterv1.MachineDeployment {
	return &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: clusterNamespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
		},
	}
}

func TestHelperFunctions(t *testing.T) {
	g := NewWithT(t)
	// Create cluster topology with mixed placement
	cluster := newCluster(clusterName, clusterNamespace, true, true)
	cluster.Spec.Topology.Workers.MachineDeployments = clusterv1.MachineDeploymentTopology{
		newMDTopology(mdName1, 3, ""),    // Automatic placement
		newMDTopology(mdName2, 5, zoneB), // Explicit placement
	}
	g.Expect(cluster.Spec.Topology.IsDefined()).To(BeTrue())

	// Test isExplicitPlacement
	explicit, err := isExplicitPlacement(cluster)
	g.Expect(err).NotTo(HaveOccurred())
	// Should be true because MD2 has a FailureDomain specified
	g.Expect(explicit).To(BeTrue(), "isExplicitPlacement should be true when any MD has FD set")

	// Test getExpectedMachines
	expected := getExpectedMachines(cluster)
	// Expected total replicas: 3 + 5 = 8
	g.Expect(expected).To(BeEquivalentTo(8), "Expected machines count should be 8")

	// Test getExpectedMachines with nil replicas
	clusterNoReplicas := newCluster(clusterName, clusterNamespace, true, true)
	clusterNoReplicas.Spec.Topology.Workers.MachineDeployments = clusterv1.MachineDeploymentTopology{
		{Name: "md-1", Class: "c1"},
	}
	expectedZero := getExpectedMachines(clusterNoReplicas)
	g.Expect(expectedZero).To(BeEquivalentTo(0), "Expected machines count should be 0 for nil replicas")
}

func TestIsExplicitPlacement(t *testing.T) {
	g := NewWithT(t)

	// Setup cluster for test cases
	baseCluster := newCluster(clusterName, clusterNamespace, true, true)

	tests := struct {
		name string
		mds  clusterv1.MachineDeploymentTopology
		want bool
	}{
		{
			name: "All MDs use automatic placement (empty FD)",
			mds: clusterv1.MachineDeploymentTopology{
				newMDTopology(mdName1, 3, ""),
				newMDTopology(mdName2, 2, ""),
			},
			want: false,
		},
		{
			name: "One MD uses explicit placement (non-empty FD)",
			mds: clusterv1.MachineDeploymentTopology{
				newMDTopology(mdName1, 3, ""),
				newMDTopology(mdName2, 2, zoneA),
			},
			want: true,
		},
		{
			name: "All MDs use explicit placement",
			mds: clusterv1.MachineDeploymentTopology{
				newMDTopology(mdName1, 3, zoneA),
				newMDTopology(mdName2, 2, zoneB),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := baseCluster.DeepCopy()
			cluster.Spec.Topology.Workers.MachineDeployments = tt.mds
			got, err := isExplicitPlacement(cluster)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestGetCurrentVSphereMachines(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Define object names for workers and CPs
	cpMachineName := "cp-machine-0"
	workerMachineA1 := "worker-a-1"
	workerMachineB1 := "worker-b-1"
	// Define a machine that belongs to a non-existent MD
	strayMachine := "stray-machine"

	tests := struct {
		name string
		objectsclient.Object
		want int
	}{
		{
			name: "Success: Correctly filters worker VSphereMachines",
			objects: client.Object{
				// Active MDs
				newMachineDeployment(mdName1),
				newMachineDeployment(mdName2),
				// CAPI Machines
				newMachine(cpMachineName, "", true), // Control Plane (should be skipped)
				newMachine(workerMachineA1, mdName1, false),
				newMachine(workerMachineB1, mdName2, false),
				newMachine(strayMachine, "non-existent-md", false), // Stray worker (should be skipped)
				// VSphereMachines (Infrastructure objects)
				newVSphereMachine("vsm-cp-0", cpMachineName), // Should be skipped
				newVSphereMachine("vsm-a-1", workerMachineA1),
				newVSphereMachine("vsm-b-1", workerMachineB1),
				newVSphereMachine("vsm-stray", strayMachine), // Should be skipped
			},
			want: 2, // Only vsm-a-1 and vsm-b-1
		},
		{
			name: "No VSphereMachines found",
			objects: client.Object{
				newMachineDeployment(mdName1),
				newMachine(workerMachineA1, mdName1, false),
			},
			want: 0,
		},
		{
			name:    "No objects exist",
			objects: client.Object{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(tt.objects...).Build()
			reconciler := &VirtualMachineGroupReconciler{Client: fakeClient}

			got, err := getCurrentVSphereMachines(ctx, reconciler.Client, clusterNamespace, clusterName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(len(got)).To(Equal(tt.want))
		})
	}
}

func TestGenerateVMGPlacementLabels(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	tests := struct {
		name string
		vmg  *vmoprv1.VirtualMachineGroup
		nodepoolsstring
		want    map[string]string
		wantErr bool
	}{
		{
			name: "Success: VMG with one placed VM per nodepool",
			vmg: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace},
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: vmoprv1.GroupMember{
						// Placed member for MD1
						newVMGMemberStatus(fmt.Sprintf("%s-%s-vm-1", clusterName, mdName1), "VirtualMachine", true, zoneA),
						// Placed member for MD2
						newVMGMemberStatus(fmt.Sprintf("%s-%s-vm-1", clusterName, mdName2), "VirtualMachine", true, zoneB),
						// Unplaced member for MD1 (should be ignored)
						newVMGMemberStatus(fmt.Sprintf("%s-%s-vm-2", clusterName, mdName1), "VirtualMachine", false, ""),
					},
				},
			},
			nodepools: string{mdName1, mdName2},
			want: map[string]string{
				fmt.Sprintf("%s/%s", VMGPlacementLabelPrefix, mdName1): zoneA,
				fmt.Sprintf("%s/%s", VMGPlacementLabelPrefix, mdName2): zoneB,
			},
			wantErr: false,
		},
		{
			name: "Success: Multiple placed VMs for the same nodepool (should take first)",
			vmg: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace},
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: vmoprv1.GroupMember{
						// First placed VM (Zone B)
						newVMGMemberStatus(fmt.Sprintf("%s-%s-vm-1", clusterName, mdName1), "VirtualMachine", true, zoneB),
						// Second placed VM (Zone A) - should be skipped
						newVMGMemberStatus(fmt.Sprintf("%s-%s-vm-2", clusterName, mdName1), "VirtualMachine", true, zoneA),
					},
				},
			},
			nodepools: string{mdName1},
			want: map[string]string{
				fmt.Sprintf("%s/%s", VMGPlacementLabelPrefix, mdName1): zoneB,
			},
			wantErr: false,
		},
		{
			name: "Error: PlacementReady true but Placement is nil",
			vmg: &vmoprv1.VirtualMachineGroup{
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: vmoprv1.GroupMember{
						{
							Name: "vm-1",
							Kind: "VirtualMachine",
							// Condition marked true, but Placement field is nil
							Conditions: metav1.Condition{{Type: vmoprv1.VirtualMachineGroupMemberConditionPlacementReady, Status: metav1.ConditionTrue}},
						},
					},
				},
			},
			nodepools: string{mdName1},
			want:      nil,
			wantErr:   true, // Expect an error about nil placement info
		},
		{
			name: "Skip: No members are PlacementReady",
			vmg: &vmoprv1.VirtualMachineGroup{
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: vmoprv1.GroupMember{
						newVMGMemberStatus(fmt.Sprintf("%s-%s-vm-1", clusterName, mdName1), "VirtualMachine", false, ""),
					},
				},
			},
			nodepools: string{mdName1},
			want:      map[string]string{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateVMGPlacementLabels(ctx, tt.vmg, tt.nodepools)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(got).To(Equal(tt.want))
			}
		})
	}
}

func TestVMGReconcile(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Define all required mock objects for a successful run
	cluster := newCluster(clusterName, clusterNamespace, true, true)
	cluster.Spec.Topology.Workers.MachineDeployments = clusterv1.MachineDeploymentTopology{
		newMDTopology(mdName1, 1, ""),
	}
	vsc := newVSphereCluster(clusterName, clusterNamespace, true, zoneA, zoneB) // Two zones
	md := newMachineDeployment(mdName1)
	machine := newMachine("worker-1", mdName1, false)
	vsMachine := newVSphereMachine("vsm-1", "worker-1")

	tests := struct {
		name             string
		initialObjects   client.Object
		expectedResult   reconcile.Result
		checkVMGExists   bool
		checkVMGMembers  string // Expected member names
		checkVMGReplicas int32  // Expected VMG replicas (for sanity check)
	}{
		{
			name:             "Exit: Cluster not found (GC)",
			initialObjects:   client.Object{},
			expectedResult:   reconcile.Result{},
			checkVMGExists:   false,
			checkVMGReplicas: 0,
		},
		{
			name: "Exit: Cluster marked for deletion",
			initialObjects: client.Object{
				func() client.Object {
					c := cluster.DeepCopy()
					c.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					return c
				}(),
			},
			expectedResult:   reconcile.Result{},
			checkVMGExists:   false,
			checkVMGReplicas: 0,
		},
		{
			name: "Exit: Explicit placement used",
			initialObjects: client.Object{
				func() client.Object {
					c := cluster.DeepCopy()
					c.Spec.Topology.Workers.MachineDeployments.FailureDomain = stringPtr(zoneA)
					return c
				}(),
			},
			expectedResult:   reconcile.Result{},
			checkVMGExists:   false,
			checkVMGReplicas: 0,
		},
		{
			name: "Requeue: VSphereCluster not ready",
			initialObjects: client.Object{
				cluster.DeepCopy(),
				newVSphereCluster(clusterName, clusterNamespace, false, zoneA, zoneB), // Not Ready
			},
			expectedResult:   reconcile.Result{RequeueAfter: reconciliationDelay},
			checkVMGExists:   false,
			checkVMGReplicas: 0,
		},
		{
			name: "Exit: Single zone detected",
			initialObjects: client.Object{
				cluster.DeepCopy(),
				newVSphereCluster(clusterName, clusterNamespace, true, zoneA), // Only one zone
			},
			expectedResult:   reconcile.Result{},
			checkVMGExists:   false,
			checkVMGReplicas: 0,
		},
		{
			name: "Requeue: ControlPlane not initialized",
			initialObjects: client.Object{
				newCluster(clusterName, clusterNamespace, false, true), // Not Initialized
				vsc.DeepCopy(),
			},
			expectedResult:   reconcile.Result{RequeueAfter: reconciliationDelay},
			checkVMGExists:   false,
			checkVMGReplicas: 0,
		},
		{
			name: "Requeue: Machines not fully created (0/1)",
			initialObjects: client.Object{
				cluster.DeepCopy(),
				vsc.DeepCopy(),
				md.DeepCopy(),
				// No Machine or VSphereMachine objects created yet
			},
			expectedResult:   reconcile.Result{RequeueAfter: reconciliationDelay},
			checkVMGExists:   false,
			checkVMGReplicas: 0,
		},
		{
			name: "Success: VMG created with correct members (1/1)",
			initialObjects: client.Object{
				cluster.DeepCopy(),
				vsc.DeepCopy(),
				md.DeepCopy(),
				machine.DeepCopy(),
				vsMachine.DeepCopy(),
			},
			expectedResult:   reconcile.Result{},
			checkVMGExists:   true,
			checkVMGMembers:  string{"vsm-1"},
			checkVMGReplicas: 1, // Replica count derived from VMG spec
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(tt.initialObjects...).Build()
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
				// Check owner reference
				g.Expect(vmg.OwnerReferences).To(HaveLen(1))
				g.Expect(vmg.OwnerReferences.Name).To(Equal(clusterName))
				g.Expect(vmg.OwnerReferences.Kind).To(Equal("Cluster"))
				g.Expect(vmg.OwnerReferences.Controller).To(PointTo(BeTrue()))

				// Check members
				g.Expect(vmg.Spec.BootOrder).To(HaveLen(1))
				members := vmg.Spec.BootOrder.Members
				g.Expect(members).To(HaveLen(len(tt.checkVMGMembers)))
				if len(members) > 0 {
					g.Expect(members.Name).To(Equal(tt.checkVMGMembers))
					g.Expect(members.Kind).To(Equal("VirtualMachine"))
				}
			} else {
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "VMG should not exist")
			}
		})
	}
}

// stringPtr converts a string to a *string
func stringPtr(s string) *string { return &s }
