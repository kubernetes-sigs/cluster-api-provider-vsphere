/*
Copyright 2022 The Kubernetes Authors.

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

package vmoperator

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	capi_util "sigs.k8s.io/cluster-api/util"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func TestRPService(t *testing.T) {
	clusterName := "test-cluster"
	vsphereClusterName := fmt.Sprintf("%s-%s", clusterName, capi_util.RandomString((6)))
	cluster := util.CreateCluster(clusterName)
	vsphereCluster := util.CreateVSphereCluster(vsphereClusterName)
	clusterCtx, controllerCtx := util.CreateClusterContext(cluster, vsphereCluster)
	ctx := context.Background()
	rpService := RPService{
		Client: controllerCtx.Client,
	}

	t.Run("Creates Resource Policy using the cluster name", func(t *testing.T) {
		g := NewWithT(t)
		name, err := rpService.ReconcileResourcePolicy(ctx, clusterCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(name).To(Equal(clusterName))

		resourcePolicy, err := rpService.getVirtualMachineSetResourcePolicy(ctx, clusterCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resourcePolicy.Spec.ResourcePool.Name).To(Equal(clusterName))
		g.Expect(resourcePolicy.Spec.Folder.Name).To(Equal(clusterName))
	})
}
