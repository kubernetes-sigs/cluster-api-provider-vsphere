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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topologyv1 "github.com/vmware-tanzu/vm-operator/external/tanzu-topology/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/vmoperator"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func TestVSphereClusterReconciler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VSphereCluster reconciler test suite")
}

var _ = Describe("Cluster Controller Tests", func() {
	const (
		clusterName           = "test-cluster"
		machineName           = "test-machine"
		controlPlaneLabelTrue = "true"
		className             = "test-className"
		imageName             = "test-imageName"
		storageClass          = "test-storageClass"
		testIP                = "127.0.0.1"
	)
	var (
		cluster        *clusterv1.Cluster
		vsphereCluster *infrav1.VSphereCluster
		vsphereMachine *infrav1.VSphereMachine
		ctx            *vmware.ClusterContext
		reconciler     *ClusterReconciler
	)

	BeforeEach(func() {
		// Create all necessary dependencies
		cluster = util.CreateCluster(clusterName)
		vsphereCluster = util.CreateVSphereCluster(clusterName)
		ctx = util.CreateClusterContext(cluster, vsphereCluster)
		vsphereMachine = util.CreateVSphereMachine(machineName, clusterName, controlPlaneLabelTrue, className, imageName, storageClass)

		reconciler = &ClusterReconciler{
			ControllerContext:   ctx.ControllerContext,
			NetworkProvider:     network.DummyNetworkProvider(),
			ControlPlaneService: vmoperator.CPService{},
		}

		Expect(ctx.Client.Create(ctx, cluster)).To(Succeed())
		Expect(ctx.Client.Create(ctx, vsphereCluster)).To(Succeed())
	})

	// Ensure that the mechanism for reconciling clusters when a control plane machine gets an IP works
	Context("Test controlPlaneMachineToCluster", func() {
		It("Returns nil if there is no IP address", func() {
			request := reconciler.VSphereMachineToCluster(vsphereMachine)
			Expect(request).Should(BeNil())
		})

		It("Returns valid request with IP address", func() {
			vsphereMachine.Status.IPAddr = testIP
			request := reconciler.VSphereMachineToCluster(vsphereMachine)
			Expect(request).ShouldNot(BeNil())
			Expect(request[0].Namespace).Should(Equal(cluster.Namespace))
			Expect(request[0].Name).Should(Equal(cluster.Name))
		})
	})

	Context("Test reconcileDelete", func() {
		It("should mark specific resources to be in deleting conditions", func() {
			ctx.VSphereCluster.Status.Conditions = append(ctx.VSphereCluster.Status.Conditions,
				clusterv1.Condition{Type: infrav1.ResourcePolicyReadyCondition, Status: corev1.ConditionTrue})
			reconciler.reconcileDelete(ctx)
			c := conditions.Get(ctx.VSphereCluster, infrav1.ResourcePolicyReadyCondition)
			Expect(c).NotTo(BeNil())
			Expect(c.Status).To(Equal(corev1.ConditionFalse))
			Expect(c.Reason).To(Equal(clusterv1.DeletingReason))
		})

		It("should not mark other resources to be in deleting conditions", func() {
			otherReady := clusterv1.ConditionType("OtherReady")
			ctx.VSphereCluster.Status.Conditions = append(ctx.VSphereCluster.Status.Conditions,
				clusterv1.Condition{Type: otherReady, Status: corev1.ConditionTrue})
			reconciler.reconcileDelete(ctx)
			c := conditions.Get(ctx.VSphereCluster, otherReady)
			Expect(c).NotTo(BeNil())
			Expect(c.Status).NotTo(Equal(corev1.ConditionFalse))
			Expect(c.Reason).NotTo(Equal(clusterv1.DeletingReason))
		})
	})

	Context("Test getFailureDomains", func() {
		fss := isFaultDomainsFSSEnabled

		BeforeEach(func() {
			isFaultDomainsFSSEnabled = func() bool { return true }
		})

		AfterEach(func() {
			isFaultDomainsFSSEnabled = fss
		})

		It("should not find FailureDomains", func() {
			fds, err := reconciler.getFailureDomains(ctx)
			Expect(err).To(BeNil())
			Expect(fds).Should(HaveLen(0))
		})

		It("should find FailureDomains", func() {
			zoneNames := []string{"homer", "marge", "bart"}
			for _, name := range zoneNames {
				zone := &topologyv1.AvailabilityZone{
					TypeMeta: metav1.TypeMeta{
						APIVersion: topologyv1.GroupVersion.String(),
						Kind:       "AvailabilityZone",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
				}

				Expect(ctx.Client.Create(ctx, zone)).To(Succeed())
			}

			fds, err := reconciler.getFailureDomains(ctx)
			Expect(err).To(BeNil())
			Expect(fds).NotTo(BeNil())
			Expect(fds).Should(HaveLen(3))
		})
	})
})
