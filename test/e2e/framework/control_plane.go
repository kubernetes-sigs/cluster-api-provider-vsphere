/*
Copyright 2020 The Kubernetes Authors.

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

package framework

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo" //nolint:golint
	. "github.com/onsi/gomega" //nolint:golint
	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	. "sigs.k8s.io/cluster-api/test/framework" //nolint:golint
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	eventuallyInterval = 10 * time.Second
)

// ControlPlaneCluster create a cluster with a one or more control plane node
// and with n worker nodes.
// Assertions:
//  * The number of nodes in the created cluster will equal the number
//    of machines in the machine deployment plus the number of control
//    plane nodes.
func ControlPlaneCluster(input *ControlplaneClusterInput) {
	Expect(input).ToNot(BeNil())
	input.SetDefaults()
	Expect(input.Management).ToNot(BeNil())
	Expect(len(input.Nodes)).To(BeNumerically(">=", 1), "one or more control plane nodes is required")

	mgmtClient, err := input.Management.GetClient()
	Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()

	By("creating an InfrastructureCluster resource")
	Expect(mgmtClient.Create(ctx, input.InfraCluster)).To(Succeed())

	// This call happens in an eventually because of a race condition with the
	// webhook server. If the latter isn't fully online then this call will
	// fail.
	By("creating a Cluster resource linked to the InfrastructureCluster resource")
	Eventually(func() error {
		return mgmtClient.Create(ctx, input.Cluster)
	}, input.CreateTimeout, eventuallyInterval).Should(Succeed())

	// Create the related resources.
	By("creating related resources")
	for _, obj := range input.RelatedResources {
		By(fmt.Sprintf("creating a/an %s resource", obj.GetObjectKind().GroupVersionKind()))
		Eventually(func() error {
			return mgmtClient.Create(ctx, obj)
		}, input.CreateTimeout, eventuallyInterval).Should(Succeed())
	}

	// expectedNumberOfNodes is the number of nodes that should be deployed to
	// the cluster. This is the number of control plane nodes and the number of
	// replicas defined for a possible MachineDeployment.
	expectedNumberOfNodes := len(input.Nodes)

	// Create the control plane machines.
	for i, node := range input.Nodes {
		By(fmt.Sprintf("creating control plane resource %d: InfrastructureMachine", i+1))
		Expect(mgmtClient.Create(ctx, node.InfraMachine)).To(Succeed())

		By(fmt.Sprintf("creating control plane resource %d: BootstrapConfig", i+1))
		Expect(mgmtClient.Create(ctx, node.BootstrapConfig)).To(Succeed())

		By(fmt.Sprintf("creating control plane resource %d: Machine", i+1))
		Expect(mgmtClient.Create(ctx, node.Machine)).To(Succeed())

		// If this is the first node then block until the control plane is
		// initialized.
		//
		// While it's possible to store cluster.Status.ControlPlaneInitialized
		// and check that instead of the index (i == 0), the current design is
		// intentional. We are asserting that the *first* node in the list
		// *must* initialize the control plane. If it does not, then we *must*
		// fail.
		if i == 0 {
			By("waiting for the control plane to be initialized")
			clusterKey := client.ObjectKey{
				Namespace: input.Cluster.GetNamespace(),
				Name:      input.Cluster.GetName(),
			}
			Eventually(func() (bool, error) {
				cluster := &clusterv1.Cluster{}
				if err := mgmtClient.Get(ctx, clusterKey, cluster); err != nil {
					return false, err
				}
				return cluster.Status.ControlPlaneInitialized, nil
			}, input.CreateTimeout, eventuallyInterval).Should(BeTrue())
		}
	}

	// Create the machine deployment if the replica count >0.
	if machineDeployment := input.MachineDeployment.MachineDeployment; machineDeployment != nil {
		if replicas := machineDeployment.Spec.Replicas; replicas != nil && *replicas > 0 {
			expectedNumberOfNodes += int(*replicas)

			By("creating a core MachineDeployment resource")
			Expect(mgmtClient.Create(ctx, machineDeployment)).To(Succeed())

			By("creating a BootstrapConfigTemplate resource")
			Expect(mgmtClient.Create(ctx, input.MachineDeployment.BootstrapConfigTemplate)).To(Succeed())

			By("creating an InfrastructureMachineTemplate resource")
			Expect(mgmtClient.Create(ctx, input.MachineDeployment.InfraMachineTemplate)).To(Succeed())
		}
	}

	By("waiting for the workload nodes to exist")
	Eventually(func() ([]v1.Node, error) {
		workloadClient, err := input.Management.GetWorkloadClient(ctx, input.Cluster.Namespace, input.Cluster.Name)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get workload client")
		}
		nodeList := v1.NodeList{}
		if err := workloadClient.List(ctx, &nodeList); err != nil {
			return nil, err
		}
		return nodeList.Items, nil
	}, input.CreateTimeout, eventuallyInterval).Should(HaveLen(expectedNumberOfNodes))
}
