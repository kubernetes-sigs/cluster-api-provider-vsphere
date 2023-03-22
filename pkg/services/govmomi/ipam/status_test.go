/*
Copyright 2023 The Kubernetes Authors.

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

package ipam

import (
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	ipamv1a1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
)

var (
	devMAC     = "0:0:0:0:a"
	myAPIGroup = "my-pool-api-group"
)

func Test_buildIPAMDeviceConfigs(t *testing.T) {
	var ctx context.VMContext
	var claim1, claim2, claim3 *ipamv1a1.IPAddressClaim
	var address1, address2, address3 *ipamv1a1.IPAddress
	var g *gomega.WithT

	before := func() {
		ctx = *fake.NewVMContext(fake.NewControllerContext(fake.NewControllerManagerContext()))

		g = gomega.NewWithT(t)
		namespace := "my-namespace"

		claim1 = &ipamv1a1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-0",
				Namespace: namespace,
			},
		}

		claim2 = &ipamv1a1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-1",
				Namespace: namespace,
			},
		}

		claim3 = &ipamv1a1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-2",
				Namespace: namespace,
			},
		}

		address1 = &ipamv1a1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-0-address0",
				Namespace: namespace,
			},
		}
		address2 = &ipamv1a1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-1-address1",
				Namespace: namespace,
			},
		}

		address3 = &ipamv1a1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-2-address2",
				Namespace: namespace,
			},
		}
	}

	t.Run("when a device has a IPAddressPool", func(_ *testing.T) {
		before()
		ctx.VSphereVM = &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1",
				Namespace: "my-namespace",
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Network: infrav1.NetworkSpec{
						Devices: []infrav1.NetworkDeviceSpec{
							{
								MACAddr: devMAC,
								AddressesFromPools: []corev1.TypedLocalObjectReference{
									{
										APIGroup: &myAPIGroup,
										Name:     "my-pool-1",
										Kind:     "my-pool-kind",
									},
									{
										APIGroup: &myAPIGroup,
										Name:     "my-pool-1",
										Kind:     "my-pool-kind",
									},
									{
										APIGroup: &myAPIGroup,
										Name:     "my-pool-ipv6",
										Kind:     "my-pool-kind",
									},
								},
							},
						},
					},
				},
			},
		}

		// Creates ip address claims
		g.Expect(ctx.Client.Create(ctx, claim1)).NotTo(gomega.HaveOccurred())
		g.Expect(ctx.Client.Create(ctx, claim2)).NotTo(gomega.HaveOccurred())
		g.Expect(ctx.Client.Create(ctx, claim3)).NotTo(gomega.HaveOccurred())

		// IP provider has not provided Addresses yet
		_, err := buildIPAMDeviceConfigs(ctx)
		g.Expect(err).To(gomega.Equal(ErrWaitingForIPAddr))

		// Simulate IP provider reconciling one claim
		g.Expect(ctx.Client.Create(ctx, address3)).NotTo(gomega.HaveOccurred())

		ipAddrClaim := &ipamv1a1.IPAddressClaim{}
		ipAddrClaimKey := apitypes.NamespacedName{
			Namespace: ctx.VSphereVM.Namespace,
			Name:      "vsphereVM1-0-2",
		}
		g.Expect(ctx.Client.Get(ctx, ipAddrClaimKey, ipAddrClaim)).NotTo(gomega.HaveOccurred())
		ipAddrClaim.Status.AddressRef.Name = "vsphereVM1-0-2-address2"
		g.Expect(ctx.Client.Update(ctx, ipAddrClaim)).NotTo(gomega.HaveOccurred())

		// Only the last claim has been bound
		_, err = buildIPAMDeviceConfigs(ctx)
		g.Expect(err).To(gomega.Equal(ErrWaitingForIPAddr))

		// Simulate IP provider reconciling remaining claims
		g.Expect(ctx.Client.Create(ctx, address1)).NotTo(gomega.HaveOccurred())
		g.Expect(ctx.Client.Create(ctx, address2)).NotTo(gomega.HaveOccurred())

		ipAddrClaimKey = apitypes.NamespacedName{
			Namespace: ctx.VSphereVM.Namespace,
			Name:      "vsphereVM1-0-0",
		}
		g.Expect(ctx.Client.Get(ctx, ipAddrClaimKey, ipAddrClaim)).NotTo(gomega.HaveOccurred())
		ipAddrClaim.Status.AddressRef.Name = "vsphereVM1-0-0-address0"
		g.Expect(ctx.Client.Update(ctx, ipAddrClaim)).NotTo(gomega.HaveOccurred())

		ipAddrClaimKey = apitypes.NamespacedName{
			Namespace: ctx.VSphereVM.Namespace,
			Name:      "vsphereVM1-0-1",
		}
		g.Expect(ctx.Client.Get(ctx, ipAddrClaimKey, ipAddrClaim)).NotTo(gomega.HaveOccurred())
		ipAddrClaim.Status.AddressRef.Name = "vsphereVM1-0-1-address1"
		g.Expect(ctx.Client.Update(ctx, ipAddrClaim)).NotTo(gomega.HaveOccurred())

		// Now that claims are fulfilled, reconciling should update
		// ipAddrs on network spec
		configs, err := buildIPAMDeviceConfigs(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(configs).To(gomega.HaveLen(1))

		config := configs[0]
		g.Expect(config.MACAddress).To(gomega.Equal(devMAC))
		g.Expect(config.DeviceIndex).To(gomega.Equal(0))
		g.Expect(config.IPAMAddresses).To(gomega.HaveLen(3))
	})

	t.Run("when a device has no pools", func(_ *testing.T) {
		before()
		ctx.VSphereVM = &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1",
				Namespace: "my-namespace",
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Network: infrav1.NetworkSpec{
						Devices: []infrav1.NetworkDeviceSpec{
							{
								MACAddr: devMAC,
								DHCP4:   true,
							},
						},
					},
				},
			},
		}

		// The IPAddressClaimed condition should not be added
		config, err := buildIPAMDeviceConfigs(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(config[0].IPAMAddresses).To(gomega.HaveLen(0))
	})
}

func Test_BuildState(t *testing.T) {
	var ctx context.VMContext
	var claim1, claim2, claim3 *ipamv1a1.IPAddressClaim
	var address1, address2, address3 *ipamv1a1.IPAddress
	var g *gomega.WithT

	before := func() {
		ctx = *fake.NewVMContext(fake.NewControllerContext(fake.NewControllerManagerContext()))

		g = gomega.NewWithT(t)

		claim1 = &ipamv1a1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-0",
				Namespace: "my-namespace",
			},
		}

		claim2 = &ipamv1a1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-1",
				Namespace: "my-namespace",
			},
		}

		claim3 = &ipamv1a1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-2",
				Namespace: "my-namespace",
			},
		}

		address1 = &ipamv1a1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-0-address0",
				Namespace: "my-namespace",
			},
			Spec: ipamv1a1.IPAddressSpec{
				Address: "10.0.0.50",
				Prefix:  24,
				Gateway: "10.0.0.1",
			},
		}
		address2 = &ipamv1a1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-1-address1",
				Namespace: "my-namespace",
			},
			Spec: ipamv1a1.IPAddressSpec{
				Address: "10.0.1.50",
				Prefix:  30,
				Gateway: "10.0.0.1",
			},
		}

		address3 = &ipamv1a1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1-0-2-address2",
				Namespace: "my-namespace",
			},
			Spec: ipamv1a1.IPAddressSpec{
				Address: "fe80::cccc:12",
				Prefix:  64,
				Gateway: "fe80::cccc:1",
			},
		}
	}

	t.Run("when a device has a IPAddressPool", func(_ *testing.T) {
		before()
		ctx.VSphereVM = &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1",
				Namespace: "my-namespace",
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Network: infrav1.NetworkSpec{
						Devices: []infrav1.NetworkDeviceSpec{
							{
								MACAddr: devMAC,
								AddressesFromPools: []corev1.TypedLocalObjectReference{
									{
										APIGroup: &myAPIGroup,
										Name:     "my-pool-1",
										Kind:     "my-pool-kind",
									},
									{
										APIGroup: &myAPIGroup,
										Name:     "my-pool-1",
										Kind:     "my-pool-kind",
									},
									{
										APIGroup: &myAPIGroup,
										Name:     "my-pool-ipv6",
										Kind:     "my-pool-kind",
									},
								},
							},
						},
					},
				},
			},
		}

		// Creates ip address claims
		g.Expect(ctx.Client.Create(ctx, claim1)).NotTo(gomega.HaveOccurred())
		g.Expect(ctx.Client.Create(ctx, claim2)).NotTo(gomega.HaveOccurred())
		g.Expect(ctx.Client.Create(ctx, claim3)).NotTo(gomega.HaveOccurred())

		// IP provider has not provided Addresses yet
		_, err := BuildState(ctx)
		g.Expect(err).To(gomega.Equal(ErrWaitingForIPAddr))

		// Simulate IP provider reconciling one claim
		g.Expect(ctx.Client.Create(ctx, address3)).NotTo(gomega.HaveOccurred())

		ipAddrClaim := &ipamv1a1.IPAddressClaim{}
		ipAddrClaimKey := apitypes.NamespacedName{
			Namespace: ctx.VSphereVM.Namespace,
			Name:      "vsphereVM1-0-2",
		}
		g.Expect(ctx.Client.Get(ctx, ipAddrClaimKey, ipAddrClaim)).NotTo(gomega.HaveOccurred())
		ipAddrClaim.Status.AddressRef.Name = "vsphereVM1-0-2-address2"
		g.Expect(ctx.Client.Update(ctx, ipAddrClaim)).NotTo(gomega.HaveOccurred())

		// Only the last claim has been bound
		_, err = BuildState(ctx)
		g.Expect(err).To(gomega.Equal(ErrWaitingForIPAddr))

		// Simulate IP provider reconciling remaining claims
		g.Expect(ctx.Client.Create(ctx, address1)).NotTo(gomega.HaveOccurred())
		g.Expect(ctx.Client.Create(ctx, address2)).NotTo(gomega.HaveOccurred())

		ipAddrClaimKey = apitypes.NamespacedName{
			Namespace: ctx.VSphereVM.Namespace,
			Name:      "vsphereVM1-0-0",
		}
		g.Expect(ctx.Client.Get(ctx, ipAddrClaimKey, ipAddrClaim)).NotTo(gomega.HaveOccurred())
		ipAddrClaim.Status.AddressRef.Name = "vsphereVM1-0-0-address0"
		g.Expect(ctx.Client.Update(ctx, ipAddrClaim)).NotTo(gomega.HaveOccurred())

		ipAddrClaimKey = apitypes.NamespacedName{
			Namespace: ctx.VSphereVM.Namespace,
			Name:      "vsphereVM1-0-1",
		}
		g.Expect(ctx.Client.Get(ctx, ipAddrClaimKey, ipAddrClaim)).NotTo(gomega.HaveOccurred())
		ipAddrClaim.Status.AddressRef.Name = "vsphereVM1-0-1-address1"
		g.Expect(ctx.Client.Update(ctx, ipAddrClaim)).NotTo(gomega.HaveOccurred())

		// Now that claims are fulfilled, reconciling should update
		// ipAddrs on network spec
		state, err := BuildState(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(state).To(gomega.HaveLen(1))

		g.Expect(state[devMAC].IPAddrs).To(gomega.HaveLen(3))
		g.Expect(state[devMAC].IPAddrs[0]).To(gomega.Equal("10.0.0.50/24"))
		g.Expect(state[devMAC].Gateway4).To(gomega.Equal("10.0.0.1"))
		g.Expect(state[devMAC].IPAddrs[1]).To(gomega.Equal("10.0.1.50/30"))
		g.Expect(state[devMAC].Gateway4).To(gomega.Equal("10.0.0.1"))
		g.Expect(state[devMAC].IPAddrs[2]).To(gomega.Equal("fe80::cccc:12/64"))
		g.Expect(state[devMAC].Gateway6).To(gomega.Equal("fe80::cccc:1"))
	})

	t.Run("when a device has no pools", func(_ *testing.T) {
		before()
		ctx.VSphereVM = &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphereVM1",
				Namespace: "my-namespace",
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Network: infrav1.NetworkSpec{
						Devices: []infrav1.NetworkDeviceSpec{
							{
								MACAddr: devMAC,
								DHCP4:   true,
							},
						},
					},
				},
			},
		}

		state, err := BuildState(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(state).To(gomega.HaveLen(0))
	})

	t.Run("when realized IP addresses are incorrect", func(t *testing.T) {
		beforeWithClaimsAndAddressCreated := func() {
			before()

			claim1 = &ipamv1a1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1-0-0",
					Namespace: "my-namespace",
				},
				Status: ipamv1a1.IPAddressClaimStatus{
					AddressRef: corev1.LocalObjectReference{
						Name: "vsphereVM1-0-0",
					},
				},
			}

			claim2 = &ipamv1a1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1-0-1",
					Namespace: "my-namespace",
				},
				Status: ipamv1a1.IPAddressClaimStatus{
					AddressRef: corev1.LocalObjectReference{
						Name: "vsphereVM1-0-1",
					},
				},
			}

			claim3 = &ipamv1a1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1-1-0",
					Namespace: "my-namespace",
				},
				Status: ipamv1a1.IPAddressClaimStatus{
					AddressRef: corev1.LocalObjectReference{
						Name: "vsphereVM1-1-0",
					},
				},
			}

			address1 = &ipamv1a1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1-0-0",
					Namespace: "my-namespace",
				},
				Spec: ipamv1a1.IPAddressSpec{
					Address: "10.0.1.50",
					Prefix:  24,
					Gateway: "10.0.0.1",
				},
			}

			address2 = &ipamv1a1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1-0-1",
					Namespace: "my-namespace",
				},
				Spec: ipamv1a1.IPAddressSpec{
					Address: "10.0.1.51",
					Prefix:  24,
					Gateway: "10.0.0.1",
				},
			}

			address3 = &ipamv1a1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1-1-0",
					Namespace: "my-namespace",
				},
				Spec: ipamv1a1.IPAddressSpec{
					Address: "11.0.1.50",
					Prefix:  24,
					Gateway: "11.0.0.1",
				},
			}

			ctx.VSphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphereVM1",
					Namespace: "my-namespace",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
						Network: infrav1.NetworkSpec{
							Devices: []infrav1.NetworkDeviceSpec{
								{
									AddressesFromPools: []corev1.TypedLocalObjectReference{
										{
											APIGroup: &myAPIGroup,
											Name:     "my-pool-1",
											Kind:     "my-pool-kind",
										},
										{
											APIGroup: &myAPIGroup,
											Name:     "my-pool-2",
											Kind:     "my-pool-kind",
										},
									},
								},
								{
									AddressesFromPools: []corev1.TypedLocalObjectReference{
										{
											APIGroup: &myAPIGroup,
											Name:     "my-pool-3",
											Kind:     "my-pool-kind",
										},
									},
								},
							},
						},
					},
				},
			}

			g.Expect(ctx.Client.Create(ctx, claim1)).NotTo(gomega.HaveOccurred())
			g.Expect(ctx.Client.Create(ctx, claim2)).NotTo(gomega.HaveOccurred())
			g.Expect(ctx.Client.Create(ctx, claim3)).NotTo(gomega.HaveOccurred())

			g.Expect(ctx.Client.Create(ctx, address1)).NotTo(gomega.HaveOccurred())
			g.Expect(ctx.Client.Create(ctx, address2)).NotTo(gomega.HaveOccurred())
			g.Expect(ctx.Client.Create(ctx, address3)).NotTo(gomega.HaveOccurred())
		}

		t.Run("when a provider assigns an IPAddress without an Address field", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()

			address1.Spec.Address = ""
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError("IPAddress my-namespace/vsphereVM1-0-0 has invalid ip address: \"/24\""))
		})

		t.Run("when a provider assigns an IPAddress with an invalid IP in the Address field", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()
			// Simulate an invalid ip address was provided: the address is not a valid ip
			address1.Spec.Address = "invalid-ip"
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError("IPAddress my-namespace/vsphereVM1-0-0 has invalid ip address: \"invalid-ip/24\""))
		})

		t.Run("when a provider assigns an IPAddress with an invalid value in the Prefix field", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()
			// Simulate an invalid prefix address was provided: the prefix is out of bounds
			address1.Spec.Prefix = 200
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError("IPAddress my-namespace/vsphereVM1-0-0 has invalid ip address: \"10.0.1.50/200\""))
		})

		t.Run("when a provider assigns an IPAddress with an invalid value in the Gateway field", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()
			// Simulate an invalid gateway was provided: the gateway is an invalid ip
			address1.Spec.Gateway = "invalid-gateway"
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError("IPAddress my-namespace/vsphereVM1-0-0 has invalid gateway: \"invalid-gateway\""))
		})

		t.Run("when a provider assigns an IPAddress where the Gateway and Address fields are mismatched", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()
			// Simulate mismatch address and gateways were provided
			address1.Spec.Address = "10.0.1.50"
			address1.Spec.Gateway = "fd01::1"
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError("IPAddress my-namespace/vsphereVM1-0-0 has mismatched gateway and address IP families"))

			// Simulate mismatch address and gateways were provided
			address1.Spec.Address = "fd00:cccc::1"
			address1.Spec.Gateway = "10.0.0.1"
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())

			_, err = BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError("IPAddress my-namespace/vsphereVM1-0-0 has mismatched gateway and address IP families"))
		})

		t.Run("when there are multiple IPAddresses for a device with different Gateways", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()
			// Simulate multiple gateways were provided
			address1.Spec.Address = "10.0.1.50"
			address1.Spec.Gateway = "10.0.0.2"
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())
			address2.Spec.Address = "10.0.1.51"
			address2.Spec.Gateway = "10.0.0.3"
			g.Expect(ctx.Client.Update(ctx, address2)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError("the IPv4 IPAddresses assigned to the same device (index 0) do not have the same gateway"))

			// Simulate multiple gateways were provided
			address1.Spec.Address = "fd00:cccc::2"
			address1.Spec.Gateway = "fd00::1"
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())
			address2.Spec.Address = "fd00:cccc::3"
			address2.Spec.Gateway = "fd00::2"
			g.Expect(ctx.Client.Update(ctx, address2)).NotTo(gomega.HaveOccurred())

			_, err = BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError("the IPv6 IPAddresses assigned to the same device (index 0) do not have the same gateway"))
		})

		t.Run("when a user specified gateway does not match the gateway provided by IPAM", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()

			ctx.VSphereVM.Spec.VirtualMachineCloneSpec.Network.Devices[0].Gateway4 = "10.10.10.1"
			ctx.VSphereVM.Spec.VirtualMachineCloneSpec.Network.Devices[0].Gateway6 = "fd00::2"
			address2.Spec.Address = "fd00:cccc::1"
			address2.Spec.Gateway = "fd00::1"
			g.Expect(ctx.Client.Update(ctx, address2)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("the IPv4 Gateway for IPAddress vsphereVM1-0-0 does not match the Gateway4 already configured on device (index 0)")))

			// Fix the Gateway4 for dev0
			ctx.VSphereVM.Spec.VirtualMachineCloneSpec.Network.Devices[0].Gateway4 = "10.0.0.1"
			_, err = BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError("the IPv6 Gateway for IPAddress vsphereVM1-0-1 does not match the Gateway6 already configured on device (index 0)"))
		})

		t.Run("when there are multiple IPAM ip configuration issues on one vm, it notes all of the problems", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()

			beforeWithClaimsAndAddressCreated()

			address1.Spec.Address = "10.10.10.10.10"
			address2.Spec.Address = "11.11.11.11.11"
			address3.Spec.Address = "12.12.12.12.12"
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())
			g.Expect(ctx.Client.Update(ctx, address2)).NotTo(gomega.HaveOccurred())
			g.Expect(ctx.Client.Update(ctx, address3)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError(
				gomega.ContainSubstring("IPAddress my-namespace/vsphereVM1-0-0 has invalid ip address: \"10.10.10.10.10/24\"")))
			g.Expect(err).To(gomega.MatchError(
				gomega.ContainSubstring("IPAddress my-namespace/vsphereVM1-0-1 has invalid ip address: \"11.11.11.11.11/24\"")))
			g.Expect(err).To(gomega.MatchError(
				gomega.ContainSubstring("IPAddress my-namespace/vsphereVM1-1-0 has invalid ip address: \"12.12.12.12.12/24\"")))
		})

		t.Run("when there are multiple IPAM gateway configuration issues on one vm, it notes all of the problems", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()

			address1.Spec.Gateway = "10.10.10.10.10"
			address2.Spec.Gateway = "11.11.11.11.11"
			address3.Spec.Gateway = "12.12.12.12.12"
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())
			g.Expect(ctx.Client.Update(ctx, address2)).NotTo(gomega.HaveOccurred())
			g.Expect(ctx.Client.Update(ctx, address3)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError(
				gomega.ContainSubstring("IPAddress my-namespace/vsphereVM1-0-0 has invalid gateway: \"10.10.10.10.10\"")))
			g.Expect(err).To(gomega.MatchError(
				gomega.ContainSubstring("IPAddress my-namespace/vsphereVM1-0-1 has invalid gateway: \"11.11.11.11.11\"")))
			g.Expect(err).To(gomega.MatchError(
				gomega.ContainSubstring("IPAddress my-namespace/vsphereVM1-1-0 has invalid gateway: \"12.12.12.12.12\"")))
		})

		t.Run("when there are duplicate IPAddresses", func(_ *testing.T) {
			beforeWithClaimsAndAddressCreated()

			address1.Spec.Address = "10.0.0.50"
			address2.Spec.Address = "10.0.0.50"
			g.Expect(ctx.Client.Update(ctx, address1)).NotTo(gomega.HaveOccurred())
			g.Expect(ctx.Client.Update(ctx, address2)).NotTo(gomega.HaveOccurred())

			_, err := BuildState(ctx)
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err).To(gomega.MatchError(
				gomega.ContainSubstring("IPAddress my-namespace/vsphereVM1-0-1 is a duplicate of another address: \"10.0.0.50/24\"")))
		})
	})
}
