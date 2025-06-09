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
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirecord "k8s.io/client-go/tools/record"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	topologyv1 "sigs.k8s.io/cluster-api-provider-vsphere/internal/apis/topology/v1alpha1"
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
			vsphereMachine.Status.IPAddr = testIP
			request := reconciler.VSphereMachineToCluster(ctx, vsphereMachine)
			Expect(request).ShouldNot(BeNil())
			Expect(request[0].Namespace).Should(Equal(cluster.Namespace))
			Expect(request[0].Name).Should(Equal(cluster.Name))
		})
	})

	Context("Test reconcileDelete", func() {
		It("should mark specific resources to be in deleting conditions", func() {
			clusterCtx.VSphereCluster.Status.Conditions = append(clusterCtx.VSphereCluster.Status.Conditions,
				clusterv1beta1.Condition{Type: vmwarev1.ResourcePolicyReadyCondition, Status: corev1.ConditionTrue})
			reconciler.reconcileDelete(clusterCtx)
			c := v1beta1conditions.Get(clusterCtx.VSphereCluster, vmwarev1.ResourcePolicyReadyCondition)
			Expect(c).NotTo(BeNil())
			Expect(c.Status).To(Equal(corev1.ConditionFalse))
			Expect(c.Reason).To(Equal(clusterv1beta1.DeletingReason))
		})

		It("should not mark other resources to be in deleting conditions", func() {
			otherReady := clusterv1beta1.ConditionType("OtherReady")
			clusterCtx.VSphereCluster.Status.Conditions = append(clusterCtx.VSphereCluster.Status.Conditions,
				clusterv1beta1.Condition{Type: otherReady, Status: corev1.ConditionTrue})
			reconciler.reconcileDelete(clusterCtx)
			c := v1beta1conditions.Get(clusterCtx.VSphereCluster, otherReady)
			Expect(c).NotTo(BeNil())
			Expect(c.Status).NotTo(Equal(corev1.ConditionFalse))
			Expect(c.Reason).NotTo(Equal(clusterv1beta1.DeletingReason))
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
		want        clusterv1beta1.FailureDomains
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
			name:        "Cluster-Wide: should find FailureDomains if only cluster-wide exist",
			objects:     []client.Object{availabilityZone("c-one")},
			want:        failureDomains("c-one"),
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
			want:        failureDomains("ns-one", "ns-two", "ns-three"),
			wantErr:     false,
			featureGate: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ClusterReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(append([]client.Object{namespace}, tt.objects...)...).
					Build(),
			}
			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.NamespaceScopedZones, tt.featureGate)
			got, err := r.getFailureDomains(ctx, namespace.Name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterReconciler.getFailureDomains() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterReconciler.getFailureDomains() = %v, want %v", got, tt.want)
			}
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

func failureDomains(names ...string) clusterv1beta1.FailureDomains {
	fds := clusterv1beta1.FailureDomains{}
	for _, name := range names {
		fds[name] = clusterv1beta1.FailureDomainSpec{
			ControlPlane: true,
		}
	}
	return fds
}
