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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	vmwarehelpers "sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	inframanager "sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
)

type dummyDualStackNetworkProvider struct {
	services.NetworkProvider
}

func (d *dummyDualStackNetworkProvider) SupportsIPv6DualStack() bool {
	return true
}

func (d *dummyDualStackNetworkProvider) HasLoadBalancer() bool {
	return true
}

type noSupervisorServiceNetworkProvider struct {
	services.NetworkProvider
}

func (n *noSupervisorServiceNetworkProvider) SupportsSupervisorService() bool {
	return false
}

type testNetworkProviderFactory struct {
	np services.NetworkProvider
}

func (f *testNetworkProviderFactory) ForCluster(_ context.Context, _ *vmwarev1.VSphereCluster) (services.NetworkProvider, error) {
	return f.np, nil
}

func newTestNetworkProviderFactory(np services.NetworkProvider) inframanager.NetworkProviderFactory {
	return &testNetworkProviderFactory{np: np}
}

var _ = Describe("ServiceDiscoveryReconciler reconcileNormal", serviceDiscoveryUnitTestsReconcileNormal)

func serviceDiscoveryUnitTestsReconcileNormal() {
	var (
		controllerCtx  *vmwarehelpers.UnitTestContextForController
		vsphereCluster vmwarev1.VSphereCluster
		initObjects    []client.Object
		reconciler     serviceDiscoveryReconciler
		netProvider    services.NetworkProvider
	)
	namespace := capiutil.RandomString(6)
	BeforeEach(func() {
		netProvider = network.DummyNetworkProvider()
	})
	JustBeforeEach(func() {
		vsphereCluster = fake.NewVSphereCluster(namespace)
		controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
		reconciler = serviceDiscoveryReconciler{
			Client:                 controllerCtx.ControllerManagerContext.Client,
			NetworkProviderFactory: newTestNetworkProviderFactory(netProvider),
		}
		err := reconciler.reconcileNormal(ctx, controllerCtx.GuestClusterContext, netProvider)
		Expect(err).NotTo(HaveOccurred())
	})
	JustAfterEach(func() {
		controllerCtx = nil
	})
	Context("When no VIP or FIP is available ", func() {
		It("Should reconcile headless svc", func() {
			By("creating a service and no endpoint in the guest cluster")
			assertHeadlessSvcWithNoEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.VSphereClusterServiceDiscoveryNotReadyReason)
		})
	})
	Context("When VIP is available", func() {
		BeforeEach(func() {
			initObjects = []client.Object{ //nolint:prealloc
				newTestSupervisorLBServiceWithIPStatus(),
			}
			initObjects = append(initObjects, newTestHeadlessSvcEndpoints()...)
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and endpoints using the VIP in the guest cluster")
			assertHeadlessSvcWithVIPEndpoints(ctx, controllerCtx.GuestClient, vmwarev1.SupervisorHeadlessSvcNamespace, vmwarev1.SupervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionTrue, "", vmwarev1.VSphereClusterServiceDiscoveryReadyReason)
		})
		It("Should get supervisor master endpoint IP", func() {
			r := &serviceDiscoveryReconciler{
				Client: controllerCtx.ControllerManagerContext.Client,
			}
			supervisorEndpointIPs, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, network.DummyNetworkProvider())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(supervisorEndpointIPs).To(Equal([]string{testSupervisorAPIServerVIP}))
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
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionTrue, "", vmwarev1.VSphereClusterServiceDiscoveryReadyReason)
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
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionTrue, "", vmwarev1.VSphereClusterServiceDiscoveryReadyReason)
		})
	})
	Context("When VIP is an hostname", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithHostnameStatus()}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and no endpoint in the guest cluster")
			assertHeadlessSvcWithNoEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			// Updated assertion: A hostname configuration inside a single-stack VIP field yields a discovery error.
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.VSphereClusterServiceDiscoveryNotReadyReason)
		})
	})
	Context("When FIP is an hostname", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithHost(testSupervisorAPIServerFIPHostName),
			}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and no endpoint in the guest cluster")
			assertHeadlessSvcWithNoEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionFalse, "must be an IP address",
				vmwarev1.VSphereClusterServiceDiscoveryNotReadyReason)
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
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.VSphereClusterServiceDiscoveryNotReadyReason)
		})
	})
	Context("When VIP is an invalid host", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestConfigMapWithHost("host^name"),
			}
		})
		It("Should reconcile headless svc", func() {
			By("creating a service and no endpoint in the guest cluster")
			assertHeadlessSvcWithNoEndpoints(ctx, controllerCtx.GuestClient, supervisorHeadlessSvcNamespace, supervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.VSphereClusterServiceDiscoveryNotReadyReason)
		})
	})
	Context("When DualStack is supported and IPv4/IPv6 VIPs are available", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithDualStackStatus(),
			}
		})
		It("Should get dual stack supervisor master endpoint IPs", func() {
			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{
					CIDRBlocks: []string{"192.168.0.0/16", "fd00:100:96::/48"},
				},
			}

			r := &serviceDiscoveryReconciler{
				Client: controllerCtx.ControllerManagerContext.Client,
			}
			supervisorEndpointIPs, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, &dummyDualStackNetworkProvider{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(supervisorEndpointIPs).To(Equal([]string{testSupervisorAPIServerVIP, testSupervisorAPIServerIPv6VIP}))
		})
	})
	Context("When DualStack is supported and only IPv6 VIP is available (IPv6 Single Stack)", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithIPv6Status(),
			}
		})
		It("Should get IPv6 supervisor master endpoint IP", func() {
			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{
					CIDRBlocks: []string{"fd00:100:96::/48"},
				},
			}

			r := &serviceDiscoveryReconciler{
				Client: controllerCtx.ControllerManagerContext.Client,
			}
			supervisorEndpointIPs, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, &dummyDualStackNetworkProvider{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(supervisorEndpointIPs).To(Equal([]string{testSupervisorAPIServerIPv6VIP}))
		})
	})
	Context("When DualStack is supported and VIP contains a hostname", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithHostnameStatus(),
			}
		})
		It("Should not panic and return error if no valid IP is found", func() {
			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{
					CIDRBlocks: []string{"192.168.0.0/16", "fd00:100:96::/48"},
				},
			}

			r := &serviceDiscoveryReconciler{
				Client: controllerCtx.ControllerManagerContext.Client,
			}
			_, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, &dummyDualStackNetworkProvider{})
			Expect(err).To(HaveOccurred())
			// Updated assertion: Verifies that failing to parse an IP correctly returns a distinct API server VIP error.
			Expect(err.Error()).To(ContainSubstring("failed to discover supervisor API server VIPs"))
		})
	})
	Context("getSupervisorAPIServerAddresses permutations", func() {
		var (
			r  serviceDiscoveryReconciler
			np services.NetworkProvider
		)

		JustBeforeEach(func() {
			np = &dummyDualStackNetworkProvider{}
			r = serviceDiscoveryReconciler{
				Client: controllerCtx.ControllerManagerContext.Client,
			}
		})

		It("should return error for IPv6 single stack cluster when only IPv4 VIP is available", func() {
			initObjects = []client.Object{newTestSupervisorLBServiceWithIPStatus()}
			vsphereCluster = fake.NewVSphereCluster(namespace)
			controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
			r.Client = controllerCtx.ControllerManagerContext.Client

			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"fd00:1::/64"}},
			}

			_, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, np)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no supervisor apiserver IPv6 VIP found for IPv6 single stack cluster"))
		})

		It("should return error when more than 2 VIPs are found", func() {
			svc := newTestSupervisorLBService()
			svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
				{IP: "10.0.0.1"},
				{IP: "10.0.0.2"},
				{IP: "fd00::1"},
			}
			initObjects = []client.Object{svc}
			vsphereCluster = fake.NewVSphereCluster(namespace)
			controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
			r.Client = controllerCtx.ControllerManagerContext.Client

			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16", "fd00:1::/64"}},
			}

			_, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, np)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("found too many VIPs"))
		})

		It("should return error when an invalid IP is found in VIPs", func() {
			svc := newTestSupervisorLBService()
			svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
				{IP: "invalid-ip"},
			}
			initObjects = []client.Object{svc}
			vsphereCluster = fake.NewVSphereCluster(namespace)
			controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
			r.Client = controllerCtx.ControllerManagerContext.Client

			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16", "fd00:1::/64"}},
			}

			_, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, np)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must be an IP address"))
		})

		It("should return IPv4 only for dual stack cluster when only IPv4 VIP is available", func() {
			initObjects = []client.Object{newTestSupervisorLBServiceWithIPStatus()}
			vsphereCluster = fake.NewVSphereCluster(namespace)
			controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
			r.Client = controllerCtx.ControllerManagerContext.Client

			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16", "fd00:1::/64"}},
			}

			vips, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, np)
			Expect(err).NotTo(HaveOccurred())
			Expect(vips).To(Equal([]string{testSupervisorAPIServerVIP}))
		})

		It("should return IPv6 only for dual stack cluster when only IPv6 VIP is available", func() {
			initObjects = []client.Object{newTestSupervisorLBServiceWithIPv6Status()}
			vsphereCluster = fake.NewVSphereCluster(namespace)
			controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
			r.Client = controllerCtx.ControllerManagerContext.Client

			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16", "fd00:1::/64"}},
			}

			vips, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, np)
			Expect(err).NotTo(HaveOccurred())
			Expect(vips).To(Equal([]string{testSupervisorAPIServerIPv6VIP}))
		})

		It("should succeed via FIP/VIP fallback when dual stack is NOT supported and cluster has NO CIDR blocks", func() {
			// Setup: NetworkProvider does not support dual stack
			np = network.DummyNetworkProvider()

			// Setup: VIP and FIP are available
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithIPStatus(),
				newTestConfigMapWithHost(testSupervisorAPIServerFIP),
			}
			vsphereCluster = fake.NewVSphereCluster(namespace)
			controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
			r.Client = controllerCtx.ControllerManagerContext.Client

			// Setup: Cluster has NO CIDR blocks
			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{}

			vips, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, np)
			Expect(err).NotTo(HaveOccurred())
			Expect(vips).To(Equal([]string{testSupervisorAPIServerVIP}))
		})

		It("should return IPv4 only for IPv4SingleStack cluster in a dual-stack enabled environment", func() {
			// Setup: NetworkProvider supports dual stack
			np = &dummyDualStackNetworkProvider{}

			// Setup: VIP and FIP are available
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithIPStatus(),
				newTestConfigMapWithHost(testSupervisorAPIServerFIP),
			}
			vsphereCluster = fake.NewVSphereCluster(namespace)
			controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
			r.Client = controllerCtx.ControllerManagerContext.Client

			// Setup: Cluster is IPv4 single stack
			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16"}},
			}

			vips, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, np)
			Expect(err).NotTo(HaveOccurred())
			Expect(vips).To(Equal([]string{testSupervisorAPIServerVIP}))
		})

		It("should return addresses in correct order for DualStackIPv4Primary", func() {
			np = &dummyDualStackNetworkProvider{}
			initObjects = []client.Object{newTestSupervisorLBServiceWithDualStackStatus()}
			vsphereCluster = fake.NewVSphereCluster(namespace)
			controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
			r.Client = controllerCtx.ControllerManagerContext.Client

			// IPv4 primary
			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16", "fd00:1::/64"}},
			}

			vips, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, np)
			Expect(err).NotTo(HaveOccurred())
			Expect(vips).To(Equal([]string{testSupervisorAPIServerVIP, testSupervisorAPIServerIPv6VIP}))
		})

		It("should return addresses in correct order for DualStackIPv6Primary", func() {
			np = &dummyDualStackNetworkProvider{}
			initObjects = []client.Object{newTestSupervisorLBServiceWithDualStackStatus()}
			vsphereCluster = fake.NewVSphereCluster(namespace)
			controllerCtx = vmwarehelpers.NewUnitTestContextForController(ctx, namespace, &vsphereCluster, false, initObjects, nil)
			r.Client = controllerCtx.ControllerManagerContext.Client

			// IPv6 primary
			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"fd00:1::/64", "192.168.0.0/16"}},
			}

			vips, err := r.getSupervisorAPIServerAddresses(ctx, controllerCtx.Cluster, np)
			Expect(err).NotTo(HaveOccurred())
			Expect(vips).To(Equal([]string{testSupervisorAPIServerIPv6VIP, testSupervisorAPIServerVIP}))
		})
	})
	Context("When DualStack is supported and IPv4/IPv6 VIPs are available (reconcileNormal)", func() {
		BeforeEach(func() {
			initObjects = []client.Object{
				newTestSupervisorLBServiceWithDualStackStatus(),
			}
			netProvider = &dummyDualStackNetworkProvider{}
		})
		It("Should reconcile headless svc with dual stack endpoints", func() {
			controllerCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{
					CIDRBlocks: []string{"192.168.0.0/16", "fd00:100:96::/48"},
				},
			}

			// We need to re-run reconcileNormal because JustBeforeEach already ran with the default cluster network
			err := reconciler.reconcileNormal(ctx, controllerCtx.GuestClusterContext, netProvider)
			Expect(err).NotTo(HaveOccurred())

			By("creating a service and endpoints using both VIPs in the guest cluster")
			assertHeadlessSvcWithDualStackEndpoints(ctx, controllerCtx.GuestClient, vmwarev1.SupervisorHeadlessSvcNamespace, vmwarev1.SupervisorHeadlessSvcName)
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionTrue, "", vmwarev1.VSphereClusterServiceDiscoveryReadyReason)
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
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.VSphereClusterServiceDiscoveryNotReadyReason)
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
			assertServiceDiscoveryCondition(controllerCtx.VSphereCluster, metav1.ConditionFalse, "Failed to discover supervisor API server endpoint",
				vmwarev1.VSphereClusterServiceDiscoveryNotReadyReason)
		})
	})
}

var _ = Describe("ServiceDiscoveryReconciler Reconcile skip supervisor service", func() {
	It("marks ServiceDiscoveryReady and creates no Service/Endpoints when SupportsSupervisorService is false", func() {
		namespace := capiutil.RandomString(6)
		clusterName := "skip-supervisor-svc"
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: namespace,
			},
		}
		vsphereCluster := &vmwarev1.VSphereCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
			},
		}

		controllerManagerCtx := fake.NewControllerManagerContext(cluster, vsphereCluster)
		guestClient := fake.NewFakeGuestClusterClient()
		np := &noSupervisorServiceNetworkProvider{NetworkProvider: network.DummyNetworkProvider()}
		reconciler := serviceDiscoveryReconciler{
			Client:                 controllerManagerCtx.Client,
			NetworkProviderFactory: newTestNetworkProviderFactory(np),
			// clusterCache intentionally nil: skip path must return before GetClient.
		}

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(vsphereCluster),
		})
		Expect(err).NotTo(HaveOccurred())

		updated := &vmwarev1.VSphereCluster{}
		Expect(controllerManagerCtx.Client.Get(ctx, client.ObjectKeyFromObject(vsphereCluster), updated)).To(Succeed())
		assertServiceDiscoveryCondition(updated, metav1.ConditionTrue, "", vmwarev1.VSphereClusterServiceDiscoveryReadyReason)

		svc := &corev1.Service{}
		err = guestClient.Get(ctx, client.ObjectKey{Namespace: supervisorHeadlessSvcNamespace, Name: supervisorHeadlessSvcName}, svc)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		eps := &corev1.Endpoints{}
		err = guestClient.Get(ctx, client.ObjectKey{Namespace: supervisorHeadlessSvcNamespace, Name: supervisorHeadlessSvcName}, eps)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
})
