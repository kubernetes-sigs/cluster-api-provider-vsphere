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
	goctx "context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/vmware/govmomi/simulator"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers"
)

var _ = Describe("VSphereDeploymentZoneReconciler", func() {
	var (
		simr *helpers.Simulator
		ctx  goctx.Context

		vsphereDeploymentZone *infrav1.VSphereDeploymentZone
		vsphereFailureDomain  *infrav1.VSphereFailureDomain
	)

	BeforeEach(func() {
		model := simulator.VPX()
		model.Pool = 1

		var err error
		simr, err = helpers.VCSimBuilder().
			WithModel(model).
			WithOperations().
			Build()
		Expect(err).NotTo(HaveOccurred())

		operations := []string{
			"tags.category.create -t Datacenter,ClusterComputeResource k8s-region",
			"tags.category.create -t Datacenter,ClusterComputeResource k8s-zone",
			"tags.create -c k8s-region k8s-region-west",
			"tags.create -c k8s-zone k8s-zone-west-1",
			"tags.attach -c k8s-region k8s-region-west /DC0",
			"tags.attach -c k8s-zone k8s-zone-west-1 /DC0/host/DC0_C0",
		}
		for _, op := range operations {
			Expect(simr.Run(op, gbytes.NewBuffer(), gbytes.NewBuffer())).To(Succeed())
		}

		ctx = goctx.Background()
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, vsphereDeploymentZone, vsphereFailureDomain)).To(Succeed())
	})

	It("should create a deployment zone & failure domain", func() {
		dzName := "blah"
		fdName := "blah-fd"

		vsphereDeploymentZone = &infrav1.VSphereDeploymentZone{
			ObjectMeta: metav1.ObjectMeta{
				Name: dzName,
			},
			Spec: infrav1.VSphereDeploymentZoneSpec{
				Server:        simr.ServerURL().Host,
				FailureDomain: fdName,
				ControlPlane:  pointer.Bool(true),
				PlacementConstraint: infrav1.PlacementConstraint{
					ResourcePool: "DC0_C0_RP1",
					Folder:       "/",
				},
			}}
		Expect(testEnv.Create(ctx, vsphereDeploymentZone)).To(Succeed())

		vsphereFailureDomain = &infrav1.VSphereFailureDomain{
			ObjectMeta: metav1.ObjectMeta{
				Name: fdName,
			},
			Spec: infrav1.VSphereFailureDomainSpec{
				Region: infrav1.FailureDomain{
					Name:          "k8s-region-west",
					Type:          infrav1.DatacenterFailureDomain,
					TagCategory:   "k8s-region",
					AutoConfigure: pointer.Bool(false),
				},
				Zone: infrav1.FailureDomain{
					Name:          "k8s-zone-west-1",
					Type:          infrav1.ComputeClusterFailureDomain,
					TagCategory:   "k8s-zone",
					AutoConfigure: pointer.Bool(false),
				},
				Topology: infrav1.Topology{
					Datacenter:     "DC0",
					ComputeCluster: pointer.String("DC0_C0"),
					Datastore:      "LocalDS_0",
					Networks:       []string{"VM Network"},
				},
			},
		}
		Expect(testEnv.Create(ctx, vsphereFailureDomain)).To(Succeed())

		Eventually(func() bool {
			if err := testEnv.Get(ctx, client.ObjectKey{Name: dzName}, vsphereDeploymentZone); err != nil {
				return false
			}
			return len(vsphereDeploymentZone.Finalizers) > 0
		}, timeout).Should(BeTrue())

		Eventually(func() bool {
			if err := testEnv.Get(ctx, client.ObjectKey{Name: dzName}, vsphereDeploymentZone); err != nil {
				return false
			}
			return conditions.IsTrue(vsphereDeploymentZone, infrav1.VCenterAvailableCondition) &&
				conditions.IsTrue(vsphereDeploymentZone, infrav1.PlacementConstraintMetCondition) &&
				conditions.IsTrue(vsphereDeploymentZone, infrav1.VSphereFailureDomainValidatedCondition)
		}, timeout).Should(BeTrue())

		By("sets the owner ref on the vsphereFailureDomain object")
		Expect(testEnv.Get(ctx, client.ObjectKey{Name: fdName}, vsphereFailureDomain)).To(Succeed())
		ownerRefs := vsphereFailureDomain.GetOwnerReferences()
		Expect(ownerRefs).To(HaveLen(1))
		Expect(ownerRefs[0].Name).To(Equal(dzName))
		Expect(ownerRefs[0].Kind).To(Equal("VSphereDeploymentZone"))
	})

	Context("With incorrect details: when resource pool is not owned by compute cluster", func() {
		It("should fail creation of deployment zone", func() {
			vsphereDeploymentZone = &infrav1.VSphereDeploymentZone{
				ObjectMeta: metav1.ObjectMeta{
					Name: "blah-two",
				},
				Spec: infrav1.VSphereDeploymentZoneSpec{
					Server:        simr.ServerURL().Host,
					FailureDomain: "blah-fd-two",
					ControlPlane:  pointer.Bool(true),
					PlacementConstraint: infrav1.PlacementConstraint{
						ResourcePool: "DC0_C1_RP1",
						Folder:       "/",
					},
				}}
			Expect(testEnv.Create(ctx, vsphereDeploymentZone)).To(Succeed())

			vsphereFailureDomain = &infrav1.VSphereFailureDomain{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VSphereFailureDomain",
					APIVersion: infrav1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "blah-fd-two",
				},
				Spec: infrav1.VSphereFailureDomainSpec{
					Region: infrav1.FailureDomain{
						Name:          "k8s-region-west",
						Type:          infrav1.DatacenterFailureDomain,
						TagCategory:   "k8s-region",
						AutoConfigure: pointer.Bool(false),
					},
					Zone: infrav1.FailureDomain{
						Name:          "k8s-zone-west-1",
						Type:          infrav1.ComputeClusterFailureDomain,
						TagCategory:   "k8s-zone",
						AutoConfigure: pointer.Bool(false),
					},
					Topology: infrav1.Topology{
						Datacenter:     "DC0",
						ComputeCluster: pointer.String("DC0_C0"),
						Datastore:      "LocalDS_0",
						Networks:       []string{"VM Network"},
					},
				},
			}
			Expect(testEnv.Create(ctx, vsphereFailureDomain)).To(Succeed())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, client.ObjectKey{Name: "blah-two"}, vsphereDeploymentZone); err != nil {
					return false
				}
				return conditions.IsFalse(vsphereDeploymentZone, infrav1.PlacementConstraintMetCondition)
			}, timeout).Should(BeTrue())
		})
	})

	Context("Delete VSphereDeploymentZone", func() {
		It("should delete the associated failure domain", func() {
			fdName := "blah-fd-three"

			vsphereDeploymentZone = &infrav1.VSphereDeploymentZone{
				ObjectMeta: metav1.ObjectMeta{
					Name: "blah-three",
				},
				Spec: infrav1.VSphereDeploymentZoneSpec{
					Server:        simr.ServerURL().Host,
					FailureDomain: fdName,
					ControlPlane:  pointer.Bool(true),
					PlacementConstraint: infrav1.PlacementConstraint{
						ResourcePool: "DC0_C0_RP1",
						Folder:       "/",
					},
				}}
			Expect(testEnv.Create(ctx, vsphereDeploymentZone)).To(Succeed())

			vsphereFailureDomain = &infrav1.VSphereFailureDomain{
				ObjectMeta: metav1.ObjectMeta{
					Name: fdName,
				},
				Spec: infrav1.VSphereFailureDomainSpec{
					Region: infrav1.FailureDomain{
						Name:          "k8s-region-west",
						Type:          infrav1.DatacenterFailureDomain,
						TagCategory:   "k8s-region",
						AutoConfigure: pointer.Bool(false),
					},
					Zone: infrav1.FailureDomain{
						Name:          "k8s-zone-west-1",
						Type:          infrav1.ComputeClusterFailureDomain,
						TagCategory:   "k8s-zone",
						AutoConfigure: pointer.Bool(false),
					},
					Topology: infrav1.Topology{
						Datacenter:     "DC0",
						ComputeCluster: pointer.String("DC0_C0"),
						Datastore:      "LocalDS_0",
						Networks:       []string{"VM Network"},
					},
				},
			}
			Expect(testEnv.Create(ctx, vsphereFailureDomain)).To(Succeed())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, client.ObjectKey{Name: "blah-three"}, vsphereDeploymentZone); err != nil {
					return false
				}
				return pointer.BoolDeref(vsphereDeploymentZone.Status.Ready, false) &&
					conditions.IsTrue(vsphereDeploymentZone, clusterv1.ReadyCondition)
			}, timeout).Should(BeTrue())

			By("deleting the vsphere deployment zone")
			Expect(testEnv.Delete(ctx, vsphereFailureDomain)).To(Succeed())

			Eventually(func() bool {
				fd := &infrav1.VSphereFailureDomain{}
				err := testEnv.Get(ctx, client.ObjectKey{Name: fdName}, fd)
				return apierrors.IsNotFound(err)
			}, timeout).Should(BeTrue())
		})
	})
})

func TestVsphereDeploymentZone_Failed_ReconcilePlacementConstraint(t *testing.T) {
	tests := []struct {
		name                string
		placementConstraint infrav1.PlacementConstraint
	}{
		{
			name: "when resource pool is not found",
			placementConstraint: infrav1.PlacementConstraint{
				ResourcePool: "DC0_C1_RP3",
				Folder:       "/",
			},
		},
		{
			name: "when folder is not found",
			placementConstraint: infrav1.PlacementConstraint{
				ResourcePool: "DC0_C1_RP1",
				Folder:       "/does-not-exist",
			},
		},
	}

	for _, tt := range tests {
		// Looks odd, but need to reinitialize test variable
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			model := simulator.VPX()
			model.Cluster = 2
			model.Pool = 2

			simr, err := helpers.VCSimBuilder().
				WithModel(model).
				Build()
			if err != nil {
				t.Fatalf("unable to create simulator %s", err)
			}
			defer simr.Destroy()

			mgmtContext := fake.NewControllerManagerContext()
			mgmtContext.Username = simr.ServerURL().User.Username()
			pass, _ := simr.ServerURL().User.Password()
			mgmtContext.Password = pass

			controllerCtx := fake.NewControllerContext(mgmtContext)

			deploymentZoneCtx := &context.VSphereDeploymentZoneContext{
				ControllerContext: controllerCtx,
				VSphereDeploymentZone: &infrav1.VSphereDeploymentZone{Spec: infrav1.VSphereDeploymentZoneSpec{
					Server:              simr.ServerURL().Host,
					FailureDomain:       "blah",
					ControlPlane:        pointer.Bool(true),
					PlacementConstraint: tt.placementConstraint,
				}},
				VSphereFailureDomain: &infrav1.VSphereFailureDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name: "blah",
					},
					Spec: infrav1.VSphereFailureDomainSpec{
						Topology: infrav1.Topology{
							Datacenter:     "DC0",
							ComputeCluster: pointer.String("DC0_C0"),
						},
					},
				},
				Logger: logr.DiscardLogger{},
			}

			reconciler := vsphereDeploymentZoneReconciler{controllerCtx}
			_, err = reconciler.reconcileNormal(deploymentZoneCtx)
			g.Expect(err).To(HaveOccurred())
		})
	}
}
