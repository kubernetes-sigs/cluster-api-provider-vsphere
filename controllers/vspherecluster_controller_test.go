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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/simulator"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers"
)

const (
	timeout = time.Second * 30
)

var _ = Describe("ClusterReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an VSphereCluster", func() {
		It("should create a cluster", func() {
			ctx := context.Background()

			fakeVCenter := startVcenter()
			vcURL := fakeVCenter.ServerURL()
			defer fakeVCenter.Destroy()

			// Create the secret containing the credentials
			password, _ := vcURL.User.Password()
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    "default",
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
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
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
							APIVersion: "api-version",
							Kind:       "cluster",
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
				return Expect(*actual).Should(conditions.MatchCondition(clusterv1.Condition{
					Type:     infrav1.VCenterAvailableCondition,
					Status:   corev1.ConditionFalse,
					Severity: clusterv1.ConditionSeverityError,
					Reason:   infrav1.VCenterUnreachableReason,
				}))
			}, timeout).Should(BeTrue())
		})

		It("should remove vspherecluster finalizer if the secret does not exist", func() {
			ctx := context.Background()

			// Create the VSphereCluster object
			instance := &infrav1.VSphereCluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "vsphere-test1",
					Namespace:    "default",
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
			//Expect(testEnv.Get(ctx, key, instance)).To(Succeed())

			capiCluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test1-",
					Namespace:    "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
						Kind:       "VsphereCluster",
						Name:       "vsphere-test1",
					},
				},
			}
			// Create the CAPI cluster (owner) object
			Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

			//key := client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}

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

	})

	Context("With Deployment Zones", func() {
		var (
			ctx         context.Context
			namespace   *corev1.Namespace
			capiCluster *clusterv1.Cluster
			instance    *infrav1.VSphereCluster
			zoneOne     *infrav1.VSphereDeploymentZone
		)

		BeforeEach(func() {
			var err error
			namespace, err = testEnv.CreateNamespace(ctx, "dz-test")
			Expect(err).NotTo(HaveOccurred())

			ctx = context.Background()
		})

		AfterEach(func() {
			Expect(testEnv.Cleanup(ctx, namespace, capiCluster, instance, zoneOne)).To(Succeed())
		})

		It("should reconcile a cluster", func() {
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
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("zone-one")},
				Spec: infrav1.VSphereDeploymentZoneSpec{
					Server:        testEnv.Simulator.ServerURL().Host,
					FailureDomain: "fd-one",
					ControlPlane:  pointer.BoolPtr(true),
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
					Server: testEnv.Simulator.ServerURL().Host,
				},
			}
			Expect(testEnv.Create(ctx, instance)).To(Succeed())

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
				zoneOne.Status.Ready = pointer.BoolPtr(true)
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
	})
})

func TestClusterReconciler_ReconcileDeploymentZones(t *testing.T) {
	server := "vcenter123.foo.com"
	g := NewWithT(t)

	t.Run("with no deployment zones", func(t *testing.T) {
		controllerCtx := fake.NewControllerContext(fake.NewControllerManagerContext())
		ctx := fake.NewClusterContext(controllerCtx)
		ctx.VSphereCluster = &infrav1.VSphereCluster{
			Spec: infrav1.VSphereClusterSpec{Server: server},
		}

		r := clusterReconciler{controllerCtx}
		reconciled, err := r.reconcileDeploymentZones(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(reconciled).To(BeTrue())
	})

	tests := []struct {
		name       string
		initObjs   []runtime.Object
		reconciled bool
		assert     func(*infrav1.VSphereCluster)
	}{
		{
			name: "with deployment zone status not reported",
			initObjs: []runtime.Object{
				deploymentZone(server, "zone-1", pointer.BoolPtr(false), nil),
				deploymentZone(server, "zone-2", pointer.BoolPtr(true), pointer.BoolPtr(false)),
			},
			assert: func(zone *infrav1.VSphereCluster) {
				g.Expect(conditions.IsFalse(zone, infrav1.FailureDomainsAvailableCondition)).To(BeTrue())
				g.Expect(conditions.Get(zone, infrav1.FailureDomainsAvailableCondition).Reason).To(Equal(infrav1.WaitingForFailureDomainStatusReason))
			},
		},
		{
			name:       "with some deployment zones statuses as not ready",
			reconciled: true,
			initObjs: []runtime.Object{
				deploymentZone(server, "zone-1", pointer.BoolPtr(false), pointer.BoolPtr(false)),
				deploymentZone(server, "zone-2", pointer.BoolPtr(true), pointer.BoolPtr(true)),
			},
			assert: func(zone *infrav1.VSphereCluster) {
				g.Expect(conditions.IsFalse(zone, infrav1.FailureDomainsAvailableCondition)).To(BeTrue())
				g.Expect(conditions.Get(zone, infrav1.FailureDomainsAvailableCondition).Reason).To(Equal(infrav1.FailureDomainsSkippedReason))
			},
		},
		{
			name:       "with all deployment zone statuses as ready",
			reconciled: true,
			initObjs: []runtime.Object{
				deploymentZone(server, "zone-1", pointer.BoolPtr(false), pointer.BoolPtr(true)),
				deploymentZone(server, "zone-2", pointer.BoolPtr(true), pointer.BoolPtr(true)),
			},
			assert: func(zone *infrav1.VSphereCluster) {
				g.Expect(conditions.IsTrue(zone, infrav1.FailureDomainsAvailableCondition)).To(BeTrue())
			},
		},
	}

	// nolint:scopelint
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			controllerCtx := fake.NewControllerContext(fake.NewControllerManagerContext(tt.initObjs...))
			ctx := fake.NewClusterContext(controllerCtx)
			ctx.VSphereCluster.Spec.Server = server

			r := clusterReconciler{controllerCtx}
			reconciled, err := r.reconcileDeploymentZones(ctx)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(reconciled).To(Equal(tt.reconciled))
			tt.assert(ctx.VSphereCluster)
		})
	}
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

func startVcenter() *helpers.Simulator {
	model := simulator.VPX()
	model.Pool = 1

	simr, err := helpers.VCSimBuilder().
		WithModel(model).
		Build()
	if err != nil {
		panic(fmt.Sprintf("unable to create simulator %s", err))
	}

	return simr
}
