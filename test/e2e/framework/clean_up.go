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
	"time"

	. "github.com/onsi/gomega" //nolint:golint
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	. "sigs.k8s.io/cluster-api/test/framework" //nolint:golint
)

// CleanUpX deletes the cluster and waits for everything to be gone.
// Generally this test can be reused for many tests since the implementation is so simple.
func CleanUpX(input *CleanUpInput) {
	// TODO: check that all the things we expect have the cluster label or else they can't get deleted
	input.SetDefaults()
	ctx := context.Background()
	client, err := input.Management.GetClient()
	Expect(err).ToNot(HaveOccurred(), "stack: %+v", err)
	Expect(client.Delete(ctx, input.Cluster)).To(Succeed())

	Eventually(func() ([]clusterv1.Cluster, error) {
		clusterList := &clusterv1.ClusterList{}
		if err := client.List(ctx, clusterList); err != nil {
			return nil, err
		}
		return clusterList.Items, nil
	}, input.DeleteTimeout, 10*time.Second).Should(HaveLen(0))
}
