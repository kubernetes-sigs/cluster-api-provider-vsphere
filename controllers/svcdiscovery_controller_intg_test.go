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
	goctx "context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/builder"
)

var _ = Describe("Service Discovery controller integration tests", func() {
	var (
		intCtx      *builder.IntegrationTestContext
		initObjects []client.Object
	)
	BeforeEach(func() {
		serviceDiscoveryTestSuite.SetIntegrationTestClient(testEnv.Manager.GetClient())
		intCtx = serviceDiscoveryTestSuite.NewIntegrationTestContextWithClusters(goctx.Background(), testEnv.Manager.GetClient(), true)
	})
	AfterEach(func() {
		intCtx.AfterEach()
		intCtx = nil
	})

	Context("When VIP is available", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithIPStatus(),
			}
			createObjects(intCtx, intCtx.Client, initObjects)
			Expect(intCtx.Client.Status().Update(ctx, newTestSupervisorLBServiceWithIPStatus())).To(Succeed())
		})
		AfterEach(func() {
			deleteObjects(intCtx, intCtx.Client, initObjects)
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the VIP in the guest cluster")
			headlessSvc := &corev1.Service{}
			assertEventuallyExistsInNamespace(intCtx, intCtx.Client, "kube-system", "kube-apiserver-lb-svc", headlessSvc)
			assertHeadlessSvcWithVIPEndpoints(intCtx, intCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
		})
	})

	Context("When FIP is available", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithHost(testSupervisorAPIServerFIP)}
			createObjects(intCtx, intCtx.Client, initObjects)
		})
		AfterEach(func() {
			deleteObjects(intCtx, intCtx.Client, initObjects)
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the FIP in the guest cluster")
			assertHeadlessSvcWithFIPEndpoints(intCtx, intCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
		})
	})
	Context("When headless svc and endpoints already exists", func() {
		BeforeEach(func() {
			// Create the svc & endpoint objects in guest cluster
			createObjects(intCtx, intCtx.GuestClient, newTestHeadlessSvcEndpoints())
			// Init objects in the supervisor cluster
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithIPStatus()}
			createObjects(intCtx, intCtx.Client, initObjects)
			Expect(intCtx.Client.Status().Update(ctx, newTestSupervisorLBServiceWithIPStatus())).To(Succeed())
		})
		AfterEach(func() {
			deleteObjects(intCtx, intCtx.Client, initObjects)
			// Note: No need to delete guest cluster objects as a new guest cluster testenv endpoint is created for each test.
		})
		It("Should reconcile headless svc", func() {
			By("updating the service and endpoints using the VIP in the guest cluster")
			assertHeadlessSvcWithUpdatedVIPEndpoints(intCtx, intCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
		})
	})
})
