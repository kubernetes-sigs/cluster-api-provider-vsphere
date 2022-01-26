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

package integration

import (
	"fmt"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// The purpose of this test is to start up a CAPI controller against a real API
// server and run basic checks.
var _ = Describe("Sanity tests", func() {
	var (
		mf            *Manifests
		controlPlane  *ControlPlaneComponents
		testNamespace string
	)

	JustBeforeEach(func() {
		testNamespace = fmt.Sprintf("test-ns-%s", uuid.New())
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		createNonNamespacedResource(namespacesResource, ns)
		By("Creating a dummy VM Image")
		dummyVMImage := generateVirtualMachineImage()
		createNonNamespacedResource(virtualmachineimageResource, dummyVMImage)

		By("Generating manifests")
		mf = generateManifests(testNamespace)

		// Only the first control plane machine is used since any additional
		// ones require an initialized control plane, something which is no
		// longer possible to simulate in CAPI v1a2 without a working API
		// endpoint.
		Expect(mf.ControlPlaneComponentsList).Should(HaveLen(1), "control plane must have exactly one machine")
		controlPlane = mf.ControlPlaneComponentsList[0]

		// CREATE the CAPI Cluster and VSphereCluster resources.
		createResource(clustersResource, mf.ClusterComponents.Cluster)
		createResource(vsphereclustersResource, mf.ClusterComponents.VSphereCluster)

		// ASSERT the CAPI Cluster and the VSphereCluster resources eventually exist
		// and that the VSphereCluster has an OwnerRef that points to the CAPI Cluster.
		cluster := assertEventuallyExists(clustersResource, mf.ClusterComponents.Cluster.Name, mf.ClusterComponents.Cluster.Namespace, nil)
		clusterOwnerRef := toOwnerRef(cluster)
		clusterOwnerRef.Controller = pointer.BoolPtr(true)
		clusterOwnerRef.BlockOwnerDeletion = pointer.BoolPtr(true)
		assertEventuallyExists(vsphereclustersResource, mf.ClusterComponents.Cluster.Name, mf.ClusterComponents.Cluster.Namespace, clusterOwnerRef)

		// CREATE the CAPI Machine, VSphereMachine, and KubeadmConfig resources for
		// the control plane machine.
		createResource(machinesResource, controlPlane.Machine)
		createResource(vspheremachinesResource, controlPlane.VSphereMachine)
		createResource(kubeadmconfigResources, controlPlane.KubeadmConfig)

		// ASSERT the CAPI Machine, VSphereMachine, and KubeadmConfig resources
		// for the control plane machine eventually exist and that the
		// VSphereMachine and KubeadmConfig resources have OwnerRefs that point to
		// the CAPI Machine.
		machine := assertEventuallyExists(machinesResource, controlPlane.Machine.Name, controlPlane.Machine.Namespace, nil)
		machineOwnerRef := toOwnerRef(machine)
		machineOwnerRef.Controller = pointer.BoolPtr(true)
		machineOwnerRef.BlockOwnerDeletion = pointer.BoolPtr(true)
		assertEventuallyExists(vspheremachinesResource, controlPlane.Machine.Name, controlPlane.Machine.Namespace, machineOwnerRef)
		assertEventuallyExists(kubeadmconfigResources, controlPlane.Machine.Name, controlPlane.Machine.Namespace, machineOwnerRef)
	})

	AfterEach(func() {
		// DELETE the testNamespace
		deleteNonNamespacedResource(namespacesResource, testNamespace, nil)
		mf = nil
		controlPlane = nil
	})

	JustAfterEach(func() {
		// DELETE the CAPI Machine, VSphereMachine, and KubeadmConfig resources for
		// the control plane machine.
		deleteResource(machinesResource, controlPlane.Machine.Name, controlPlane.Machine.Namespace, nil)

		// ASSERT the CAPI Machine, VSphereMachine, KubeadmConfig, VM
		// Operator VirtualMachine, and bootstrap data ConfigMap
		// resources for the control plane machine are eventually
		// deleted.
		assertEventuallyDoesNotExist(configmapsResource, infrautilv1.GetBootstrapConfigMapName(controlPlane.Machine.Name), controlPlane.Machine.Namespace)
		assertEventuallyDoesNotExist(virtualmachinesResource, controlPlane.Machine.Name, controlPlane.Machine.Namespace)
		assertEventuallyDoesNotExist(vspheremachinesResource, controlPlane.Machine.Name, controlPlane.Machine.Namespace)
		assertEventuallyDoesNotExist(kubeadmconfigResources, controlPlane.Machine.Name, controlPlane.Machine.Namespace)
		assertEventuallyDoesNotExist(machinesResource, controlPlane.Machine.Name, controlPlane.Machine.Namespace)

		// DELETE the CAPI Cluster.
		deleteResource(clustersResource, mf.ClusterComponents.Cluster.Name, mf.ClusterComponents.Cluster.Namespace, nil)

		// ASSERT the CAPI Cluster and VSphereCLuster are eventually deleted.
		assertEventuallyDoesNotExist(vsphereclustersResource, mf.ClusterComponents.Cluster.Name, mf.ClusterComponents.Cluster.Namespace)
		assertEventuallyDoesNotExist(clustersResource, mf.ClusterComponents.Cluster.Name, mf.ClusterComponents.Cluster.Namespace)
	})

	Context("Happy paths", func() {
		BeforeEach(func() {
			testClusterName = "sanity-testcluster"
		})

		It("Check Basic VirtualMachine creation", func() {
			// GET the associated CAPI Machine.
			machine := &clusterv1.Machine{}
			getResource(machinesResource, controlPlane.Machine.Name, controlPlane.Machine.Namespace, machine)

			// GET the associated VSphereMachine.
			vsphereMachine := &infrav1.VSphereMachine{}
			getResource(vspheremachinesResource, controlPlane.Machine.Name, controlPlane.Machine.Namespace, vsphereMachine)

			// ASSERT the VirtualMachine and bootstrap data ConfigMap resources
			// eventually exist. Ensure ConfigMap has OwnerRef set to the VSphereMachine.
			vmObj := assertEventuallyExists(virtualmachinesResource, controlPlane.Machine.Name, controlPlane.Machine.Namespace, nil)
			assertEventuallyExists(configmapsResource, infrautilv1.GetBootstrapConfigMapName(controlPlane.Machine.Name), controlPlane.Machine.Namespace, toControllerOwnerRef(vsphereMachine))
			vm := &vmoprv1.VirtualMachine{}
			toStructured(controlPlane.Machine.Name, vm, vmObj)

			// ASSERT the VirtualMachine resource has the expected state.
			assertVirtualMachineState(machine, vm)
		})
	})
})
