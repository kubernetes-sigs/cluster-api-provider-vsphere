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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	netopv1 "github.com/vmware-tanzu/net-operator-api/api/v1alpha1"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha1"
	ncpv1 "github.com/vmware-tanzu/vm-operator/external/ncp/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/network"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func getVirtualMachineService(cpService CPService, ctx *vmware.ClusterContext) *vmoprv1.VirtualMachineService {
	vms := newVirtualMachineService(ctx)
	nsname := types.NamespacedName{
		Namespace: vms.Namespace,
		Name:      vms.Name,
	}
	err := ctx.Client.Get(ctx, nsname, vms)
	if apierrors.IsNotFound(err) {
		return nil
	}
	Expect(err).Should(BeNil())
	return vms
}

func createVnet(ctx *vmware.ClusterContext) {
	vnet := &ncpv1.VirtualNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.VSphereCluster.Namespace,
			Name:      network.GetNSXTVirtualNetworkName(ctx.VSphereCluster.Name),
		},
	}
	Expect(ctx.Client.Create(ctx, vnet)).To(Succeed())
}

func createDefaultNetwork(ctx *vmware.ClusterContext) {
	defaultNetwork := &netopv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummy-network",
			Namespace: ctx.VSphereCluster.Namespace,
			Labels:    map[string]string{network.CAPVDefaultNetworkLabel: "true"},
		},
		Spec: netopv1.NetworkSpec{
			Type: netopv1.NetworkTypeVDS,
		},
	}
	Expect(ctx.Client.Create(ctx, defaultNetwork)).To(Succeed())
}

func updateVMServiceWithVIP(cpService CPService, ctx *vmware.ClusterContext, vip string) {
	vmService := getVirtualMachineService(cpService, ctx)
	vmService.Status.LoadBalancer.Ingress = []vmoprv1.LoadBalancerIngress{{IP: vip}}
	err := ctx.Client.Status().Update(ctx, vmService)
	Expect(err).Should(BeNil())
}

var _ = Describe("ControlPlaneEndpoint Tests", func() {
	const (
		clusterName          = "test-cluster"
		vip                  = "127.0.0.1"
		noNetworkFailure     = "failed to get provider VirtualMachineService annotations"
		waitingForVIPFailure = "VirtualMachineService LoadBalancer does not have any Ingresses"
	)
	var (
		err                         error
		expectReconcileError        bool
		expectAPIEndpoint           bool
		expectVMS                   bool
		expectedType                vmoprv1.VirtualMachineServiceType
		expectedHost                string
		expectedPort                int
		expectedAnnotations         map[string]string
		expectedClusterRoleVMLabels map[string]string
		expectedConditions          clusterv1.Conditions

		cluster        *clusterv1.Cluster
		vsphereCluster *infrav1.VSphereCluster
		ctx            *vmware.ClusterContext

		apiEndpoint *clusterv1.APIEndpoint
		vms         *vmoprv1.VirtualMachineService

		cpService = CPService{}
	)

	BeforeEach(func() {
		// Default values
		expectedHost = ""
		expectedPort = 0
		expectedAnnotations = make(map[string]string)
		expectedConditions = nil

		// Create all necessary dependencies
		cluster = util.CreateCluster(clusterName)
		vsphereCluster = util.CreateVSphereCluster(clusterName)
		ctx = util.CreateClusterContext(cluster, vsphereCluster)
		expectedClusterRoleVMLabels = clusterRoleVMLabels(ctx, true)
	})

	Context("Reconcile ControlPlaneEndpointService", func() {
		verifyOutput := func() {
			Expect(err != nil).Should(Equal(expectReconcileError))

			Expect(apiEndpoint != nil).Should(Equal(expectAPIEndpoint))
			if apiEndpoint != nil {
				Expect(apiEndpoint.Host).To(Equal(expectedHost))
				Expect(apiEndpoint.Port).To(BeEquivalentTo(expectedPort))
			}

			vms = getVirtualMachineService(cpService, ctx)
			Expect(vms != nil).Should(Equal(expectVMS))
			if vms != nil {
				Expect(vms.Spec.Type).To(Equal(expectedType))
				for k, v := range expectedAnnotations {
					Expect(vms.Annotations).To(HaveKeyWithValue(k, v))
				}
				Expect(len(vms.Spec.Ports)).To(Equal(1))
				Expect(vms.Spec.Ports[0].Name).To(Equal(controlPlaneServiceAPIServerPortName))
				Expect(vms.Spec.Ports[0].Protocol).To(Equal("TCP"))
				Expect(vms.Spec.Ports[0].Port).To(Equal(int32(defaultAPIBindPort)))
				Expect(vms.Spec.Ports[0].TargetPort).To(Equal(int32(defaultAPIBindPort)))
				Expect(vms.Spec.Selector).To(Equal(expectedClusterRoleVMLabels))
			}

			for _, expectedCondition := range expectedConditions {
				c := conditions.Get(ctx.VSphereCluster, expectedCondition.Type)
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

		// If there is no load balancer, Reconcile should be a no-op
		Specify("NetworkProvider has no LoadBalancer", func() {
			expectReconcileError = false
			expectAPIEndpoint = false
			expectVMS = false
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, network.DummyNetworkProvider())
			Expect(conditions.Get(ctx.VSphereCluster, infrav1.LoadBalancerReadyCondition)).To(BeNil())
			verifyOutput()
		})

		Specify("DummyLBNetworkProvider has a LoadBalancer", func() {
			expectReconcileError = true // VirtualMachineService LB does not yet have VIP assigned
			expectVMS = true
			expectedType = vmoprv1.VirtualMachineServiceTypeLoadBalancer
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, network.DummyLBNetworkProvider())
			verifyOutput()
		})

		Specify("Reconcile VirtualMachineService for NetOp", func() {
			// Reconcile should return an error up and until all prerequisites have been met
			expectReconcileError = true
			// An APIEndpoint is only returned if reconcile succeeds
			expectAPIEndpoint = false
			// A VirtualMachineService is only created once all prerequisites have been met
			expectVMS = false
			expectedType = vmoprv1.VirtualMachineServiceTypeLoadBalancer

			// The NetOp network provider looks a Network. If one does not exist, it will fail.
			By("NetOp NetworkProvider has no Network")
			netOpProvider := network.NetOpNetworkProvider(ctx.Client)
			// we expect the reconciliation fail because lack of bootstrap data
			expectedConditions = append(expectedConditions, clusterv1.Condition{
				Type:    infrav1.LoadBalancerReadyCondition,
				Status:  corev1.ConditionFalse,
				Reason:  infrav1.LoadBalancerCreationFailedReason,
				Message: noNetworkFailure,
			})
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, netOpProvider)
			verifyOutput()

			// If a Network is present, a VirtualMachineService should be created
			By("NetOp NetworkProvider has a Network with no VIP")
			// A VirtualMachineService should be created and will wait for a VIP to be assigned
			expectedAnnotations["netoperator.vmware.com/network-name"] = "dummy-network"
			expectVMS = true
			createDefaultNetwork(ctx)
			expectedConditions[0].Reason = infrav1.WaitingForLoadBalancerIPReason
			expectedConditions[0].Message = waitingForVIPFailure
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, netOpProvider)
			verifyOutput()

			// Once a VIP has been created, a VirtualMachineService should exist with a valid endpoint
			By("NetOP NetworkProvider has a Service with a VIP")
			expectReconcileError = false
			expectAPIEndpoint = true
			expectedPort = defaultAPIBindPort
			expectedHost = vip
			updateVMServiceWithVIP(cpService, ctx, vip)
			expectedConditions[0].Status = corev1.ConditionTrue
			expectedConditions[0].Reason = ""
			expectedConditions[0].Message = ""
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, netOpProvider)
			verifyOutput()
		})

		Specify("Reconcile VirtualMachineService for NSX-T", func() {
			// Reconcile should return an error up and until all prerequisites have been met
			expectReconcileError = true
			// An APIEndpoint is only returned if reconcile succeeds
			expectAPIEndpoint = false
			// A VirtualMachineService is only created once all prerequisites have been met
			expectVMS = false
			expectedType = vmoprv1.VirtualMachineServiceTypeLoadBalancer
			expectedConditions = append(expectedConditions, clusterv1.Condition{
				Type:    infrav1.LoadBalancerReadyCondition,
				Status:  corev1.ConditionFalse,
				Reason:  infrav1.LoadBalancerCreationFailedReason,
				Message: noNetworkFailure,
			})

			// The NSXT network provider looks for a real vnet. If one does not exist, it will fail.
			By("NSXT NetworkProvider has no vnet")
			nsxtProvider := network.NsxtNetworkProvider(ctx.Client, "false")
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, nsxtProvider)
			verifyOutput()

			// If a vnet is present, a VirtualMachineService should be created
			By("NSXT NetworkProvider has a vnet with no VIP")
			// A VirtualMachineService should be created and will wait for a VIP to be assigned
			expectedVnetName := network.GetNSXTVirtualNetworkName(clusterName)
			expectedAnnotations["ncp.vmware.com/virtual-network-name"] = expectedVnetName
			expectVMS = true
			expectedConditions[0].Reason = infrav1.WaitingForLoadBalancerIPReason
			expectedConditions[0].Message = waitingForVIPFailure
			createVnet(ctx)
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, nsxtProvider)
			verifyOutput()

			// Once a VIP has been created, a VirtualMachineService should exist with a valid endpoint
			By("NSXT NetworkProvider has a vnet with a VIP")
			expectReconcileError = false
			expectAPIEndpoint = true
			expectedPort = defaultAPIBindPort
			expectedHost = vip
			expectedConditions[0].Status = corev1.ConditionTrue
			expectedConditions[0].Reason = ""
			expectedConditions[0].Message = ""
			updateVMServiceWithVIP(cpService, ctx, vip)
			apiEndpoint, err = cpService.ReconcileControlPlaneEndpointService(ctx, nsxtProvider)
			verifyOutput()
		})
	})
})
