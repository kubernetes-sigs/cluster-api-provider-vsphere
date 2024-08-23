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

package network

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	netopv1 "github.com/vmware-tanzu/net-operator-api/api/v1alpha1"
	nsxopv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/v1alpha1"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	ncpv1 "github.com/vmware-tanzu/vm-operator/external/ncp/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

const (
	// Mocked virtualnetwork status reason and message.
	testNetworkNotRealizedReason  = "Cannot realize network"
	testNetworkNotRealizedMessage = "NetworkNotRealized"
)

// MockNSXTVpcNetworkProvider is the mock.
type MockNSXTNetworkProvider struct {
	*nsxtNetworkProvider
}

func (m *MockNSXTNetworkProvider) ProvisionClusterNetwork(ctx context.Context, clusterCtx *vmware.ClusterContext) error {
	err := m.nsxtNetworkProvider.ProvisionClusterNetwork(ctx, clusterCtx)

	if err != nil {
		// Check if the error contains the string "virtual network ready status"
		if strings.Contains(err.Error(), "virtual network ready status") {
			conditions.MarkTrue(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)
			return nil
		}
		// return the original error if it doesn't contain the specific string
		return err
	}
	return nil
}

// MockNSXTVpcNetworkProvider is the mock.
type MockNSXTVpcNetworkProvider struct {
	*nsxtVPCNetworkProvider
}

func (m *MockNSXTVpcNetworkProvider) ProvisionClusterNetwork(ctx context.Context, clusterCtx *vmware.ClusterContext) error {
	err := m.nsxtVPCNetworkProvider.ProvisionClusterNetwork(ctx, clusterCtx)

	if err != nil {
		// Check if the error contains the string "subnetset ready status"
		if strings.Contains(err.Error(), "subnetset ready status") {
			conditions.MarkTrue(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)
			return nil
		}
		// return the original error if it doesn't contain the specific string
		return err
	}
	return nil
}

func createUnReadyNsxtVirtualNetwork(ctx *vmware.ClusterContext, status ncpv1.VirtualNetworkStatus) *ncpv1.VirtualNetwork {
	// create an nsxt vnet with unready status caused by certain reasons from ncp
	cluster := ctx.VSphereCluster
	return &ncpv1.VirtualNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      GetNSXTVirtualNetworkName(cluster.Name),
		},
		Status: status,
	}
}

var _ = Describe("Network provider", func() {
	var (
		dummyNs          = "dummy-ns"
		dummyCluster     = "dummy-cluster"
		dummyVM          = "dummy-vm"
		fakeSNATIP       = "192.168.10.2"
		clusterKind      = "Cluster"
		infraClusterKind = "VSphereCluster"
		ctx              = context.Background()
		clusterCtx       *vmware.ClusterContext
		err              error
		np               services.NetworkProvider
		cluster          *clusterv1.Cluster
		vSphereCluster   *vmwarev1.VSphereCluster
		vm               *vmoprv1.VirtualMachine
		hasLB            bool
	)
	BeforeEach(func() {
		cluster = &clusterv1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       clusterKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dummyCluster,
				Namespace: dummyNs,
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: vmwarev1.GroupVersion.String(),
					Kind:       infraClusterKind,
					Name:       dummyCluster,
				},
			},
		}
		vSphereCluster = &vmwarev1.VSphereCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: vmwarev1.GroupVersion.String(),
				Kind:       infraClusterKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dummyCluster,
				Namespace: dummyNs,
			},
		}
		clusterCtx, _ = util.CreateClusterContext(cluster, vSphereCluster)
		vm = &vmoprv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: dummyNs,
				Name:      dummyVM,
			},
		}
	})

	Context("ConfigureVirtualMachine", func() {
		JustBeforeEach(func() {
			err = np.ConfigureVirtualMachine(ctx, clusterCtx, vm)
		})

		Context("with dummy network provider", func() {
			BeforeEach(func() {
				np = DummyNetworkProvider()
			})
			It("should not add network interface", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(vm.Spec.Network).To(BeNil())
			})
		})

		Context("with netop network provider", func() {
			var defaultNetwork *netopv1.Network

			testWithLabelFunc := func(label string) {
				BeforeEach(func() {
					defaultNetwork = &netopv1.Network{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-network",
							Namespace: dummyNs,
							Labels:    map[string]string{label: "true"},
						},
						Spec: netopv1.NetworkSpec{
							Type: netopv1.NetworkTypeVDS,
						},
					}
				})

				Context("ConfigureVirtualMachine", func() {
					BeforeEach(func() {
						scheme := runtime.NewScheme()
						Expect(netopv1.AddToScheme(scheme)).To(Succeed())
						client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(defaultNetwork).Build()
						np = NetOpNetworkProvider(client)
					})

					AfterEach(func() {
						Expect(err).ToNot(HaveOccurred())
						Expect(vm.Spec.Network).ToNot(BeNil())
						Expect(vm.Spec.Network.Interfaces).To(HaveLen(1))
						Expect(vm.Spec.Network.Interfaces[0].Network.TypeMeta.Kind).To(Equal("Network"))
						Expect(vm.Spec.Network.Interfaces[0].Network.TypeMeta.APIVersion).To(Equal(netopv1.SchemeGroupVersion.String()))
					})

					It("should add vds type network interface", func() {
					})

					It("vds network interface already exists", func() {
						err = np.ConfigureVirtualMachine(ctx, clusterCtx, vm)
					})
				})
			}

			Context("with new CAPV default network label", func() {
				testWithLabelFunc(CAPVDefaultNetworkLabel)
			})

			Context("with legacy default network label", func() {
				testWithLabelFunc(legacyDefaultNetworkLabel)
			})
		})

		Context("with nsx-t network provider", func() {
			BeforeEach(func() {
				scheme := runtime.NewScheme()
				Expect(ncpv1.AddToScheme(scheme)).To(Succeed())
				client := fake.NewClientBuilder().WithScheme(scheme).Build()
				np = NsxtNetworkProvider(client, "false")
			})

			It("should add nsx-t type network interface", func() {
			})
			It("nsx-t network interface already exists", func() {
				err = np.ConfigureVirtualMachine(ctx, clusterCtx, vm)
			})
			AfterEach(func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(vm.Spec.Network).ToNot(BeNil())
				Expect(vm.Spec.Network.Interfaces).To(HaveLen(1))
				Expect(vm.Spec.Network.Interfaces[0].Network.Name).To(Equal(GetNSXTVirtualNetworkName(vSphereCluster.Name)))
				Expect(vm.Spec.Network.Interfaces[0].Network.TypeMeta.Kind).To(Equal("VirtualNetwork"))
				Expect(vm.Spec.Network.Interfaces[0].Network.TypeMeta.APIVersion).To(Equal(ncpv1.SchemeGroupVersion.String()))
			})
		})

		Context("with NSX-VPC network provider", func() {
			BeforeEach(func() {
				scheme := runtime.NewScheme()
				Expect(ncpv1.AddToScheme(scheme)).To(Succeed())
				client := fake.NewClientBuilder().WithScheme(scheme).Build()
				np = NSXTVpcNetworkProvider(client)
			})

			It("should add nsx-t-subnetset type network interface", func() {
				err = np.ConfigureVirtualMachine(ctx, clusterCtx, vm)
				Expect(err).ToNot(HaveOccurred())
				Expect(vm.Spec.Network.Interfaces).To(HaveLen(1))
			})

			It("nsx-t-subnetset type network interface already exists", func() {
				err = np.ConfigureVirtualMachine(ctx, clusterCtx, vm)
				Expect(err).ToNot(HaveOccurred())
				Expect(vm.Spec.Network).ToNot(BeNil())
				Expect(vm.Spec.Network.Interfaces).To(HaveLen(1))
			})

			AfterEach(func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(vm.Spec.Network).ToNot(BeNil())
				Expect(vm.Spec.Network.Interfaces).To(HaveLen(1))
				Expect(vm.Spec.Network.Interfaces[0].Network.Name).To(Equal(vSphereCluster.Name))
				Expect(vm.Spec.Network.Interfaces[0].Network.TypeMeta.Kind).To(Equal("SubnetSet"))
				Expect(vm.Spec.Network.Interfaces[0].Network.TypeMeta.APIVersion).To(Equal(nsxopv1.SchemeGroupVersion.String()))
			})
		})
	})

	Context("ProvisionClusterNetwork", func() {
		var (
			scheme             *runtime.Scheme
			client             runtimeclient.Client
			nsxNp              *nsxtNetworkProvider
			vpcNp              *nsxtVPCNetworkProvider
			runtimeObjs        []runtime.Object
			vnetObj            runtime.Object
			configmapObj       runtime.Object
			systemNamespaceObj runtime.Object
		)

		BeforeEach(func() {
			vnetObj = &ncpv1.VirtualNetwork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				},
				Status: ncpv1.VirtualNetworkStatus{
					Conditions: []ncpv1.VirtualNetworkCondition{
						{
							Type:   "Ready",
							Status: "True",
						},
					},
				},
			}
			configmapObj = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      util.NCPVersionConfigMap,
					Namespace: util.NCPNamespace,
				},
				Data: map[string]string{
					util.NCPVersionKey: util.NCPVersionSupportFW,
				},
			}
			systemNamespaceObj = &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Namespace",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: SystemNamespace,
					Annotations: map[string]string{
						util.NCPSNATKey: fakeSNATIP,
					},
				},
			}
			runtimeObjs = []runtime.Object{
				systemNamespaceObj,
				configmapObj,
				vnetObj,
			}
			scheme = runtime.NewScheme()
			Expect(ncpv1.AddToScheme(scheme)).To(Succeed())
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(vmwarev1.AddToScheme(scheme)).To(Succeed())
			Expect(nsxopv1.AddToScheme(scheme)).To(Succeed())
		})

		Context("with dummy network provider", func() {
			BeforeEach(func() {
				np = DummyNetworkProvider()
			})
			JustBeforeEach(func() {
				err = np.ProvisionClusterNetwork(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should succeed", func() {
				vnet, err := np.GetClusterNetworkName(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(vnet).To(BeEmpty())
			})
		})

		Context("with netop network provider", func() {
			BeforeEach(func() {
				scheme := runtime.NewScheme()
				Expect(netopv1.AddToScheme(scheme)).To(Succeed())
				client = fake.NewClientBuilder().WithScheme(scheme).Build()
				np = NetOpNetworkProvider(client)
			})
			JustBeforeEach(func() {
				// noop for netop
				err = np.ProvisionClusterNetwork(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should succeed", func() {
				Expect(conditions.IsTrue(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW not enabled and VNET exists", func() {
			BeforeEach(func() {
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(runtimeObjs...).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "true").(*nsxtNetworkProvider)
				np = nsxNp
				err = np.ProvisionClusterNetwork(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not update vnet with whitelist_source_ranges in spec", func() {
				vnet, err := np.GetClusterNetworkName(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(clusterCtx.VSphereCluster.Name)))

				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)

				Expect(err).ToNot(HaveOccurred())
				Expect(createdVNET.Spec.WhitelistSourceRanges).To(BeEmpty())
			})

			// The organization of these tests are inverted so easiest to put this here because
			// NCP will eventually be removed.
			It("GetVMServiceAnnotations", func() {
				annotations, err := np.GetVMServiceAnnotations(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(annotations).To(HaveKeyWithValue("ncp.vmware.com/virtual-network-name", GetNSXTVirtualNetworkName(clusterCtx.VSphereCluster.Name)))
				Expect(conditions.IsTrue(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW not enabled and VNET does not exist", func() {
			BeforeEach(func() {
				// no pre-existing vnet obj
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(configmapObj, systemNamespaceObj).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "true").(*nsxtNetworkProvider)
				np = nsxNp
				// The ProvisionClusterNetwork function would fail due to the absence of
				// ncp to set the `virtual network ready` condition.
				// We use the mock function to disregard this specific error.
				// mocknp is an instance of MockNSXNetworkProvider initialized with nsxNp.
				mocknp := &MockNSXTNetworkProvider{nsxNp}
				err = mocknp.ProvisionClusterNetwork(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should create vnet without whitelist_source_ranges in spec", func() {
				vnet, err := np.GetClusterNetworkName(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(clusterCtx.VSphereCluster.Name)))

				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)

				Expect(err).ToNot(HaveOccurred())
				Expect(createdVNET.Spec.WhitelistSourceRanges).To(BeEmpty())
				Expect(conditions.IsTrue(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW enabled and NCP version >= 3.0.1 and VNET exists", func() {
			BeforeEach(func() {
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(runtimeObjs...).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "false").(*nsxtNetworkProvider)
				np = nsxNp
				err = np.ProvisionClusterNetwork(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should update vnet with whitelist_source_ranges in spec", func() {
				vnet, err := np.GetClusterNetworkName(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(clusterCtx.VSphereCluster.Name)))

				// Verify WhitelistSourceRanges have been updated
				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)

				Expect(err).ToNot(HaveOccurred())
				Expect(createdVNET.Spec.WhitelistSourceRanges).To(Equal(fakeSNATIP + "/32"))
				Expect(conditions.IsTrue(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW enabled and NCP version >= 3.0.1 and VNET does not exist", func() {
			BeforeEach(func() {
				// no pre-existing vnet obj
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(configmapObj, systemNamespaceObj).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "false").(*nsxtNetworkProvider)
				np = nsxNp
				// The ProvisionClusterNetwork function would fail due to the absence of
				// ncp to set the `virtual network ready` condition.
				// We use the mock function to disregard this specific error.
				// mocknp is an instance of MockNSXNetworkProvider initialized with nsxNp.
				mocknp := &MockNSXTNetworkProvider{nsxNp}
				err = mocknp.ProvisionClusterNetwork(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should create new vnet with whitelist_source_ranges in spec", func() {
				vnet, err := np.GetClusterNetworkName(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(clusterCtx.VSphereCluster.Name)))

				// Verify WhitelistSourceRanges have been updated
				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdVNET.Spec.WhitelistSourceRanges).To(Equal(fakeSNATIP + "/32"))
				// err is not empty, but it is because vnetObj does not have status mocked in this test

				Expect(conditions.IsTrue(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW enabled and NCP version < 3.0.1 and VNET exists", func() {
			BeforeEach(func() {
				// test if NCP version is 3.0.0
				configmapObj.(*corev1.ConfigMap).Data[util.NCPVersionKey] = "3.0.0"
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(runtimeObjs...).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "false").(*nsxtNetworkProvider)
				np = nsxNp
				err = np.ProvisionClusterNetwork(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not update vnet with whitelist_source_ranges in spec", func() {
				vnet, err := np.GetClusterNetworkName(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(clusterCtx.VSphereCluster.Name)))

				// Verify WhitelistSourceRanges is not included
				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdVNET.Spec.WhitelistSourceRanges).To(BeEmpty())
				// err is not empty, but it is because vnetObj does not have status mocked in this test

				Expect(conditions.IsTrue(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})

			AfterEach(func() {
				// change NCP version back
				configmapObj.(*corev1.ConfigMap).Data[util.NCPVersionKey] = util.NCPVersionSupportFW
			})
		})

		Context("with nsx-t network provider failure", func() {
			var (
				client  runtimeclient.Client
				nsxNp   *nsxtNetworkProvider
				scheme  *runtime.Scheme
				vnetObj runtime.Object
			)
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				Expect(ncpv1.AddToScheme(scheme)).To(Succeed())
			})

			It("should return error when vnet ready status is false", func() {
				By("create a cluster with virtual network in not ready status")
				status := ncpv1.VirtualNetworkStatus{
					Conditions: []ncpv1.VirtualNetworkCondition{
						{Type: "Ready", Status: "False", Reason: testNetworkNotRealizedReason, Message: testNetworkNotRealizedMessage},
					},
				}
				vnetObj = createUnReadyNsxtVirtualNetwork(clusterCtx, status)
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(vnetObj).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "false").(*nsxtNetworkProvider)
				np = nsxNp

				err = np.VerifyNetworkStatus(ctx, clusterCtx, vnetObj)

				expectedErrorMessage := fmt.Sprintf("virtual network ready status is: '%s' in cluster %s. reason: %s, message: %s",
					"False", apitypes.NamespacedName{Namespace: dummyNs, Name: dummyCluster}, testNetworkNotRealizedReason, testNetworkNotRealizedMessage)
				Expect(err).To(MatchError(expectedErrorMessage))
				Expect(conditions.IsFalse(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})

			It("should return error when vnet ready status is not set", func() {
				By("create a cluster with virtual network has no ready status")
				status := ncpv1.VirtualNetworkStatus{
					Conditions: []ncpv1.VirtualNetworkCondition{},
				}
				vnetObj = createUnReadyNsxtVirtualNetwork(clusterCtx, status)
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(vnetObj).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "false").(*nsxtNetworkProvider)
				np = nsxNp

				err = np.VerifyNetworkStatus(ctx, clusterCtx, vnetObj)

				expectedErrorMessage := fmt.Sprintf("virtual network ready status in cluster %s has not been set", apitypes.NamespacedName{Namespace: dummyNs, Name: dummyCluster})
				Expect(err).To(MatchError(expectedErrorMessage))
				Expect(conditions.IsFalse(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with NSX-VPC network provider and subnetset exists", func() {
			BeforeEach(func() {
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(configmapObj, systemNamespaceObj).Build()
				vpcNp, _ = NSXTVpcNetworkProvider(client).(*nsxtVPCNetworkProvider)
				np = vpcNp
				// The ProvisionClusterNetwork function would fail due to the absence of
				// a netoperator to set the `subnetset ready` condition.
				// We use the mock function to disregard this specific error.
				// mocknp is an instance of MockNSXTVpcNetworkProvider initialized with nsxvpcNp.
				mocknp := &MockNSXTVpcNetworkProvider{vpcNp}
				err = mocknp.ProvisionClusterNetwork(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not update subnetset", func() {
				// Fetch the SubnetSet before the operation
				initialSubnetSet := &nsxopv1.SubnetSet{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      dummyCluster,
					Namespace: dummyNs,
				}, initialSubnetSet)
				Expect(err).NotTo(HaveOccurred())
				status := nsxopv1.SubnetSetStatus{
					Conditions: []nsxopv1.Condition{
						{
							Type:   "Ready",
							Status: "True",
						},
					},
				}
				initialSubnetSet.Status = status

				// Presumably there's code here that might modify the SubnetSet...
				subnetset, err := np.GetClusterNetworkName(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(subnetset).To(Equal(clusterCtx.VSphereCluster.Name))

				createdSubnetSet := &nsxopv1.SubnetSet{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      dummyCluster,
					Namespace: dummyNs,
				}, createdSubnetSet)
				Expect(err).ToNot(HaveOccurred())
				// Check that the SubnetSetSpec was not changed
				Expect(createdSubnetSet.Spec).To(Equal(initialSubnetSet.Spec), "SubnetSetSpec should not have been modified")
			})

			It("should successfully retrieve VM service annotations, including the annotation to enable LB healthcheck", func() {
				annotations, err := np.GetVMServiceAnnotations(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(annotations).To(HaveKey("lb.iaas.vmware.com/enable-endpoint-health-check"))
			})

		})

		Context("with NSX-VPC network provider and subnetset does not exist", func() {
			var nsxvpcNp *nsxtVPCNetworkProvider

			BeforeEach(func() {
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(configmapObj, systemNamespaceObj).Build()
				nsxvpcNp, _ = NSXTVpcNetworkProvider(client).(*nsxtVPCNetworkProvider)
				// The ProvisionClusterNetwork function would fail due to the absence of
				// a netoperator to set the `subnetset ready` condition.
				// We use the mock function to disregard this specific error.
				// mocknp is an instance of MockNSXTVpcNetworkProvider initialized with nsxvpcNp.
				mocknp := &MockNSXTVpcNetworkProvider{nsxvpcNp}
				err = mocknp.ProvisionClusterNetwork(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should create subnetset with new spec", func() {
				subnetset, err := nsxvpcNp.GetClusterNetworkName(ctx, clusterCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(subnetset).To(Equal(clusterCtx.VSphereCluster.Name))

				createdSubnetSet := &nsxopv1.SubnetSet{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      dummyCluster,
					Namespace: dummyNs,
				}, createdSubnetSet)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdSubnetSet.Spec.AdvancedConfig.StaticIPAllocation.Enable).To(BeTrue())
			})
		})

		Context("with NSX-VPC network provider and object passed to VerifyNetworkStatus is not a SubnetSet", func() {
			var nsxvpcNp *nsxtVPCNetworkProvider

			BeforeEach(func() {
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(configmapObj, systemNamespaceObj).Build()
				nsxvpcNp, _ = NSXTVpcNetworkProvider(client).(*nsxtVPCNetworkProvider)
				np = nsxvpcNp
			})

			It("should return error when non-SubnetSet object passed to VerifyNetworkStatus", func() {
				dummyObj := &ncpv1.VirtualNetwork{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cluster.Namespace,
						Name:      GetNSXTVirtualNetworkName(cluster.Name),
					},
				}
				err = nsxvpcNp.VerifyNetworkStatus(ctx, clusterCtx, dummyObj)
				Expect(err).To(HaveOccurred()) // Expect error because dummyObj is not a SubnetSet
				Expect(err.Error()).To(Equal(fmt.Sprintf("expected NSX VPC SubnetSet but got %T", dummyObj)))
			})
		})

		Context("with NSX-VPC network provider failure", func() {
			var (
				client       runtimeclient.Client
				nsxvpcNp     *nsxtVPCNetworkProvider
				scheme       *runtime.Scheme
				subnetsetObj runtime.Object
			)

			BeforeEach(func() {
				scheme = runtime.NewScheme()
				Expect(nsxopv1.AddToScheme(scheme)).To(Succeed())
				nsxvpcNp, _ = NSXTVpcNetworkProvider(client).(*nsxtVPCNetworkProvider)
				np = nsxvpcNp
			})

			It("should return error when subnetset ready status is false", func() {
				status := nsxopv1.SubnetSetStatus{
					Conditions: []nsxopv1.Condition{
						{
							Type:    "Ready",
							Status:  "False",
							Reason:  testNetworkNotRealizedReason,
							Message: testNetworkNotRealizedMessage,
						},
					},
				}
				subnetsetObj = &nsxopv1.SubnetSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cluster.Namespace,
						Name:      cluster.Name,
					},
					Status: status,
				}
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(subnetsetObj).Build()
				err = np.VerifyNetworkStatus(ctx, clusterCtx, subnetsetObj)
				expectedErrorMessage := fmt.Sprintf("subnetset ready status is: '%s' in cluster %s. reason: %s, message: %s",
					"False", apitypes.NamespacedName{Namespace: dummyNs, Name: dummyCluster}, testNetworkNotRealizedReason, testNetworkNotRealizedMessage)
				Expect(err).To(MatchError(expectedErrorMessage))
				Expect(conditions.IsFalse(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})

			It("should return error when subnetset ready status is not set", func() {
				status := nsxopv1.SubnetSetStatus{
					Conditions: []nsxopv1.Condition{},
				}
				subnetsetObj = &nsxopv1.SubnetSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cluster.Namespace,
						Name:      cluster.Name,
					},
					Status: status,
				}
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(subnetsetObj).Build()
				err = np.VerifyNetworkStatus(ctx, clusterCtx, subnetsetObj)
				expectedErrorMessage := fmt.Sprintf("subnetset ready status in cluster %s has not been set", apitypes.NamespacedName{Namespace: dummyNs, Name: dummyCluster})
				Expect(err).To(MatchError(expectedErrorMessage))
				Expect(conditions.IsFalse(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})
	})

	Context("GetVMServiceAnnotations", func() {
		Context("with netop network provider", func() {
			var defaultNetwork *netopv1.Network

			testWithLabelFunc := func(label string) {
				BeforeEach(func() {
					defaultNetwork = &netopv1.Network{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-network",
							Namespace: dummyNs,
							Labels:    map[string]string{label: "true"},
						},
						Spec: netopv1.NetworkSpec{
							Type: netopv1.NetworkTypeVDS,
						},
					}
					scheme := runtime.NewScheme()
					Expect(netopv1.AddToScheme(scheme)).To(Succeed())
					client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(defaultNetwork).Build()
					np = NetOpNetworkProvider(client)
				})

				Context("with default network", func() {
					It("Should return expected annotations", func() {
						annotations, err := np.GetVMServiceAnnotations(ctx, clusterCtx)
						Expect(err).ToNot(HaveOccurred())
						Expect(annotations).To(HaveKeyWithValue("netoperator.vmware.com/network-name", defaultNetwork.Name))
					})
				})
			}

			Context("with new CAPV default network label", func() {
				testWithLabelFunc(CAPVDefaultNetworkLabel)
			})

			Context("with legacy default network label", func() {
				testWithLabelFunc(legacyDefaultNetworkLabel)
			})

		})
	})

	Context("HasLoadBalancer", func() {
		JustBeforeEach(func() {
			hasLB = np.HasLoadBalancer()
		})

		Context("with dummy network provider", func() {
			BeforeEach(func() {
				np = DummyNetworkProvider()
			})
			It("should not have a loadbalancer", func() {
				Expect(hasLB).To(BeFalse())
			})
		})

		Context("with dummy LB network provider", func() {
			BeforeEach(func() {
				np = DummyLBNetworkProvider()
			})
			It("should have a loadbalancer", func() {
				Expect(hasLB).To(BeTrue())
			})
		})

		Context("with netop network provider", func() {
			BeforeEach(func() {
				scheme := runtime.NewScheme()
				Expect(netopv1.AddToScheme(scheme)).To(Succeed())
				client := fake.NewClientBuilder().WithScheme(scheme).Build()
				np = NetOpNetworkProvider(client)
			})
			It("should have a loadbalancer", func() {
				Expect(hasLB).To(BeTrue())
			})
		})

		Context("with nsx-t network provider", func() {
			BeforeEach(func() {
				scheme := runtime.NewScheme()
				Expect(ncpv1.AddToScheme(scheme)).To(Succeed())
				client := fake.NewClientBuilder().WithScheme(scheme).Build()
				np = NsxtNetworkProvider(client, "false")
			})
			It("should have a loadbalancer", func() {
				Expect(hasLB).To(BeTrue())
			})
		})
	})
})
