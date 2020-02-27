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

	. "github.com/onsi/ginkgo" // nolint:golint,stylecheck
	. "github.com/onsi/gomega" // nolint:golint,stylecheck
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
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
func ControlPlaneCluster(input *framework.ControlplaneClusterInput) {
	Expect(input).ToNot(BeNil())
	input.SetDefaults()
	Expect(input.Management).ToNot(BeNil())
	// expectedNumberOfNodes is the number of nodes that should be deployed to
	// the cluster. This is the number of control plane
	// nodes (either through input.Nodes or kubeadmControlPlane) and the number of
	// replicas defined for a possible MachineDeployment.
	expectedNumberOfNodes := len(input.Nodes)

	if input.ControlPlane != nil {
		expectedNumberOfNodes = int(*input.ControlPlane.Spec.Replicas)
	}
	Expect(expectedNumberOfNodes).To(BeNumerically(">=", 1), "one or more control plane nodes is required")

	mgmtClient, err := input.Management.GetClient()
	Expect(err).ToNot(HaveOccurred())
	Expect(mgmtClient).ToNot(BeNil())
	ctx := context.Background()

	By(logCreatingBy(input.InfraCluster))
	Expect(mgmtClient.Create(ctx, input.InfraCluster)).To(Succeed())

	// This call happens in an eventually because of a race condition with the
	// webhook server. If the latter isn't fully online then this call will
	// fail.
	By(logCreatingBy(input.Cluster))
	Eventually(func() error {
		return mgmtClient.Create(ctx, input.Cluster)
	}, input.CreateTimeout, eventuallyInterval).Should(Succeed())

	// Create the related resources.
	if len(input.RelatedResources) > 0 {
		By("creating related resources")
		for _, obj := range input.RelatedResources {
			obj := obj
			By(logCreatingBy(obj))
			Eventually(func() error {
				return mgmtClient.Create(ctx, obj)
			}, input.CreateTimeout, eventuallyInterval).Should(Succeed())
		}
	}

	if input.ControlPlane != nil {
		By("creating kubeadm control plane resources")
		Expect(input.MachineTemplate).ToNot(BeNil(), "input.ControlPlane is not-nil")

		By(logCreatingBy(input.MachineTemplate))
		Expect(mgmtClient.Create(ctx, input.MachineTemplate)).To(Succeed())

		By(logCreatingBy(input.ControlPlane))
		Expect(mgmtClient.Create(ctx, input.ControlPlane)).To(Succeed())

		waitForControlPlaneInitialized(ctx, input, mgmtClient)
	} else {
		By("creating control plane resources")
		for i, node := range input.Nodes {
			By(logCreatingWithIndex(node.InfraMachine, i))
			Expect(mgmtClient.Create(ctx, node.InfraMachine)).To(Succeed())

			By(logCreatingWithIndex(node.BootstrapConfig, i))
			Expect(mgmtClient.Create(ctx, node.BootstrapConfig)).To(Succeed())

			By(logCreatingWithIndex(node.Machine, i))
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
				waitForControlPlaneInitialized(ctx, input, mgmtClient)
			}
		}
	}

	// Create the machine deployment if the replica count >0.
	if machineDeployment := input.MachineDeployment.MachineDeployment; machineDeployment != nil {
		if replicas := machineDeployment.Spec.Replicas; replicas != nil && *replicas > 0 {
			By("creating machine deployment resources")
			expectedNumberOfNodes += int(*replicas)

			By(logCreatingBy(machineDeployment))
			Expect(mgmtClient.Create(ctx, machineDeployment)).To(Succeed())

			By(logCreatingBy(input.MachineDeployment.BootstrapConfigTemplate))
			Expect(mgmtClient.Create(ctx, input.MachineDeployment.BootstrapConfigTemplate)).To(Succeed())

			By(logCreatingBy(input.MachineDeployment.InfraMachineTemplate))
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

func waitForControlPlaneInitialized(ctx context.Context, input *framework.ControlplaneClusterInput, mgmtClient framework.Getter) {
	By("waiting for the control plane to be initialized")
	clusterKey := client.ObjectKey{
		Namespace: input.Cluster.GetNamespace(),
		Name:      input.Cluster.GetName(),
	}
	Eventually(func() (string, error) {
		cluster := &clusterv1.Cluster{}
		if err := mgmtClient.Get(ctx, clusterKey, cluster); err != nil {
			return "", err
		}
		return cluster.Status.Phase, nil
	}, input.CreateTimeout, eventuallyInterval).Should(Equal(string(clusterv1.ClusterPhaseProvisioned)))
}

func logCreatingBy(obj runtime.Object) string {
	return logCreatingWithIndex(obj, -1)
}

func logCreatingWithIndex(obj runtime.Object, index int) string {
	Expect(obj).ToNot(BeNil())
	metaObj, ok := obj.(metav1.Object)
	Expect(ok).To(BeTrue(), "obj must implement metav1.Object")
	if index >= 0 {
		return fmt.Sprintf("creating [%d] %s %s/%s", index, obj.GetObjectKind().GroupVersionKind(), metaObj.GetNamespace(), metaObj.GetName())
	}
	return fmt.Sprintf("creating %s %s/%s", obj.GetObjectKind().GroupVersionKind(), metaObj.GetNamespace(), metaObj.GetName())
}
