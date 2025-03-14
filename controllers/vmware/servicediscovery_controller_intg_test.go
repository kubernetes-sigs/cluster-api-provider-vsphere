/*
Copyright 2022 The Kubernetes Authors.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers"
	vmwarehelpers "sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vmware"
)

var _ = Describe("Service Discovery controller integration tests", func() {
	// This test suite requires its own management cluster and controllers including clustercache.
	// Otherwise the workload cluster's kube-apiserver would not shutdown due to the global
	// test's clustercache still being running.
	var (
		intCtx             *vmwarehelpers.IntegrationTestContext
		testEnv            *helpers.TestEnvironment
		clusterCache       clustercache.ClusterCache
		clusterCacheCancel context.CancelFunc
		initObjects        []client.Object
	)
	BeforeEach(func() {
		var clusterCacheCtx context.Context
		clusterCacheCtx, clusterCacheCancel = context.WithCancel(ctx)
		testEnv, clusterCache = setup(clusterCacheCtx)
		intCtx = vmwarehelpers.NewIntegrationTestContextWithClusters(ctx, testEnv.Manager.GetClient())

		By(fmt.Sprintf("Creating the Cluster (%s), vSphereCluster (%s) and KubeconfigSecret", intCtx.Cluster.Name, intCtx.VSphereCluster.Name), func() {
			vmwarehelpers.CreateAndWait(ctx, intCtx.Client, intCtx.Cluster)
			vmwarehelpers.CreateAndWait(ctx, intCtx.Client, intCtx.VSphereCluster)
			vmwarehelpers.CreateAndWait(ctx, intCtx.Client, intCtx.KubeconfigSecret)
			vmwarehelpers.ClusterInfrastructureReady(ctx, intCtx.Client, clusterCache, intCtx.Cluster)
		})

		By("Verifying that the guest cluster client works")
		var guestClient client.Client
		var err error
		Eventually(func() error {
			guestClient, err = clusterCache.GetClient(ctx, client.ObjectKeyFromObject(intCtx.Cluster))
			return err
		}, time.Minute, 5*time.Second).Should(Succeed())
		// Note: Create a Service informer, so the test later doesn't fail if this doesn't work.
		Expect(guestClient.List(ctx, &corev1.ServiceList{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
	})
	AfterEach(func() {
		// Stop clustercache
		clusterCacheCancel()

		deleteTestResource(ctx, intCtx.Client, intCtx.VSphereCluster)
		deleteTestResource(ctx, intCtx.Client, intCtx.Cluster)
		deleteTestResource(ctx, intCtx.Client, intCtx.KubeconfigSecret)
		intCtx.AfterEach()
		Expect(testEnv.Stop()).To(Succeed())
	})

	Context("When VIP is available", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithIPStatus(),
			}
			createObjects(ctx, intCtx.Client, initObjects)
			Expect(intCtx.Client.Status().Update(ctx, newTestSupervisorLBServiceWithIPStatus())).To(Succeed())
		})
		AfterEach(func() {
			deleteObjects(ctx, intCtx.Client, initObjects)
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the VIP in the guest cluster")
			headlessSvc := &corev1.Service{}
			assertEventuallyExistsInNamespace(ctx, testEnv.Manager.GetAPIReader(), "kube-system", "kube-apiserver-lb-svc", headlessSvc)
			assertHeadlessSvcWithVIPEndpoints(ctx, intCtx.GuestAPIReader, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
		})
	})

	Context("When FIP is available", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithHost(testSupervisorAPIServerFIP)}
			createObjects(ctx, intCtx.Client, initObjects)
		})
		AfterEach(func() {
			deleteObjects(ctx, intCtx.Client, initObjects)
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the FIP in the guest cluster")
			assertHeadlessSvcWithFIPEndpoints(ctx, intCtx.GuestAPIReader, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
		})
	})
	Context("When headless svc and endpoints already exists", func() {
		BeforeEach(func() {
			// Create the svc & endpoint objects in guest cluster
			// NOTE: the service account controller is not creating this service because it gets re-queued for 2 minutes
			// after being created - when the cluster cache client is not ready.
			createObjects(ctx, intCtx.GuestClient, newTestHeadlessSvcEndpoints())

			// Init objects in the supervisor cluster
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithIPStatus()}
			createObjects(ctx, intCtx.Client, initObjects)
			Expect(intCtx.Client.Status().Update(ctx, newTestSupervisorLBServiceWithIPStatus())).To(Succeed())
		})
		AfterEach(func() {
			deleteObjects(ctx, intCtx.Client, initObjects)
			// Note: No need to delete guest cluster objects as a new guest cluster testenv endpoint is created for each test.
		})
		It("Should reconcile headless svc", func() {
			By("updating the service and endpoints using the VIP in the guest cluster")
			assertHeadlessSvcWithUpdatedVIPEndpoints(ctx, intCtx.GuestAPIReader, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
		})
	})
})
