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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
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
			g.Expect(np1.Name()).To(Equal(name))

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
	g.Expect(np1.Name()).To(Equal(VDSNetworkProvider))

	np2, err := factory.ForCluster(ctx, clusterWithProvider(NSXVPCNetworkProvider))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(np2).To(BeIdenticalTo(np1))

	// ForObject ignores the object and returns the same static provider.
	np3, err := factory.ForObject(ctx, c, &vmwarev1.VSphereMachine{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(np3).To(BeIdenticalTo(np1))
}

func TestPerClusterNetworkProviderFactory_ForObject(t *testing.T) {
	const (
		namespace   = "test-ns"
		clusterName = "test-cluster"
		vsphereName = "test-cluster-infra"
	)

	machine := func(withLabel bool) *vmwarev1.VSphereMachine {
		labels := map[string]string{}
		if withLabel {
			labels[clusterv1.ClusterNameLabel] = clusterName
		}
		return &vmwarev1.VSphereMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "test-machine",
				Labels:    labels,
			},
		}
	}

	cluster := func(withInfraRef bool) *clusterv1.Cluster {
		c := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName},
		}
		if withInfraRef {
			c.Spec.InfrastructureRef = clusterv1.ContractVersionedObjectReference{
				APIGroup: vmwarev1.GroupVersion.Group,
				Kind:     "VSphereCluster",
				Name:     vsphereName,
			}
		}
		return c
	}

	vsphereCluster := func(provider string) *vmwarev1.VSphereCluster {
		return &vmwarev1.VSphereCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vsphereName},
			Spec:       vmwarev1.VSphereClusterSpec{Network: vmwarev1.Network{Provider: provider}},
		}
	}

	tests := []struct {
		name         string
		obj          *vmwarev1.VSphereMachine
		initObjs     []client.Object
		wantProvider string
		wantErr      bool
		wantErrIs    error
		errSubstring string
	}{
		{
			name:      "missing cluster-name label",
			obj:       machine(false),
			wantErr:   true,
			wantErrIs: ErrNoOwningCluster,
		},
		{
			name:         "Cluster not found",
			obj:          machine(true),
			wantErr:      true,
			errSubstring: "failed to get Cluster",
		},
		{
			name:         "Cluster has no infrastructureRef",
			obj:          machine(true),
			initObjs:     []client.Object{cluster(false)},
			wantErr:      true,
			errSubstring: "does not have a spec.infrastructureRef set",
		},
		{
			name:         "VSphereCluster not found",
			obj:          machine(true),
			initObjs:     []client.Object{cluster(true)},
			wantErr:      true,
			errSubstring: "failed to get VSphereCluster",
		},
		{
			name:      "spec.network.provider is empty",
			obj:       machine(true),
			initObjs:  []client.Object{cluster(true), vsphereCluster("")},
			wantErr:   true,
			wantErrIs: ErrNetworkProviderEmpty,
		},
		{
			name:         "spec.network.provider is set",
			obj:          machine(true),
			initObjs:     []client.Object{cluster(true), vsphereCluster(NSXVPCNetworkProvider)},
			wantProvider: NSXVPCNetworkProvider,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()

			scheme := runtime.NewScheme()
			g.Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
			g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
			g.Expect(vmwarev1.AddToScheme(scheme)).To(Succeed())
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.initObjs...).Build()

			factory, err := NewPerClusterNetworkProviderFactory(ctx, c)
			g.Expect(err).ToNot(HaveOccurred())

			np, err := factory.ForObject(ctx, c, tc.obj)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tc.wantErrIs != nil {
					g.Expect(errors.Is(err, tc.wantErrIs)).To(BeTrue())
				}
				if tc.errSubstring != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errSubstring))
				}
				g.Expect(np).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(np.Name()).To(Equal(tc.wantProvider))
		})
	}
}
