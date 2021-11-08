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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	netopv1alpha1 "github.com/vmware-tanzu/net-operator-api/api/v1alpha1"
	"github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	ncpv1 "github.com/vmware-tanzu/vm-operator/external/ncp/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

const (
	// Mocked virtualnetwork status reason and message.
	testVnetNotRealizedReason  = "Cannot realize network"
	testVnetNotRealizedMessage = "NetworkNotRealized"
)

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
		ctx              *vmware.ClusterContext
		err              error
		np               services.NetworkProvider
		cluster          *clusterv1.Cluster
		vSphereCluster   *infrav1.VSphereCluster
		vm               *v1alpha1.VirtualMachine
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
				InfrastructureRef: &v1.ObjectReference{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       infraClusterKind,
					Name:       dummyCluster,
				},
			},
		}
		vSphereCluster = &infrav1.VSphereCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       infraClusterKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dummyCluster,
				Namespace: dummyNs,
			},
		}
		ctx = util.CreateClusterContext(cluster, vSphereCluster)
		vm = &v1alpha1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: dummyNs,
				Name:      dummyVM,
			},
		}
	})

	Context("ConfigureVirtualMachine", func() {
		JustBeforeEach(func() {
			err = np.ConfigureVirtualMachine(ctx, vm)
		})

		Context("with dummy network provider", func() {
			BeforeEach(func() {
				np = DummyNetworkProvider()
			})
			It("should not add network interface", func() {
				Expect(err).To(BeNil())
				Expect(vm.Spec.NetworkInterfaces).To(BeNil())
			})
		})

		Context("with netop network provider", func() {
			var defaultNetwork *netopv1alpha1.Network

			BeforeEach(func() {
				defaultNetwork = &netopv1alpha1.Network{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy-network",
						Namespace: dummyNs,
						Labels:    map[string]string{CAPVDefaultNetworkLabel: "true"},
					},
					Spec: netopv1alpha1.NetworkSpec{
						Type: netopv1alpha1.NetworkTypeVDS,
					},
				}
			})

			Context("ConfigureVirtualMachine", func() {
				BeforeEach(func() {
					scheme := runtime.NewScheme()
					Expect(netopv1alpha1.AddToScheme(scheme)).To(Succeed())
					client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(defaultNetwork).Build()
					np = NetOpNetworkProvider(client)
				})

				AfterEach(func() {
					Expect(err).To(BeNil())
					Expect(vm.Spec.NetworkInterfaces).To(HaveLen(1))
					Expect(vm.Spec.NetworkInterfaces[0].NetworkType).To(Equal("vsphere-distributed"))
				})

				It("should add vds type network interface", func() {
				})

				It("vds network interface already exists", func() {
					err = np.ConfigureVirtualMachine(ctx, vm)
				})
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
				err = np.ConfigureVirtualMachine(ctx, vm)
			})
			AfterEach(func() {
				Expect(err).To(BeNil())
				Expect(vm.Spec.NetworkInterfaces[0].NetworkType).To(Equal("nsx-t"))
			})
		})
	})

	Context("ProvisionClusterNetwork", func() {
		var (
			scheme             *runtime.Scheme
			client             runtimeclient.Client
			nsxNp              *nsxtNetworkProvider
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
			configmapObj = &v1.ConfigMap{
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
			systemNamespaceObj = &v1.Namespace{
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
			Expect(v1.AddToScheme(scheme)).To(Succeed())
		})

		Context("with dummy network provider", func() {
			BeforeEach(func() {
				np = DummyNetworkProvider()
			})
			JustBeforeEach(func() {
				err = np.ProvisionClusterNetwork(ctx)
			})
			It("should succeed", func() {
				Expect(err).To(BeNil())
				vnet, localerr := np.GetClusterNetworkName(ctx)
				Expect(localerr).To(BeNil())
				Expect(vnet).To(BeEmpty())
			})
		})

		Context("with netop network provider", func() {
			BeforeEach(func() {
				scheme := runtime.NewScheme()
				Expect(netopv1alpha1.AddToScheme(scheme)).To(Succeed())
				client = fake.NewClientBuilder().WithScheme(scheme).Build()
				np = NetOpNetworkProvider(client)
			})
			JustBeforeEach(func() {
				// noop for netop
				err = np.ProvisionClusterNetwork(ctx)
			})
			It("should succeed", func() {
				Expect(err).To(BeNil())
				Expect(conditions.IsTrue(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW not enabled and VNET exists", func() {
			BeforeEach(func() {
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(runtimeObjs...).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "true").(*nsxtNetworkProvider)
				np = nsxNp
				err = np.ProvisionClusterNetwork(ctx)
			})

			It("should not update vnet with whitelist_source_ranges in spec", func() {
				Expect(err).To(BeNil())
				vnet, localerr := np.GetClusterNetworkName(ctx)
				Expect(localerr).To(BeNil())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(ctx.VSphereCluster.Name)))

				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)

				Expect(err).To(BeNil())
				Expect(createdVNET.Spec.WhitelistSourceRanges).To(BeEmpty())
			})

			// The organization of these tests are inverted so easiest to put this here because
			// NCP will eventually be removed.
			It("GetVMServiceAnnotations", func() {
				annotations, err := np.GetVMServiceAnnotations(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(annotations).To(HaveKeyWithValue("ncp.vmware.com/virtual-network-name", GetNSXTVirtualNetworkName(ctx.VSphereCluster.Name)))
				Expect(conditions.IsTrue(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW not enabled and VNET does not exist", func() {
			BeforeEach(func() {
				// no pre-existing vnet obj
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(configmapObj, systemNamespaceObj).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "true").(*nsxtNetworkProvider)
				np = nsxNp
				err = np.ProvisionClusterNetwork(ctx)
			})

			It("should create vnet without whitelist_source_ranges in spec", func() {
				Expect(err).To(BeNil())
				vnet, localerr := np.GetClusterNetworkName(ctx)
				Expect(localerr).To(BeNil())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(ctx.VSphereCluster.Name)))

				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)

				Expect(err).To(BeNil())
				Expect(createdVNET.Spec.WhitelistSourceRanges).To(BeEmpty())
				Expect(conditions.IsTrue(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW enabled and NCP version >= 3.0.1 and VNET exists", func() {
			BeforeEach(func() {
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(runtimeObjs...).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "false").(*nsxtNetworkProvider)
				np = nsxNp
				err = np.ProvisionClusterNetwork(ctx)
			})

			It("should update vnet with whitelist_source_ranges in spec", func() {
				Expect(err).To(BeNil())
				vnet, localerr := np.GetClusterNetworkName(ctx)
				Expect(localerr).To(BeNil())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(ctx.VSphereCluster.Name)))

				// Verify WhitelistSourceRanges have been updated
				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)

				Expect(err).To(BeNil())
				Expect(createdVNET.Spec.WhitelistSourceRanges).To(Equal(fakeSNATIP + "/32"))
				Expect(conditions.IsTrue(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW enabled and NCP version >= 3.0.1 and VNET does not exist", func() {
			BeforeEach(func() {
				// no pre-existing vnet obj
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(configmapObj, systemNamespaceObj).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "false").(*nsxtNetworkProvider)
				np = nsxNp
				err = np.ProvisionClusterNetwork(ctx)
			})

			It("should create new vnet with whitelist_source_ranges in spec", func() {
				Expect(err).To(BeNil())
				vnet, localerr := np.GetClusterNetworkName(ctx)
				Expect(localerr).To(BeNil())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(ctx.VSphereCluster.Name)))

				// Verify WhitelistSourceRanges have been updated
				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)

				Expect(createdVNET.Spec.WhitelistSourceRanges).To(Equal(fakeSNATIP + "/32"))
				// err is not empty, but it is because vnetObj does not have status mocked in this test

				Expect(conditions.IsTrue(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})

		Context("with nsx-t network provider and FW enabled and NCP version < 3.0.1 and VNET exists", func() {
			BeforeEach(func() {
				// test if NCP version is 3.0.0
				configmapObj.(*v1.ConfigMap).Data[util.NCPVersionKey] = "3.0.0"
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(runtimeObjs...).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "false").(*nsxtNetworkProvider)
				np = nsxNp
				err = np.ProvisionClusterNetwork(ctx)
			})

			It("should not update vnet with whitelist_source_ranges in spec", func() {
				Expect(err).To(BeNil())
				vnet, localerr := np.GetClusterNetworkName(ctx)
				Expect(localerr).To(BeNil())
				Expect(vnet).To(Equal(GetNSXTVirtualNetworkName(ctx.VSphereCluster.Name)))

				// Verify WhitelistSourceRanges is not included
				createdVNET := &ncpv1.VirtualNetwork{}
				err = client.Get(ctx, apitypes.NamespacedName{
					Name:      GetNSXTVirtualNetworkName(dummyCluster),
					Namespace: dummyNs,
				}, createdVNET)

				Expect(createdVNET.Spec.WhitelistSourceRanges).To(BeEmpty())
				// err is not empty, but it is because vnetObj does not have status mocked in this test

				Expect(conditions.IsTrue(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition)).To(BeTrue())
			})

			AfterEach(func() {
				// change NCP version back
				configmapObj.(*v1.ConfigMap).Data[util.NCPVersionKey] = util.NCPVersionSupportFW
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
						{Type: "Ready", Status: "False", Reason: testVnetNotRealizedReason, Message: testVnetNotRealizedMessage},
					},
				}
				vnetObj = createUnReadyNsxtVirtualNetwork(ctx, status)
				client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(vnetObj).Build()
				nsxNp, _ = NsxtNetworkProvider(client, "false").(*nsxtNetworkProvider)
				np = nsxNp

				err = np.VerifyNetworkStatus(ctx, vnetObj)

				expectedErrorMessage := fmt.Sprintf("virtual network ready status is: '%s' in cluster %s. reason: %s, message: %s",
					"False", apitypes.NamespacedName{Namespace: dummyNs, Name: dummyCluster}, testVnetNotRealizedReason, testVnetNotRealizedMessage)
				Expect(err).To(MatchError(expectedErrorMessage))
				Expect(conditions.IsFalse(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition)).To(BeTrue())
			})
		})
	})

	Context("GetVMServiceAnnotations", func() {
		Context("with netop network provider", func() {
			var defaultNetwork *netopv1alpha1.Network

			BeforeEach(func() {
				defaultNetwork = &netopv1alpha1.Network{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy-network",
						Namespace: dummyNs,
						Labels:    map[string]string{CAPVDefaultNetworkLabel: "true"},
					},
					Spec: netopv1alpha1.NetworkSpec{
						Type: netopv1alpha1.NetworkTypeVDS,
					},
				}
				scheme := runtime.NewScheme()
				Expect(netopv1alpha1.AddToScheme(scheme)).To(Succeed())
				client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(defaultNetwork).Build()
				np = NetOpNetworkProvider(client)
			})

			Context("with default network", func() {
				It("Should return expected annotations", func() {
					annotations, err := np.GetVMServiceAnnotations(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(annotations).To(HaveKeyWithValue("netoperator.vmware.com/network-name", defaultNetwork.Name))
				})
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
				Expect(netopv1alpha1.AddToScheme(scheme)).To(Succeed())
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
