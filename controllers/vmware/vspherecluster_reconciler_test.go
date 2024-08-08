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
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirecord "k8s.io/client-go/tools/record"
	utilfeature "k8s.io/component-base/featuregate/testing"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	topologyv1 "sigs.k8s.io/cluster-api-provider-vsphere/internal/apis/topology/v1alpha1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/vmoperator"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func TestVSphereClusterReconciler(t *testing.T) {
	RegisterFailHandler(Fail)

	reporterConfig := types.NewDefaultReporterConfig()
	if artifactFolder, exists := os.LookupEnv("ARTIFACTS"); exists {
		reporterConfig.JUnitReport = filepath.Join(artifactFolder, "junit.ginkgo.controllers_vmware.xml")
	}
	RunSpecs(t, "VSphereCluster Controller Suite", reporterConfig)
}

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
		ctx                      = ctrl.SetupSignalHandler()
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
				clusterv1.Condition{Type: vmwarev1.ResourcePolicyReadyCondition, Status: corev1.ConditionTrue})
			reconciler.reconcileDelete(clusterCtx)
			c := conditions.Get(clusterCtx.VSphereCluster, vmwarev1.ResourcePolicyReadyCondition)
			Expect(c).NotTo(BeNil())
			Expect(c.Status).To(Equal(corev1.ConditionFalse))
			Expect(c.Reason).To(Equal(clusterv1.DeletingReason))
		})

		It("should not mark other resources to be in deleting conditions", func() {
			otherReady := clusterv1.ConditionType("OtherReady")
			clusterCtx.VSphereCluster.Status.Conditions = append(clusterCtx.VSphereCluster.Status.Conditions,
				clusterv1.Condition{Type: otherReady, Status: corev1.ConditionTrue})
			reconciler.reconcileDelete(clusterCtx)
			c := conditions.Get(clusterCtx.VSphereCluster, otherReady)
			Expect(c).NotTo(BeNil())
			Expect(c.Status).NotTo(Equal(corev1.ConditionFalse))
			Expect(c.Reason).NotTo(Equal(clusterv1.DeletingReason))
		})
	})

	Context("Test getFailureDomains", func() {
		It("should not find any FailureDomains if neither AvailabilityZone nor Zone exists", func() {
			fds, err := reconciler.getFailureDomains(ctx, clusterCtx.VSphereCluster.Namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(fds).Should(BeEmpty())
		})

		Context("when only AvailabilityZone exists", func() {
			BeforeEach(func() {
				azNames := []string{"az-1", "az-2", "az-3"}
				for _, name := range azNames {
					az := &topologyv1.AvailabilityZone{
						TypeMeta: metav1.TypeMeta{
							APIVersion: topologyv1.GroupVersion.String(),
							Kind:       "AvailabilityZone",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
					}

					Expect(controllerManagerContext.Client.Create(ctx, az)).To(Succeed())
				}
			})

			It("should discover FailureDomains using AvailabilityZone by default", func() {
				fds, err := reconciler.getFailureDomains(ctx, clusterCtx.VSphereCluster.Namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(fds).NotTo(BeNil())
				Expect(fds).Should(HaveLen(3))
			})

			It("should return nil when NamespaceScopedZone is enabled", func() {
				defer utilfeature.SetFeatureGateDuringTest(GinkgoTB(), feature.Gates, feature.NamespaceScopedZone, true)()
				fds, err := reconciler.getFailureDomains(ctx, clusterCtx.VSphereCluster.Namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(fds).To(BeNil())
			})
		})

		Context("when AvailabilityZone and Zone co-exists", func() {
			BeforeEach(func() {
				azNames := []string{"az-1", "az-2"}
				for _, name := range azNames {
					az := &topologyv1.AvailabilityZone{
						TypeMeta: metav1.TypeMeta{
							APIVersion: topologyv1.GroupVersion.String(),
							Kind:       "AvailabilityZone",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
					}
					Expect(controllerManagerContext.Client.Create(ctx, az)).To(Succeed())

				}
				zoneNames := []string{"zone-1", "zone-2", "zone-3"}
				for _, name := range zoneNames {
					zone := &topologyv1.Zone{
						TypeMeta: metav1.TypeMeta{
							APIVersion: topologyv1.GroupVersion.String(),
							Kind:       "Zone",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: clusterCtx.VSphereCluster.Namespace,
						},
					}

					Expect(controllerManagerContext.Client.Create(ctx, zone)).To(Succeed())
				}
			})

			It("should discover FailureDomains using AvailabilityZone by default", func() {
				fds, err := reconciler.getFailureDomains(ctx, clusterCtx.VSphereCluster.Namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(fds).NotTo(BeNil())
				Expect(fds).Should(HaveLen(2))
			})

			It("should discover FailureDomains using Zone when NamespaceScopedZone is enabled", func() {
				defer utilfeature.SetFeatureGateDuringTest(GinkgoTB(), feature.Gates, feature.NamespaceScopedZone, true)()

				fds, err := reconciler.getFailureDomains(ctx, clusterCtx.VSphereCluster.Namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(fds).NotTo(BeNil())
				Expect(fds).Should(HaveLen(3))
			})
		})

		Context("when Zone is marked for deleteion", func() {
			BeforeEach(func() {
				zoneNames := []string{"zone-1", "zone-2", "zone-3"}
				zoneNamespace := clusterCtx.VSphereCluster.Namespace
				for _, name := range zoneNames {
					zone := &topologyv1.Zone{
						TypeMeta: metav1.TypeMeta{
							APIVersion: topologyv1.GroupVersion.String(),
							Kind:       "Zone",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:       name,
							Namespace:  zoneNamespace,
							Finalizers: []string{"zone.test.finalizer"},
						},
					}

					Expect(controllerManagerContext.Client.Create(ctx, zone)).To(Succeed())

					if name == "zone-3" {
						// Delete the zone to set the deletion timestamp
						Expect(controllerManagerContext.Client.Delete(ctx, zone)).To(Succeed())
						Zone3 := &topologyv1.Zone{}
						Expect(controllerManagerContext.Client.Get(ctx, client.ObjectKey{Namespace: zoneNamespace, Name: name}, Zone3)).To(Succeed())

						// Validate the deletion timestamp
						Expect(Zone3.DeletionTimestamp.IsZero()).To(BeFalse())
					}
				}

			})

			It("should discover FailureDomains using Zone and filter out Zone marked for deletion", func() {
				defer utilfeature.SetFeatureGateDuringTest(GinkgoTB(), feature.Gates, feature.NamespaceScopedZone, true)()

				fds, err := reconciler.getFailureDomains(ctx, clusterCtx.VSphereCluster.Namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(fds).NotTo(BeNil())
				Expect(fds).Should(HaveLen(2))
			})

		})

	})
})
