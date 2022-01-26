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

package test

import (
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

const (
	namespace = "default"
)

// newInfraCluster returns an Infra cluster with the same name as the target
// cluster.
func newInfraCluster(cluster *clusterv1.Cluster) client.Object {
	// func newInfraCluster(cluster *clusterv1.Cluster) runtimeObject {
	return &vmwarev1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: namespace,
		},
	}
}

// newAnonInfraCluster returns an Infra cluster with a generated name.
func newAnonInfraCluster() client.Object {
	return &vmwarev1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    namespace,
		},
	}
}

// newInfraMachine creates an Infra machine with the same name as the target
// machine.
func newInfraMachine(machine *clusterv1.Machine) client.Object {
	return &vmwarev1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machine.Name,
			Namespace: namespace,
		},
	}
}

// newInfraMachine creates an Infra machine with a generated name.
func newAnonInfraMachine() client.Object {
	return &vmwarev1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    namespace,
		},
	}
}

// Creates and deploys a Cluster and VSphereCluster in order. Function does not
// block on VSphereCluster creation.
func deployCluster() (client.ObjectKey, *clusterv1.Cluster, client.Object) {
	// A finalizer is added to prevent it from being deleted until its
	// dependents are removed.
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    namespace,
			Finalizers:   []string{"test"},
		},
		Spec: clusterv1.ClusterSpec{},
	}
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

	clusterKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}
	Eventually(func() error {
		return k8sClient.Get(ctx, clusterKey, cluster)
	}, time.Second*30).Should(Succeed())

	By("Create the infrastructure cluster and wait for it to have a finalizer")
	infraCluster := newInfraCluster(cluster)
	infraCluster.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: cluster.APIVersion,
			Kind:       cluster.Kind,
			Name:       cluster.Name,
			UID:        cluster.UID,
		},
	})
	Expect(k8sClient.Create(ctx, infraCluster)).To(Succeed())

	return clusterKey, cluster, infraCluster
}

// Creates and deploys a CAPI Machine. Function does not block on Machine
// creation.
func deployCAPIMachine(cluster *clusterv1.Cluster) (client.ObjectKey, *clusterv1.Machine) {
	// A finalizer is added to prevent it from being deleted until its
	// dependents are removed.
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    namespace,
			Finalizers:   []string{"test"},
			Labels: map[string]string{
				clusterv1.ClusterLabelName:             cluster.Name,
				clusterv1.MachineControlPlaneLabelName: "true",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cluster.APIVersion,
					Kind:       cluster.Kind,
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: cluster.Name,
		},
	}
	Expect(k8sClient.Create(ctx, machine)).To(Succeed())
	machineKey := client.ObjectKey{Namespace: machine.Namespace, Name: machine.Name}
	return machineKey, machine
}

// Creates and deploys a VSphereMachine. Function does not block on Machine
// creation.
func deployInfraMachine(machine *clusterv1.Machine, finalizers []string) (client.ObjectKey, client.Object) {
	infraMachine := newInfraMachine(machine)
	infraMachine.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: machine.APIVersion,
			Kind:       machine.Kind,
			Name:       machine.Name,
			UID:        machine.UID,
		},
	})
	infraMachine.SetFinalizers(finalizers)
	Expect(k8sClient.Create(ctx, infraMachine)).To(Succeed())
	infraMachineKey := client.ObjectKey{Namespace: infraMachine.GetNamespace(), Name: infraMachine.GetName()}
	return infraMachineKey, infraMachine
}

// Updates the InfrastructureRef of a CAPI Cluster to a VSphereCluster. Function
// does not block on update success.
func updateClusterInfraRef(cluster *clusterv1.Cluster, infraCluster client.Object) {
	cluster.Spec.InfrastructureRef = &corev1.ObjectReference{
		APIVersion: infraCluster.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       infraCluster.GetObjectKind().GroupVersionKind().Kind,
		Name:       infraCluster.GetName(),
	}
	Expect(k8sClient.Update(ctx, cluster)).To(Succeed())
}

// Cache the type names of the infrastructure cluster and machine.
var (
	infraClusterTypeName = reflect.TypeOf(newAnonInfraCluster()).Elem().Name()
	infraMachineTypeName = reflect.TypeOf(newAnonInfraCluster()).Elem().Name()
)

// The spec conformance tests assert that the infrastructure types
// can be submitted to the API server without any errors.
var _ = Describe("Spec conformance tests", func() {
	var (
		obj client.Object
		key *client.ObjectKey
	)

	// assertObjEventuallyExists is used to assert that eventually obj can be
	// retrieved from the API server.
	assertObjEventuallyExists := func() {
		EventuallyWithOffset(1, func() error {
			return k8sClient.Get(ctx, *key, obj)
		}, time.Second*30).Should(Succeed())
	}

	JustBeforeEach(func() {
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		key = &client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}
	})
	JustAfterEach(func() {
		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
	})
	AfterEach(func() {
		obj = nil
		key = nil
	})
	Context(infraClusterTypeName, func() {
		BeforeEach(func() {
			obj = newAnonInfraCluster()
		})
		It("Will be created and wait on an OwnerRef", func() {
			assertObjEventuallyExists()
		})
	})

	Context(infraMachineTypeName, func() {
		BeforeEach(func() {
			obj = newAnonInfraMachine()
		})
		It("Will be created and wait on an OwnerRef", func() {
			assertObjEventuallyExists()
		})
	})
})

// Verifies that the infrastructure types have finalizers set when an OwnerRef
// is set that points to the corresponding CAPI resource.
var _ = Describe("Reconciler tests", func() {
	// assertEventuallyFinalizers is used to assert an object eventually has one
	// or more finalizers.
	assertEventuallyFinalizers := func(key client.ObjectKey, obj client.Object) {
		EventuallyWithOffset(1, func() (int, error) {
			if err := k8sClient.Get(ctx, key, obj); err != nil {
				return 0, err
			}
			return len(obj.GetFinalizers()), nil
		}, time.Second*30).Should(BeNumerically(">", 0))
	}

	assertEventuallyVMStatus := func(key client.ObjectKey, obj client.Object, expectedState vmwarev1.VirtualMachineState) {
		EventuallyWithOffset(1, func() (vmwarev1.VirtualMachineState, error) {
			if err := k8sClient.Get(ctx, key, obj); err != nil {
				return "", err
			}
			vSphereMachine := obj.(*vmwarev1.VSphereMachine) //nolint:forcetypeassert
			return vSphereMachine.Status.VMStatus, nil
		}, time.Second*30).Should(Equal(expectedState))
	}

	assertEventuallyControlPlaneEndpoint := func(key client.ObjectKey, obj client.Object, expectedIP string) {
		EventuallyWithOffset(1, func() (string, error) {
			if err := k8sClient.Get(ctx, key, obj); err != nil {
				return "", err
			}
			vsphereCluster := obj.(*vmwarev1.VSphereCluster) //nolint:forcetypeassert
			return vsphereCluster.Spec.ControlPlaneEndpoint.Host, nil
		}, time.Second*30).Should(Equal(expectedIP))
	}

	deleteAndWait := func(key client.ObjectKey, obj client.Object, removeFinalizers bool) {
		// Delete the object.
		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())

		// Issues updates until the patch to remove the finalizers is
		// successful.
		if removeFinalizers {
			EventuallyWithOffset(1, func() error {
				if err := k8sClient.Get(ctx, key, obj); err != nil {
					return err
				}
				obj.SetFinalizers([]string{})
				return k8sClient.Update(ctx, obj)
			}, time.Second*30).Should(Succeed())
		}

		// Wait for the object to no longer be available.
		EventuallyWithOffset(1, func() error {
			return k8sClient.Get(ctx, key, obj)
		}, time.Second*30).ShouldNot(Succeed())
	}

	Specify("Infrastructure resources should have finalizers after reconciliation", func() {
		By("Create the CAPI Cluster and wait for it to exist")
		clusterKey, cluster, infraCluster := deployCluster()

		// Assert that eventually the infrastructure cluster will have a
		// finalizer.
		infraClusterKey := client.ObjectKey{Namespace: infraCluster.GetNamespace(), Name: infraCluster.GetName()}
		assertEventuallyFinalizers(infraClusterKey, infraCluster)

		By("Update the CAPI Cluster's InfrastructureRef")
		updateClusterInfraRef(cluster, infraCluster)

		By("Expect a ResourcePolicy to exist")
		rpKey := client.ObjectKey{Namespace: infraCluster.GetNamespace(), Name: infraCluster.GetName()}
		resourcePolicy := &vmoprv1.VirtualMachineSetResourcePolicy{}
		Expect(k8sClient.Get(ctx, rpKey, resourcePolicy)).To(Succeed())
		Expect(len(resourcePolicy.Spec.ClusterModules)).To(BeEquivalentTo(2))

		By("Create the CAPI Machine and wait for it to exist")
		machineKey, machine := deployCAPIMachine(cluster)
		Eventually(func() error {
			return k8sClient.Get(ctx, machineKey, machine)
		}, time.Second*30).Should(Succeed())

		By("Create the infrastructure machine and wait for it to have a finalizer")
		infraMachineKey, infraMachine := deployInfraMachine(machine, nil)
		assertEventuallyFinalizers(infraMachineKey, infraMachine)

		// Delete the CAPI Cluster. To simulate the CAPI components we must:
		//
		// 1. Delete a resource.
		// 2. Remove its finalizers (if its a CAPI object).
		// 3. Update the resource.
		// 4. Wait for the resource to be deleted.
		By("Delete the infrastructure machine and wait for it to be removed")
		deleteAndWait(infraMachineKey, infraMachine, false)

		By("Delete the CAPI machine and wait for it to be removed")
		deleteAndWait(machineKey, machine, true)

		By("Delete the infrastructure cluster and wait for it to be removed")
		deleteAndWait(infraClusterKey, infraCluster, false)

		By("Delete the CAPI cluster and wait for it to be removed")
		deleteAndWait(clusterKey, cluster, true)
	})

	Specify("VSphereClusters can be deleted without a corresponding Cluster", func() {
		By("Creating an infrastructure cluster with no owner or cluster reference")
		infraCluster := newAnonInfraCluster()
		Expect(k8sClient.Create(ctx, infraCluster)).To(Succeed())
		infraClusterKey := client.ObjectKey{Namespace: infraCluster.GetNamespace(), Name: infraCluster.GetName()}

		// We add the finalizer here to simulate an observed situation where
		// there was no Cluster OwnerRef, but there _was_ a finalizer. This
		// _shouldn't_ ever happen during normal operation, but it can happen
		// if users muck with things.
		infraCluster.SetFinalizers([]string{vmwarev1.ClusterFinalizer})
		Expect(k8sClient.Update(ctx, infraCluster)).To(Succeed())
		assertEventuallyFinalizers(infraClusterKey, infraCluster)

		By("Deleting the infrastructure cluster and waiting for it to be removed")
		deleteAndWait(infraClusterKey, infraCluster, false)
	})

	Specify("Create and Delete a VSphereMachine with a Machine but without a Cluster", func() {
		By("Create the CAPI Machine and wait for it to exist")
		// A finalizer is added to prevent it from being deleted until its
		// dependents are removed.
		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    namespace,
				Finalizers:   []string{"test"},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: "crud",
			},
		}
		Expect(k8sClient.Create(ctx, machine)).To(Succeed())
		machineKey := client.ObjectKey{Namespace: machine.Namespace, Name: machine.Name}
		Eventually(func() error {
			return k8sClient.Get(ctx, machineKey, machine)
		}, time.Second*30).Should(Succeed())

		By("Create the infrastructure machine and set a finalizer")
		infraMachineKey, infraMachine := deployInfraMachine(machine, []string{infrav1.MachineFinalizer})
		Eventually(func() error {
			return k8sClient.Get(ctx, infraMachineKey, infraMachine)
		}, time.Second*30).Should(Succeed())

		By("Delete the InfraMachine and wait for it to be removed")
		deleteAndWait(infraMachineKey, infraMachine, false)
	})

	Specify("A VM gets properly reconciled for a Machine and reflects appropriate VM status", func() {
		By("Create the CAPI Cluster and wait for it to exist")
		_, cluster, infraCluster := deployCluster()
		updateClusterInfraRef(cluster, infraCluster)
		infraClusterKey := client.ObjectKey{Namespace: infraCluster.GetNamespace(), Name: infraCluster.GetName()}
		Eventually(func() error {
			return k8sClient.Get(ctx, infraClusterKey, infraCluster)
		}, time.Second*30).Should(Succeed())
		updateClusterInfraRef(cluster, infraCluster)

		By("Create the CAPI Machine and wait for it to exist")
		machineKey, machine := deployCAPIMachine(cluster)
		Eventually(func() error {
			return k8sClient.Get(ctx, machineKey, machine)
		}, time.Second*30).Should(Succeed())

		By("Create the infrastructure machine and wait for it to exist")
		infraMachineKey, infraMachine := deployInfraMachine(machine, nil)
		Eventually(func() error {
			return k8sClient.Get(ctx, infraMachineKey, infraMachine)
		}, time.Second*30).Should(Succeed())

		By("Add bootstrap data to the machine")
		data := "test-bootstrap-data"
		version := "test-version"
		secretName := machine.GetName() + "-data"
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: machine.GetNamespace(),
			},
			Data: map[string][]byte{
				"value": []byte(data),
			},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		machine.Spec.Version = &version
		machine.Spec.Bootstrap.DataSecretName = &secretName
		Expect(k8sClient.Update(ctx, machine)).To(Succeed())

		// At this point, the reconcile loop should create a VirtualMachine Note
		// that the reconcile loop will continue to run while a VirtualMachine is
		// going through its various stages of initialization due to vmoperator
		// code returning reconcile errors

		By("Expect the VSphereMachine to have its Status.VMStatus initialized to a new VM")
		assertEventuallyVMStatus(infraMachineKey, infraMachine, vmwarev1.VirtualMachineStatePending)

		By("Expect the VM to have been successfully created")
		newVM := &vmoprv1.VirtualMachine{}
		Expect(k8sClient.Get(ctx, machineKey, newVM)).Should(Succeed())

		By("Modifying the VM to simulate it having been created")
		// These two lines must be initialized as requirements of having valid Status
		newVM.Status.Volumes = []vmoprv1.VirtualMachineVolumeStatus{}
		newVM.Status.Phase = vmoprv1.Created
		Expect(k8sClient.Status().Update(ctx, newVM)).Should(Succeed())

		By("Expect the VSphereMachine VM status to reflect VM Created status")
		assertEventuallyVMStatus(infraMachineKey, infraMachine, vmwarev1.VirtualMachineStateCreated)

		By("Modifying the VM to simulate it having been powered on")
		newVM.Status.PowerState = vmoprv1.VirtualMachinePoweredOn
		Expect(k8sClient.Status().Update(ctx, newVM)).Should(Succeed())

		By("Expect the VSphereMachine VM status to reflect VM PoweredOn status")
		assertEventuallyVMStatus(infraMachineKey, infraMachine, vmwarev1.VirtualMachineStatePoweredOn)

		By("Modifying the VM to simulate it having been successfully booted")
		newVM.Status.VmIp = "1.2.3.4"
		newVM.Status.BiosUUID = "test-bios-uuid"
		Expect(k8sClient.Status().Update(ctx, newVM)).Should(Succeed())

		By("Expect the VSphereMachine VM status to reflect VM Ready status")
		assertEventuallyVMStatus(infraMachineKey, infraMachine, vmwarev1.VirtualMachineStateReady)

		// In the case of a LoadBalanced endpoint, ControlPlaneEndpoint is a
		// load-balancer Testing load-balanced endpoints is done in
		// control_plane_endpoint_test.go
		if !isLB {
			By("Expect the Cluster to have the IP from the VM as an APIEndpoint")
			assertEventuallyControlPlaneEndpoint(infraClusterKey, infraCluster, newVM.Status.VmIp)
		}
	})
})
