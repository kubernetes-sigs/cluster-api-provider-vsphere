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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/vmware/govmomi/simulator"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vcsim"
)

var _ = Describe("VSphereDeploymentZoneReconciler", func() {
	var (
		simr *vcsim.Simulator

		failureDomainKey, deploymentZoneKey client.ObjectKey

		vsphereDeploymentZone *infrav1.VSphereDeploymentZone
		vsphereFailureDomain  *infrav1.VSphereFailureDomain
	)

	BeforeEach(func() {
		model := simulator.VPX()
		model.Pool = 1

		var err error
		simr, err = vcsim.NewBuilder().
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

	})

	BeforeEach(func() {
		vsphereFailureDomain = &infrav1.VSphereFailureDomain{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "blah-fd-",
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

		vsphereDeploymentZone = &infrav1.VSphereDeploymentZone{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "blah-",
			},
			Spec: infrav1.VSphereDeploymentZoneSpec{
				Server:        simr.ServerURL().Host,
				FailureDomain: vsphereFailureDomain.Name,
				ControlPlane:  pointer.Bool(true),
				PlacementConstraint: infrav1.PlacementConstraint{
					ResourcePool: "DC0_C0_RP1",
					Folder:       "/",
				},
			}}
		Expect(testEnv.Create(ctx, vsphereDeploymentZone)).To(Succeed())
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, vsphereDeploymentZone, vsphereFailureDomain)).To(Succeed())
		simr.Destroy()
	})

	It("should create a deployment zone & failure domain", func() {
		deploymentZoneKey = client.ObjectKey{Name: vsphereDeploymentZone.Name}
		failureDomainKey = client.ObjectKey{Name: vsphereFailureDomain.Name}

		Eventually(func() bool {
			if err := testEnv.Get(ctx, deploymentZoneKey, vsphereDeploymentZone); err != nil {
				return false
			}
			return len(vsphereDeploymentZone.Finalizers) > 0
		}, timeout).Should(BeTrue())

		Eventually(func() bool {
			if err := testEnv.Get(ctx, deploymentZoneKey, vsphereDeploymentZone); err != nil {
				return false
			}
			return conditions.IsTrue(vsphereDeploymentZone, infrav1.VCenterAvailableCondition) &&
				conditions.IsTrue(vsphereDeploymentZone, infrav1.PlacementConstraintMetCondition) &&
				conditions.IsTrue(vsphereDeploymentZone, infrav1.VSphereFailureDomainValidatedCondition)
		}, timeout).Should(BeTrue())

		Expect(testEnv.Get(ctx, failureDomainKey, vsphereFailureDomain)).To(Succeed())
		ownerRefs := vsphereFailureDomain.GetOwnerReferences()
		Expect(ownerRefs).To(HaveLen(1))
		Expect(ownerRefs[0].Name).To(Equal(deploymentZoneKey.Name))
		Expect(ownerRefs[0].Kind).To(Equal("VSphereDeploymentZone"))
	})

	Context("With incorrect details: when resource pool is not owned by compute cluster", func() {
		It("should fail creation of deployment zone", func() {
			vsphereFailureDomain = &infrav1.VSphereFailureDomain{
				TypeMeta: metav1.TypeMeta{
					Kind:       "VSphereFailureDomain",
					APIVersion: infrav1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "blah-fd-",
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

			vsphereDeploymentZone = &infrav1.VSphereDeploymentZone{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "blah-",
				},
				Spec: infrav1.VSphereDeploymentZoneSpec{
					Server:        simr.ServerURL().Host,
					FailureDomain: vsphereFailureDomain.Name,
					ControlPlane:  pointer.Bool(true),
					PlacementConstraint: infrav1.PlacementConstraint{
						ResourcePool: "DC0_C1_RP1",
						Folder:       "/",
					},
				}}
			Expect(testEnv.Create(ctx, vsphereDeploymentZone)).To(Succeed())

			deploymentZoneKey = client.ObjectKey{Name: vsphereDeploymentZone.Name}
			failureDomainKey = client.ObjectKey{Name: vsphereFailureDomain.Name}

			Eventually(func() bool {
				if err := testEnv.Get(ctx, deploymentZoneKey, vsphereDeploymentZone); err != nil {
					return false
				}
				return conditions.IsFalse(vsphereDeploymentZone, infrav1.PlacementConstraintMetCondition)
			}, timeout).Should(BeTrue())
		})
	})

	Context("Delete VSphereDeploymentZone", func() {

		BeforeEach(func() {
			deploymentZoneKey = client.ObjectKey{Name: vsphereDeploymentZone.Name}
			failureDomainKey = client.ObjectKey{Name: vsphereFailureDomain.Name}

			Eventually(func() bool {
				deploymentZoneWithFinalizers := &infrav1.VSphereDeploymentZone{}
				if err := testEnv.Get(ctx, deploymentZoneKey, deploymentZoneWithFinalizers); err != nil {
					return false
				}
				return len(deploymentZoneWithFinalizers.Finalizers) > 0
			}, timeout).Should(BeTrue())
		})

		It("should delete the associated failure domain", func() {
			Expect(testEnv.Delete(ctx, vsphereDeploymentZone)).To(Succeed())

			Eventually(func() bool {
				fd := &infrav1.VSphereFailureDomain{}
				err := testEnv.Get(ctx, failureDomainKey, fd)
				return apierrors.IsNotFound(err)
			}, timeout).Should(BeTrue())
		})

		Context("With machines being present", func() {
			var machineNamespace *corev1.Namespace

			BeforeEach(func() {
				var err error
				machineNamespace, err = testEnv.CreateNamespace(ctx, "multi-az-test")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				_ = testEnv.Cleanup(ctx, machineNamespace)
			})

			Context("when machines are using Deployment Zone", func() {
				It("should block deletion", func() {
					machineUsingDeplZone := createMachine("machine-using-zone", "cluster-using-zone", machineNamespace.Name, false)
					machineUsingDeplZone.Spec.FailureDomain = pointer.String(vsphereDeploymentZone.Name)
					Expect(testEnv.Create(ctx, machineUsingDeplZone)).To(Succeed())

					Expect(testEnv.Delete(ctx, vsphereDeploymentZone)).To(Succeed())

					Eventually(func() bool {
						if err := testEnv.Get(ctx, deploymentZoneKey, vsphereDeploymentZone); err != nil {
							return false
						}
						return !vsphereDeploymentZone.DeletionTimestamp.IsZero() &&
							len(vsphereDeploymentZone.Finalizers) > 0
					}, timeout).Should(BeTrue())
				})

				It("should not block deletion if machines are being deleted", func() {
					machineBeingDeleted := createMachine("machine-deleted", "cluster-deleted", machineNamespace.Name, false)
					machineBeingDeleted.Spec.FailureDomain = pointer.String(vsphereDeploymentZone.Name)
					machineBeingDeleted.Finalizers = []string{clusterv1.MachineFinalizer}
					Expect(testEnv.Create(ctx, machineBeingDeleted)).To(Succeed())

					Expect(testEnv.Delete(ctx, machineBeingDeleted)).To(Succeed())

					Expect(testEnv.Delete(ctx, vsphereDeploymentZone)).To(Succeed())

					Eventually(func() bool {
						return apierrors.IsNotFound(testEnv.Get(ctx, deploymentZoneKey, vsphereDeploymentZone))
					}, timeout).Should(BeTrue())
				})
			})

			It("should not block deletion if machines are not using Deployment Zone", func() {
				machineNotUsingDeplZone := createMachine("machine-without-zone", "cluster-without-zone", machineNamespace.Name, true)
				Expect(testEnv.Create(ctx, machineNotUsingDeplZone)).To(Succeed())

				Expect(testEnv.Delete(ctx, vsphereDeploymentZone)).To(Succeed())

				Eventually(func() bool {
					err := testEnv.Get(ctx, deploymentZoneKey, &infrav1.VSphereDeploymentZone{})
					return apierrors.IsNotFound(err)
				}, timeout).Should(BeTrue())
			})
		})
	})
})

func TestVSphereDeploymentZone_Reconcile(t *testing.T) {
	g := NewWithT(t)
	model := simulator.VPX()
	model.Pool = 1

	simr, err := vcsim.NewBuilder().
		WithModel(model).
		WithOperations().
		Build()
	g.Expect(err).NotTo(HaveOccurred())
	defer func() {
		simr.Destroy()
	}()

	operations := []string{
		"tags.category.create -t Datacenter,ClusterComputeResource k8s-region",
		"tags.category.create -t Datacenter,ClusterComputeResource k8s-zone",
		"tags.create -c k8s-region k8s-region-west",
		"tags.create -c k8s-zone k8s-zone-west-1",
		"tags.attach -c k8s-region k8s-region-west /DC0",
		"tags.attach -c k8s-zone k8s-zone-west-1 /DC0/host/DC0_C0",
	}
	for _, op := range operations {
		g.Expect(simr.Run(op, gbytes.NewBuffer(), gbytes.NewBuffer())).To(Succeed())
	}

	t.Run("should create a deployment zone & failure domain", func(t *testing.T) {
		g := NewWithT(t)

		vsphereFailureDomain := &infrav1.VSphereFailureDomain{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "blah-fd-",
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
		g.Expect(testEnv.Create(ctx, vsphereFailureDomain)).To(Succeed())

		vsphereDeploymentZone := &infrav1.VSphereDeploymentZone{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "blah-",
			},
			Spec: infrav1.VSphereDeploymentZoneSpec{
				Server:        simr.ServerURL().Host,
				FailureDomain: vsphereFailureDomain.Name,
				ControlPlane:  pointer.Bool(true),
				PlacementConstraint: infrav1.PlacementConstraint{
					ResourcePool: "DC0_C0_RP1",
					Folder:       "/",
				},
			}}
		g.Expect(testEnv.Create(ctx, vsphereDeploymentZone)).To(Succeed())

		defer func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(vsphereDeploymentZone, vsphereFailureDomain)

		g.Eventually(func() bool {
			if err := testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereDeploymentZone), vsphereDeploymentZone); err != nil {
				return false
			}
			return len(vsphereDeploymentZone.Finalizers) > 0
		}, timeout).Should(BeTrue())

		g.Eventually(func() bool {
			if err := testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereDeploymentZone), vsphereDeploymentZone); err != nil {
				return false
			}
			return conditions.IsTrue(vsphereDeploymentZone, infrav1.VCenterAvailableCondition) &&
				conditions.IsTrue(vsphereDeploymentZone, infrav1.PlacementConstraintMetCondition) &&
				conditions.IsTrue(vsphereDeploymentZone, infrav1.VSphereFailureDomainValidatedCondition)
		}, timeout).Should(BeTrue())

		g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereFailureDomain), vsphereFailureDomain)).To(Succeed())
		ownerRefs := vsphereFailureDomain.GetOwnerReferences()
		g.Expect(ownerRefs).To(HaveLen(1))
		g.Expect(ownerRefs[0].Name).To(Equal(vsphereDeploymentZone.Name))
		g.Expect(ownerRefs[0].Kind).To(Equal("VSphereDeploymentZone"))
	})

	t.Run("it should delete associated failure domain", func(t *testing.T) {
		g := NewWithT(t)

		vsphereFailureDomain := &infrav1.VSphereFailureDomain{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "blah-fd-",
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
		g.Expect(testEnv.Create(ctx, vsphereFailureDomain)).To(Succeed())

		vsphereDeploymentZone := &infrav1.VSphereDeploymentZone{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "blah-",
			},
			Spec: infrav1.VSphereDeploymentZoneSpec{
				Server:        simr.ServerURL().Host,
				FailureDomain: vsphereFailureDomain.Name,
				ControlPlane:  pointer.Bool(true),
				PlacementConstraint: infrav1.PlacementConstraint{
					ResourcePool: "DC0_C0_RP1",
					Folder:       "/",
				},
			}}
		g.Expect(testEnv.Create(ctx, vsphereDeploymentZone)).To(Succeed())

		defer func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(vsphereDeploymentZone, vsphereFailureDomain)

		g.Eventually(func() bool {
			if err := testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereDeploymentZone), vsphereDeploymentZone); err != nil {
				return false
			}
			return len(vsphereDeploymentZone.Finalizers) > 0
		}, timeout).Should(BeTrue())

		g.Expect(testEnv.Delete(ctx, vsphereDeploymentZone)).To(Succeed())

		g.Eventually(func() bool {
			failureDomainErr := testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereFailureDomain), vsphereFailureDomain)
			deploymentZoneErr := testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereDeploymentZone), vsphereDeploymentZone)
			return apierrors.IsNotFound(failureDomainErr) && apierrors.IsNotFound(deploymentZoneErr)
		}, timeout).Should(BeTrue())
	})

	t.Run("VSphereDeploymentZone should never become ready if VSphereFailureDomain does not exist", func(t *testing.T) {
		g := NewWithT(t)

		vsphereDeploymentZone := &infrav1.VSphereDeploymentZone{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "blah-",
			},
			Spec: infrav1.VSphereDeploymentZoneSpec{
				Server:        simr.ServerURL().Host,
				FailureDomain: "fd1",
				ControlPlane:  pointer.Bool(true),
				PlacementConstraint: infrav1.PlacementConstraint{
					ResourcePool: "DC0_C0_RP1",
					Folder:       "/",
				},
			}}
		g.Expect(testEnv.Create(ctx, vsphereDeploymentZone)).To(Succeed())

		defer func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(vsphereDeploymentZone)

		g.Eventually(func() bool {
			if err := testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereDeploymentZone), vsphereDeploymentZone); err != nil {
				return false
			}
			return len(vsphereDeploymentZone.Finalizers) > 0
		}, timeout).Should(BeTrue())

		g.Consistently(func(g Gomega) bool {
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereDeploymentZone), vsphereDeploymentZone)).To(Succeed())
			return vsphereDeploymentZone.Status.Ready != nil && !*vsphereDeploymentZone.Status.Ready
		}, timeout).Should(BeFalse())
	})

	t.Run("Delete the VSphereDeploymentZone when the associated VSphereFailureDomain does not exist", func(t *testing.T) {
		g := NewWithT(t)

		vsphereDeploymentZone := &infrav1.VSphereDeploymentZone{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "blah-",
			},
			Spec: infrav1.VSphereDeploymentZoneSpec{
				Server:        simr.ServerURL().Host,
				FailureDomain: "fd1",
				ControlPlane:  pointer.Bool(true),
				PlacementConstraint: infrav1.PlacementConstraint{
					ResourcePool: "DC0_C0_RP1",
					Folder:       "/",
				},
			}}
		g.Expect(testEnv.Create(ctx, vsphereDeploymentZone)).To(Succeed())

		defer func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(vsphereDeploymentZone)

		g.Eventually(func() bool {
			if err := testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereDeploymentZone), vsphereDeploymentZone); err != nil {
				return false
			}
			return len(vsphereDeploymentZone.Finalizers) > 0
		}, timeout).Should(BeTrue())

		g.Expect(testEnv.Delete(ctx, vsphereDeploymentZone)).To(Succeed())

		g.Eventually(func() bool {
			deploymentZoneErr := testEnv.Get(ctx, client.ObjectKeyFromObject(vsphereDeploymentZone), vsphereDeploymentZone)
			return apierrors.IsNotFound(deploymentZoneErr)
		}, timeout).Should(BeTrue())
	})
}

func createMachine(machineName, clusterName, namespace string, isControlPlane bool) *clusterv1.Machine {
	m := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: clusterv1.MachineSpec{
			Version: pointer.String("v1.22.0"),
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					APIVersion: bootstrapv1.GroupVersion.String(),
					Name:       machineName,
				},
			},
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "VSphereMachine",
				Name:       machineName,
			},
			ClusterName: clusterName,
		},
	}
	if isControlPlane {
		m.Labels[clusterv1.MachineControlPlaneLabel] = ""
	}
	return m
}

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

			simr, err := vcsim.NewBuilder().
				WithModel(model).
				Build()
			if err != nil {
				t.Fatalf("unable to create simulator %s", err)
			}
			defer simr.Destroy()

			controllerManagerContext := fake.NewControllerManagerContext()
			controllerManagerContext.Username = simr.ServerURL().User.Username()
			pass, _ := simr.ServerURL().User.Password()
			controllerManagerContext.Password = pass

			Expect(controllerManagerContext.Client.Create(ctx, &infrav1.VSphereFailureDomain{
				ObjectMeta: metav1.ObjectMeta{
					Name: "blah",
				},
				Spec: infrav1.VSphereFailureDomainSpec{
					Topology: infrav1.Topology{
						Datacenter:     "DC0",
						ComputeCluster: pointer.String("DC0_C0"),
					},
				},
			})).To(Succeed())

			deploymentZoneCtx := &capvcontext.VSphereDeploymentZoneContext{
				ControllerManagerContext: controllerManagerContext,
				VSphereDeploymentZone: &infrav1.VSphereDeploymentZone{Spec: infrav1.VSphereDeploymentZoneSpec{
					Server:              simr.ServerURL().Host,
					FailureDomain:       "blah",
					ControlPlane:        pointer.Bool(true),
					PlacementConstraint: tt.placementConstraint,
				}},
			}

			reconciler := vsphereDeploymentZoneReconciler{controllerManagerContext}
			err = reconciler.reconcileNormal(ctx, deploymentZoneCtx)
			g.Expect(err).To(HaveOccurred())
		})
	}
}

func TestVSphereDeploymentZoneReconciler_ReconcileDelete(t *testing.T) {
	vsphereDeploymentZone := &infrav1.VSphereDeploymentZone{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VSphereDeploymentZone",
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "blah",
			Finalizers: []string{infrav1.DeploymentZoneFinalizer},
		},
		Spec: infrav1.VSphereDeploymentZoneSpec{
			FailureDomain: "blah-fd",
		},
	}

	vsphereFailureDomain := &infrav1.VSphereFailureDomain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "blah-fd",
		},
		Spec: infrav1.VSphereFailureDomainSpec{
			Topology: infrav1.Topology{
				Datacenter:     "DC0",
				ComputeCluster: pointer.String("DC0_C0"),
			},
		},
	}

	t.Run("when machines are using deployment zone", func(t *testing.T) {
		machineUsingDeplZone := createMachine("machine-1", "cluster-1", "ns", false)
		machineUsingDeplZone.Spec.FailureDomain = pointer.String("blah")

		t.Run("should block deletion", func(t *testing.T) {
			controllerManagerContext := fake.NewControllerManagerContext(machineUsingDeplZone, vsphereFailureDomain)
			deploymentZoneCtx := &capvcontext.VSphereDeploymentZoneContext{
				ControllerManagerContext: controllerManagerContext,
				VSphereDeploymentZone:    vsphereDeploymentZone,
			}

			g := NewWithT(t)
			reconciler := vsphereDeploymentZoneReconciler{controllerManagerContext}
			err := reconciler.reconcileDelete(ctx, deploymentZoneCtx)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(MatchRegexp(".*[is currently in use]{1}.*"))
			g.Expect(vsphereDeploymentZone.Finalizers).To(HaveLen(1))
		})

		t.Run("for machines being deleted, should not block deletion", func(t *testing.T) {
			deletionTime := metav1.Now()
			machineUsingDeplZone.DeletionTimestamp = &deletionTime
			machineUsingDeplZone.Finalizers = append(machineUsingDeplZone.Finalizers, "keep-this-for-the-test")

			controllerManagerContext := fake.NewControllerManagerContext(machineUsingDeplZone, vsphereFailureDomain)
			deploymentZoneCtx := &capvcontext.VSphereDeploymentZoneContext{
				ControllerManagerContext: controllerManagerContext,
				VSphereDeploymentZone:    vsphereDeploymentZone,
			}

			g := NewWithT(t)
			reconciler := vsphereDeploymentZoneReconciler{controllerManagerContext}
			err := reconciler.reconcileDelete(ctx, deploymentZoneCtx)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vsphereDeploymentZone.Finalizers).To(BeEmpty())
		})
	})

	t.Run("when machines are not using deployment zone", func(t *testing.T) {
		machineNotUsingDeplZone := createMachine("machine-1", "cluster-1", "ns", false)
		controllerManagerContext := fake.NewControllerManagerContext(machineNotUsingDeplZone, vsphereFailureDomain)
		deploymentZoneCtx := &capvcontext.VSphereDeploymentZoneContext{
			ControllerManagerContext: controllerManagerContext,
			VSphereDeploymentZone:    vsphereDeploymentZone,
		}

		g := NewWithT(t)
		reconciler := vsphereDeploymentZoneReconciler{controllerManagerContext}
		err := reconciler.reconcileDelete(ctx, deploymentZoneCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(vsphereDeploymentZone.Finalizers).To(BeEmpty())
	})

	t.Run("when no machines are present", func(t *testing.T) {
		controllerManagerContext := fake.NewControllerManagerContext(vsphereFailureDomain)
		deploymentZoneCtx := &capvcontext.VSphereDeploymentZoneContext{
			ControllerManagerContext: controllerManagerContext,
			VSphereDeploymentZone:    vsphereDeploymentZone,
		}

		g := NewWithT(t)
		reconciler := vsphereDeploymentZoneReconciler{controllerManagerContext}
		err := reconciler.reconcileDelete(ctx, deploymentZoneCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(vsphereDeploymentZone.Finalizers).To(BeEmpty())
	})

	t.Run("delete failure domain", func(t *testing.T) {
		vsphereFailureDomain.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       vsphereDeploymentZone.Kind,
			Name:       vsphereDeploymentZone.Name,
		}}

		t.Run("when not used by other deployment zones", func(t *testing.T) {
			controllerManagerContext := fake.NewControllerManagerContext(vsphereFailureDomain)
			deploymentZoneCtx := &capvcontext.VSphereDeploymentZoneContext{
				ControllerManagerContext: controllerManagerContext,
				VSphereDeploymentZone:    vsphereDeploymentZone,
			}

			g := NewWithT(t)
			reconciler := vsphereDeploymentZoneReconciler{controllerManagerContext}
			err := reconciler.reconcileDelete(ctx, deploymentZoneCtx)
			g.Expect(err).NotTo(HaveOccurred())
		})

		t.Run("when used by other deployment zones", func(t *testing.T) {
			vsphereFailureDomain.OwnerReferences = append(vsphereFailureDomain.OwnerReferences, metav1.OwnerReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       vsphereDeploymentZone.Kind,
				Name:       "another-deployment-zone",
			})

			controllerManagerContext := fake.NewControllerManagerContext(vsphereFailureDomain)
			deploymentZoneCtx := &capvcontext.VSphereDeploymentZoneContext{
				ControllerManagerContext: controllerManagerContext,
				VSphereDeploymentZone:    vsphereDeploymentZone,
			}

			g := NewWithT(t)
			reconciler := vsphereDeploymentZoneReconciler{controllerManagerContext}
			err := reconciler.reconcileDelete(ctx, deploymentZoneCtx)
			g.Expect(err).NotTo(HaveOccurred())

			fetchedFailureDomain := &infrav1.VSphereFailureDomain{}
			g.Expect(controllerManagerContext.Client.Get(ctx, client.ObjectKey{Name: vsphereFailureDomain.Name}, fetchedFailureDomain)).To(Succeed())
			g.Expect(fetchedFailureDomain.OwnerReferences).To(HaveLen(1))
		})
	})
}
