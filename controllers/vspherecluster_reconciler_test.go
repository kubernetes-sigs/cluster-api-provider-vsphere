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
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/simulator"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vcsim"
)

const (
	timeout = time.Second * 30
)

var _ = Describe("VIM based VSphere ClusterReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an VSphereCluster", func() {
		It("should create a cluster", func() {
			fakeVCenter := startVcenter()
			vcURL := fakeVCenter.ServerURL()
			defer fakeVCenter.Destroy()

			// Create the secret containing the credentials
			password, _ := vcURL.User.Password()
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "bitnami.com/v1alpha1",
							Kind:       "SealedSecret",
							Name:       "some-name",
							UID:        "some-uid",
						},
					},
				},
				Data: map[string][]byte{
					identity.UsernameKey: []byte(vcURL.User.Username()),
					identity.PasswordKey: []byte(password),
				},
			}
			Expect(testEnv.Create(ctx, secret)).To(Succeed())

			// Create the VSphereCluster object
			instance := &infrav1.VSphereCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vsphere-test1",
					Namespace:    "default",
				},
				Spec: infrav1.VSphereClusterSpec{
					IdentityRef: &infrav1.VSphereIdentityReference{
						Kind: infrav1.SecretKind,
						Name: secret.Name,
					},
					Server: fmt.Sprintf("%s://%s", vcURL.Scheme, vcURL.Host),
				},
			}
			Expect(testEnv.Create(ctx, instance)).To(Succeed())
			key := client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}
			defer func() {
				Expect(testEnv.Delete(ctx, instance)).To(Succeed())
			}()

			capiCluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test1-",
					Namespace:    "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "VsphereCluster",
						Name:       instance.Name,
					},
				},
			}
			// Create the CAPI cluster (owner) object
			Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())
			defer func() {
				Expect(testEnv.Cleanup(ctx, capiCluster)).To(Succeed())
			}()

			// Make sure the VSphereCluster exists.
			Eventually(func() error {
				return testEnv.Get(ctx, key, instance)
			}, timeout).Should(BeNil())

			By("setting the OwnerRef on the VSphereCluster")
			Eventually(func() error {
				ph, err := patch.NewHelper(instance, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				instance.OwnerReferences = append(instance.OwnerReferences, metav1.OwnerReference{
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
					Name:       capiCluster.Name,
					UID:        "blah",
				})
				return ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})
			}, timeout).Should(BeNil())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, instance); err != nil {
					return false
				}
				return len(instance.Finalizers) > 0
			}, timeout).Should(BeTrue())

			// checking cluster is setting the ownerRef on the secret
			secretKey := client.ObjectKey{Namespace: secret.Namespace, Name: secret.Name}
			Eventually(func() bool {
				if err := testEnv.Get(ctx, secretKey, secret); err != nil {
					return false
				}
				return len(secret.OwnerReferences) > 0
			}, timeout).Should(BeTrue())

			By("setting the VSphereCluster's VCenterAvailableCondition to true")
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, instance); err != nil {
					return false
				}
				return conditions.IsTrue(instance, infrav1.VCenterAvailableCondition)
			}, timeout).Should(BeTrue())
		})

		It("should error if secret is already owned by a different cluster", func() {
			ctx := context.Background()
			capiCluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test1-",
					Namespace:    "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "VsphereCluster",
						Name:       "vsphere-test1",
					},
				},
			}
			// Create the CAPI cluster (owner) object
			Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

			// Create the secret containing the credentials
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1.GroupVersion.String(),
							Kind:       "VSphereClusterIdentity",
							Name:       "another-cluster",
							UID:        "some-uid",
						},
					},
				},
			}
			Expect(testEnv.Create(ctx, secret)).To(Succeed())

			// Create the VSphereCluster object
			instance := &infrav1.VSphereCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vsphere-cluster",
					Namespace:    "default",
				},
				Spec: infrav1.VSphereClusterSpec{
					IdentityRef: &infrav1.VSphereIdentityReference{
						Kind: infrav1.SecretKind,
						Name: secret.Name,
					},
				},
			}

			Expect(testEnv.Create(ctx, instance)).To(Succeed())
			key := client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}
			defer func() {
				err := testEnv.Delete(ctx, instance)
				Expect(err).NotTo(HaveOccurred())
			}()
			By("setting the OwnerRef on the VSphereCluster")
			Eventually(func() bool {
				ph, err := patch.NewHelper(instance, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				instance.OwnerReferences = append(instance.OwnerReferences, metav1.OwnerReference{Kind: "Cluster", APIVersion: clusterv1.GroupVersion.String(), Name: capiCluster.Name, UID: "blah"})
				Expect(ph.Patch(ctx, instance, patch.WithStatusObservedGeneration{})).ShouldNot(HaveOccurred())
				return true
			}, timeout).Should(BeTrue())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, instance); err != nil {
					return false
				}

				actual := conditions.Get(instance, infrav1.VCenterAvailableCondition)
				if actual == nil {
					return false
				}
				actual.Message = ""
				return Expect(actual).Should(conditions.HaveSameStateOf(&clusterv1.Condition{
					Type:     infrav1.VCenterAvailableCondition,
					Status:   corev1.ConditionFalse,
					Severity: clusterv1.ConditionSeverityError,
					Reason:   infrav1.VCenterUnreachableReason,
				}))
			}, timeout).Should(BeTrue())
		})
	})

	It("should remove vspherecluster finalizer if the secret does not exist", func() {
		ctx := context.Background()
		capiCluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    "default",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "VsphereCluster",
					Name:       "vsphere-test1",
				},
			},
		}
		// Create the CAPI cluster (owner) object
		Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

		// Create the VSphereCluster object
		instance := &infrav1.VSphereCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphere-test1",
				Namespace: "default",
			},
			Spec: infrav1.VSphereClusterSpec{
				IdentityRef: &infrav1.VSphereIdentityReference{
					Kind: infrav1.SecretKind,
					Name: "foo",
				},
			},
		}

		Expect(testEnv.Create(ctx, instance)).To(Succeed())
		key := client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}

		// Make sure the VSphereCluster exists.
		Eventually(func() bool {
			err := testEnv.Get(ctx, key, instance)
			return err == nil
		}, timeout).Should(BeTrue())

		By("deleting the vspherecluster while the secret is gone")
		Eventually(func() bool {
			err := testEnv.Delete(ctx, instance)
			return err == nil
		}, timeout).Should(BeTrue())

		Eventually(func() bool {
			err := testEnv.Get(ctx, key, instance)
			return apierrors.IsNotFound(err)
		}, timeout).Should(BeTrue())
	})

	Context("With Deployment Zones", func() {
		var (
			namespace   *corev1.Namespace
			capiCluster *clusterv1.Cluster
			instance    *infrav1.VSphereCluster
			zoneOne     *infrav1.VSphereDeploymentZone
		)

		BeforeEach(func() {
			var err error
			namespace, err = testEnv.CreateNamespace(ctx, "dz-test")
			Expect(err).NotTo(HaveOccurred())

			capiCluster = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test1-",
					Namespace:    namespace.Name,
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "VSphereCluster",
						Name:       "vsphere-test2",
					},
				},
			}
			// Create the CAPI cluster (owner) object
			Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())
			Expect(testEnv.CreateKubeconfigSecret(ctx, capiCluster)).To(Succeed())

			By("Create the VSphere Deployment Zone")
			zoneOne = &infrav1.VSphereDeploymentZone{
				ObjectMeta: metav1.ObjectMeta{Name: "zone-one"},
				Spec: infrav1.VSphereDeploymentZoneSpec{
					Server:        testEnv.Simulator.ServerURL().Host,
					FailureDomain: "fd-one",
					ControlPlane:  pointer.Bool(true),
				},
				Status: infrav1.VSphereDeploymentZoneStatus{},
			}
			Expect(testEnv.Create(ctx, zoneOne)).To(Succeed())

			By("Create the VSphere Cluster")
			instance = &infrav1.VSphereCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vsphere-test2",
					Namespace: namespace.Name,
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       "Cluster",
						APIVersion: clusterv1.GroupVersion.String(),
						Name:       capiCluster.Name,
						UID:        "blah",
					}},
				},
				Spec: infrav1.VSphereClusterSpec{
					FailureDomainSelector: &metav1.LabelSelector{MatchLabels: map[string]string{}},
					Server:                testEnv.Simulator.ServerURL().Host,
				},
			}
			Expect(testEnv.Create(ctx, instance)).To(Succeed())
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, namespace, capiCluster, instance, zoneOne)).To(Succeed())
		})

		It("should reconcile a cluster", func() {
			key := client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}
			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, instance); err != nil {
					return false
				}
				return conditions.Has(instance, infrav1.FailureDomainsAvailableCondition) &&
					conditions.IsFalse(instance, infrav1.FailureDomainsAvailableCondition) &&
					conditions.Get(instance, infrav1.FailureDomainsAvailableCondition).Reason == infrav1.WaitingForFailureDomainStatusReason
			}, timeout).Should(BeTrue())

			By("Setting the status of the Deployment Zone to true")
			Eventually(func() error {
				ph, err := patch.NewHelper(zoneOne, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				zoneOne.Status.Ready = pointer.Bool(true)
				return ph.Patch(ctx, zoneOne, patch.WithStatusObservedGeneration{})
			}, timeout).Should(BeNil())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, instance); err != nil {
					return false
				}
				return conditions.Has(instance, infrav1.FailureDomainsAvailableCondition) &&
					conditions.IsTrue(instance, infrav1.FailureDomainsAvailableCondition)
			}, timeout).Should(BeTrue())
		})

		Context("when deployment zones are deleted", func() {
			BeforeEach(func() {
				By("Setting the status of the Deployment Zone to true")
				Eventually(func() error {
					ph, err := patch.NewHelper(zoneOne, testEnv)
					Expect(err).ShouldNot(HaveOccurred())
					zoneOne.Status.Ready = pointer.Bool(true)
					return ph.Patch(ctx, zoneOne, patch.WithStatusObservedGeneration{})
				}, timeout).Should(BeNil())
			})

			It("should remove the FailureDomainsAvailable condition from the cluster", func() {
				key := client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}
				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, instance); err != nil {
						return false
					}
					return conditions.Has(instance, infrav1.FailureDomainsAvailableCondition) &&
						conditions.IsTrue(instance, infrav1.FailureDomainsAvailableCondition)
				}, timeout).Should(BeTrue())

				By("Deleting the Deployment Zone", func() {
					Expect(testEnv.Delete(ctx, zoneOne)).To(Succeed())
				})

				Eventually(func() bool {
					if err := testEnv.Get(ctx, key, instance); err != nil {
						return false
					}
					return conditions.Has(instance, infrav1.FailureDomainsAvailableCondition)
				}, timeout).Should(BeFalse())
			})
		})
	})
})

func TestClusterReconciler_ReconcileDeploymentZones(t *testing.T) {
	server := "vcenter123.foo.com"

	t.Run("with nil selectors", func(t *testing.T) {
		g := NewWithT(t)
		tests := []struct {
			name       string
			initObjs   []client.Object
			reconciled bool
			assert     func(*infrav1.VSphereCluster)
		}{
			{
				name:       "with no deployment zones",
				reconciled: true,
				assert: func(vsphereCluster *infrav1.VSphereCluster) {
					g.Expect(conditions.Has(vsphereCluster, infrav1.FailureDomainsAvailableCondition)).To(BeFalse())
				},
			},
			{
				name:       "with all deployment zone statuses as ready",
				reconciled: true,
				initObjs: []client.Object{
					deploymentZone(server, "zone-1", pointer.Bool(false), pointer.Bool(true)),
					deploymentZone(server, "zone-2", pointer.Bool(true), pointer.Bool(true)),
				},
				assert: func(vsphereCluster *infrav1.VSphereCluster) {
					g.Expect(conditions.Has(vsphereCluster, infrav1.FailureDomainsAvailableCondition)).To(BeFalse())
				},
			},
		}

		for _, tt := range tests {
			// Looks odd, but need to reinit test variable
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				g := NewWithT(t)
				controllerManagerContext := fake.NewControllerManagerContext(tt.initObjs...)
				clusterCtx := fake.NewClusterContext(ctx, controllerManagerContext)
				clusterCtx.VSphereCluster.Spec.Server = server

				r := clusterReconciler{
					ControllerManagerContext: controllerManagerContext,
					Client:                   controllerManagerContext.Client,
				}
				reconciled, err := r.reconcileDeploymentZones(ctx, clusterCtx)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(reconciled).To(Equal(tt.reconciled))
				tt.assert(clusterCtx.VSphereCluster)
			})
		}
	})

	t.Run("with empty selectors", func(t *testing.T) {
		g := NewWithT(t)
		tests := []struct {
			name       string
			initObjs   []client.Object
			reconciled bool
			assert     func(*infrav1.VSphereCluster)
		}{
			{
				name:       "with no deployment zones",
				reconciled: true,
				assert: func(vsphereCluster *infrav1.VSphereCluster) {
					g.Expect(conditions.Has(vsphereCluster, infrav1.FailureDomainsAvailableCondition)).To(BeFalse())
				},
			},
			{
				name: "with deployment zone status not reported",
				initObjs: []client.Object{
					deploymentZone(server, "zone-1", pointer.Bool(false), nil),
					deploymentZone(server, "zone-2", pointer.Bool(true), pointer.Bool(false)),
				},
				assert: func(vsphereCluster *infrav1.VSphereCluster) {
					g.Expect(conditions.IsFalse(vsphereCluster, infrav1.FailureDomainsAvailableCondition)).To(BeTrue())
					g.Expect(conditions.Get(vsphereCluster, infrav1.FailureDomainsAvailableCondition).Reason).To(Equal(infrav1.WaitingForFailureDomainStatusReason))
				},
			},
			{
				name:       "with some deployment zones statuses as not ready",
				reconciled: true,
				initObjs: []client.Object{
					deploymentZone(server, "zone-1", pointer.Bool(false), pointer.Bool(false)),
					deploymentZone(server, "zone-2", pointer.Bool(true), pointer.Bool(true)),
				},
				assert: func(vsphereCluster *infrav1.VSphereCluster) {
					g.Expect(conditions.IsFalse(vsphereCluster, infrav1.FailureDomainsAvailableCondition)).To(BeTrue())
					g.Expect(conditions.Get(vsphereCluster, infrav1.FailureDomainsAvailableCondition).Reason).To(Equal(infrav1.FailureDomainsSkippedReason))
				},
			},
			{
				name:       "with all deployment zone statuses as ready",
				reconciled: true,
				initObjs: []client.Object{
					deploymentZone(server, "zone-1", pointer.Bool(false), pointer.Bool(true)),
					deploymentZone(server, "zone-2", pointer.Bool(true), pointer.Bool(true)),
				},
				assert: func(vsphereCluster *infrav1.VSphereCluster) {
					g.Expect(conditions.IsTrue(vsphereCluster, infrav1.FailureDomainsAvailableCondition)).To(BeTrue())
				},
			},
		}

		for _, tt := range tests {
			// Looks odd, but need to reinit test variable
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				g := NewWithT(t)
				controllerManagerContext := fake.NewControllerManagerContext(tt.initObjs...)
				clusterCtx := fake.NewClusterContext(ctx, controllerManagerContext)
				clusterCtx.VSphereCluster.Spec.Server = server
				clusterCtx.VSphereCluster.Spec.FailureDomainSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}

				r := clusterReconciler{
					ControllerManagerContext: controllerManagerContext,
					Client:                   controllerManagerContext.Client,
				}
				reconciled, err := r.reconcileDeploymentZones(ctx, clusterCtx)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(reconciled).To(Equal(tt.reconciled))
				tt.assert(clusterCtx.VSphereCluster)
			})
		}
	})

	t.Run("with zone selectors", func(t *testing.T) {
		g := NewWithT(t)

		zoneOne := deploymentZone(server, "zone-1", pointer.Bool(false), pointer.Bool(true))
		zoneOne.Labels = map[string]string{
			"zone":       "rack-one",
			"datacenter": "ohio",
		}
		zoneTwo := deploymentZone(server, "zone-2", pointer.Bool(false), pointer.Bool(true))
		zoneTwo.Labels = map[string]string{
			"zone":       "rack-two",
			"datacenter": "ohio",
		}
		zoneThree := deploymentZone(server, "zone-3", pointer.Bool(false), pointer.Bool(true))
		zoneThree.Labels = map[string]string{
			"datacenter": "oregon",
		}

		assertNumberOfZones := func(selector *metav1.LabelSelector, selectedZones int) {
			controllerManagerContext := fake.NewControllerManagerContext(zoneOne, zoneTwo, zoneThree)
			clusterCtx := fake.NewClusterContext(ctx, controllerManagerContext)
			clusterCtx.VSphereCluster.Spec.Server = server
			clusterCtx.VSphereCluster.Spec.FailureDomainSelector = selector

			r := clusterReconciler{
				ControllerManagerContext: controllerManagerContext,
				Client:                   controllerManagerContext.Client,
			}
			_, err := r.reconcileDeploymentZones(ctx, clusterCtx)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(clusterCtx.VSphereCluster.Status.FailureDomains).To(HaveLen(selectedZones))
		}

		t.Run("with no zones matching labels", func(_ *testing.T) {
			assertNumberOfZones(&metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}, 0)
		})

		t.Run("with all zones matching some labels", func(_ *testing.T) {
			assertNumberOfZones(&metav1.LabelSelector{MatchLabels: map[string]string{"datacenter": "ohio"}}, 2)
		})

		t.Run("with selector and all matching labels", func(_ *testing.T) {
			assertNumberOfZones(&metav1.LabelSelector{MatchLabels: map[string]string{
				"zone":       "rack-two",
				"datacenter": "ohio",
			}}, 1)
		})

		t.Run("with no selector", func(_ *testing.T) {
			assertNumberOfZones(nil, 0)
		})

		t.Run("with selector and a negation label matcher", func(_ *testing.T) {
			assertNumberOfZones(&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "datacenter",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"ohio"},
					},
				},
			}, 1)
		})

		t.Run("with selector and a key-only label matcher", func(_ *testing.T) {
			assertNumberOfZones(&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "zone",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			}, 2)
		})

		t.Run("with selector and a multi value label matcher", func(_ *testing.T) {
			assertNumberOfZones(&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "datacenter",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"ohio", "oregon"},
					},
				},
			}, 3)
		})
	})
}

func deploymentZone(server, fdName string, cp, ready *bool) *infrav1.VSphereDeploymentZone {
	return &infrav1.VSphereDeploymentZone{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("zone-%s", fdName)},
		Spec: infrav1.VSphereDeploymentZoneSpec{
			Server:        server,
			FailureDomain: fdName,
			ControlPlane:  cp,
		},
		Status: infrav1.VSphereDeploymentZoneStatus{Ready: ready},
	}
}

func startVcenter() *vcsim.Simulator {
	model := simulator.VPX()
	model.Pool = 1

	simr, err := vcsim.NewBuilder().WithModel(model).Build()
	if err != nil {
		panic(fmt.Sprintf("unable to create simulator %s", err))
	}

	return simr
}
