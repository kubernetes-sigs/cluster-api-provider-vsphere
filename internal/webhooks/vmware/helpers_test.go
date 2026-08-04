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

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

// newTestNetworkProviderFactory returns a NetworkProviderFactory for webhook unit tests.
// When the ClusterNetworkProvider gate is enabled it returns a per-cluster factory;
// otherwise it returns a static factory for the given provider name.
func newTestNetworkProviderFactory(t *testing.T, networkProvider string) manager.NetworkProviderFactory {
	t.Helper()
	ctx := context.Background()
	c := fake.NewClientBuilder().Build()

	var (
		factory manager.NetworkProviderFactory
		err     error
	)
	if feature.Gates.Enabled(feature.ClusterNetworkProvider) {
		factory, err = manager.NewPerClusterNetworkProviderFactory(ctx, c)
	} else {
		factory, err = manager.NewStaticNetworkProviderFactory(ctx, c, networkProvider)
	}
	if err != nil {
		t.Fatalf("failed to create network provider factory: %v", err)
	}
	return factory
}

// newTestStaticNetworkProviderFactory returns a static NetworkProviderFactory for webhook unit tests.
func newTestStaticNetworkProviderFactory(t *testing.T, networkProvider string) manager.NetworkProviderFactory {
	t.Helper()
	factory, err := manager.NewStaticNetworkProviderFactory(context.Background(), fake.NewClientBuilder().Build(), networkProvider)
	if err != nil {
		t.Fatalf("failed to create static network provider factory: %v", err)
	}
	return factory
}
