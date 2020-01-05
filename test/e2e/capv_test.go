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
	"crypto/sha1" //nolint:gosec
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo" //nolint:golint
	. "github.com/onsi/gomega" //nolint:golint
	corev1 "k8s.io/api/core/v1"
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
			clusterLabelSelector client.MatchingLabels
			clusterGen           ClusterGenerator
			haproxyGen           HAProxyLoadBalancerGenerator
			nodeGen              = &NodeGenerator{}
			machineDeploymentGen = &MachineDeploymentGenerator{}
			pollTimeout          = 10 * time.Minute
			pollInterval         = 10 * time.Second
		)

		BeforeEach(func() {
			randomUUID := uuid.New()
			hash7 := fmt.Sprintf("%x", sha1.Sum(randomUUID[:]))[:7] //nolint:gosec
			clusterName = fmt.Sprintf("test-%s", hash7)
			clusterLabelSelector = client.MatchingLabels{clusterv1.ClusterLabelName: clusterName}
		})

		AfterEach(func() {
			By("asserting all VSphereVM resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.VSphereVM, error) {
				list := &infrav1.VSphereVMList{}
				if err := mgmt.Client.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, pollTimeout, pollInterval).Should(HaveLen(0))

			By("asserting all VSphereMachine resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.VSphereMachine, error) {
				list := &infrav1.VSphereMachineList{}
				if err := mgmt.Client.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, pollTimeout, pollInterval).Should(HaveLen(0))

			By("asserting all VSphereCluster resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.VSphereCluster, error) {
				list := &infrav1.VSphereClusterList{}
				if err := mgmt.Client.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, pollTimeout, pollInterval).Should(HaveLen(0))

			By("asserting all HAProxyLoadBalancer resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.HAProxyLoadBalancer, error) {
				list := &infrav1.HAProxyLoadBalancerList{}
				if err := mgmt.Client.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, pollTimeout, pollInterval).Should(HaveLen(0))

			destroyVMsWithPrefix(clusterName)
		})

		Context("Single-node control plane with one worker node", func() {
			var (
				cluster           *clusterv1.Cluster
				infraCluster      *infrav1.VSphereCluster
				controlPlaneNode  framework.Node
				machineDeployment frameworkx.MachineDeployment
				input             *frameworkx.SingleNodeControlPlaneInput
			)

			BeforeEach(func() {
				cluster, infraCluster = clusterGen.Generate("default", clusterName)
				controlPlaneNode = nodeGen.Generate(cluster.Namespace, cluster.Name)
				machineDeployment = machineDeploymentGen.Generate(cluster.Namespace, cluster.Name, 1)
				input = &frameworkx.SingleNodeControlPlaneInput{
					Management:        mgmt,
					Cluster:           cluster,
					InfraCluster:      infraCluster,
					ControlPlaneNode:  controlPlaneNode,
					MachineDeployment: machineDeployment,
					CreateTimeout:     10 * time.Minute,
				}
			})

			AfterEach(func() {
				By("cleaning up e2e resources")
				frameworkx.CleanUpX(&framework.CleanUpInput{
					Management: mgmt,
					Cluster:    cluster,
				})
			})

			It("should create a single-node control plane with one worker node", func() {
				frameworkx.SingleNodeControlPlane(input)
			})
		})

		Context("Two-node control plane with one worker node", func() {
			var (
				cluster                     *clusterv1.Cluster
				infraCluster                *infrav1.VSphereCluster
				haproxyLB                   *infrav1.HAProxyLoadBalancer
				primaryControlPlaneNode     framework.Node
				additionalControlPlaneNodes []framework.Node
				machineDeployment           frameworkx.MachineDeployment
				input                       *frameworkx.MultiNodeControlPlaneInput
			)

			BeforeEach(func() {
				cluster, infraCluster = clusterGen.Generate("default", clusterName)
				haproxyLB = haproxyGen.Generate(cluster.Namespace, cluster.Name)
				infraCluster.Spec.LoadBalancerRef = &corev1.ObjectReference{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       framework.TypeToKind(haproxyLB),
					Namespace:  haproxyLB.GetNamespace(),
					Name:       haproxyLB.GetName(),
				}
				primaryControlPlaneNode = nodeGen.Generate(cluster.Namespace, cluster.Name)
				additionalControlPlaneNodes = []framework.Node{
					nodeGen.Generate(cluster.Namespace, cluster.Name),
				}
				machineDeployment = machineDeploymentGen.Generate(cluster.Namespace, cluster.Name, 1)
				input = &frameworkx.MultiNodeControlPlaneInput{
					Management:                  mgmt,
					Cluster:                     cluster,
					InfraCluster:                infraCluster,
					PrimaryControlPlaneNode:     primaryControlPlaneNode,
					AdditionalControlPlaneNodes: additionalControlPlaneNodes,
					MachineDeployment:           machineDeployment,
					RelatedResources:            []runtime.Object{haproxyLB},
					CreateTimeout:               10 * time.Minute,
				}
			})

			AfterEach(func() {
				By("cleaning up e2e resources")
				frameworkx.CleanUpX(&framework.CleanUpInput{
					Management: mgmt,
					Cluster:    cluster,
				})
			})

			It("should create a two-node control plane with one worker node", func() {
				frameworkx.MultiNodeControlPlane(input)
			})
		})
	})
})
