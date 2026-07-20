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
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestConvertNetworkProviderName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "legacy NSX",
			input:    legacyNSXNetworkProvider,
			expected: NSXNetworkProvider,
		},
		{
			name:     "legacy NSX-VPC",
			input:    legacyNSXVPCNetworkProvider,
			expected: NSXVPCNetworkProvider,
		},
		{
			name:     "legacy vsphere-network",
			input:    legacyVDSNetworkProvider,
			expected: VDSNetworkProvider,
		},
		{
			name:     "PascalCase name is unchanged",
			input:    NSXNetworkProvider,
			expected: NSXNetworkProvider,
		},
		{
			name:     "VPC is unchanged",
			input:    NSXVPCNetworkProvider,
			expected: NSXVPCNetworkProvider,
		},
		{
			name:     "VSphereDistributed is unchanged",
			input:    VDSNetworkProvider,
			expected: VDSNetworkProvider,
		},
		{
			name:     "unknown name is unchanged",
			input:    "unknown-provider",
			expected: "unknown-provider",
		},
		{
			name:     "empty name is unchanged",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(ConvertNetworkProviderName(tt.input)).To(Equal(tt.expected))
		})
	}
}

func TestOptionsDefaultsConvertsLegacyNetworkProvider(t *testing.T) {
	g := NewWithT(t)

	opts := &Options{
		KubeConfig:      &rest.Config{},
		NetworkProvider: legacyNSXVPCNetworkProvider,
	}
	opts.defaults()

	g.Expect(opts.NetworkProvider).To(Equal(NSXVPCNetworkProvider))
}

func TestOptionsDefaultsPassthroughPascalCaseNetworkProvider(t *testing.T) {
	g := NewWithT(t)

	opts := &Options{
		KubeConfig:      &rest.Config{},
		NetworkProvider: VDSNetworkProvider,
	}
	opts.defaults()

	g.Expect(opts.NetworkProvider).To(Equal(VDSNetworkProvider))
}

func TestGetNetworkProviderExternallyManaged(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c := fake.NewClientBuilder().Build()

	np, err := GetNetworkProvider(ctx, c, ExternallyManagedNetworkProvider)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(np).ToNot(BeNil())
	g.Expect(np.HasLoadBalancer()).To(BeFalse())
	g.Expect(np.SupportsVMReadinessProbe()).To(BeFalse())
	g.Expect(np.SupportsSupervisorService()).To(BeFalse())
}
