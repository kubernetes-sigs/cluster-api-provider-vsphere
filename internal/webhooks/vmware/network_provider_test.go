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

package vmware

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

func TestResolveNetworkProvider(t *testing.T) {
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
		gateEnabled  bool
		obj          *vmwarev1.VSphereMachine
		initObjs     []client.Object
		wantProvider string
		wantErr      bool
		errSubstring string
	}{
		{
			name:         "gate disabled returns the static provider without loading the cluster",
			gateEnabled:  false,
			obj:          machine(false),
			wantProvider: manager.VDSNetworkProvider,
		},
		{
			name:         "gate enabled and missing cluster-name label",
			gateEnabled:  true,
			obj:          machine(false),
			wantErr:      true,
			errSubstring: "cannot resolve the owning Cluster",
		},
		{
			name:         "gate enabled and Cluster not found",
			gateEnabled:  true,
			obj:          machine(true),
			wantErr:      true,
			errSubstring: "failed to get Cluster",
		},
		{
			name:         "gate enabled and Cluster has no infrastructureRef",
			gateEnabled:  true,
			obj:          machine(true),
			initObjs:     []client.Object{cluster(false)},
			wantErr:      true,
			errSubstring: "does not have a spec.infrastructureRef set",
		},
		{
			name:         "gate enabled and VSphereCluster not found",
			gateEnabled:  true,
			obj:          machine(true),
			initObjs:     []client.Object{cluster(true)},
			wantErr:      true,
			errSubstring: "failed to get VSphereCluster",
		},
		{
			name:         "gate enabled and spec.network.provider is empty",
			gateEnabled:  true,
			obj:          machine(true),
			initObjs:     []client.Object{cluster(true), vsphereCluster("")},
			wantErr:      true,
			errSubstring: "is empty",
		},
		{
			name:         "gate enabled and spec.network.provider is set",
			gateEnabled:  true,
			obj:          machine(true),
			initObjs:     []client.Object{cluster(true), vsphereCluster(manager.NSXVPCNetworkProvider)},
			wantProvider: manager.NSXVPCNetworkProvider,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			setupFeatureGates(t, map[string]bool{"ClusterNetworkProvider": tc.gateEnabled})

			c := fake.NewControllerManagerContext(tc.initObjs...).Client
			provider, err := resolveNetworkProvider(context.Background(), c, manager.VDSNetworkProvider, tc.obj)

			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tc.errSubstring != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errSubstring))
				}
				g.Expect(provider).To(BeEmpty())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(provider).To(Equal(tc.wantProvider))
		})
	}
}
