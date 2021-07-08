package controllers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/util/conditions"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"

	//capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
)

var _ = Describe("VsphereMachineReconciler", func() {

	var (
		capiCluster *clusterv1.Cluster
		capiMachine *clusterv1.Machine

		infraCluster *infrav1.VSphereCluster
		infraMachine *infrav1.VSphereMachine
		//vm           *infrav1.VSphereVM

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

		/*vm = &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNs.Name,
				Name:      capiMachine.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: infrav1.GroupVersion.String(),
						// Kind:       infraMachine.Kind,
						Kind: "VSphereMachine",
						Name: infraMachine.Name,
						UID:  "blah",
					},
				},
			},
			Spec: infrav1.VSphereVMSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Template: "ubuntu-k9s-1.19",
					Network: infrav1.NetworkSpec{
						Devices: []infrav1.NetworkDeviceSpec{
							{NetworkName: "network-1", DHCP4: true},
						},
					},
					Server: testEnv.Simulator.ServerURL().Host,
				},
			},
		}
		Expect(testEnv.Create(ctx, vm)).To(Succeed())*/

		key = client.ObjectKey{Namespace: testNs.Name, Name: infraMachine.Name}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, capiCluster, testNs)).To(Succeed())
	})

	// TODO(srm09): Move this to a different test, since requires a more complicated setup
	/*Context("In case of errors on the VSphereVM", func() {
		It("should surface the errors to the Machine", func() {
			By("setting the failure message and reason on the VM")
			Eventually(func() error {
				ph, err := patch.NewHelper(vm, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				vm.Status.FailureReason = capierrors.MachineStatusErrorPtr(capierrors.UpdateMachineError)
				vm.Status.FailureMessage = pointer.StringPtr("some failure here")
				return ph.Patch(ctx, vm, patch.WithStatusObservedGeneration{})
			}, timeout).Should(BeNil())

			Eventually(func() bool {
				if err := testEnv.Get(ctx, key, infraMachine); err != nil {
					return false
				}
				return infraMachine.Status.FailureReason != nil
			}, timeout).Should(BeTrue())
		})
	})*/

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
				Expect(testEnv.List(ctx, &vms)).To(Succeed())
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

	/*Context("waits for bootstrap secret to be available", func() {
		BeforeEach(func() {
			capiCluster.Status.InfrastructureReady = true
			Expect(testEnv.Update(ctx, capiCluster)).To(Succeed())
			capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{
				DataSecretName: pointer.String("some-secret"),
			}
			Expect(testEnv.Update(ctx, capiMachine)).To(Succeed())
		})

		It("updates the condition reason to WaitingForBootstrapData", func() {
			key := client.ObjectKey{Namespace: testNs.Name, Name: infraMachine.Name}
			Expect(testEnv.Get(ctx, key, infraMachine)).To(Succeed())
			Expect(conditions.Has(infraMachine, infrav1.VMProvisionedCondition)).To(BeTrue())
			vmProvisionedCondition := conditions.Get(infraMachine, infrav1.VMProvisionedCondition)
			Expect(vmProvisionedCondition.Status).To(Equal(corev1.ConditionFalse))
			Expect(vmProvisionedCondition.Reason).To(Equal(infrav1.WaitingForBootstrapDataReason))

			By("setting the cluster status to be ready")
			Eventually(func() error {
				ph, err := patch.NewHelper(capiMachine, testEnv)
				Expect(err).ShouldNot(HaveOccurred())
				capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{
					DataSecretName: pointer.String("some-secret"),
				}
				return ph.Patch(ctx, capiMachine, patch.WithStatusObservedGeneration{})
			}, timeout).Should(BeNil())

			Eventually(func() bool {
				key := client.ObjectKey{Namespace: testNs.Name, Name: infraMachine.Name}
				Expect(testEnv.Get(ctx, key, infraMachine)).To(Succeed())
				Expect(conditions.Has(infraMachine, infrav1.VMProvisionedCondition)).To(BeTrue())
				return conditions.Get(infraMachine, infrav1.VMProvisionedCondition).Reason != infrav1.WaitingForBootstrapDataReason
			}, timeout).Should(BeTrue())
		})
	})*/
})
