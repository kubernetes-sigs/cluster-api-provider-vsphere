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

package controllers

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	helpers "sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vmware"
)

var _ = Describe("Service Discovery controller integration tests", func() {
	var (
		intCtx      *helpers.IntegrationTestContext
		initObjects []client.Object
	)
	BeforeEach(func() {
		intCtx = helpers.NewIntegrationTestContextWithClusters(ctx, testEnv.Manager.GetClient())
		By(fmt.Sprintf("Creating the Cluster (%s), vSphereCluster (%s) and KubeconfigSecret", intCtx.Cluster.Name, intCtx.VSphereCluster.Name), func() {
			helpers.CreateAndWait(ctx, intCtx.Client, intCtx.Cluster)
			helpers.CreateAndWait(ctx, intCtx.Client, intCtx.VSphereCluster)
			helpers.CreateAndWait(ctx, intCtx.Client, intCtx.KubeconfigSecret)
		})

		By("Verifying that the guest cluster client works")
		var guestClient client.Client
		var err error
		Eventually(func() error {
			guestClient, err = tracker.GetClient(ctx, client.ObjectKeyFromObject(intCtx.Cluster))
			return err
		}, time.Minute, 5*time.Second).Should(Succeed())
		// Note: Create a Service informer, so the test later doesn't fail if this doesn't work.
		Expect(guestClient.List(ctx, &corev1.ServiceList{}, client.InNamespace(metav1.NamespaceDefault))).To(Succeed())
	})
	AfterEach(func() {
		deleteTestResource(ctx, intCtx.Client, intCtx.VSphereCluster)
		deleteTestResource(ctx, intCtx.Client, intCtx.Cluster)
		deleteTestResource(ctx, intCtx.Client, intCtx.KubeconfigSecret)
		intCtx.AfterEach()
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
