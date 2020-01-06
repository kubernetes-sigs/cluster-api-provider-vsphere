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
	"bytes"
	"context"

	. "github.com/onsi/ginkgo" //nolint:golint
	. "github.com/onsi/gomega" //nolint:golint

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	. "sigs.k8s.io/cluster-api/test/framework" //nolint:golint
	"sigs.k8s.io/cluster-api/test/framework/generators"
	"sigs.k8s.io/cluster-api/test/framework/management/kind"

	"sigs.k8s.io/cluster-api-provider-vsphere/test/e2e/docker"
)

// InitManagementClusterInput is the input used to initialize the management
// cluster.
type InitManagementClusterInput struct {
	// Name is the name of the management cluster.
	//
	// Defaults to mgmt.
	Name string

	// CapiImage is the image used to install the core CAPI components.
	//
	// Defaults to Flags.CapiImage.
	CapiImage string

	// CapiGitRef is the git reference used to run remote kustomize against the
	// CAPI repository.
	//
	// Defaults to Flags.CapiGitRef.
	CapiGitRef string

	// CertGitRef is the git reference used to run remote kustomize against the
	// cert-manager repository.
	//
	// Defaults to Flags.CertGitRef.
	CertGitRef string

	// InfraImage is the image used to install the infrastructure provider
	// components.
	//
	// Defaults to Flags.InfraImage.
	InfraImage string

	// InfraNamespace is the namespace in which the infrastructure provider
	// components are located.
	//
	// Defaults to Flags.InfraNamespace.
	InfraNamespace string

	// InfraComponentGenerators is a list of component generators required by
	// the infrastructure provider.
	InfraComponentGenerators []ComponentGenerator

	// Scheme enables adding the infrastructure provider scheme or additional
	// schemes to the test suite.
	Scheme *runtime.Scheme
}

// SetDefaults assigns default values to the object.
func (i *InitManagementClusterInput) SetDefaults() {
	if i.Name == "" {
		i.Name = Flags.ManagementClusterName
	}
	if i.Scheme == nil {
		i.Scheme = runtime.NewScheme()
	}
	if i.CapiImage == "" {
		i.CapiImage = Flags.CapiImage
	}
	if i.CapiGitRef == "" {
		i.CapiGitRef = Flags.CapiGitRef
	}
	if i.CertGitRef == "" {
		i.CertGitRef = Flags.CertGitRef
	}
	if i.InfraImage == "" {
		i.InfraImage = Flags.InfraImage
	}
	if i.InfraNamespace == "" {
		i.InfraNamespace = Flags.InfraNamespace
	}
}

// InitManagementCluster initializes the management cluster.
func InitManagementCluster(ctx context.Context, input *InitManagementClusterInput) *kind.Cluster {
	By("initializing the management cluster")
	Expect(input).ToNot(BeNil())
	input.SetDefaults()

	Expect(input.InfraImage).ToNot(BeEmpty(), "input.InfraImage is required")
	Expect(input.InfraNamespace).ToNot(BeEmpty(), "input.InfraNamespace is required")

	By("pulling the core capi image")
	Expect(docker.Pull(ctx, input.CapiImage)).To(Succeed())

	// Set up the provider component generators based on master
	coreComponents := &componentGeneratorWrapper{parent: &generators.ClusterAPI{GitRef: input.CapiGitRef}}
	coreComponents.Manifests(ctx)

	// Set up the certificate manager.
	certComponents := &generators.CertManager{ReleaseVersion: input.CertGitRef}

	// Add the core CAPI schemes.
	Expect(corev1.AddToScheme(input.Scheme)).To(Succeed())
	Expect(clusterv1.AddToScheme(input.Scheme)).To(Succeed())
	Expect(bootstrapv1.AddToScheme(input.Scheme)).To(Succeed())

	// Create the management cluster
	By("creating the management cluster")
	managementCluster, err := kind.NewCluster(ctx, input.Name, input.Scheme, input.InfraImage, input.CapiImage)
	Expect(err).ToNot(HaveOccurred())
	Expect(managementCluster).ToNot(BeNil())

	// Install all components.

	// The cert-manager components are installed first as subsequent CRDs may
	// depend upon the cert-manager CRDs.
	By("installing the certificate manager components")
	InstallComponents(ctx, managementCluster, certComponents)

	// Wait for cert manager service.
	// TODO: consider finding a way to make this service name dynamic.
	By("waiting for the certificate manager service")
	WaitForAPIServiceAvailable(ctx, managementCluster, "v1beta1.webhook.cert-manager.io")

	// Install the remaining components.
	By("installing the remaining components")
	componentGenerators := append([]ComponentGenerator{coreComponents}, input.InfraComponentGenerators...)
	InstallComponents(ctx, managementCluster, componentGenerators...)

	// Wait for the pods to be ready before returning control to the caller.
	By("waiting for the pods to be ready")
	WaitForPodsReadyInNamespace(ctx, managementCluster, "capi-system")
	WaitForPodsReadyInNamespace(ctx, managementCluster, "cert-manager")
	WaitForPodsReadyInNamespace(ctx, managementCluster, input.InfraNamespace)

	return managementCluster
}

type componentGeneratorWrapper struct {
	parent ComponentGenerator
}

func (g *componentGeneratorWrapper) GetName() string {
	return g.parent.GetName()
}

func (g *componentGeneratorWrapper) Manifests(ctx context.Context) ([]byte, error) {
	buf, err := g.parent.Manifests(ctx)
	if err != nil {
		return nil, err
	}
	return bytes.Replace(buf, []byte("gcr.io/k8s-staging-cluster-api/cluster-api-controller:master"), []byte(Flags.CapiImage), -1), nil
}
