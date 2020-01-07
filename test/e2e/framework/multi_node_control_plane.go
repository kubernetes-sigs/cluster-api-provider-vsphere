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

package framework

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo" //nolint:golint
	. "github.com/onsi/gomega" //nolint:golint
	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	. "sigs.k8s.io/cluster-api/test/framework" //nolint:golint
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MultiNodeControlPlaneInput defines the necessary dependencies to run a
// multi-node control plane.
type MultiNodeControlPlaneInput struct {
	Management                  ManagementCluster
	Cluster                     *clusterv1.Cluster
	InfraCluster                runtime.Object
	PrimaryControlPlaneNode     Node
	AdditionalControlPlaneNodes []Node
	MachineDeployment           MachineDeployment
	RelatedResources            []runtime.Object
	CreateTimeout               time.Duration
}

// SetDefaults defaults the struct fields if necessary.
func (m *MultiNodeControlPlaneInput) SetDefaults() {
	if m.CreateTimeout == 0 {
		m.CreateTimeout = 10 * time.Minute
	}
}

// MultiNodeControlPlane create a cluster with a one or more control plane node
// and with n worker nodes.
// Assertions:
//  * The number of nodes in the created cluster will equal the number
//    of machines in the machine deployment plus the number of control
//    plane nodes.
func MultiNodeControlPlane(input *MultiNodeControlPlaneInput) {
	Expect(input).ToNot(BeNil())
	input.SetDefaults()
	Expect(input.Management).ToNot(BeNil())

	mgmtClient, err := input.Management.GetClient()
	Expect(err).ToNot(HaveOccurred(), "stack: %+v", err)

	ctx := context.Background()

	// Create the related resources.
	By("creating related resources")
	for _, obj := range input.RelatedResources {
		By(fmt.Sprintf("creating a/an %s resource", obj.GetObjectKind().GroupVersionKind()))
		Eventually(func() error {
			return mgmtClient.Create(ctx, obj)
		}, input.CreateTimeout, 10*time.Second).Should(BeNil())
	}

	By("creating an InfrastructureCluster resource")
	Expect(mgmtClient.Create(ctx, input.InfraCluster)).To(Succeed())

	// This call happens in an eventually because of a race condition with the
	// webhook server. If the latter isn't fully online then this call will
	// fail.
	By("creating a Cluster resource linked to the InfrastructureCluster resource")
	Eventually(func() error {
		return mgmtClient.Create(ctx, input.Cluster)
	}, input.CreateTimeout, 10*time.Second).Should(BeNil())

	// expectedNumberOfNodes is the number of nodes that should be deployed to
	// the cluster. This is the control plane node and the number of replicas
	// defined for a possible MachineDeployment.
	expectedNumberOfNodes := 1

	// Create the control plane machine.
	By("creating the primary InfrastructureMachine resource")
	Expect(mgmtClient.Create(ctx, input.PrimaryControlPlaneNode.InfraMachine)).To(Succeed())

	By("creating the primary BootstrapConfig resource")
	Expect(mgmtClient.Create(ctx, input.PrimaryControlPlaneNode.BootstrapConfig)).To(Succeed())

	By("creating the primary core Machine resource with a linked InfrastructureMachine and BootstrapConfig")
	Expect(mgmtClient.Create(ctx, input.PrimaryControlPlaneNode.Machine)).To(Succeed())

	for _, node := range input.AdditionalControlPlaneNodes {
		expectedNumberOfNodes++

		By("creating additional InfrastructureMachine resource")
		Expect(mgmtClient.Create(ctx, node.InfraMachine)).To(Succeed())

		By("creating additional BootstrapConfig resource")
		Expect(mgmtClient.Create(ctx, node.BootstrapConfig)).To(Succeed())

		By("creating additional Machine resource with a linked InfrastructureMachine and BootstrapConfig")
		Expect(mgmtClient.Create(ctx, node.Machine)).To(Succeed())
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

	By("waiting for cluster to enter the provisioned phase")
	Eventually(func() (string, error) {
		cluster := &clusterv1.Cluster{}
		key := client.ObjectKey{
			Namespace: input.Cluster.GetNamespace(),
			Name:      input.Cluster.GetName(),
		}
		if err := mgmtClient.Get(ctx, key, cluster); err != nil {
			return "", err
		}
		return cluster.Status.Phase, nil
	}, input.CreateTimeout, 10*time.Second).Should(Equal(string(clusterv1.ClusterPhaseProvisioned)))

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
	}, input.CreateTimeout, 10*time.Second).Should(HaveLen(expectedNumberOfNodes))
}
