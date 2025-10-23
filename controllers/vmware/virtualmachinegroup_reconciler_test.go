package vmware

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var s = runtime.NewScheme()

const (
	clusterName      = "test-cluster"
	clusterNamespace = "test-ns"
	mdName1          = "md-worker-a"
	mdName2          = "md-worker-b"
	zoneA            = "zone-a"
	zoneB            = "zone-b"
)

func TestGetExpectedVSphereMachines(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name     string
		cluster  *clusterv1.Cluster
		expected int32
	}{
		{
			name:     "Defined topology with replicas",
			cluster:  newCluster(clusterName, clusterNamespace, true, 3, 2),
			expected: 5,
		},
		{
			name:     "Defined topology with zero replicas",
			cluster:  newCluster(clusterName, clusterNamespace, true, 0, 0),
			expected: 0,
		},
		{
			name: "Undefined topology",
			cluster: func() *clusterv1.Cluster {
				c := newCluster(clusterName, clusterNamespace, true, 1, 1)
				c.Spec.Topology = clusterv1.Topology{}
				return c
			}(),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g.Expect(getExpectedVSphereMachines(tt.cluster)).To(Equal(tt.expected))
		})
	}
}

func TestGetCurrentVSphereMachines(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// VM names are based on CAPI Machine names, not VSphereMachine names, but we use VSM objects here.
	vsm1 := newVSphereMachine("vsm-1", mdName1, false, nil)
	vsm2 := newVSphereMachine("vsm-2", mdName2, false, nil)
	vsmDeleting := newVSphereMachine("vsm-3", mdName1, true, nil) // Deleting
	vsmControlPlane := newVSphereMachine("vsm-cp", "cp-md", false, nil)
	vsmControlPlane.Labels[clusterv1.MachineControlPlaneLabel] = "true" // Should be filtered by label in production, but here filtered implicitly by only listing MD-labelled objects

	tests := []struct {
		name    string
		objects []client.Object
		want    int
	}{
		{
			name: "Success: Filtered non-deleting worker VSphereMachines",
			objects: []client.Object{
				vsm1,
				vsm2,
				vsmDeleting,
				vsmControlPlane,
			},
			want: 2, // Should exclude vsm-3 (deleting) and vsm-cp (no MD label used in the actual listing logic)
		},
		{
			name:    "No VSphereMachines found",
			objects: []client.Object{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(tt.objects...).Build()
			got, err := getCurrentVSphereMachines(ctx, fakeClient, clusterNamespace, clusterName)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(len(got)).To(Equal(tt.want))

			// Check that the correct machines are present (e.g., vsm1 and vsm2)
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
	vmName2 := fmt.Sprintf("%s-%s-vm-1", clusterName, mdName2)
	vmNameUnplaced := fmt.Sprintf("%s-%s-vm-2", clusterName, mdName1)
	vmNameWrongKind := "not-a-vm"

	tests := []struct {
		name               string
		vmg                *vmoprv1.VirtualMachineGroup
		machineDeployments []string
		wantAnnotations    map[string]string
		wantErr            bool
	}{
		{
			name: "Success: Two placed VMs for two MDs",
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
			machineDeployments: []string{mdName1, mdName2},
			wantAnnotations: map[string]string{
				fmt.Sprintf("zone.cluster.x-k8s.io/%s", mdName1): zoneA,
				fmt.Sprintf("zone.cluster.x-k8s.io/%s", mdName2): zoneB,
			},
			wantErr: false,
		},
		{
			name: "Skip: Unplaced VM (PlacementReady false)",
			vmg: &vmoprv1.VirtualMachineGroup{
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vmName1, "VirtualMachine", false, ""),
					},
				},
			},
			machineDeployments: []string{mdName1},
			wantAnnotations:    map[string]string{},
			wantErr:            false,
		},
		{
			name: "Skip: PlacementReady but missing Zone info",
			vmg: &vmoprv1.VirtualMachineGroup{
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vmName1, "VirtualMachine", true, ""),
					},
				},
			},
			machineDeployments: []string{mdName1},
			wantAnnotations:    map[string]string{},
			wantErr:            false,
		},
		{
			name: "Skip: Placement already found for MD",
			vmg: &vmoprv1.VirtualMachineGroup{
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						// First VM sets the placement
						newVMGMemberStatus(vmName1, "VirtualMachine", true, zoneA),
						// Second VM is ignored (logic skips finding placement twice)
						newVMGMemberStatus(vmNameUnplaced, "VirtualMachine", true, zoneB),
					},
				},
			},
			machineDeployments: []string{mdName1},
			wantAnnotations: map[string]string{
				fmt.Sprintf("zone.cluster.x-k8s.io/%s", mdName1): zoneA,
			},
			wantErr: false,
		},
		{
			name: "Error: Member Kind is not VirtualMachine",
			vmg: &vmoprv1.VirtualMachineGroup{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace},
				Status: vmoprv1.VirtualMachineGroupStatus{
					Members: []vmoprv1.VirtualMachineGroupMemberStatus{
						newVMGMemberStatus(vmNameWrongKind, "Pod", true, zoneA),
					},
				},
			},
			machineDeployments: []string{mdName1},
			wantAnnotations:    nil,
			wantErr:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock client is needed for the logging in the function, but not used for API calls
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

	// Initial objects for the successful VMG creation path (Expected: 1, Current: 1)
	cluster := newCluster(clusterName, clusterNamespace, true, 1, 0) // Expect 1 machine
	vsm1 := newVSphereMachine("vsm-1", mdName1, false, nil)
	md1 := newMachineDeployment(mdName1)

	tests := []struct {
		name           string
		initialObjects []client.Object
		expectedResult reconcile.Result
		checkVMGExists bool
	}{
		{
			name:           "Exit: Cluster Not Found",
			initialObjects: []client.Object{},
			expectedResult: reconcile.Result{},
			checkVMGExists: false,
		},
		{
			name: "Exit: Cluster Deletion Timestamp Set",
			initialObjects: []client.Object{
				func() client.Object {
					c := cluster.DeepCopy()
					c.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					return c
				}(),
			},
			expectedResult: reconcile.Result{},
			checkVMGExists: false,
		},
		{
			name: "Requeue: ControlPlane Not Initialized",
			initialObjects: []client.Object{
				newCluster(clusterName, clusterNamespace, false, 1, 0), // Not Initialized
			},
			expectedResult: reconcile.Result{RequeueAfter: reconciliationDelay},
			checkVMGExists: false,
		},
		{
			name: "Requeue: VMG Not Found, Machines Missing (0/1)",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				md1.DeepCopy(),
			},
			expectedResult: reconcile.Result{RequeueAfter: reconciliationDelay},
			checkVMGExists: false,
		},
		{
			name: "Success: VMG Created (1/1)",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				md1.DeepCopy(),
				vsm1.DeepCopy(),
			},
			expectedResult: reconcile.Result{},
			checkVMGExists: true,
		},
		{
			name: "Success: VMG Updated (Already Exists)",
			initialObjects: []client.Object{
				cluster.DeepCopy(),
				md1.DeepCopy(),
				vsm1.DeepCopy(),
				&vmoprv1.VirtualMachineGroup{ // Pre-existing VMG
					ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterNamespace},
				},
			},
			expectedResult: reconcile.Result{},
			checkVMGExists: true,
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
				// Check that the core fields were set by the MutateFn
				g.Expect(vmg.Labels).To(HaveKeyWithValue(clusterv1.ClusterNameLabel, clusterName))
				g.Expect(vmg.Spec.BootOrder).To(HaveLen(1))
				g.Expect(vmg.Spec.BootOrder[0].Members).To(HaveLen(int(getExpectedVSphereMachines(cluster))))

				// VMG members should match the VSphereMachine (name: vsm-1)
				g.Expect(vmg.Spec.BootOrder[0].Members[0].Name).To(ContainElement("vsm-1"))
			} else {
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "VMG should not exist or NotFound should be handled gracefully")
			}
		})
	}
}

// Helper function to create a *string
func stringPtr(s string) *string { return &s }

// Helper function to create a basic Cluster object
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

// Helper function to create a VSphereMachine (worker, owned by a CAPI Machine)
func newVSphereMachine(name, mdName string, deleted bool, namingStrategy *vmwarev1.VirtualMachineNamingStrategy) *vmwarev1.VSphereMachine {
	vsm := &vmwarev1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: clusterNamespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:           clusterName,
				clusterv1.MachineDeploymentNameLabel: mdName,
			},
		},
		Spec: vmwarev1.VSphereMachineSpec{
			NamingStrategy: namingStrategy,
		},
	}
	if deleted {
		vsm.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}
	return vsm
}

// Helper function to create a VMG member status with placement info
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

// Helper function to create a MachineDeployment (for listing MD names)
func newMachineDeployment(name string) *clusterv1.MachineDeployment {
	return &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: clusterNamespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
		},
	}
}
