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
	controlplane "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	frameworkx "sigs.k8s.io/cluster-api-provider-vsphere/test/e2e/framework"
)

var _ = Describe("CAPV", func() {
	Describe("Cluster Creation", func() {
		var (
			clusterName            string
			clusterGen             ClusterGenerator
			loadBalancerGen        LoadBalancerGenerator
			machineDeploymentGen   MachineDeploymentGenerator
			controlPlaneNodeGen    ControlPlaneNodeGenerator
			kubeadmControlPlaneGen *KubeadmControlPlaneGenerator
			pollTimeout            = 20 * time.Minute
			pollInterval           = 10 * time.Second

			numControlPlaneMachines int32
			numWorkerMachines       int32
			input                   *framework.ControlplaneClusterInput
		)

		BeforeEach(func() {
			// The default number of control plane and worker nodes for each
			// test is one. Tests may override these values.
			numControlPlaneMachines = 1
			numWorkerMachines = 1
		})

		JustBeforeEach(func() {

			clusterName = fmt.Sprintf("test-%s", Hash7())
			clusterNamespace := "default"
			input = &framework.ControlplaneClusterInput{
				Management:    mgmt,
				CreateTimeout: pollTimeout,
				DeleteTimeout: pollTimeout,
			}

			cluster, infraCluster := clusterGen.Generate(clusterNamespace, clusterName)
			By(logGenerated(cluster))
			By(logGenerated(infraCluster))

			if loadBalancerGen != nil {
				By("generating load balancer")
				loadBalancer := loadBalancerGen.Generate(cluster.Namespace, cluster.Name)
				loadBalancerObj, ok := loadBalancer.(metav1.Object)
				Expect(ok).To(BeTrue())
				Expect(loadBalancerObj).ToNot(BeNil())
				loadBalancerGVK := loadBalancer.GetObjectKind().GroupVersionKind()
				By(logGenerated(loadBalancer))
				infraCluster.Spec.LoadBalancerRef = &corev1.ObjectReference{
					APIVersion: loadBalancerGVK.GroupVersion().String(),
					Kind:       loadBalancerGVK.Kind,
					Namespace:  loadBalancerObj.GetNamespace(),
					Name:       loadBalancerObj.GetName(),
				}
				input.RelatedResources = append(input.RelatedResources, loadBalancer)
			}

			if kubeadmControlPlaneGen != nil {
				input.ControlPlane, input.MachineTemplate = kubeadmControlPlaneGen.Generate(clusterNamespace, clusterName, numControlPlaneMachines)
				cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
					APIVersion: controlplane.GroupVersion.String(),
					Kind:       framework.TypeToKind(input.ControlPlane),
					Namespace:  input.ControlPlane.GetNamespace(),
					Name:       input.ControlPlane.GetName(),
				}
				By(logGenerated(input.ControlPlane))
				By(logGenerated(input.MachineTemplate))
			} else {
				By("generating control plane resources")
				input.Nodes = make([]framework.Node, numControlPlaneMachines)
				for i := range input.Nodes {
					input.Nodes[i] = controlPlaneNodeGen.Generate(clusterNamespace, clusterName)
					By(logGenerated(input.Nodes[i].Machine))
					By(logGenerated(input.Nodes[i].InfraMachine))
					By(logGenerated(input.Nodes[i].BootstrapConfig))
				}
			}

			if numWorkerMachines > 0 {
				By("generating machine deployment resources")
				input.MachineDeployment = machineDeploymentGen.Generate(cluster.Namespace, cluster.Name, numWorkerMachines)
				By(logGenerated(input.MachineDeployment.MachineDeployment))
				By(logGenerated(input.MachineDeployment.InfraMachineTemplate))
				By(logGenerated(input.MachineDeployment.BootstrapConfigTemplate))
			}

			input.Cluster = cluster
			input.InfraCluster = infraCluster
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
			kubeadmControlPlaneGen = nil
		})

		Context("Single-node control plane with one worker node", func() {
			It("should create a single-node control plane with one worker node", func() {
				frameworkx.ControlPlaneCluster(input)
			})
		})

		Context("Single-node kubeadm control plane with one worker node", func() {
			BeforeEach(func() {
				loadBalancerGen = HAProxyLoadBalancerGenerator{}
				kubeadmControlPlaneGen = &KubeadmControlPlaneGenerator{}
			})
			It("should create a single-node kubeadm control plane with one worker node", func() {
				frameworkx.ControlPlaneCluster(input)
			})
		})
	})
})

func logGenerated(obj runtime.Object) string {
	return logGeneratedWithIndex(obj, -1)
}

func logGeneratedWithIndex(obj runtime.Object, index int) string {
	Expect(obj).ToNot(BeNil())
	metaObj, ok := obj.(metav1.Object)
	Expect(ok).To(BeTrue(), "obj must implement metav1.Object")
	if index >= 0 {
		return fmt.Sprintf("generated [%d] %s %s/%s", index, obj.GetObjectKind().GroupVersionKind(), metaObj.GetNamespace(), metaObj.GetName())
	}
	return fmt.Sprintf("generated %s %s/%s", obj.GetObjectKind().GroupVersionKind(), metaObj.GetNamespace(), metaObj.GetName())
}
