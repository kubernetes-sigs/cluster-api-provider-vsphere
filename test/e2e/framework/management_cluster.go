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

	. "github.com/onsi/ginkgo" //nolint:golint
	. "github.com/onsi/gomega" //nolint:golint

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	. "sigs.k8s.io/cluster-api/test/framework" //nolint:golint
	"sigs.k8s.io/cluster-api/test/framework/management/kind"
)

// InitManagementClusterInput is the input used to initialize the management
// cluster.
type InitManagementClusterInput struct {
	Config

	// ComponentGenerators is a list of objects that generate component YAML to
	// apply to the management cluster.
	ComponentGenerators []ComponentGenerator

	// Scheme enables adding the infrastructure provider scheme or additional
	// schemes to the test suite.
	Scheme *runtime.Scheme
}

// Defaults assigns default values to the object.
func (i *InitManagementClusterInput) Defaults() {
	i.Config.Defaults()
	if i.Scheme == nil {
		i.Scheme = runtime.NewScheme()
	}
}

// InitManagementCluster initializes the management cluster.
func InitManagementCluster(ctx context.Context, input *InitManagementClusterInput) *kind.Cluster {
	By("initializing the management cluster")
	Expect(input).ToNot(BeNil())
	input.Defaults()

	// Add the core schemes.
	Expect(corev1.AddToScheme(input.Scheme)).To(Succeed())

	// Add the core CAPI scheme.
	Expect(clusterv1.AddToScheme(input.Scheme)).To(Succeed())

	// Add the kubeadm bootstrapper scheme.
	Expect(bootstrapv1.AddToScheme(input.Scheme)).To(Succeed())

	// Add the kubeadm controlplane scheme.
	Expect(controlplanev1.AddToScheme(input.Scheme)).To(Succeed())

	// Create the management cluster
	By("creating the management cluster")
	managementCluster, err := kind.NewCluster(ctx, input.ManagementClusterName, input.Scheme)
	Expect(err).ToNot(HaveOccurred())
	Expect(managementCluster).ToNot(BeNil())

	// Load the images.
	for _, image := range input.Images {
		By(fmt.Sprintf("loading %s", image))
		Expect(managementCluster.LoadImage(ctx, image)).To(Succeed())
	}

	// Install the YAML from the component generators.
	for _, componentGenerator := range input.ComponentGenerators {
		InstallComponents(ctx, managementCluster, componentGenerator)
	}

	// Install all components.
	for _, component := range input.Components {
		for _, source := range component.Sources {
			name := component.Name
			if source.Name != "" {
				name = fmt.Sprintf("%s/%s", component.Name, source.Name)
			}
			source.Name = name
			InstallComponents(ctx, managementCluster, ComponentGeneratorForComponentSource(source))
		}
		for _, waiter := range component.Waiters {
			switch waiter.Type {
			case PodsWaiter:
				WaitForPodsReadyInNamespace(ctx, managementCluster, waiter.Value)
			case ServiceWaiter:
				WaitForAPIServiceAvailable(ctx, managementCluster, waiter.Value)
			}
		}
	}

	return managementCluster
}
