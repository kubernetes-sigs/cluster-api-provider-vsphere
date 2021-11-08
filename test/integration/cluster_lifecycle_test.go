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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	infrautilv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// The purpose of this test is to start up a CAPI controller against a real API
// server and run Cluster tests.
var _ = Describe("Cluster lifecycle tests", func() {
	var (
		mf            *Manifests
		controlPlane  *ControlPlaneComponents
		worker        *WorkerComponents
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
		// Expect(mf.ControlPlaneComponentsList).Should(HaveLen(1), "control plane must have exactly one machine")
		controlPlane = mf.ControlPlaneComponentsList[0]
		worker = mf.WorkerComponents
	})

	AfterEach(func() {
		deleteResource(namespacesResource, testNamespace, "", nil)
		mf = nil
		controlPlane = nil
		worker = nil
	})

	Context("Create a cluster", func() {
		JustBeforeEach(func() {
			// CREATE the CAPI Cluster and VSphereCluster resources.
			createResource(clustersResource, mf.ClusterComponents.Cluster)
			createResource(vsphereclustersResource, mf.ClusterComponents.VSphereCluster)

			// ASSERT the CAPI Cluster and the VSphereCluster resources eventually exist
			// and that the VSphereCluster has an OwnerRef that points to the CAPI
			// Cluster.
			cluster := assertEventuallyExists(clustersResource, mf.ClusterComponents.Cluster.Name, testNamespace, nil)
			clusterOwnerRef := toOwnerRef(cluster)
			clusterOwnerRef.Controller = pointer.BoolPtr(true)
			clusterOwnerRef.BlockOwnerDeletion = pointer.BoolPtr(true)
			assertEventuallyExists(vsphereclustersResource, mf.ClusterComponents.Cluster.Name, testNamespace, clusterOwnerRef)
		})

		JustAfterEach(func() {
			// DELETE the CAPI Cluster.
			deleteResource(clustersResource, mf.ClusterComponents.Cluster.Name, testNamespace, nil)

			// ASSERT the CAPI Cluster and VSphereCluster are eventually deleted.
			assertEventuallyDoesNotExist(vsphereclustersResource, mf.ClusterComponents.Cluster.Name, testNamespace)
			assertEventuallyDoesNotExist(clustersResource, mf.ClusterComponents.Cluster.Name, testNamespace)
		})

		Context("with no machines", func() {
			BeforeEach(func() {
				testClusterName = "cc-testcluster1"
			})
			It("should delete successfully with default policy", func() {
				// Handled by JustAfterEach
			})
		})

		Context("with machines", func() {
			JustBeforeEach(func() {
				// CREATE the CAPI Machine, VSphereMachine, and KubeadmConfig resources for
				// the control plane machine.
				createResource(machinesResource, controlPlane.Machine)
				createResource(vspheremachinesResource, controlPlane.VSphereMachine)
				createResource(kubeadmconfigResources, controlPlane.KubeadmConfig)

				// CREATE the CAPI MachineDeplopyment, VSphereMachineTemplate,
				// and KubeadmConfigTemplate resources for the worker nodes.
				createResource(machinedeploymentResource, worker.MachineDeployment)
				createResource(vspheremachinetemplateResource, worker.VSphereMachineTemplate)
				createResource(kubeadmconfigtemplateResource, worker.KubeadmConfigTemplate)

				// ASSERT the CAPI Machine, VSphereMachine, KubeadmConfig, and VM
				// Operator VirtualMachine, and bootstrap data ConfigMap
				// resources for the control plane machine eventually exist, the
				// VSphereMachine and KubeadmConfig resources have OwnerRefs that
				// point to the CAPI Machine, and the ConfigMap resource has a
				// controller OwnerRef that points to the VSphereMachine.
				machine := assertEventuallyExists(machinesResource, controlPlane.Machine.Name, testNamespace, nil)
				machineOwnerRef := toOwnerRef(machine)
				machineOwnerRef.Controller = pointer.BoolPtr(true)
				machineOwnerRef.BlockOwnerDeletion = pointer.BoolPtr(true)
				assertEventuallyExists(kubeadmconfigResources, controlPlane.Machine.Name, testNamespace, machineOwnerRef)

				vsphereMachine := assertEventuallyExists(vspheremachinesResource, controlPlane.Machine.Name, testNamespace, machineOwnerRef)
				vsphereMachineOwnerRef := toControllerOwnerRef(vsphereMachine)

				assertEventuallyExists(virtualmachinesResource, controlPlane.Machine.Name, testNamespace, nil)
				assertEventuallyExists(configmapsResource, infrautilv1.GetBootstrapConfigMapName(controlPlane.Machine.Name), testNamespace, vsphereMachineOwnerRef)

				// TODO: gab-satchi these should also be looking for correct ownerReferences before proceeding to a delete in the AfterEach
				assertEventuallyExists(machinedeploymentResource, worker.MachineDeployment.Name, testNamespace, nil)
				assertEventuallyExists(vspheremachinetemplateResource, worker.VSphereMachineTemplate.Name, testNamespace, nil)
				assertEventuallyExists(kubeadmconfigtemplateResource, worker.KubeadmConfigTemplate.Name, testNamespace, nil)
			})
			AfterEach(func() {
				// ASSERT the CAPI Machine, VSphereMachine, KubeadmConfig, VM
				// Operator VirtualMachine, and bootstrap data ConfigMap
				// resources for the control plane machine are eventually
				// deleted.
				assertEventuallyDoesNotExist(configmapsResource, infrautilv1.GetBootstrapConfigMapName(controlPlane.Machine.Name), testNamespace)
				assertEventuallyDoesNotExist(virtualmachinesResource, controlPlane.Machine.Name, testNamespace)
				assertEventuallyDoesNotExist(vspheremachinesResource, controlPlane.Machine.Name, testNamespace)
				assertEventuallyDoesNotExist(kubeadmconfigResources, controlPlane.Machine.Name, testNamespace)
				assertEventuallyDoesNotExist(machinesResource, controlPlane.Machine.Name, testNamespace)

				// Assert that MachineDeployment and its descendents are eventually deleted
				assertEventuallyDoesNotExist(machinedeploymentResource, worker.MachineDeployment.Name, testNamespace)
				assertEventuallyDoesNotExist(vspheremachinetemplateResource, worker.VSphereMachineTemplate.Name, testNamespace)
				assertEventuallyDoesNotExist(kubeadmconfigtemplateResource, worker.KubeadmConfigTemplate.Name, testNamespace)
			})
			Context("that are not explicitly deleted", func() {
				BeforeEach(func() {
					testClusterName = "cc-testcluster2"
				})
				It("should delete the cluster, deleting the machines via propagation with default policy", func() {
					// Handled by JustBeforeEach and AfterEach
				})
				It("cluster should have a ControlPlaneEndpoint when a ControlPlane machine gets an IP", func() {
					ipAddress := "127.0.0.1"
					setIPAddressOnMachine(controlPlane.Machine.Name, testNamespace, ipAddress)
					assertClusterEventuallyGetsControlPlaneEndpoint(testClusterName, testNamespace, ipAddress)
				})
			})
			Context("that are explicitly deleted before the cluster", func() {
				BeforeEach(func() {
					testClusterName = "cc-testcluster3"
				})
				It("should delete both the machines and cluster successfully when Machine is deleted", func() {
					// DELETE the CAPI Machine, VSphereMachine, and KubeadmConfig resources for
					// the control plane machine.
					// These are all deleted as a side effect of deleting the Machine due to ownerReferences
					deleteResource(machinesResource, controlPlane.Machine.Name, testNamespace, nil)
				})
				It("should delete both the machines and cluster successfully when VSphereMachine is deleted", func() {
					// DELETE the VSphereMachine resource for the control plane machine
					// Expect the cluster and everything else to be cleaned up by JustAfterEach
					deleteResource(vspheremachinesResource, controlPlane.Machine.Name, testNamespace, nil)
				})
			})
		})
	})
})
