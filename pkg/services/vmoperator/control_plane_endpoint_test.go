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

package vmoperator

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	netopv1 "github.com/vmware-tanzu/net-operator-api/api/v1alpha1"
	ncpv1 "github.com/vmware-tanzu/vm-operator/external/ncp/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/component-base/featuregate"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func getVirtualMachineService(ctx context.Context, clusterCtx *vmware.ClusterContext, _ ctrlclient.Client, cpService CPService) *vmoprvhub.VirtualMachineService {
	vms, err := cpService.getVMControlPlaneService(ctx, clusterCtx)
	if err != nil {
		// If it's a NotFound OR our specific Zombie errors, return nil so verifyOutput can run
		if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "owned by a different VSphereCluster instance") ||
			strings.Contains(err.Error(), "exists but is being deleted") {
			return nil
		}
		// If it's any other error, the test should fail immediately
		Expect(err).NotTo(HaveOccurred())
	}

	return vms
}

func createVnet(ctx context.Context, clusterCtx *vmware.ClusterContext, c ctrlclient.Client) {
	vnet := &ncpv1.VirtualNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterCtx.VSphereCluster.Namespace,
			Name:      network.GetNSXTVirtualNetworkName(clusterCtx.VSphereCluster.Name),
		},
	}
	Expect(c.Create(ctx, vnet)).To(Succeed())
}

func createDefaultNetwork(ctx context.Context, clusterCtx *vmware.ClusterContext, c ctrlclient.Client) {
	defaultNetwork := &netopv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummy-network",
			Namespace: clusterCtx.VSphereCluster.Namespace,
			Labels:    map[string]string{network.CAPVDefaultNetworkLabel: "true"},
		},
		Spec: netopv1.NetworkSpec{
			Type: netopv1.NetworkTypeVDS,
		},
	}
	Expect(c.Create(ctx, defaultNetwork)).To(Succeed())
}

func updateVMServiceWithVIPs(ctx context.Context, clusterCtx *vmware.ClusterContext, c ctrlclient.Client, cpService CPService, vips ...string) {
	vmService := getVirtualMachineService(ctx, clusterCtx, c, cpService)

	sOriginal := vmService.DeepCopy()
	ingresses := make([]vmoprvhub.LoadBalancerIngress, 0, len(vips))
	for _, vip := range vips {
		ingresses = append(ingresses, vmoprvhub.LoadBalancerIngress{IP: vip})
	}
	vmService.Status.LoadBalancer.Ingress = ingresses

	err := c.Status().Patch(ctx, vmService, ctrlclient.MergeFrom(sOriginal))
	Expect(err).ShouldNot(HaveOccurred())
}

type dummyDualStackNetworkProvider struct {
	services.NetworkProvider
}

func (d *dummyDualStackNetworkProvider) SupportsIPv6DualStack() bool {
	return true
}

func (d *dummyDualStackNetworkProvider) HasLoadBalancer() bool {
	return true
}

var _ services.NetworkProvider = &dummyDualStackNetworkProvider{}

var _ = Describe("ControlPlaneEndpoint Tests", func() {
	const (
		clusterName          = "test-cluster"
		vip                  = "127.0.0.1"
		noNetworkFailure     = "failed to get provider VirtualMachineService annotations"
		waitingForVIPFailure = "LoadBalancer does not have any Ingresses"
	)
	var (
		err                         error
		expectReconcileError        bool
		expectAPIEndpoint           bool
		expectVMS                   bool
		expectedType                vmoprvhub.VirtualMachineServiceType
		expectedHost                string
		expectedPort                int
		expectedAnnotations         map[string]string
		expectedClusterRoleVMLabels map[string]string
		expectedConditions          []metav1.Condition

		cluster                  *clusterv1.Cluster
		vsphereCluster           *vmwarev1.VSphereCluster
		ctx                      = context.Background()
		clusterCtx               *vmware.ClusterContext
		controllerManagerContext *capvcontext.ControllerManagerContext
		c                        ctrlclient.Client

		apiEndpoint *vmwarev1.APIEndpoint
		vms         *vmoprvhub.VirtualMachineService

		cpService CPService
	)

	BeforeEach(func() {
		// Default values
		expectedHost = ""
		expectedPort = 0
		expectedAnnotations = make(map[string]string)
		expectedConditions = nil

		// Create all necessary dependencies
		cluster = util.CreateCluster(clusterName)
		cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
			Pods: clusterv1.NetworkRanges{
				CIDRBlocks: []string{"192.168.0.0/16"},
			},
			Services: clusterv1.NetworkRanges{
				CIDRBlocks: []string{"10.96.0.0/12"},
			},
		}
		vsphereCluster = util.CreateVSphereCluster(clusterName)
		clusterCtx, controllerManagerContext = util.CreateClusterContext(cluster, vsphereCluster)
		c = controllerManagerContext.Client
		expectedClusterRoleVMLabels = clusterRoleVMLabels(clusterCtx, true)
		cpService = CPService{
			Client: c,
		}
	})

	Context("Reconcile ControlPlaneEndpointService", func() {
		verifyOutput := func() {
			Expect(err != nil).Should(Equal(expectReconcileError))

			Expect(apiEndpoint != nil).Should(Equal(expectAPIEndpoint))
			if apiEndpoint != nil {
				Expect(apiEndpoint.Host).To(Equal(expectedHost))
				Expect(apiEndpoint.Port).To(BeEquivalentTo(expectedPort))
			}

			vms = getVirtualMachineService(ctx, clusterCtx, c, cpService)
			Expect(vms != nil).Should(Equal(expectVMS))
			if vms != nil {
				Expect(vms.Spec.Type).To(Equal(expectedType))
				for k, v := range expectedAnnotations {
					Expect(vms.Annotations).To(HaveKeyWithValue(k, v))
				}
				Expect(vms.Spec.Ports).To(HaveLen(1))
				Expect(vms.Spec.Ports[0].Name).To(Equal(controlPlaneServiceAPIServerPortName))
				Expect(vms.Spec.Ports[0].Protocol).To(Equal("TCP"))
				Expect(vms.Spec.Ports[0].Port).To(Equal(int32(defaultAPIBindPort)))
				Expect(vms.Spec.Ports[0].TargetPort).To(Equal(int32(defaultAPIBindPort)))
				Expect(vms.Spec.Selector).To(Equal(expectedClusterRoleVMLabels))
			}

			for _, expectedCondition := range expectedConditions {
				c := conditions.Get(clusterCtx.VSphereCluster, expectedCondition.Type)
				Expect(c).NotTo(BeNil())
				Expect(c.Status).To(Equal(expectedCondition.Status))
				Expect(c.Reason).To(Equal(expectedCondition.Reason))
				if expectedCondition.Message != "" {
					Expect(c.Message).To(ContainSubstring(expectedCondition.Message))
				} else {
					Expect(c.Message).To(BeEmpty())
				}
			}
		}

		Specify("Handle Zombie VSphereCluster owned VirtualMachineService", func() {
			By("Creating a VirtualMachineService with a stale Owner UID")
			vmServiceName := controlPlaneVMServiceName(clusterCtx.Cluster.Name)
			staleUID := types.UID("stale-cluster-uid-123")
			zombieVMS := &vmoprvhub.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vmServiceName,
					Namespace: clusterCtx.Cluster.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: vmwarev1.GroupVersion.String(),
							Kind:       "VSphereCluster",
							Name:       clusterCtx.VSphereCluster.Name,
							UID:        staleUID,
						},
					},
				},
			}
			Expect(c.Create(ctx, zombieVMS)).To(Succeed())

			By("Expect the specific UID mismatch error")
			expectReconcileError = true
			expectAPIEndpoint = false
			expectVMS = false
			expectedConditions = []metav1.Condition{
				{
					Type:    vmwarev1.VSphereClusterLoadBalancerReadyCondition,
					Status:  metav1.ConditionFalse,
					Reason:  vmwarev1.VSphereClusterLoadBalancerNotReadyReason,
					Message: fmt.Sprintf("owned by a different VSphereCluster instance %s", staleUID),
				},
			}

			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, network.DummyLBNetworkProvider())

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("owned by a different VSphereCluster instance"))
			verifyOutput()
		})

		Specify("Handle Zombie resource being deleted", func() {
			By("Creating a VirtualMachineService with a DeletionTimestamp")
			vmServiceName := controlPlaneVMServiceName(clusterCtx.Cluster.Name)
			deletingVMS := &vmoprvhub.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name:       vmServiceName,
					Namespace:  clusterCtx.Cluster.Namespace,
					Finalizers: []string{"capv.vmware.com/test-finalizer"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: vmwarev1.GroupVersion.String(),
							Kind:       "VSphereCluster",
							Name:       clusterCtx.VSphereCluster.Name,
							UID:        clusterCtx.VSphereCluster.UID,
						},
					},
				},
			}
			Expect(c.Create(ctx, deletingVMS)).To(Succeed())
			Expect(c.Delete(ctx, deletingVMS)).To(Succeed())

			By("Expect the specific deletion error and condition update")
			expectReconcileError = true
			expectAPIEndpoint = false
			expectVMS = false
			expectedConditions = []metav1.Condition{
				{
					Type:    vmwarev1.VSphereClusterLoadBalancerReadyCondition,
					Status:  metav1.ConditionFalse,
					Reason:  vmwarev1.VSphereClusterLoadBalancerNotReadyReason,
					Message: "exists but is being deleted",
				},
			}

			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, network.DummyLBNetworkProvider())

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exists but is being deleted"))
			verifyOutput()
		})

		// If there is no load balancer, Reconcile should be a no-op
		Specify("NetworkProvider has no LoadBalancer", func() {
			expectReconcileError = false
			expectAPIEndpoint = false
			expectVMS = false
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, network.DummyNetworkProvider())
			Expect(conditions.Get(clusterCtx.VSphereCluster, vmwarev1.VSphereClusterLoadBalancerReadyCondition)).To(BeNil())
			verifyOutput()
		})

		Specify("DummyLBNetworkProvider has a LoadBalancer", func() {
			expectReconcileError = true // VirtualMachineService LB does not yet have VIP assigned
			expectAPIEndpoint = false
			expectVMS = true
			expectedType = vmoprvhub.VirtualMachineServiceTypeLoadBalancer
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, network.DummyLBNetworkProvider())
			verifyOutput()

			// Set a VIP and reconcile again.
			expectReconcileError = false
			expectAPIEndpoint = true
			updateVMServiceWithVIPs(ctx, clusterCtx, c, cpService, vip)
			expectedPort = defaultAPIBindPort
			expectedHost = vip
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, network.DummyLBNetworkProvider())
			verifyOutput()
			vmService := getVirtualMachineService(ctx, clusterCtx, c, cpService)
			Expect(vmService.Name).To(Equal(controlPlaneVMServiceName(clusterCtx.Cluster.Name)))

			// Delete the apiservice and recreate using the legacy name and reconcile again.
			Expect(c.Delete(ctx, vmService.DeepCopy())).To(Succeed())
			vmService.Name = legacyControlPlaneVMServiceName(clusterCtx.Cluster.Name)
			vmService.ResourceVersion = ""
			Expect(c.Create(ctx, vmService)).To(Succeed())
			updateVMServiceWithVIPs(ctx, clusterCtx, c, cpService, vip)
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, network.DummyLBNetworkProvider())
			verifyOutput()
			vmService = getVirtualMachineService(ctx, clusterCtx, c, cpService)
			Expect(vmService.Name).To(Equal(legacyControlPlaneVMServiceName(clusterCtx.Cluster.Name)))
		})

		Specify("Reconcile VirtualMachineService for NetOp", func() {
			// Reconcile should return an error up and until all prerequisites have been met
			expectReconcileError = true
			// An APIEndpoint is only returned if reconcile succeeds
			expectAPIEndpoint = false
			// A VirtualMachineService is only created once all prerequisites have been met
			expectVMS = false
			expectedType = vmoprvhub.VirtualMachineServiceTypeLoadBalancer

			// The NetOp network provider looks a Network. If one does not exist, it will fail.
			By("NetOp NetworkProvider has no Network")
			netOpProvider := network.NetOpNetworkProvider(c)
			// we expect the reconciliation fail because lack of bootstrap data
			expectedConditions = append(expectedConditions, metav1.Condition{
				Type:    vmwarev1.VSphereClusterLoadBalancerReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  vmwarev1.VSphereClusterLoadBalancerNotReadyReason,
				Message: noNetworkFailure,
			})
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, netOpProvider)
			verifyOutput()

			// If a Network is present, a VirtualMachineService should be created
			By("NetOp NetworkProvider has a Network with no VIP")
			// A VirtualMachineService should be created and will wait for a VIP to be assigned
			expectedAnnotations["netoperator.vmware.com/network-name"] = "dummy-network"
			expectVMS = true
			createDefaultNetwork(ctx, clusterCtx, c)
			expectedConditions[0].Reason = vmwarev1.VSphereClusterLoadBalancerWaitingForIPReason
			expectedConditions[0].Message = waitingForVIPFailure
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, netOpProvider)
			verifyOutput()

			// Once a VIP has been created, a VirtualMachineService should exist with a valid endpoint
			By("NetOP NetworkProvider has a Service with a VIP")
			expectReconcileError = false
			expectAPIEndpoint = true
			expectedPort = defaultAPIBindPort
			expectedHost = vip
			updateVMServiceWithVIPs(ctx, clusterCtx, c, cpService, vip)
			expectedConditions[0].Status = metav1.ConditionTrue
			expectedConditions[0].Reason = vmwarev1.VSphereClusterLoadBalancerReadyReason
			expectedConditions[0].Message = ""
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, netOpProvider)
			verifyOutput()
		})

		Specify("Reconcile VirtualMachineService for NSX-T", func() {
			// Reconcile should return an error up and until all prerequisites have been met
			expectReconcileError = true
			// An APIEndpoint is only returned if reconcile succeeds
			expectAPIEndpoint = false
			// A VirtualMachineService is only created once all prerequisites have been met
			expectVMS = false
			expectedType = vmoprvhub.VirtualMachineServiceTypeLoadBalancer
			expectedConditions = append(expectedConditions, metav1.Condition{
				Type:    vmwarev1.VSphereClusterLoadBalancerReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  vmwarev1.VSphereClusterLoadBalancerNotReadyReason,
				Message: noNetworkFailure,
			})

			// The NSXT network provider looks for a real vnet. If one does not exist, it will fail.
			By("NSXT NetworkProvider has no vnet")
			nsxtProvider := network.NsxtNetworkProvider(c, "false")
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, nsxtProvider)
			verifyOutput()

			// If a vnet is present, a VirtualMachineService should be created
			By("NSXT NetworkProvider has a vnet with no VIP")
			// A VirtualMachineService should be created and will wait for a VIP to be assigned
			expectedVnetName := network.GetNSXTVirtualNetworkName(clusterName)
			expectedAnnotations["ncp.vmware.com/virtual-network-name"] = expectedVnetName
			expectVMS = true
			expectedConditions[0].Reason = vmwarev1.VSphereClusterLoadBalancerWaitingForIPReason
			expectedConditions[0].Message = waitingForVIPFailure
			createVnet(ctx, clusterCtx, c)
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, nsxtProvider)
			verifyOutput()

			// Once a VIP has been created, a VirtualMachineService should exist with a valid endpoint
			By("NSXT NetworkProvider has a vnet with a VIP")
			expectReconcileError = false
			expectAPIEndpoint = true
			expectedPort = defaultAPIBindPort
			expectedHost = vip
			expectedConditions[0].Status = metav1.ConditionTrue
			expectedConditions[0].Reason = vmwarev1.VSphereClusterLoadBalancerReadyReason
			expectedConditions[0].Message = ""
			updateVMServiceWithVIPs(ctx, clusterCtx, c, cpService, vip)
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, nsxtProvider)
			verifyOutput()
		})
	})

	Context("Reconcile ControlPlaneEndpointService with IPFamily settings (v1alpha6)", func() {

		BeforeEach(func() {
			// Override clusterCtx with v1alpha6 context
			clusterCtx, controllerManagerContext = util.CreateClusterContextV1Alpha6(cluster, vsphereCluster)
			c = controllerManagerContext.Client
			cpService = CPService{Client: c}

			// Enable feature gate for these tests
			err := feature.Gates.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=true", feature.IPv6DualStack))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			// Disable feature gate
			err := feature.Gates.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=false", feature.IPv6DualStack))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set IPFamilyPolicy and IPFamilies for IPv4 single stack", func() {
			clusterCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16"}},
			}

			_, err := cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, &dummyDualStackNetworkProvider{NetworkProvider: network.DummyLBNetworkProvider()})
			Expect(err).To(HaveOccurred()) // Waiting for VIP

			vms := getVirtualMachineService(ctx, clusterCtx, c, cpService)
			Expect(vms).NotTo(BeNil())
			Expect(vms.Spec.IPFamilyPolicy).NotTo(BeNil())
			Expect(*vms.Spec.IPFamilyPolicy).To(Equal(corev1.IPFamilyPolicySingleStack))
			Expect(vms.Spec.IPFamilies).To(Equal([]corev1.IPFamily{corev1.IPv4Protocol}))
		})

		It("should set IPFamilyPolicy and IPFamilies for IPv6 single stack", func() {
			clusterCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"fd00:1::/64"}},
			}

			_, err := cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, &dummyDualStackNetworkProvider{NetworkProvider: network.DummyLBNetworkProvider()})
			Expect(err).To(HaveOccurred()) // Waiting for VIP

			vms := getVirtualMachineService(ctx, clusterCtx, c, cpService)
			Expect(vms).NotTo(BeNil())
			Expect(vms.Spec.IPFamilyPolicy).NotTo(BeNil())
			Expect(*vms.Spec.IPFamilyPolicy).To(Equal(corev1.IPFamilyPolicySingleStack))
			Expect(vms.Spec.IPFamilies).To(Equal([]corev1.IPFamily{corev1.IPv6Protocol}))
		})

		It("should set IPFamilyPolicy and IPFamilies for DualStack IPv4 primary", func() {
			clusterCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16", "fd00:1::/64"}},
			}

			_, err := cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, &dummyDualStackNetworkProvider{NetworkProvider: network.DummyLBNetworkProvider()})
			Expect(err).To(HaveOccurred()) // Waiting for VIP

			vms := getVirtualMachineService(ctx, clusterCtx, c, cpService)
			Expect(vms).NotTo(BeNil())
			Expect(vms.Spec.IPFamilyPolicy).NotTo(BeNil())
			Expect(*vms.Spec.IPFamilyPolicy).To(Equal(corev1.IPFamilyPolicyRequireDualStack))
			Expect(vms.Spec.IPFamilies).To(Equal([]corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}))
		})

		It("should set IPFamilyPolicy and IPFamilies for DualStack IPv6 primary", func() {
			clusterCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"fd00:1::/64", "192.168.0.0/16"}},
			}

			_, err := cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, &dummyDualStackNetworkProvider{NetworkProvider: network.DummyLBNetworkProvider()})
			Expect(err).To(HaveOccurred()) // Waiting for VIP

			vms := getVirtualMachineService(ctx, clusterCtx, c, cpService)
			Expect(vms).NotTo(BeNil())
			Expect(vms.Spec.IPFamilyPolicy).NotTo(BeNil())
			Expect(*vms.Spec.IPFamilyPolicy).To(Equal(corev1.IPFamilyPolicyRequireDualStack))
			Expect(vms.Spec.IPFamilies).To(Equal([]corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv4Protocol}))
		})

		It("should return error if topology is DualStack but network provider does not support it", func() {
			clusterCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16", "fd00:1::/64"}},
			}

			_, err := cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, network.DummyLBNetworkProvider())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("IPv6 and DualStack require the IPv6DualStack feature gate, VM Operator v1alpha6+, and a network provider that supports it"))
		})

		It("should fall back to IPv4SingleStack logic when ipv6DualStackSupported is false", func() {
			// Setup: IPv4 cluster
			clusterCtx.Cluster.Spec.ClusterNetwork = clusterv1.ClusterNetwork{
				Pods: clusterv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16"}},
			}

			// Setup: Network provider does NOT support dual stack
			netProvider := network.DummyLBNetworkProvider() // DummyLBNetworkProvider returns false for SupportsIPv6DualStack by default

			// 1. Reconcile - should create VMS and wait for VIP
			_, err := cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, netProvider)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(waitingForVIPFailure))

			vms := getVirtualMachineService(ctx, clusterCtx, c, cpService)
			Expect(vms).NotTo(BeNil())
			// IPFamilyPolicy and IPFamilies should NOT be set because dual stack is not supported
			Expect(vms.Spec.IPFamilyPolicy).To(BeNil())
			Expect(vms.Spec.IPFamilies).To(BeNil())

			// 2. Assign VIP and reconcile again
			updateVMServiceWithVIPs(ctx, clusterCtx, c, cpService, "10.0.0.1")
			endpoint, err := cpService.ReconcileControlPlaneEndpointService(ctx, clusterCtx, netProvider)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpoint).NotTo(BeNil())
			Expect(endpoint.Host).To(Equal("10.0.0.1"))
		})
	})
})

var _ = Describe("VIP helper functions", func() {
	var (
		vmService *vmoprvhub.VirtualMachineService
	)

	BeforeEach(func() {
		vmService = &vmoprvhub.VirtualMachineService{
			Spec: vmoprvhub.VirtualMachineServiceSpec{
				Type: vmoprvhub.VirtualMachineServiceTypeLoadBalancer,
			},
		}
	})

	Context("getAndValidateVIPs", func() {
		It("should return IPv4 VIP for IPv4SingleStack", func() {
			vmService.Status.LoadBalancer.Ingress = []vmoprvhub.LoadBalancerIngress{
				{IP: "10.0.0.1"},
				{IP: "fd00::1"},
			}
			primary, required, err := getAndValidateVIPs(vmService, util.IPv4SingleStack)
			Expect(err).NotTo(HaveOccurred())
			Expect(primary).To(Equal("10.0.0.1"))
			Expect(required).To(Equal([]string{"10.0.0.1"}))
		})

		It("should return IPv6 VIP for IPv6SingleStack", func() {
			vmService.Status.LoadBalancer.Ingress = []vmoprvhub.LoadBalancerIngress{
				{IP: "10.0.0.1"},
				{IP: "fd00::1"},
			}
			primary, required, err := getAndValidateVIPs(vmService, util.IPv6SingleStack)
			Expect(err).NotTo(HaveOccurred())
			Expect(primary).To(Equal("fd00::1"))
			Expect(required).To(Equal([]string{"fd00::1"}))
		})

		It("should return both VIPs for DualStackIPv4Primary", func() {
			vmService.Status.LoadBalancer.Ingress = []vmoprvhub.LoadBalancerIngress{
				{IP: "10.0.0.1"},
				{IP: "fd00::1"},
			}
			primary, required, err := getAndValidateVIPs(vmService, util.DualStackIPv4Primary)
			Expect(err).NotTo(HaveOccurred())
			Expect(primary).To(Equal("10.0.0.1"))
			Expect(required).To(Equal([]string{"10.0.0.1", "fd00::1"}))
		})

		It("should return both VIPs for DualStackIPv6Primary", func() {
			vmService.Status.LoadBalancer.Ingress = []vmoprvhub.LoadBalancerIngress{
				{IP: "10.0.0.1"},
				{IP: "fd00::1"},
			}
			primary, required, err := getAndValidateVIPs(vmService, util.DualStackIPv6Primary)
			Expect(err).NotTo(HaveOccurred())
			Expect(primary).To(Equal("fd00::1"))
			Expect(required).To(Equal([]string{"fd00::1", "10.0.0.1"}))
		})

		It("should return the first of each family", func() {
			vmService.Status.LoadBalancer.Ingress = []vmoprvhub.LoadBalancerIngress{
				{IP: "10.0.0.1"},
				{IP: "10.0.0.2"},
				{IP: "fd00::1"},
				{IP: "fd00::2"},
			}
			primary, required, err := getAndValidateVIPs(vmService, util.DualStackIPv4Primary)
			Expect(err).NotTo(HaveOccurred())
			Expect(primary).To(Equal("10.0.0.1"))
			Expect(required).To(Equal([]string{"10.0.0.1", "fd00::1"}))
		})

		It("should error if required family is missing", func() {
			vmService.Status.LoadBalancer.Ingress = []vmoprvhub.LoadBalancerIngress{
				{IP: "10.0.0.1"},
			}
			_, _, err := getAndValidateVIPs(vmService, util.IPv6SingleStack)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not yet have IPv6 VIP assigned"))

			_, _, err = getAndValidateVIPs(vmService, util.DualStackIPv4Primary)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must have both IPv4 and IPv6 ingress for dual stack cluster"))
		})

		It("should return error for invalid IP addresses", func() {
			vmService.Status.LoadBalancer.Ingress = []vmoprvhub.LoadBalancerIngress{
				{IP: "invalid-ip"},
				{IP: "10.0.0.1"},
			}
			_, _, err := getAndValidateVIPs(vmService, util.IPv4SingleStack)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid load balancer ingress IP address \"invalid-ip\""))
		})
	})
})

// buildKCPScheme returns a scheme containing KubeadmControlPlane types, used for ensureKCPReady tests.
func buildKCPScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = controlplanev1.AddToScheme(s)
	_ = clusterv1.AddToScheme(s)
	return s
}

// makeKCP constructs a KubeadmControlPlane with the given certSANs and observedGeneration.
func makeKCP(namespace, name, clusterName string, generation, observedGen int64, certSANs []string) *controlplanev1.KubeadmControlPlane {
	kcp := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: generation,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
				ClusterConfiguration: bootstrapv1.ClusterConfiguration{
					APIServer: bootstrapv1.APIServer{
						CertSANs: certSANs,
					},
				},
			},
		},
		Status: controlplanev1.KubeadmControlPlaneStatus{
			ObservedGeneration: observedGen,
		},
	}
	return kcp
}

var _ = Describe("ensureKCPReadyForControlPlaneEndpoint", func() {
	const (
		ns          = "default"
		clusterName = "test-cluster"
		kcpName     = "test-cluster-kcp"
		ipv4VIP     = "10.0.0.1"
		ipv6VIP     = "fd00::1"
	)

	makeClusterCtx := func(clusterName, kcpName string) *vmware.ClusterContext {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
		}
		if kcpName != "" {
			cluster.Spec.ControlPlaneRef = clusterv1.ContractVersionedObjectReference{
				APIGroup: controlplanev1.GroupVersion.Group,
				Kind:     "KubeadmControlPlane",
				Name:     kcpName,
			}
		}
		return &vmware.ClusterContext{Cluster: cluster}
	}

	It("should return nil when requiredIPs is empty", func() {
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).Build()
		clusterCtx := makeClusterCtx(clusterName, kcpName)
		Expect(ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, nil)).To(Succeed())
	})

	It("should return nil when cluster is nil", func() {
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).Build()
		Expect(ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, &vmware.ClusterContext{Cluster: nil}, []string{ipv4VIP, ipv6VIP})).To(Succeed())
	})

	It("should return error when ControlPlaneRef is not defined", func() {
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).Build()
		clusterCtx := makeClusterCtx(clusterName, "")
		err := ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, []string{ipv4VIP, ipv6VIP})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ControlPlaneRef is not defined"))
	})

	It("should return error when KCP not found for dual stack cluster (>1 requiredIPs)", func() {
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).Build()
		clusterCtx := makeClusterCtx(clusterName, kcpName)
		err := ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, []string{ipv4VIP, ipv6VIP})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to get KubeadmControlPlane"))
	})

	It("should return nil when KCP not found for single stack (1 requiredIP)", func() {
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).Build()
		clusterCtx := makeClusterCtx(clusterName, kcpName)
		Expect(ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, []string{ipv4VIP})).To(Succeed())
	})

	It("should return error when KCP observedGeneration does not match generation", func() {
		kcp := makeKCP(ns, kcpName, clusterName, 2, 1 /* stale */, []string{ipv4VIP, ipv6VIP})
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).WithObjects(kcp).WithStatusSubresource(kcp).Build()
		clusterCtx := makeClusterCtx(clusterName, kcpName)
		err := ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, []string{ipv4VIP, ipv6VIP})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("has not yet observed the current generation 2 (observed: 1"))
	})

	It("should return nil when KCP observedGeneration is stale but a condition has the current generation", func() {
		kcp := makeKCP(ns, kcpName, clusterName, 2, 1 /* stale top-level */, []string{ipv4VIP, ipv6VIP})
		kcp.Status.Conditions = []metav1.Condition{
			{
				Type:               "Available",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 2, // matches generation
			},
		}
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).WithObjects(kcp).WithStatusSubresource(kcp).Build()
		clusterCtx := makeClusterCtx(clusterName, kcpName)
		Expect(ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, []string{ipv4VIP, ipv6VIP})).To(Succeed())
	})

	It("should return error when certSANs missing IPv4 VIP", func() {
		kcp := makeKCP(ns, kcpName, clusterName, 1, 1, []string{ipv6VIP}) // missing ipv4VIP
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).WithObjects(kcp).WithStatusSubresource(kcp).Build()
		clusterCtx := makeClusterCtx(clusterName, kcpName)
		err := ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, []string{ipv4VIP, ipv6VIP})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("certSANs does not contain \"10.0.0.1\" yet"))
	})

	It("should return error when certSANs missing IPv6 VIP", func() {
		kcp := makeKCP(ns, kcpName, clusterName, 1, 1, []string{ipv4VIP}) // missing ipv6VIP
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).WithObjects(kcp).WithStatusSubresource(kcp).Build()
		clusterCtx := makeClusterCtx(clusterName, kcpName)
		err := ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, []string{ipv4VIP, ipv6VIP})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("certSANs does not contain \"fd00::1\" yet"))
	})

	It("should return nil when KCP has both VIPs in certSANs and observedGeneration matches", func() {
		kcp := makeKCP(ns, kcpName, clusterName, 1, 1, []string{ipv4VIP, ipv6VIP})
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).WithObjects(kcp).WithStatusSubresource(kcp).Build()
		clusterCtx := makeClusterCtx(clusterName, kcpName)
		Expect(ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, []string{ipv4VIP, ipv6VIP})).To(Succeed())
	})

	It("should skip gate check when KCP is being deleted", func() {
		kcp := makeKCP(ns, kcpName, clusterName, 1, 0 /* stale */, nil /* no certSANs */)
		now := metav1.Now()
		kcp.DeletionTimestamp = &now
		kcp.Finalizers = []string{"test-finalizer"}
		c := fake.NewClientBuilder().WithScheme(buildKCPScheme()).WithObjects(kcp).WithStatusSubresource(kcp).Build()
		clusterCtx := makeClusterCtx(clusterName, kcpName)
		Expect(ensureKCPReadyForControlPlaneEndpoint(context.Background(), c, clusterCtx, []string{ipv4VIP, ipv6VIP})).To(Succeed())
	})
})
