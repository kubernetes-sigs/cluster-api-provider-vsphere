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
			clusterGen           = ClusterGenerator{}
			nodeGen              = &NodeGenerator{}
			machineDeploymentGen = &MachineDeploymentGenerator{}
		)

		BeforeEach(func() {
			randomUUID := uuid.New()
			hash7 := fmt.Sprintf("%x", sha1.Sum(randomUUID[:]))[:7] //nolint:gosec
			clusterName = fmt.Sprintf("test-%s", hash7)
		})

		AfterEach(func() {

			// This label selector may be used to list resources related to
			// the current Clsuter.
			clusterLabelSelector := client.MatchingLabels{clusterv1.ClusterLabelName: clusterName}

			// The amount of time to wait when verify that eventually some
			// resource no longer exists.
			timeout := 10 * time.Minute

			By("asserting all VSphereVM resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.VSphereVM, error) {
				list := &infrav1.VSphereVMList{}
				if err := mgmt.Client.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, timeout, 10*time.Second).Should(HaveLen(0))

			By("asserting all VSphereMachine resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.VSphereMachine, error) {
				list := &infrav1.VSphereMachineList{}
				if err := mgmt.Client.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, timeout, 10*time.Second).Should(HaveLen(0))

			By("asserting all VSphereCluster resources related to this test are eventually removed")
			Eventually(func() ([]infrav1.VSphereCluster, error) {
				list := &infrav1.VSphereClusterList{}
				if err := mgmt.Client.List(ctx, list, clusterLabelSelector); err != nil {
					return nil, err
				}
				return list.Items, nil
			}, timeout, 10*time.Second).Should(HaveLen(0))

			destroyVMsWithPrefix(clusterName)
		})

		Context("Single-node control plane with one worker nodes", func() {
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

			It("should create a single-node control plane with one worker nodes", func() {
				frameworkx.SingleNodeControlPlane(input)
			})
		})
	})
})
