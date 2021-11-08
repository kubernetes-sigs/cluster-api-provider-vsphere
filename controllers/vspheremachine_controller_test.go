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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

var _ = Describe("VsphereMachineReconciler", func() {
	var (
		capiCluster *clusterv1.Cluster
		capiMachine *clusterv1.Machine

		infraCluster *infrav1.VSphereCluster
		infraMachine *infrav1.VSphereMachine

		testNs *corev1.Namespace
		key    client.ObjectKey
	)

	isPresentAndFalseWithReason := func(getter conditions.Getter, condition clusterv1.ConditionType, reason string) bool {
		ExpectWithOffset(1, testEnv.Get(ctx, key, getter)).To(Succeed())
		if !conditions.Has(getter, condition) {
			return false
		}
		objectCondition := conditions.Get(getter, condition)
		return objectCondition.Status == corev1.ConditionFalse &&
			objectCondition.Reason == reason
	}

	BeforeEach(func() {
		var err error
		testNs, err = testEnv.CreateNamespace(ctx, "vsphere-machine-reconciler")
		Expect(err).NotTo(HaveOccurred())

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test1-",
				Namespace:    testNs.Name,
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
					Kind:       "VSphereCluster",
					Name:       "vsphere-test1",
				},
			},
		}
		Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())

		infraCluster = &infrav1.VSphereCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphere-test1",
				Namespace: testNs.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "cluster.x-k8s.io/v1alpha4",
						Kind:       "Cluster",
						Name:       capiCluster.Name,
						UID:        "blah",
					},
				},
			},
			Spec: infrav1.VSphereClusterSpec{},
		}
		Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())

		capiMachine = &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "machine-created-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.MachineFinalizer},
				Labels: map[string]string{
					clusterv1.ClusterLabelName: capiCluster.Name,
				},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: capiCluster.Name,
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
					Kind:       "VSphereMachine",
					Name:       "vsphere-machine-1",
				},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		infraMachine = &infrav1.VSphereMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphere-machine-1",
				Namespace: testNs.Name,
				Labels: map[string]string{
					clusterv1.ClusterLabelName:             capiCluster.Name,
					clusterv1.MachineControlPlaneLabelName: "",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       capiMachine.Name,
						UID:        "blah",
					},
				},
			},
			Spec: infrav1.VSphereMachineSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Template: "ubuntu-k9s-1.19",
					Network: infrav1.NetworkSpec{
						Devices: []infrav1.NetworkDeviceSpec{
							{NetworkName: "network-1", DHCP4: true},
						},
					},
				},
			},
		}
		Expect(testEnv.Create(ctx, infraMachine)).To(Succeed())

		key = client.ObjectKey{Namespace: testNs.Name, Name: infraMachine.Name}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, testNs, capiCluster, infraCluster, capiMachine, infraMachine)).To(Succeed())
	})

	It("waits for cluster status to be ready", func() {
		Eventually(func() bool {
			// this is to make sure that the VSphereMachine is created before the next check for the
			// presence of conditions on the VSphereMachine proceeds.
			if err := testEnv.Get(ctx, key, infraMachine); err != nil {
				return false
			}
			return isPresentAndFalseWithReason(infraMachine, infrav1.VMProvisionedCondition, infrav1.WaitingForClusterInfrastructureReason)
		}, timeout).Should(BeTrue())

		By("setting the cluster infrastructure to be ready")
		Eventually(func() error {
			ph, err := patch.NewHelper(capiCluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())
			capiCluster.Status.InfrastructureReady = true
			return ph.Patch(ctx, capiCluster, patch.WithStatusObservedGeneration{})
		}, timeout).Should(BeNil())

		Eventually(func() bool {
			return isPresentAndFalseWithReason(infraMachine, infrav1.VMProvisionedCondition, infrav1.WaitingForClusterInfrastructureReason)
		}, timeout).Should(BeFalse())
	})

	Context("With Cluster Infrastructure status ready", func() {
		BeforeEach(func() {
			ph, err := patch.NewHelper(capiCluster, testEnv)
			Expect(err).ShouldNot(HaveOccurred())
			capiCluster.Status.InfrastructureReady = true
			Expect(ph.Patch(ctx, capiCluster, patch.WithStatusObservedGeneration{})).To(Succeed())
		})

		It("moves to VSphere VM creation", func() {
			Eventually(func() bool {
				vms := infrav1.VSphereVMList{}
				Expect(testEnv.List(ctx, &vms, client.InNamespace(testNs.Name), client.MatchingLabels{
					clusterv1.ClusterLabelName: capiCluster.Name,
				})).To(Succeed())
				return isPresentAndFalseWithReason(infraMachine, infrav1.VMProvisionedCondition, infrav1.WaitingForBootstrapDataReason) &&
					len(vms.Items) == 0
			}, timeout).Should(BeTrue())

			By("setting the bootstrap data")
			Eventually(func() error {
				ph, err := patch.NewHelper(capiMachine, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{
					DataSecretName: pointer.String("some-secret"),
				}
				return ph.Patch(ctx, capiMachine, patch.WithStatusObservedGeneration{})
			}, timeout).Should(BeNil())

			Eventually(func() int {
				vms := infrav1.VSphereVMList{}
				Expect(testEnv.List(ctx, &vms)).To(Succeed())
				return len(vms.Items)
			}, timeout).Should(BeNumerically(">", 0))
		})
	})
})
