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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/govmomi/v1beta2"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	conversionapi "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api"
	conversionclient "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/client"
	inframanager "sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/vmoperator"
	infrautil "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
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

	isPresentAndFalseWithReason := func(object client.Object, condition string, reason string) bool {
		ExpectWithOffset(1, testEnv.Get(ctx, key, object)).To(Succeed())

		getter, ok := object.(conditions.Getter)
		Expect(ok).To(BeTrue())

		if !conditions.Has(getter, condition) {
			return false
		}
		objectCondition := conditions.Get(getter, condition)
		return objectCondition.Status == metav1.ConditionFalse &&
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
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "VSphereCluster",
					Name:     "vsphere-test1",
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
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       capiCluster.Name,
						UID:        "blah",
					},
				},
			},
			Spec: infrav1.VSphereClusterSpec{
				Server: "test-server",
			},
		}
		Expect(testEnv.Create(ctx, infraCluster)).To(Succeed())

		capiMachine = &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "machine-created-",
				Namespace:    testNs.Name,
				Finalizers:   []string{clusterv1.MachineFinalizer},
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: capiCluster.Name,
				},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: capiCluster.Name,
				Bootstrap: clusterv1.Bootstrap{
					ConfigRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: "bootstrap.cluster.x-k8s.io",
						Kind:     "BootstrapConfig",
						Name:     "does-no-ext", // Does not have to exist for these tests.
					},
				},
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "VSphereMachine",
					Name:     "vsphere-machine-1",
				},
			},
		}
		Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		infraMachine = &infrav1.VSphereMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphere-machine-1",
				Namespace: testNs.Name,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel:         capiCluster.Name,
					clusterv1.MachineControlPlaneLabel: "",
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
							{NetworkName: "network-1", DHCP4: ptr.To(true)},
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
			return isPresentAndFalseWithReason(infraMachine, infrav1.VSphereVMVirtualMachineProvisionedCondition, infrav1.VSphereMachineVirtualMachineWaitingForClusterInfrastructureReadyReason)
		}, timeout).Should(BeTrue())

		By("setting the cluster infrastructure to be ready")
		Eventually(func() error {
			ph, err := patch.NewHelper(capiCluster, testEnv)
			Expect(err).ToNot(HaveOccurred())
			capiCluster.Status.Initialization.InfrastructureProvisioned = ptr.To(true)
			return ph.Patch(ctx, capiCluster, patch.WithStatusObservedGeneration{})
		}, timeout).Should(Succeed())

		Eventually(func() bool {
			return isPresentAndFalseWithReason(infraMachine, infrav1.VSphereVMVirtualMachineProvisionedCondition, infrav1.VSphereMachineVirtualMachineWaitingForClusterInfrastructureReadyReason)
		}, timeout).Should(BeFalse())

		By("reporting Supervisor VM infrastructure maintenance on the Ready condition")
		featuregatetesting.SetFeatureGateDuringTest(GinkgoTB(), feature.Gates, feature.HostMaintenanceMode, true)
		supervisorCluster := &vmwarev1.VSphereCluster{ObjectMeta: metav1.ObjectMeta{Name: infraCluster.Name, Namespace: testNs.Name}}
		supervisorMachine := &vmwarev1.VSphereMachine{ObjectMeta: metav1.ObjectMeta{
			Name: infraMachine.Name, Namespace: testNs.Name, Labels: infraMachine.Labels,
			Finalizers: []string{infrav1.MachineFinalizer}, OwnerReferences: infraMachine.OwnerReferences,
		}}
		supervisorMachine.Spec.Naming.Template = "{{ .machine.name }}-vm"
		conditions.Set(supervisorMachine, metav1.Condition{Type: clusterv1.PausedCondition, Status: metav1.ConditionFalse, Reason: clusterv1.NotPausedReason})
		owner := capiMachine.DeepCopy()
		owner.Spec.Version = "test-version"
		owner.Spec.Bootstrap.DataSecretName = ptr.To("bootstrap-data")
		owner.Labels[clusterv1.MachineControlPlaneLabel] = ""
		vm := &vmoprv1alpha5.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{Name: owner.Name + "-vm", Namespace: testNs.Name},
			Spec:       vmoprv1alpha5.VirtualMachineSpec{PowerState: vmoprv1alpha5.VirtualMachinePowerStateOff},
			Status: vmoprv1alpha5.VirtualMachineStatus{PowerState: vmoprv1alpha5.VirtualMachinePowerStateOff, Conditions: []metav1.Condition{
				{Type: vmoprv1alpha5.VirtualMachineConditionCreated, Status: metav1.ConditionTrue, Reason: "Created"},
				{Type: vmoprv1alpha5.VirtualMachinePowerStateSynced, Status: metav1.ConditionFalse, Reason: "InfraInMaintenance"},
			}},
		}
		// The existing fixture uses govmomi types; use the same CAPI objects with a fake Supervisor client.
		_, managerContext := infrautil.CreateClusterContext(capiCluster, supervisorCluster)
		vmReads := 0
		baseClient := fake.NewClientBuilder().WithScheme(managerContext.Scheme).
			WithStatusSubresource(supervisorMachine, vm).
			WithObjects(capiCluster.DeepCopy(), owner, supervisorCluster, supervisorMachine, vm).
			WithInterceptorFuncs(interceptor.Funcs{Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*vmoprv1alpha5.VirtualMachine); ok {
					vmReads++
				}
				return c.Get(ctx, key, obj, opts...)
			}}).Build()
		supervisorClient, err := conversionclient.NewWithConverter(baseClient, conversionapi.DefaultConverterFor(vmoprv1alpha5.GroupVersion))
		Expect(err).NotTo(HaveOccurred())
		factory, err := inframanager.NewStaticNetworkProviderFactory(ctx, supervisorClient, inframanager.DummyLBNetworkProvider)
		Expect(err).NotTo(HaveOccurred())
		reconciler := &machineReconciler{Client: supervisorClient, supervisorBased: true, networkProviderFactory: factory, VMService: &vmoperator.VmopMachineService{Client: supervisorClient}}
		supervisorKey := client.ObjectKeyFromObject(supervisorMachine)
		reconcileAndRefresh := func() {
			_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: supervisorKey})
			Expect(err).NotTo(HaveOccurred())
			Expect(supervisorClient.Get(ctx, supervisorKey, supervisorMachine)).To(Succeed())
		}
		reconcileAndRefresh()
		Expect(conditions.IsFalse(supervisorMachine, infrav1.VSphereMachineReadyCondition)).To(BeTrue())
		Expect(conditions.GetReason(supervisorMachine, infrav1.VSphereMachineReadyCondition)).To(Equal(vmwarev1.VSphereMachineHostMaintenanceReason))
		Expect(conditions.GetReason(supervisorMachine, infrav1.VSphereMachineVirtualMachineProvisionedCondition)).To(Equal(infrav1.VSphereMachineVirtualMachinePoweringOnReason))
		Expect(supervisorClient.Get(ctx, client.ObjectKeyFromObject(vm), vm)).To(Succeed())
		Expect(vm.Spec.PowerState).To(Equal(vmoprv1alpha5.VirtualMachinePowerStateOff))

		By("keeping Ready false while a provisioned VM is in maintenance")
		vm.Status.PowerState = vmoprv1alpha5.VirtualMachinePowerStateOn
		vm.Status.Network = &vmoprv1alpha5.VirtualMachineNetworkStatus{PrimaryIP4: "192.0.2.1"}
		vm.Status.BiosUUID = "test-bios-uuid"
		vm.Status.NodeName = "test-host"
		Expect(baseClient.Status().Update(ctx, vm)).To(Succeed())
		reconcileAndRefresh()
		Expect(conditions.IsFalse(supervisorMachine, infrav1.VSphereMachineReadyCondition)).To(BeTrue())
		Expect(conditions.GetReason(supervisorMachine, infrav1.VSphereMachineReadyCondition)).To(Equal(vmwarev1.VSphereMachineHostMaintenanceReason))
		Expect(conditions.IsTrue(supervisorMachine, infrav1.VSphereMachineVirtualMachineProvisionedCondition)).To(BeTrue())

		By("using the normal Ready summary when VM reconciliation is skipped")
		Expect(baseClient.Get(ctx, client.ObjectKeyFromObject(owner), owner)).To(Succeed())
		owner.Spec.Bootstrap.DataSecretName = nil
		Expect(baseClient.Update(ctx, owner)).To(Succeed())
		vmReads = 0
		reconcileAndRefresh()
		Expect(vmReads).To(Equal(0))
		Expect(conditions.GetReason(supervisorMachine, infrav1.VSphereMachineReadyCondition)).To(Equal(infrav1.VSphereMachineNotReadyReason))
		Expect(conditions.GetReason(supervisorMachine, infrav1.VSphereMachineVirtualMachineProvisionedCondition)).To(Equal(infrav1.VSphereMachineVirtualMachineWaitingForBootstrapDataReason))

		Expect(baseClient.Get(ctx, client.ObjectKeyFromObject(owner), owner)).To(Succeed())
		owner.Spec.Bootstrap.DataSecretName = ptr.To("bootstrap-data")
		Expect(baseClient.Update(ctx, owner)).To(Succeed())

		By("restoring normal Ready reconciliation after maintenance")
		Expect(supervisorClient.Get(ctx, client.ObjectKeyFromObject(vm), vm)).To(Succeed())
		meta.SetStatusCondition(&vm.Status.Conditions, metav1.Condition{
			Type: vmoprv1alpha5.VirtualMachinePowerStateSynced, Status: metav1.ConditionTrue, Reason: "Synced",
		})
		Expect(baseClient.Status().Update(ctx, vm)).To(Succeed())
		reconcileAndRefresh()
		Expect(conditions.IsTrue(supervisorMachine, infrav1.VSphereMachineReadyCondition)).To(BeTrue())
		Expect(conditions.GetReason(supervisorMachine, infrav1.VSphereMachineReadyCondition)).To(Equal(infrav1.VSphereMachineReadyReason))
		Expect(supervisorClient.Get(ctx, client.ObjectKeyFromObject(vm), vm)).To(Succeed())
		Expect(vm.Spec.PowerState).To(Equal(vmoprv1alpha5.VirtualMachinePowerStateOn))

	})

	Context("With Cluster Infrastructure status ready", func() {
		BeforeEach(func() {
			ph, err := patch.NewHelper(capiCluster, testEnv)
			Expect(err).ToNot(HaveOccurred())
			capiCluster.Status.Initialization.InfrastructureProvisioned = ptr.To(true)
			Expect(ph.Patch(ctx, capiCluster, patch.WithStatusObservedGeneration{})).To(Succeed())
		})

		It("moves to VSphere VM creation", func() {
			Eventually(func() bool {
				vms := infrav1.VSphereVMList{}
				Expect(testEnv.List(ctx, &vms, client.InNamespace(testNs.Name), client.MatchingLabels{
					clusterv1.ClusterNameLabel: capiCluster.Name,
				})).To(Succeed())
				return isPresentAndFalseWithReason(infraMachine, infrav1.VSphereMachineVirtualMachineProvisionedCondition, infrav1.VSphereMachineVirtualMachineWaitingForBootstrapDataReason) &&
					len(vms.Items) == 0
			}, timeout).Should(BeTrue())

			By("setting the bootstrap data")
			Eventually(func() error {
				ph, err := patch.NewHelper(capiMachine, testEnv)
				Expect(err).ToNot(HaveOccurred())
				capiMachine.Spec.Bootstrap = clusterv1.Bootstrap{
					DataSecretName: ptr.To("some-secret"),
				}
				return ph.Patch(ctx, capiMachine, patch.WithStatusObservedGeneration{})
			}, timeout).Should(Succeed())

			Eventually(func() int {
				vms := infrav1.VSphereVMList{}
				Expect(testEnv.List(ctx, &vms)).To(Succeed())
				return len(vms.Items)
			}, timeout).Should(BeNumerically(">", 0))
		})
	})
})

func Test_machineReconciler_Metadata(t *testing.T) {
	g := NewWithT(t)
	ns, err := testEnv.CreateNamespace(ctx, "vsphere-machine-reconciler")
	g.Expect(err).NotTo(HaveOccurred())

	defer func() {
		if err := testEnv.Delete(ctx, ns); err != nil {
			g.Expect(err).NotTo(HaveOccurred())
		}
	}()
	capiCluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: ns.Name,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: "infrastructure.cluster.x-k8s.io",
				Kind:     "VSphereCluster",
				Name:     "vsphere-test1",
			},
		},
	}

	vSphereCluster := &infrav1.VSphereCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VSphereCluster",
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vsphere-test1",
			Namespace: ns.Name,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       capiCluster.Name,
					UID:        "blah",
				},
			},
		},
		Spec: infrav1.VSphereClusterSpec{
			Server: "test-server",
		},
	}

	capiMachine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: clusterv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "machine1",
			Namespace:  ns.Name,
			Finalizers: []string{clusterv1.MachineFinalizer},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: capiCluster.Name,
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: capiCluster.Name,
			Bootstrap:   clusterv1.Bootstrap{DataSecretName: ptr.To("data")},
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: "infrastructure.cluster.x-k8s.io",
				Kind:     "VSphereMachine",
				Name:     "vsphere-machine-1",
			},
		},
	}
	vSphereMachine := &infrav1.VSphereMachine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VSphereMachine",
			APIVersion: infrav1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vsphere-machine-1",
			Namespace: ns.Name,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:         capiCluster.Name,
				clusterv1.MachineControlPlaneLabel: "",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       capiMachine.Name,
					UID:        "blah",
				},
				// This ownerReference should be removed by the reconciler as it's no longer needed.
				// These ownerReferences were previously added by CAPV to prevent machines becoming orphaned.
				{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "VSphereCluster",
					Name:       vSphereCluster.Name,
					UID:        "blah",
				},
			},
		},
		Spec: infrav1.VSphereMachineSpec{
			VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
				Template: "ubuntu-k9s-1.19",
				Network: infrav1.NetworkSpec{
					Devices: []infrav1.NetworkDeviceSpec{
						{NetworkName: "network-1", DHCP4: ptr.To(true)},
					},
				},
			},
		},
	}

	t.Run("Should set finalizer and remove unnecessary ownerReference", func(t *testing.T) {
		g := NewWithT(t)

		// Create the Machine, Cluster, VSphereCluster object and expect the Reconcile and Deployment to be created
		g.Expect(testEnv.Create(ctx, vSphereCluster)).To(Succeed())
		g.Expect(testEnv.Create(ctx, capiCluster)).To(Succeed())
		g.Expect(testEnv.Create(ctx, capiMachine)).To(Succeed())

		g.Expect(testEnv.Create(ctx, vSphereMachine)).To(Succeed())

		key := client.ObjectKey{Namespace: vSphereMachine.Namespace, Name: vSphereMachine.Name}
		defer func() {
			err := testEnv.Delete(ctx, vSphereMachine)
			g.Expect(err).ToNot(HaveOccurred())
		}()

		// Make sure the VSphereMachine has a finalizer and the correct ownerReferences.
		g.Eventually(func() bool {
			if err := testEnv.Get(ctx, key, vSphereMachine); err != nil {
				return false
			}
			return ctrlutil.ContainsFinalizer(vSphereMachine, infrav1.MachineFinalizer) &&
				capiutil.HasOwner(vSphereMachine.GetOwnerReferences(), clusterv1.GroupVersion.String(), []string{"Machine"}) &&
				!capiutil.HasOwner(vSphereMachine.GetOwnerReferences(), infrav1.GroupVersion.String(), []string{"VSphereCluster"})
		}, timeout).Should(BeTrue())
	})

	t.Run("Should complete deletion even without Machine owner", func(t *testing.T) {
		g := NewWithT(t)

		vSphereMachine := &infrav1.VSphereMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vsphere-machine-no-ownerrefs",
				Namespace: ns.Name,
				// no ownerRefs
			},
			Spec: infrav1.VSphereMachineSpec{
				VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
					Template: "ubuntu-k9s-1.19",
					Network: infrav1.NetworkSpec{
						Devices: []infrav1.NetworkDeviceSpec{
							{NetworkName: "network-1", DHCP4: ptr.To(true)},
						},
					},
				},
			},
		}

		g.Expect(testEnv.Create(ctx, vSphereMachine)).To(Succeed())

		g.Expect(testEnv.Delete(ctx, vSphereMachine)).To(Succeed())

		// Make sure the VSphereMachine is gone.
		g.Eventually(func(g Gomega) {
			err := testEnv.Get(ctx, client.ObjectKeyFromObject(vSphereMachine), vSphereMachine)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout).Should(Succeed())
	})
}
