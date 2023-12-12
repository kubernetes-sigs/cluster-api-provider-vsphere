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

package controllers

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func Test_vmReconciler_reconcileIPAddressClaims(t *testing.T) {
	name, namespace := "test-vm", "my-namespace"
	setup := func(vsphereVM *infrav1.VSphereVM, initObjects ...client.Object) *capvcontext.VMContext {
		return &capvcontext.VMContext{
			ControllerManagerContext: fake.NewControllerManagerContext(initObjects...),
			VSphereVM:                vsphereVM,
		}
	}
	ctx := context.Background()

	t.Run("when VSphereVM Spec has address pool references", func(t *testing.T) {
		vsphereVM := &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: "my-cluster",
				},
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Network: infrav1.NetworkSpec{
						Devices: []infrav1.NetworkDeviceSpec{{
							AddressesFromPools: []corev1.TypedLocalObjectReference{
								poolRef("my-pool-1"),
							}},
							{
								AddressesFromPools: []corev1.TypedLocalObjectReference{
									poolRef("my-pool-2"),
									poolRef("my-pool-3"),
								},
							},
						},
					},
				},
			},
		}

		t.Run("when no claims exist", func(t *testing.T) {
			g := gomega.NewWithT(t)

			testCtx := setup(vsphereVM)
			err := vmReconciler{}.reconcileIPAddressClaims(ctx, testCtx)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			ipAddrClaimList := &ipamv1.IPAddressClaimList{}
			g.Expect(testCtx.Client.List(ctx, ipAddrClaimList)).To(gomega.Succeed())
			g.Expect(ipAddrClaimList.Items).To(gomega.HaveLen(3))

			for idx := range ipAddrClaimList.Items {
				claim := ipAddrClaimList.Items[idx]
				g.Expect(claim.Finalizers).To(gomega.HaveLen(1))
				g.Expect(ctrlutil.ContainsFinalizer(&claim, infrav1.IPAddressClaimFinalizer)).To(gomega.BeTrue())

				g.Expect(claim.OwnerReferences).To(gomega.HaveLen(1))
				g.Expect(claim.OwnerReferences[0].Name).To(gomega.Equal(vsphereVM.Name))
				g.Expect(claim.Labels).To(gomega.HaveKeyWithValue(clusterv1.ClusterNameLabel, "my-cluster"))
			}

			claimedCondition := conditions.Get(testCtx.VSphereVM, infrav1.IPAddressClaimedCondition)
			g.Expect(claimedCondition).NotTo(gomega.BeNil())
			g.Expect(claimedCondition.Status).To(gomega.Equal(corev1.ConditionFalse))
			g.Expect(claimedCondition.Reason).To(gomega.Equal(infrav1.IPAddressClaimsBeingCreatedReason))
			g.Expect(claimedCondition.Message).To(gomega.Equal("3/3 claims being created"))
		})

		ipAddrClaim := func(name, poolName string) *ipamv1.IPAddressClaim {
			return &ipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec:   ipamv1.IPAddressClaimSpec{PoolRef: poolRef(poolName)},
				Status: ipamv1.IPAddressClaimStatus{},
			}
		}

		t.Run("when all claims exist", func(t *testing.T) {
			g := gomega.NewWithT(t)

			testCtx := setup(vsphereVM,
				ipAddrClaim(util.IPAddressClaimName(name, 0, 0), "my-pool-1"),
				ipAddrClaim(util.IPAddressClaimName(name, 1, 0), "my-pool-2"),
				ipAddrClaim(util.IPAddressClaimName(name, 1, 1), "my-pool-3"),
			)
			err := vmReconciler{}.reconcileIPAddressClaims(ctx, testCtx)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			claimedCondition := conditions.Get(testCtx.VSphereVM, infrav1.IPAddressClaimedCondition)
			g.Expect(claimedCondition).NotTo(gomega.BeNil())
			g.Expect(claimedCondition.Status).To(gomega.Equal(corev1.ConditionFalse))
			g.Expect(claimedCondition.Reason).To(gomega.Equal(infrav1.WaitingForIPAddressReason))
			g.Expect(claimedCondition.Message).To(gomega.Equal("3/3 claims being processed"))

			ipAddrClaimList := &ipamv1.IPAddressClaimList{}
			g.Expect(testCtx.Client.List(ctx, ipAddrClaimList)).To(gomega.Succeed())

			for idx := range ipAddrClaimList.Items {
				claim := ipAddrClaimList.Items[idx]
				g.Expect(claim.Finalizers).To(gomega.HaveLen(1))
				g.Expect(ctrlutil.ContainsFinalizer(&claim, infrav1.IPAddressClaimFinalizer)).To(gomega.BeTrue())

				g.Expect(claim.OwnerReferences).To(gomega.HaveLen(1))
				g.Expect(claim.OwnerReferences[0].Name).To(gomega.Equal(vsphereVM.Name))
				g.Expect(claim.Labels).To(gomega.HaveKeyWithValue(clusterv1.ClusterNameLabel, "my-cluster"))
			}
		})

		t.Run("when all claims exist and are realized", func(t *testing.T) {
			g := gomega.NewWithT(t)

			realizedIPAddrClaimOne := ipAddrClaim(util.IPAddressClaimName(name, 0, 0), "my-pool-1")
			realizedIPAddrClaimOne.Status.AddressRef.Name = "blah-one"

			realizedIPAddrClaimTwo := ipAddrClaim(util.IPAddressClaimName(name, 1, 0), "my-pool-2")
			realizedIPAddrClaimTwo.Status.AddressRef.Name = "blah-two"

			realizedIPAddrClaimThree := ipAddrClaim(util.IPAddressClaimName(name, 1, 1), "my-pool-3")
			realizedIPAddrClaimThree.Status.AddressRef.Name = "blah-three"

			testCtx := setup(vsphereVM, realizedIPAddrClaimOne, realizedIPAddrClaimTwo, realizedIPAddrClaimThree)
			err := vmReconciler{}.reconcileIPAddressClaims(ctx, testCtx)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			claimedCondition := conditions.Get(testCtx.VSphereVM, infrav1.IPAddressClaimedCondition)
			g.Expect(claimedCondition).NotTo(gomega.BeNil())
			g.Expect(claimedCondition.Status).To(gomega.Equal(corev1.ConditionTrue))

			ipAddrClaimList := &ipamv1.IPAddressClaimList{}
			g.Expect(testCtx.Client.List(ctx, ipAddrClaimList)).To(gomega.Succeed())

			for idx := range ipAddrClaimList.Items {
				claim := ipAddrClaimList.Items[idx]
				g.Expect(claim.Finalizers).To(gomega.HaveLen(1))
				g.Expect(ctrlutil.ContainsFinalizer(&claim, infrav1.IPAddressClaimFinalizer)).To(gomega.BeTrue())

				g.Expect(claim.OwnerReferences).To(gomega.HaveLen(1))
				g.Expect(claim.OwnerReferences[0].Name).To(gomega.Equal(vsphereVM.Name))
				g.Expect(claim.Labels).To(gomega.HaveKeyWithValue(clusterv1.ClusterNameLabel, "my-cluster"))
			}
		})

		t.Run("when all existing claims have Ready Condition set", func(t *testing.T) {
			g := gomega.NewWithT(t)

			ipAddrClaimWithReadyConditionTrue := ipAddrClaim(util.IPAddressClaimName(name, 0, 0), "my-pool-1")
			ipAddrClaimWithReadyConditionTrue.Status.Conditions = clusterv1.Conditions{
				*conditions.TrueCondition(clusterv1.ReadyCondition),
			}

			ipAddrClaimWithReadyConditionFalse := ipAddrClaim(util.IPAddressClaimName(name, 1, 0), "my-pool-2")
			ipAddrClaimWithReadyConditionFalse.Status.Conditions = clusterv1.Conditions{
				*conditions.FalseCondition(clusterv1.ReadyCondition, "IPAddressFetchProgress", clusterv1.ConditionSeverityInfo, ""),
			}

			secondIPAddrClaimWithReadyConditionTrue := ipAddrClaim(util.IPAddressClaimName(name, 1, 1), "my-pool-3")
			secondIPAddrClaimWithReadyConditionTrue.Status.Conditions = clusterv1.Conditions{
				*conditions.TrueCondition(clusterv1.ReadyCondition),
			}

			testCtx := setup(vsphereVM,
				ipAddrClaimWithReadyConditionTrue,
				ipAddrClaimWithReadyConditionFalse,
				secondIPAddrClaimWithReadyConditionTrue,
			)
			err := vmReconciler{}.reconcileIPAddressClaims(ctx, testCtx)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			claimedCondition := conditions.Get(testCtx.VSphereVM, infrav1.IPAddressClaimedCondition)
			g.Expect(claimedCondition).NotTo(gomega.BeNil())
			g.Expect(claimedCondition.Status).To(gomega.Equal(corev1.ConditionFalse))
		})

		t.Run("when some existing claims have Ready Condition set", func(t *testing.T) {
			g := gomega.NewWithT(t)

			ipAddrClaimWithReadyConditionTrue := ipAddrClaim(util.IPAddressClaimName(name, 0, 0), "my-pool-1")
			ipAddrClaimWithReadyConditionTrue.Status.Conditions = clusterv1.Conditions{
				*conditions.TrueCondition(clusterv1.ReadyCondition),
			}
			ipAddrClaimWithReadyConditionTrue.Status.AddressRef.Name = "blah-one"

			ipAddrClaimWithReadyConditionFalse := ipAddrClaim(util.IPAddressClaimName(name, 1, 0), "my-pool-2")
			ipAddrClaimWithReadyConditionFalse.Status.Conditions = clusterv1.Conditions{
				*conditions.FalseCondition(clusterv1.ReadyCondition, "IPAddressFetchProgress", clusterv1.ConditionSeverityInfo, ""),
			}

			iPAddrClaimWithNoReadyCondition := ipAddrClaim(util.IPAddressClaimName(name, 1, 1), "my-pool-3")

			testCtx := setup(vsphereVM,
				ipAddrClaimWithReadyConditionTrue,
				ipAddrClaimWithReadyConditionFalse,
				iPAddrClaimWithNoReadyCondition,
			)
			err := vmReconciler{}.reconcileIPAddressClaims(ctx, testCtx)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			claimedCondition := conditions.Get(testCtx.VSphereVM, infrav1.IPAddressClaimedCondition)
			g.Expect(claimedCondition).NotTo(gomega.BeNil())
			g.Expect(claimedCondition.Status).To(gomega.Equal(corev1.ConditionFalse))
			g.Expect(claimedCondition.Reason).To(gomega.Equal(infrav1.WaitingForIPAddressReason))
			g.Expect(claimedCondition.Message).To(gomega.Equal("2/3 claims being processed"))
		})
	})
}

func poolRef(name string) corev1.TypedLocalObjectReference {
	return corev1.TypedLocalObjectReference{
		APIGroup: pointer.String("test.ipam.provider.io/v1"),
		Name:     name,
		Kind:     "my-pool-kind",
	}
}
