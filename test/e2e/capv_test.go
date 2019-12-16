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
	. "github.com/onsi/ginkgo"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"

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
				framework.CleanUp(&framework.CleanUpInput{
					Management: mgmt,
					Cluster:    cluster,
				})
				destroyVMsWithPrefix(cluster.Name)
			})

			It("should create a single-node control plane with one worker nodes", func() {
				frameworkx.SingleNodeControlPlane(input)
			})
		})
	})
})
