/*
Copyright 2021 The Kubernetes Authors.

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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topologyv1 "github.com/vmware-tanzu/vm-operator/external/tanzu-topology/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirecord "k8s.io/client-go/tools/record"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/vmoperator"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

var _ = Describe("Cluster Controller Tests", func() {
	const (
		clusterName           = "test-cluster"
		machineName           = "test-machine"
		controlPlaneLabelTrue = true
		className             = "test-className"
		imageName             = "test-imageName"
		storageClass          = "test-storageClass"
		testIP                = "127.0.0.1"
	)
	var (
		cluster                  *clusterv1.Cluster
		vsphereCluster           *vmwarev1.VSphereCluster
		vsphereMachine           *vmwarev1.VSphereMachine
		clusterCtx               *vmware.ClusterContext
		controllerManagerContext *capvcontext.ControllerManagerContext
		reconciler               *ClusterReconciler
	)

	BeforeEach(func() {
		// Create all necessary dependencies
		cluster = util.CreateCluster(clusterName)
		vsphereCluster = util.CreateVSphereCluster(clusterName)
		clusterCtx, controllerManagerContext = util.CreateClusterContext(cluster, vsphereCluster)
		vsphereMachine = util.CreateVSphereMachine(machineName, clusterName, className, imageName, storageClass, controlPlaneLabelTrue)

		reconciler = &ClusterReconciler{
			Client:          controllerManagerContext.Client,
			Recorder:        apirecord.NewFakeRecorder(100),
			NetworkProvider: network.DummyNetworkProvider(),
			ControlPlaneService: &vmoperator.CPService{
				Client: controllerManagerContext.Client,
			},
		}

		Expect(controllerManagerContext.Client.Create(ctx, cluster)).To(Succeed())
		Expect(controllerManagerContext.Client.Create(ctx, vsphereCluster)).To(Succeed())
	})

	// Ensure that the mechanism for reconciling clusters when a control plane machine gets an IP works
	Context("Test controlPlaneMachineToCluster", func() {
		It("Returns nil if there is no IP address", func() {
			request := reconciler.VSphereMachineToCluster(ctx, vsphereMachine)
			Expect(request).Should(BeNil())
		})

		It("Returns valid request with IP address", func() {
			vsphereMachine.Status.Addresses = clusterv1.MachineAddresses{
				{
					Type:    clusterv1.MachineInternalIP,
					Address: testIP,
				},
			}
			request := reconciler.VSphereMachineToCluster(ctx, vsphereMachine)
			Expect(request).ShouldNot(BeNil())
			Expect(request[0].Namespace).Should(Equal(cluster.Namespace))
			Expect(request[0].Name).Should(Equal(cluster.Name))
		})
	})

	Context("Test reconcileDelete", func() {
		It("should mark specific resources to be in deleting conditions", func() {
			clusterCtx.VSphereCluster.Status.Conditions = append(clusterCtx.VSphereCluster.Status.Conditions,
				metav1.Condition{Type: vmwarev1.VSphereClusterResourcePolicyReadyCondition, Status: metav1.ConditionTrue},
				metav1.Condition{Type: vmwarev1.VSphereClusterFailureDomainsReadyCondition, Status: metav1.ConditionTrue}, // Setup FailureDomains condition
			)
			reconciler.reconcileDelete(clusterCtx)

			// Verify ResourcePolicy condition
			c1 := conditions.Get(clusterCtx.VSphereCluster, vmwarev1.VSphereClusterResourcePolicyReadyCondition)
			Expect(c1).NotTo(BeNil())
			Expect(c1.Status).To(Equal(metav1.ConditionFalse))
			Expect(c1.Reason).To(Equal(clusterv1.DeletingReason))

			// Verify FailureDomains condition
			c2 := conditions.Get(clusterCtx.VSphereCluster, vmwarev1.VSphereClusterFailureDomainsReadyCondition)
			Expect(c2).NotTo(BeNil())
			Expect(c2.Status).To(Equal(metav1.ConditionFalse))
			Expect(c2.Reason).To(Equal(vmwarev1.VSphereClusterFailureDomainsReadyDeletingReason))
		})

		It("should not mark other resources to be in deleting conditions", func() {
			otherReady := "OtherReady"
			clusterCtx.VSphereCluster.Status.Conditions = append(clusterCtx.VSphereCluster.Status.Conditions,
				metav1.Condition{Type: otherReady, Status: metav1.ConditionTrue})
			reconciler.reconcileDelete(clusterCtx)
			c := conditions.Get(clusterCtx.VSphereCluster, otherReady)
			Expect(c).NotTo(BeNil())
			Expect(c.Status).NotTo(Equal(metav1.ConditionFalse))
			Expect(c.Reason).NotTo(Equal(clusterv1.DeletingReason))
		})
	})
})

func TestClusterReconciler_getFailureDomains(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(topologyv1.AddToScheme(scheme)).To(Succeed())

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	tests := []struct {
		name        string
		objects     []client.Object
		spec        vmwarev1.VSphereClusterSpec
		want        []clusterv1.FailureDomain
		wantErr     bool
		featureGate bool
	}{
		{
			name:        "Cluster-Wide: should not find any FailureDomains if no exists",
			objects:     []client.Object{},
			want:        nil,
			wantErr:     false,
			featureGate: false,
		},
		{
			name:        "Namespaced: should not find any FailureDomains if no exists",
			objects:     []client.Object{},
			want:        nil,
			wantErr:     false,
			featureGate: true,
		},
		{
			name:        "Cluster-Wide: should not find any FailureDomains if only namespaced exist",
			objects:     []client.Object{zone(namespace.Name, "ns-one", false)},
			want:        nil,
			wantErr:     false,
			featureGate: false,
		},
		{
			name:        "Namespaced: should not find any FailureDomains if only cluster-wide exist",
			objects:     []client.Object{availabilityZone("c-one")},
			want:        nil,
			wantErr:     false,
			featureGate: true,
		},
		{
			name: "Cluster-Wide: should find FailureDomains if only cluster-wide exist",
			objects: []client.Object{
				availabilityZone("c-one"),
				availabilityZone("c-two"),
				availabilityZone("c-three"),
			},
			want:        failureDomains("c-one", "c-three", "c-two"), // failureDomains are expected to be sorted.
			wantErr:     false,
			featureGate: false,
		},
		{
			name:        "Namespaced: should find FailureDomains if only namespaced exist",
			objects:     []client.Object{zone(namespace.Name, "ns-one", false)},
			want:        failureDomains("ns-one"),
			wantErr:     false,
			featureGate: true,
		},
		{
			name:        "Cluster-Wide: should only find cluster-wide FailureDomains if both types exist",
			objects:     []client.Object{availabilityZone("c-one"), zone(namespace.Name, "ns-one", false)},
			want:        failureDomains("c-one"),
			wantErr:     false,
			featureGate: false,
		},
		{
			name:        "Namespaced: should only find namespaced FailureDomains if both types exist",
			objects:     []client.Object{availabilityZone("c-one"), zone(namespace.Name, "ns-one", false)},
			want:        failureDomains("ns-one"),
			wantErr:     false,
			featureGate: true,
		},
		{
			name: "Namespaced: should only find non-deleting namespaced FailureDomains",
			objects: []client.Object{
				availabilityZone("c-one"),
				zone(namespace.Name, "ns-one", false),
				zone(namespace.Name, "ns-two", false),
				zone(namespace.Name, "ns-three", false),
				zone(namespace.Name, "ns-four", true),
			},
			want:        failureDomains("ns-one", "ns-three", "ns-two"), // failureDomains are expected to be sorted.
			wantErr:     false,
			featureGate: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			r := &ClusterReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(append([]client.Object{namespace}, tt.objects...)...).
					Build(),
			}
			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.NamespaceScopedZones, tt.featureGate)
			got, err := r.getFailureDomains(ctx, namespace.Name, tt.spec.FailureDomains)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterReconciler.getFailureDomains() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			g.Expect(got).To(BeComparableTo(tt.want))
		})
	}
}

func availabilityZone(name string) *topologyv1.AvailabilityZone {
	return &topologyv1.AvailabilityZone{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func zone(namespace, name string, deleting bool) *topologyv1.Zone {
	z := &topologyv1.Zone{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	if deleting {
		z.ObjectMeta.DeletionTimestamp = ptr.To(metav1.Now())
		z.ObjectMeta.Finalizers = []string{"deletion.test.io/protection"}
	}
	return z
}

func failureDomains(names ...string) []clusterv1.FailureDomain {
	fds := make([]clusterv1.FailureDomain, 0, len(names))
	for _, name := range names {
		fds = append(fds, clusterv1.FailureDomain{
			Name:         name,
			ControlPlane: ptr.To(true),
		})
	}
	return fds
}

// zoneWithLabels creates a namespaced Zone with labels.
func zoneWithLabels(namespace, name string, lbls map[string]string) *topologyv1.Zone {
	return &topologyv1.Zone{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    lbls,
		},
	}
}

func TestApplyControlPlaneFilter_Default(t *testing.T) {
	// ControlPlaneFailureDomainSelector is not set.
	// All domains must be eligible for control plane placement (backwards compatible).
	domains := []clusterv1.FailureDomain{
		{Name: "zone-a"},
		{Name: "zone-b"},
		{Name: "zone-c"},
	}
	spec := vmwarev1.VSphereClusterSpec{}
	got, err := applyControlPlaneFilter(domains, nil, spec.FailureDomains)
	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(ConsistOf(
		clusterv1.FailureDomain{Name: "zone-a", ControlPlane: ptr.To(true)},
		clusterv1.FailureDomain{Name: "zone-b", ControlPlane: ptr.To(true)},
		clusterv1.FailureDomain{Name: "zone-c", ControlPlane: ptr.To(true)},
	))
}

func TestApplyControlPlaneLabelSelector(t *testing.T) {
	tests := []struct {
		name         string
		domains      []clusterv1.FailureDomain
		domainLabels map[string]map[string]string
		selector     *metav1.LabelSelector
		wantCP       map[string]bool
		wantErr      bool
	}{
		{
			name: "domains matching selector are control-plane eligible",
			domains: []clusterv1.FailureDomain{
				{Name: "mgmt-zone"},
				{Name: "worker-zone"},
			},
			domainLabels: map[string]map[string]string{
				"mgmt-zone":   {"tanzu-topology.vmware.com/type": "MANAGEMENT"},
				"worker-zone": {"tanzu-topology.vmware.com/type": "WORKLOAD"},
			},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"tanzu-topology.vmware.com/type": "MANAGEMENT"},
			},
			wantCP: map[string]bool{"mgmt-zone": true, "worker-zone": false},
		},
		{
			name: "selector matching no domains returns error to prevent mutation",
			domains: []clusterv1.FailureDomain{
				{Name: "zone-a"},
				{Name: "zone-b"},
			},
			domainLabels: map[string]map[string]string{
				"zone-a": {"type": "workload"},
				"zone-b": {"type": "workload"},
			},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"type": "management"},
			},
			wantErr: true,
		},
		{
			name: "selector matching all domains",
			domains: []clusterv1.FailureDomain{
				{Name: "zone-a"},
				{Name: "zone-b"},
			},
			domainLabels: map[string]map[string]string{
				"zone-a": {"env": "prod"},
				"zone-b": {"env": "prod"},
			},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "prod"},
			},
			wantCP: map[string]bool{"zone-a": true, "zone-b": true},
		},
		{
			name: "invalid selector returns error",
			domains: []clusterv1.FailureDomain{
				{Name: "zone-a"},
			},
			domainLabels: map[string]map[string]string{
				"zone-a": {},
			},
			selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "k", Operator: "BadOp", Values: []string{"v"}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Call the inner function directly since we are passing a map, not a ZoneList
			got, err := applyControlPlaneLabelSelector(tt.domains, tt.domainLabels, tt.selector)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				// If we expect the custom error, verify we got exactly that error
				if tt.name == "selector matching no domains returns error to prevent mutation" {
					g.Expect(err).To(MatchError(ErrNoFailureDomainsMatched))
				}
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			for _, fd := range got {
				expected, ok := tt.wantCP[fd.Name]
				g.Expect(ok).To(BeTrue(), "unexpected domain %q in result", fd.Name)
				g.Expect(ptr.Deref(fd.ControlPlane, false)).To(Equal(expected), "domain %q controlPlane mismatch", fd.Name)
			}
		})
	}
}

func TestClusterReconciler_getFailureDomains_ControlPlaneFilter(t *testing.T) {
	// End-to-end tests for getFailureDomains with control plane placement constraints,
	// exercising both the NamespaceScopedZones (Zone) and legacy (AvailabilityZone) paths.
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(topologyv1.AddToScheme(scheme)).To(Succeed())

	const ns = "test-ns"
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}

	tests := []struct {
		name        string
		objects     []client.Object
		spec        vmwarev1.VSphereClusterSpec
		featureGate bool
		wantCP      map[string]bool // nil means skip per-domain assertion; use wantNil
		wantNil     bool
		wantErr     bool
	}{
		// ── Cluster-wide (AvailabilityZone) path ────────────────────────────
		{
			name: "Cluster-Wide: providing a selector without the NamespaceScopedZones feature gate returns an error",
			objects: []client.Object{
				availabilityZone("az-mgmt-1"),
			},
			spec: vmwarev1.VSphereClusterSpec{
				FailureDomains: vmwarev1.FailureDomainsSpec{
					ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"zone-type": "management"},
						},
					},
				},
			},
			featureGate: false,
			wantErr:     true, // Expect the new silent-failure prevention error
		},
		{
			name: "Cluster-Wide: no constraints — all domains are CP eligible",
			objects: []client.Object{
				availabilityZone("az-a"),
				availabilityZone("az-b"),
			},
			spec:        vmwarev1.VSphereClusterSpec{},
			featureGate: false,
			wantCP:      map[string]bool{"az-a": true, "az-b": true}, // Expect normal legacy behavior
		},

		// ── Namespaced (Zone) path ───────────────────────────────────────────
		{
			name: "Namespaced: label selector restricts CP eligibility",
			objects: []client.Object{
				zoneWithLabels(ns, "zone-a", map[string]string{"tanzu-topology.vmware.com/type": "MANAGEMENT"}),
				zoneWithLabels(ns, "zone-b", map[string]string{"tanzu-topology.vmware.com/type": "WORKLOAD"}),
			},
			spec: vmwarev1.VSphereClusterSpec{
				FailureDomains: vmwarev1.FailureDomainsSpec{
					ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"tanzu-topology.vmware.com/type": "MANAGEMENT"},
						},
					},
				},
			},
			featureGate: true,
			wantCP:      map[string]bool{"zone-a": true, "zone-b": false},
		},
		{
			name: "Namespaced: label selector matching no domains returns error",
			objects: []client.Object{
				zoneWithLabels(ns, "zone-a", map[string]string{"type": "workload"}),
			},
			spec: vmwarev1.VSphereClusterSpec{
				FailureDomains: vmwarev1.FailureDomainsSpec{
					ControlPlane: vmwarev1.FailureDomainsControlPlaneSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"type": "management"},
						},
					},
				},
			},
			featureGate: true,
			wantErr:     true,
		},
		{
			name: "Namespaced: no constraints — all domains are CP eligible",
			objects: []client.Object{
				zone(ns, "zone-a", false),
				zone(ns, "zone-b", false),
			},
			spec:        vmwarev1.VSphereClusterSpec{},
			featureGate: true,
			wantCP:      map[string]bool{"zone-a": true, "zone-b": true},
		},
		{
			name:        "Namespaced: no zones returns nil",
			objects:     []client.Object{},
			spec:        vmwarev1.VSphereClusterSpec{},
			featureGate: true,
			wantNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			r := &ClusterReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(append([]client.Object{namespace}, tt.objects...)...).
					Build(),
			}
			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.NamespaceScopedZones, tt.featureGate)

			got, err := r.getFailureDomains(ctx, ns, tt.spec.FailureDomains)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())

			if tt.wantNil {
				g.Expect(got).To(BeNil())
				return
			}

			g.Expect(got).To(HaveLen(len(tt.wantCP)))
			for _, fd := range got {
				expected, ok := tt.wantCP[fd.Name]
				g.Expect(ok).To(BeTrue(), "unexpected domain %q in result", fd.Name)
				g.Expect(ptr.Deref(fd.ControlPlane, false)).To(Equal(expected),
					"domain %q: expected ControlPlane=%v", fd.Name, expected)
			}
		})
	}
}
