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

package vmoperator

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func getReconciledVM(ctx *vmware.SupervisorMachineContext) *vmoprv1.VirtualMachine {
	vm := &vmoprv1.VirtualMachine{}
	nsname := types.NamespacedName{
		Namespace: ctx.Machine.Namespace,
		Name:      ctx.Machine.Name,
	}
	err := ctx.Client.Get(ctx, nsname, vm)
	if apierrors.IsNotFound(err) {
		return nil
	}
	Expect(err).Should(BeNil())
	return vm
}

func updateReconciledVM(ctx *vmware.SupervisorMachineContext, vm *vmoprv1.VirtualMachine) {
	err := ctx.Client.Status().Update(ctx, vm)
	Expect(err).Should(BeNil())
}

var _ = Describe("VirtualMachine tests", func() {
	const (
		machineName              = "test-machine"
		clusterName              = "test-cluster"
		controlPlaneLabelTrue    = "true"
		k8sVersion               = "test-k8sVersion"
		className                = "test-className"
		imageName                = "test-imageName"
		storageClass             = "test-storageClass"
		vmIP                     = "127.0.0.1"
		biosUUID                 = "test-biosUuid"
		missingK8SVersionFailure = "missing kubernetes version"
	)
	var (
		bootstrapData = "test-bootstrap-data"

		err                  error
		requeue              bool
		expectedBiosUUID     string
		expectedImageName    string
		expectedVMIP         string
		expectReconcileError bool
		expectVMOpVM         bool
		expectedState        vmwarev1.VirtualMachineState
		expectedConditions   clusterv1.Conditions
		expectedRequeue      bool

		cluster        *clusterv1.Cluster
		vsphereCluster *vmwarev1.VSphereCluster
		machine        *clusterv1.Machine
		vsphereMachine *vmwarev1.VSphereMachine
		ctx            *vmware.SupervisorMachineContext

		// vm     vmwarev1.VirtualMachine
		vmopVM *vmoprv1.VirtualMachine

		vmService = VmopMachineService{}
	)

	BeforeEach(func() {
		// The default state of a VirtualMachine before a VM is successfully reconciled
		expectedBiosUUID = ""
		expectedVMIP = ""
		expectedState = vmwarev1.VirtualMachineStatePending
		expectedConditions = nil
		expectedRequeue = false

		// Create all necessary dependencies
		cluster = util.CreateCluster(clusterName)
		vsphereCluster = util.CreateVSphereCluster(clusterName)
		machine = util.CreateMachine(machineName, clusterName, controlPlaneLabelTrue, k8sVersion)
		vsphereMachine = util.CreateVSphereMachine(machineName, clusterName, controlPlaneLabelTrue, className, imageName, storageClass)
		clusterContext := util.CreateClusterContext(cluster, vsphereCluster)
		ctx = util.CreateMachineContext(clusterContext, machine, vsphereMachine)
		ctx.ControllerContext = clusterContext.ControllerContext
	})

	Context("Reconcile VirtualMachine", func() {
		verifyOutput := func(ctx *vmware.SupervisorMachineContext) {
			Expect(err != nil).Should(Equal(expectReconcileError))
			Expect(requeue).Should(Equal(expectedRequeue))
			vsphereMachine := ctx.VSphereMachine

			Expect(vsphereMachine).ShouldNot(BeNil())
			Expect(vsphereMachine.Name).Should(Equal(machineName))
			if expectedBiosUUID == "" {
				Expect(vsphereMachine.Status.ID).To(BeNil())
			} else {
				Expect(*vsphereMachine.Status.ID).Should(Equal(expectedBiosUUID))
			}
			Expect(vsphereMachine.Status.IPAddr).Should(Equal(expectedVMIP))
			Expect(vsphereMachine.Status.VMStatus).Should(Equal(expectedState))

			vmopVM = getReconciledVM(ctx)
			Expect(vmopVM != nil).Should(Equal(expectVMOpVM))

			if vmopVM != nil {
				vms, _ := getVirtualMachinesInCluster(ctx)
				Expect(vms).Should(HaveLen(1))
				Expect(vmopVM.Spec.ImageName).To(Equal(expectedImageName))
				Expect(vmopVM.Spec.ClassName).To(Equal(className))
				Expect(vmopVM.Spec.StorageClass).To(Equal(storageClass))
				Expect(vmopVM.Spec.PowerState).To(Equal(vmoprv1.VirtualMachinePoweredOn))
				Expect(vmopVM.ObjectMeta.Annotations[ClusterModuleNameAnnotationKey]).To(Equal(ControlPlaneVMClusterModuleGroupName))
				Expect(vmopVM.ObjectMeta.Annotations[ProviderTagsAnnotationKey]).To(Equal(ControlPlaneVMVMAntiAffinityTagValue))

				Expect(vmopVM.Labels[clusterSelectorKey]).To(Equal(clusterName))
				Expect(vmopVM.Labels[nodeSelectorKey]).To(Equal(roleControlPlane))
				// for backward compatibility, will be removed in the future
				Expect(vmopVM.Labels[legacyClusterSelectorKey]).To(Equal(clusterName))
				Expect(vmopVM.Labels[legacyNodeSelectorKey]).To(Equal(roleControlPlane))
			}

			for _, expectedCondition := range expectedConditions {
				c := conditions.Get(ctx.VSphereMachine, expectedCondition.Type)
				Expect(c).NotTo(BeNil())
				Expect(c.Status).To(Equal(expectedCondition.Status))
				Expect(c.Reason).To(Equal(expectedCondition.Reason))
				if expectedCondition.Message != "" {
					Expect(c.Message).To(ContainSubstring(expectedCondition.Message))
				} else {
					Expect(c.Message).To(BeEmpty())
				}
			}
		}

		Specify("Reconcile valid Machine", func() {
			// Reconcile should return an error up and until all prerequisites have been met
			expectReconcileError = false
			// A vmoperator VM should be created unless there is an error in configuration
			expectVMOpVM = true
			// We will mutate this later in the test
			expectedImageName = imageName
			// VM Operator will wait on the bootstrap resource, but as far as
			// CAPV is concerned, the VM has started provisioning.
			//
			// TODO(akutz) Ideally CAPV would check the VM Operator VM's
			//             conditions and assert the VM is waiting on the
			//             bootstrap data resource, but VM Operator is not
			//             running in this test domain, and so the condition
			//             will not be set on the VM Operator VM.
			expectedConditions = append(expectedConditions, clusterv1.Condition{
				Type:    infrav1.VMProvisionedCondition,
				Status:  corev1.ConditionFalse,
				Reason:  vmwarev1.VMProvisionStartedReason,
				Message: "",
			})
			expectedRequeue = true

			// Do the bare minimum that will cause a vmoperator VirtualMachine to be created
			// Note that the VM returned is not a vmoperator type, but is intentionally implementation agnostic
			By("VirtualMachine is created")
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			// Provide valid bootstrap data.
			By("bootstrap data is created")
			secretName := machine.GetName() + "-data"
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: machine.GetNamespace(),
				},
				Data: map[string][]byte{
					"value": []byte(bootstrapData),
				},
			}
			Expect(ctx.Client.Create(ctx, secret)).To(Succeed())

			machine.Spec.Bootstrap.DataSecretName = &secretName
			// we expect the reconciliation waiting for VM to be created
			expectedConditions[0].Reason = vmwarev1.VMProvisionStartedReason
			expectedConditions[0].Message = ""
			expectReconcileError = false
			expectedRequeue = true
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			// Simulate VMOperator creating a vSphere VM
			By("vSphere VM is created")
			vmopVM = getReconciledVM(ctx)
			vmopVM.Status.Phase = vmoprv1.Created
			updateReconciledVM(ctx, vmopVM)
			expectedState = vmwarev1.VirtualMachineStateCreated
			// we expect the reconciliation waiting for VM to be powered on
			expectedConditions[0].Reason = vmwarev1.PoweringOnReason
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			// Simulate VMOperator powering on the VM
			By("VirtualMachine is powered on")
			vmopVM = getReconciledVM(ctx)
			vmopVM.Status.PowerState = vmoprv1.VirtualMachinePoweredOn
			updateReconciledVM(ctx, vmopVM)
			expectedState = vmwarev1.VirtualMachineStatePoweredOn
			// we expect the reconciliation waiting for VM to have an IP
			expectedConditions[0].Reason = vmwarev1.WaitingForNetworkAddressReason
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			// Simulate VMOperator assigning an IP address
			By("VirtualMachine has an IP address")
			vmopVM = getReconciledVM(ctx)
			vmopVM.Status.VmIp = vmIP
			updateReconciledVM(ctx, vmopVM)
			// we expect the reconciliation waiting for VM to have a BIOS UUID
			expectedConditions[0].Reason = vmwarev1.WaitingForBIOSUUIDReason
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			// Simulate VMOperator assigning an Bios UUID
			By("VirtualMachine has Bios UUID")
			expectReconcileError = false
			expectedRequeue = false
			expectedBiosUUID = biosUUID
			expectedVMIP = vmIP
			expectedState = vmwarev1.VirtualMachineStateReady

			vmopVM = getReconciledVM(ctx)
			vmopVM.Status.BiosUUID = biosUUID
			updateReconciledVM(ctx, vmopVM)
			// we expect the reconciliation succeeds
			expectedConditions[0].Status = corev1.ConditionTrue
			expectedConditions[0].Reason = ""
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			Expect(vmopVM.Spec.ReadinessProbe).To(BeNil())

			// Provide a callback that should modify the ImageName
			By("With VM Modifier")
			modifiedImage := "modified-image"
			expectedImageName = modifiedImage
			ctx.VMModifiers = []vmware.VMModifier{
				func(obj runtime.Object) (runtime.Object, error) {
					// No need to check the type. We know this will be a VirtualMachine
					vm, _ := obj.(*vmoprv1.VirtualMachine)
					vm.Spec.ImageName = modifiedImage
					return vm, nil
				},
			}
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)
		})

		Specify("Reconcile will add a probe once the cluster reports that the control plane is ready", func() {
			// Reconcile should prompt to requeue until the prerequisites are met
			expectedRequeue = true
			expectReconcileError = false
			// A vmoperator VM should be created unless there is an error in configuration
			expectVMOpVM = true
			// We will mutate this later in the test
			expectedImageName = imageName

			// Provide valid bootstrap data.
			By("bootstrap data is created")
			secretName := machine.GetName() + "-data"
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: machine.GetNamespace(),
				},
				Data: map[string][]byte{
					"value": []byte(bootstrapData),
				},
			}
			Expect(ctx.Client.Create(ctx, secret)).To(Succeed())

			machine.Spec.Bootstrap.DataSecretName = &secretName
			expectedConditions = append(expectedConditions, clusterv1.Condition{
				Type:    infrav1.VMProvisionedCondition,
				Status:  corev1.ConditionFalse,
				Reason:  vmwarev1.VMProvisionStartedReason,
				Message: "",
			})
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			// Simulate VMOperator creating a vSphere VM
			By("vSphere VM is created")
			vmopVM = getReconciledVM(ctx)
			vmopVM.Status.Phase = vmoprv1.Created
			updateReconciledVM(ctx, vmopVM)
			expectedState = vmwarev1.VirtualMachineStateCreated
			expectedConditions[0].Reason = vmwarev1.PoweringOnReason
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			// Simulate VMOperator powering on the VM
			By("VirtualMachine is powered on")
			vmopVM = getReconciledVM(ctx)
			vmopVM.Status.PowerState = vmoprv1.VirtualMachinePoweredOn
			updateReconciledVM(ctx, vmopVM)
			expectedState = vmwarev1.VirtualMachineStatePoweredOn
			expectedConditions[0].Reason = vmwarev1.WaitingForNetworkAddressReason
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			// Simulate VMOperator assigning an IP address
			By("VirtualMachine has an IP address")
			vmopVM = getReconciledVM(ctx)
			vmopVM.Status.VmIp = vmIP
			updateReconciledVM(ctx, vmopVM)
			expectedConditions[0].Reason = vmwarev1.WaitingForBIOSUUIDReason
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			// Simulate VMOperator assigning an Bios UUID
			By("VirtualMachine has Bios UUID")
			expectReconcileError = false
			expectedRequeue = false
			expectedBiosUUID = biosUUID
			expectedVMIP = vmIP
			expectedState = vmwarev1.VirtualMachineStateReady
			expectedConditions[0].Status = corev1.ConditionTrue
			expectedConditions[0].Reason = ""
			vmopVM = getReconciledVM(ctx)
			vmopVM.Status.BiosUUID = biosUUID
			updateReconciledVM(ctx, vmopVM)
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			Expect(vmopVM.Spec.ReadinessProbe).To(BeNil())

			By("Setting cluster.Status.ControlPlaneReady to true")
			// Set the control plane to be ready so that the new VM will have a probe
			cluster.Status.ControlPlaneReady = true

			vmopVM = getReconciledVM(ctx)
			vmopVM.Status.VmIp = vmIP
			updateReconciledVM(ctx, vmopVM)
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			Expect(vmopVM.Spec.ReadinessProbe.TCPSocket.Port.IntValue()).To(Equal(defaultAPIBindPort))
		})

		Specify("Reconcile invalid Machine", func() {
			expectReconcileError = true
			expectVMOpVM = false
			expectedImageName = imageName

			By("Machine doens't have a K8S version")
			machine.Spec.Version = nil
			expectedConditions = append(expectedConditions, clusterv1.Condition{
				Type:    infrav1.VMProvisionedCondition,
				Status:  corev1.ConditionFalse,
				Reason:  vmwarev1.VMCreationFailedReason,
				Message: missingK8SVersionFailure,
			})
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)
		})

		Specify("Reconcile machine when vm prerequisites check fails", func() {
			secretName := machine.GetName() + "-data"
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: machine.GetNamespace(),
				},
				Data: map[string][]byte{
					"value": []byte(bootstrapData),
				},
			}
			Expect(ctx.Client.Create(ctx, secret)).To(Succeed())
			machine.Spec.Bootstrap.DataSecretName = &secretName

			requeue, err = vmService.ReconcileNormal(ctx)
			vmopVM = getReconciledVM(ctx)
			errMessage := "TestVirtualMachineClassBinding not found"
			vmopVM.Status.Conditions = append(vmopVM.Status.Conditions, vmoprv1.Condition{
				Type:     vmoprv1.VirtualMachinePrereqReadyCondition,
				Status:   corev1.ConditionFalse,
				Reason:   vmoprv1.VirtualMachineClassBindingNotFoundReason,
				Severity: vmoprv1.ConditionSeverityError,
				Message:  errMessage,
			})

			updateReconciledVM(ctx, vmopVM)
			requeue, err = vmService.ReconcileNormal(ctx)

			expectedImageName = imageName
			expectReconcileError = true
			expectVMOpVM = true
			expectedConditions = append(expectedConditions, clusterv1.Condition{
				Type:     infrav1.VMProvisionedCondition,
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityError,
				Reason:   vmoprv1.VirtualMachineClassBindingNotFoundReason,
				Message:  errMessage,
			})
			verifyOutput(ctx)
		})

		Specify("Preserve changes made by other sources", func() {
			secretName := machine.GetName() + "-data"
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: machine.GetNamespace(),
				},
				Data: map[string][]byte{
					"value": []byte(bootstrapData),
				},
			}
			Expect(ctx.Client.Create(ctx, secret)).To(Succeed())
			machine.Spec.Bootstrap.DataSecretName = &secretName

			expectReconcileError = false
			expectVMOpVM = true
			expectedImageName = imageName
			expectedRequeue = true

			By("VirtualMachine is created")
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			vmVolume := vmoprv1.VirtualMachineVolume{
				Name: "test",
				PersistentVolumeClaim: &vmoprv1.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "test-pvc",
						ReadOnly:  false,
					},
				},
			}

			By("Updating the Volumes field")
			vmopVM = getReconciledVM(ctx)
			vmopVM.Spec.Volumes = []vmoprv1.VirtualMachineVolume{vmVolume}
			updateReconciledVM(ctx, vmopVM)
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			By("Checking that the Volumes field is still set after the reconcile")
			vmopVM = getReconciledVM(ctx)

			Expect(vmopVM.Spec.Volumes).To(HaveLen(1))
			Expect(vmopVM.Spec.Volumes[0]).To(BeEquivalentTo(vmVolume))
		})

		Specify("Create and attach volumes", func() {
			expectReconcileError = false
			expectVMOpVM = true
			expectedImageName = imageName
			expectedRequeue = true

			vsphereMachine.Spec.Volumes = []vmwarev1.VSphereMachineVolume{
				{
					Name: "etcd",
					Capacity: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				{
					Name: "containerd",
					Capacity: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("6Gi"),
					},
				},
			}

			By("VirtualMachine is created")
			requeue, err = vmService.ReconcileNormal(ctx)
			verifyOutput(ctx)

			By("Checking that the Volumes field is set after the reconcile")
			vmopVM = getReconciledVM(ctx)

			Expect(vmopVM.Spec.Volumes).To(HaveLen(2))

			for i, volume := range vsphereMachine.Spec.Volumes {
				name := volumeName(vsphereMachine, volume)
				vmVolume := vmoprv1.VirtualMachineVolume{
					Name: name,
					PersistentVolumeClaim: &vmoprv1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: name,
							ReadOnly:  false,
						},
					},
				}

				Expect(vmopVM.Spec.Volumes[i]).To(BeEquivalentTo(vmVolume))
			}
		})
	})

	Context("Delete tests", func() {
		timeout := time.Second * 5
		interval := time.Second * 1

		verifyDeleteFunc := func() bool {
			// Our reconcile loop calls DestroyVM until it gets the answer it's looking for
			_ = vmService.ReconcileDelete(ctx)
			Expect(ctx.VSphereMachine).ShouldNot(BeNil())
			if ctx.VSphereMachine.Status.VMStatus == vmwarev1.VirtualMachineStateNotFound {
				// If the state is NotFound, check that the VM really has gone
				Expect(getReconciledVM(ctx)).Should(BeNil())
				return true
			}
			// If the state is not NotFound, it must be Deleting
			Expect(ctx.VSphereMachine.Status.VMStatus).Should(Equal(vmwarev1.VirtualMachineStateDeleting))
			return false
		}

		BeforeEach(func() {
			requeue, err = vmService.ReconcileNormal(ctx)
			// There is no GC behavior in the unit test environment, so simulate it
			vmService.deleteFunc = func(vm *vmoprv1.VirtualMachine) error {
				if vm.GetFinalizers() == nil {
					if err := ctx.Client.Delete(ctx, vm); err != nil {
						return err
					}
				} else {
					timeNow := metav1.Now()
					vm.SetDeletionTimestamp(&timeNow)
					if err := ctx.Client.Update(ctx, vm); err != nil {
						return err
					}
				}
				return nil
			}
		})

		// Test expects DestroyVM to return NotFound eventually
		Specify("Delete VirtualMachine with no delay", func() {
			Expect(getReconciledVM(ctx)).ShouldNot(BeNil())
			Eventually(verifyDeleteFunc, timeout, interval).Should(Equal(true))
		})

		Context("With finalizers", func() {
			JustBeforeEach(func() {
				vmopVM := getReconciledVM(ctx)
				Expect(vmopVM).ShouldNot(BeNil())
				vmopVM.Finalizers = append(ctx.VSphereMachine.Finalizers, "test-finalizer")
				Expect(ctx.Client.Update(ctx, vmopVM)).To(Succeed())
			})

			// Test never removes the finalizer and expects DestroyVM to never return NotFound
			Specify("Delete VirtualMachine with finalizer", func() {
				Consistently(verifyDeleteFunc, timeout, interval).Should(Equal(false))
			})

			// Check that DestroyVM does not update VirtualMachine more than once
			Specify("DestroyVM does not continue to update the VirtualMachine", func() {
				_ = vmService.ReconcileDelete(ctx)
				vmopVM := getReconciledVM(ctx)
				Expect(vmopVM).ShouldNot(BeNil())
				deleteTimestamp := vmopVM.GetDeletionTimestamp()
				Expect(deleteTimestamp).ShouldNot(BeNil())

				_ = vmService.ReconcileDelete(ctx)
				vmopVM = getReconciledVM(ctx)
				Expect(vmopVM).ShouldNot(BeNil())

				Expect(vmopVM.GetDeletionTimestamp()).To(Equal(deleteTimestamp))
			})
		})
	})
})
