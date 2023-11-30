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

package controllers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	helpers "sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vmware"
)

var _ = Describe("ServiceDiscoveryReconciler reconcileNormal", serviceDiscoveryUnitTestsReconcileNormal)

func serviceDiscoveryUnitTestsReconcileNormal() {
	var (
		controllerCtx  *helpers.UnitTestContextForController
		vsphereCluster vmwarev1.VSphereCluster
		initObjects    []client.Object
		reconciler     serviceDiscoveryReconciler
	)
	namespace := capiutil.RandomString(6)
	JustBeforeEach(func() {
		vsphereCluster = fake.NewVSphereCluster(namespace)
		controllerCtx = helpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
		reconciler = serviceDiscoveryReconciler{
			Client: controllerCtx.ControllerManagerContext.Client,
		}
		err := reconciler.reconcileNormal(ctx, controllerCtx.GuestClusterContext)
		Expect(err).NotTo(HaveOccurred())
	})
	JustAfterEach(func() {
		controllerCtx = nil
	})
	Context("When no VIP or FIP is available ", func() {
		It("Should reconcile headless svc", func() {
			By("creating a service and no endpoint in the guest cluster")
			assertHeadlessSvcWithNoEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.SupervisorHeadlessServiceSetupFailedReason, clusterv1.ConditionSeverityWarning)
		})
	})
	Context("When VIP is available", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithIPStatus(),
			}
			initObjects = append(initObjects, newTestHeadlessSvcEndpoints()...)
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the VIP in the guest cluster")
			assertHeadlessSvcWithVIPEndpoints(ctx, controllerCtx.GuestClient, vmwarev1.SupervisorHeadlessSvcNamespace, vmwarev1.SupervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
		})
		It("Should get supervisor master endpoint IP", func() {
			r := &serviceDiscoveryReconciler{
				Client: controllerCtx.ControllerManagerContext.Client,
			}
			supervisorEndpointIP, err := r.getSupervisorAPIServerAddress(ctx)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(supervisorEndpointIP).To(Equal(testSupervisorAPIServerVIP))
		})
	})
	Context("When FIP is available", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithHost(testSupervisorAPIServerFIP)}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the FIP in the guest cluster")
			assertHeadlessSvcWithFIPEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
		})
	})
	Context("When VIP and FIP are available", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithIPStatus(),
				newTestConfigMapWithHost(testSupervisorAPIServerFIP),
			}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the VIP in the guest cluster")
			assertHeadlessSvcWithVIPEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
		})
	})
	Context("When VIP is an hostname", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithHostnameStatus()}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the VIP in the guest cluster")
			assertHeadlessSvcWithVIPHostnameEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
		})
	})
	Context("When FIP is an hostname", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithHost(testSupervisorAPIServerFIPHostName),
			}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the FIP in the guest cluster")
			assertHeadlessSvcWithFIPHostNameEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionTrue, "", "", "")
		})
	})
	Context("When FIP is an empty hostname", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithHost(""),
			}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and no endpoint in the guest cluster")
			assertHeadlessSvcWithNoEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.SupervisorHeadlessServiceSetupFailedReason, clusterv1.ConditionSeverityWarning)
		})
	})
	Context("When FIP is an invalid host", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithHost("host^name"),
			}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and no endpoint in the guest cluster")
			assertHeadlessSvcWithNoEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.SupervisorHeadlessServiceSetupFailedReason, clusterv1.ConditionSeverityWarning)
		})
	})
	Context("When FIP config map has invalid kubeconfig data", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithData(
					map[string]string{
						bootstrapapi.KubeConfigKey: "invalid-kubeconfig-data",
					}),
			}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and no endpoint in the guest cluster")
			assertHeadlessSvcWithNoEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.SupervisorHeadlessServiceSetupFailedReason, clusterv1.ConditionSeverityWarning)
		})
	})
	Context("When FIP config map has invalid kubeconfig key", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithData(
					map[string]string{
						"invalid-key": "invalid-kubeconfig-data",
					}),
			}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and no endpoint in the guest cluster")
			assertHeadlessSvcWithNoEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, corev1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.SupervisorHeadlessServiceSetupFailedReason, clusterv1.ConditionSeverityWarning)
		})
	})
}
