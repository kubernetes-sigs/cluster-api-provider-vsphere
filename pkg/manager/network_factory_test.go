/*
Copyright 2026 The Kubernetes Authors.

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

package manager

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
)

func clusterWithProvider(provider string) *vmwarev1.VSphereCluster {
	return &vmwarev1.VSphereCluster{
		Spec: vmwarev1.VSphereClusterSpec{
			Network: vmwarev1.Network{
				Provider: provider,
			},
		},
	}
}

func TestPerClusterNetworkProviderFactory(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c := fake.NewClientBuilder().Build()

	factory, err := NewPerClusterNetworkProviderFactory(ctx, c)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("empty provider returns ErrNetworkProviderEmpty", func(t *testing.T) {
		g := NewWithT(t)
		np, err := factory.ForCluster(ctx, clusterWithProvider(""))
		g.Expect(err).To(MatchError(ErrNetworkProviderEmpty))
		g.Expect(errors.Is(err, ErrNetworkProviderEmpty)).To(BeTrue())
		g.Expect(np).To(BeNil())
	})

	t.Run("known providers return a singleton", func(t *testing.T) {
		g := NewWithT(t)
		for _, name := range []string{VDSNetworkProvider, NSXNetworkProvider, NSXVPCNetworkProvider} {
			np1, err := factory.ForCluster(ctx, clusterWithProvider(name))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(np1).ToNot(BeNil())

			np2, err := factory.ForCluster(ctx, clusterWithProvider(name))
			g.Expect(err).ToNot(HaveOccurred())
			// The same singleton instance should be returned on every call.
			g.Expect(np2).To(BeIdenticalTo(np1))
		}
	})

	t.Run("unknown provider returns an error", func(t *testing.T) {
		g := NewWithT(t)
		np, err := factory.ForCluster(ctx, clusterWithProvider("does-not-exist"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("unknown network provider"))
		g.Expect(np).To(BeNil())
	})

	t.Run("ExternallyManaged is rejected when gate is disabled", func(t *testing.T) {
		g := NewWithT(t)
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.ExternallyManagedProvider, false)
		factory, err := NewPerClusterNetworkProviderFactory(ctx, c)
		g.Expect(err).ToNot(HaveOccurred())
		np, err := factory.ForCluster(ctx, clusterWithProvider(ExternallyManagedNetworkProvider))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("unknown network provider"))
		g.Expect(np).To(BeNil())
	})

	t.Run("ExternallyManaged is registered when gate is enabled", func(t *testing.T) {
		g := NewWithT(t)
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.ExternallyManagedProvider, true)
		factory, err := NewPerClusterNetworkProviderFactory(ctx, c)
		g.Expect(err).ToNot(HaveOccurred())
		np, err := factory.ForCluster(ctx, clusterWithProvider(ExternallyManagedNetworkProvider))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(np).ToNot(BeNil())
		g.Expect(np.SupportsSupervisorService()).To(BeFalse())
	})
}

func TestStaticNetworkProviderFactory(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c := fake.NewClientBuilder().Build()

	factory, err := NewStaticNetworkProviderFactory(ctx, c, VDSNetworkProvider)
	g.Expect(err).ToNot(HaveOccurred())

	// The static factory always returns the flag provider, regardless of the
	// cluster's spec.network.provider value (including empty).
	np1, err := factory.ForCluster(ctx, clusterWithProvider(""))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(np1).ToNot(BeNil())

	np2, err := factory.ForCluster(ctx, clusterWithProvider(NSXVPCNetworkProvider))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(np2).To(BeIdenticalTo(np1))
}
