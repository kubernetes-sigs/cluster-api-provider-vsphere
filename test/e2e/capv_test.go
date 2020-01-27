/*
Copyright 2019 The Kubernetes Authors.

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

package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo" //nolint:golint
	. "github.com/onsi/gomega" //nolint:golint
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	frameworkx "sigs.k8s.io/cluster-api-provider-vsphere/test/e2e/framework"
)

var _ = Describe("CAPV", func() {
	Describe("Cluster Creation", func() {
		var (
			clusterName          string
			clusterGen           ClusterGenerator
			loadBalancerGen      LoadBalancerGenerator
			machineDeploymentGen MachineDeploymentGenerator
			controlPlaneNodeGen  ControlPlaneNodeGenerator
			pollTimeout          = 10 * time.Minute
			pollInterval         = 10 * time.Second

			numControlPlaneMachines int32
			numWorkerMachines       int32
			input                   *framework.ControlplaneClusterInput
		)

		JustBeforeEach(func() {
			clusterName = fmt.Sprintf("test-%s", Hash7())

			cluster, infraCluster := clusterGen.Generate("default", clusterName)
			controlPlaneNodes := make([]framework.Node, numControlPlaneMachines)
			for i := range controlPlaneNodes {
				controlPlaneNodes[i] = controlPlaneNodeGen.Generate(cluster.Namespace, cluster.Name)
			}
			machineDeployment := machineDeploymentGen.Generate(cluster.Namespace, cluster.Name, numWorkerMachines)
			relatedResources := []runtime.Object{}

			if loadBalancerGen != nil {
				By("generating load balancer")
				loadBalancer := loadBalancerGen.Generate(cluster.Namespace, cluster.Name)
				loadBalancerObj, ok := loadBalancer.(metav1.Object)
				Expect(ok).To(BeTrue())
				Expect(loadBalancerObj).ToNot(BeNil())
				loadBalancerGVK := loadBalancer.GetObjectKind().GroupVersionKind()
				By(fmt.Sprintf("generated %s", loadBalancerGVK))
				infraCluster.Spec.LoadBalancerRef = &corev1.ObjectReference{
					APIVersion: loadBalancerGVK.GroupVersion().String(),
					Kind:       loadBalancerGVK.Kind,
					Namespace:  loadBalancerObj.GetNamespace(),
					Name:       loadBalancerObj.GetName(),
				}
				relatedResources = append(relatedResources, loadBalancer)
			}

			input = &framework.ControlplaneClusterInput{
				Management:        mgmt,
				Cluster:           cluster,
				InfraCluster:      infraCluster,
				Nodes:             controlPlaneNodes,
				MachineDeployment: machineDeployment,
				RelatedResources:  relatedResources,
				CreateTimeout:     pollTimeout,
				DeleteTimeout:     pollTimeout,
			}
		})

		AfterEach(func() {
			By("cleaning up e2e resources")
			input.CleanUpCoreArtifacts()

			clusterLabelSelector := client.MatchingLabels{clusterv1.ClusterLabelName: clusterName}

			By("asserting all VSphereVM resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.VSphereVM, error) {
				list := &infrav1.VSphereVMList{}
				if err := mgmtClient.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, pollTimeout, pollInterval).Should(HaveLen(0))

			By("asserting all VSphereMachine resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.VSphereMachine, error) {
				list := &infrav1.VSphereMachineList{}
				if err := mgmtClient.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, pollTimeout, pollInterval).Should(HaveLen(0))

			By("asserting all VSphereCluster resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.VSphereCluster, error) {
				list := &infrav1.VSphereClusterList{}
				if err := mgmtClient.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, pollTimeout, pollInterval).Should(HaveLen(0))

			By("asserting all HAProxyLoadBalancer resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.HAProxyLoadBalancer, error) {
				list := &infrav1.HAProxyLoadBalancerList{}
				if err := mgmtClient.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, pollTimeout, pollInterval).Should(HaveLen(0))

			destroyVMsWithPrefix(clusterName)
			loadBalancerGen = nil
			numControlPlaneMachines = 0
			numWorkerMachines = 0
		})

		Context("Single-node control plane with one worker node", func() {
			BeforeEach(func() {
				numControlPlaneMachines = 1
				numWorkerMachines = 1
			})
			It("should create a single-node control plane with one worker node", func() {
				frameworkx.ControlPlaneCluster(input)
			})
		})

		Context("Two-node control plane with one worker node", func() {
			BeforeEach(func() {
				loadBalancerGen = HAProxyLoadBalancerGenerator{}
				numControlPlaneMachines = 2
				numWorkerMachines = 1
			})
			It("should create a two-node control plane with one worker node", func() {
				frameworkx.ControlPlaneCluster(input)
			})
		})
	})
})
