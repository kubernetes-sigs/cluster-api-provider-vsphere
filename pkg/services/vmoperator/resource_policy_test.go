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
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	capi_util "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func TestRPService(t *testing.T) {
	ctx := context.Background()

	t.Run("Creates Resource Policy with legacy worker module group when WorkerStrictAntiAffinity is disabled", func(t *testing.T) {
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.WorkerStrictAntiAffinity, false)

		clusterName := "test-cluster-v2-disabled"
		vsphereClusterName := fmt.Sprintf("%s-%s", clusterName, capi_util.RandomString(6))
		cluster := util.CreateCluster(clusterName)
		vsphereCluster := util.CreateVSphereCluster(vsphereClusterName)
		clusterCtx, controllerCtx := util.CreateClusterContext(cluster, vsphereCluster)
		rpService := RPService{
			Client: controllerCtx.Client,
		}

		g := NewWithT(t)
		err := rpService.ReconcileResourcePolicy(ctx, clusterCtx)
		g.Expect(err).NotTo(HaveOccurred())

		// NOTE: use vm-operator native types for testing (the reconciler uses the internal hub version).
		resourcePolicy := &vmoprv1alpha5.VirtualMachineSetResourcePolicy{}
		err = rpService.Client.Get(ctx, client.ObjectKey{
			Namespace: clusterCtx.Cluster.Namespace,
			Name:      clusterCtx.Cluster.Name,
		}, resourcePolicy)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resourcePolicy.Spec.ResourcePool.Name).To(Equal(clusterName))
		g.Expect(resourcePolicy.Spec.Folder).To(Equal(clusterName))
		g.Expect(resourcePolicy.Spec.ClusterModuleGroups).To(ConsistOf(
			ControlPlaneVMClusterModuleGroupName,
			getMachineDeploymentNameForCluster(clusterCtx.Cluster),
		))
	})

	t.Run("Creates Resource Policy with only control plane module group for greenfield cluster when WorkerStrictAntiAffinity is enabled", func(t *testing.T) {
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.WorkerStrictAntiAffinity, true)

		clusterName := "test-cluster-greenfield"
		vsphereClusterName := fmt.Sprintf("%s-%s", clusterName, capi_util.RandomString(6))
		cluster := util.CreateCluster(clusterName)
		vsphereCluster := util.CreateVSphereCluster(vsphereClusterName)
		clusterCtx, controllerCtx := util.CreateClusterContext(cluster, vsphereCluster)
		rpService := RPService{
			Client: controllerCtx.Client,
		}

		g := NewWithT(t)
		err := rpService.ReconcileResourcePolicy(ctx, clusterCtx)
		g.Expect(err).NotTo(HaveOccurred())

		resourcePolicy := &vmoprv1alpha5.VirtualMachineSetResourcePolicy{}
		err = rpService.Client.Get(ctx, client.ObjectKey{
			Namespace: clusterCtx.Cluster.Namespace,
			Name:      clusterCtx.Cluster.Name,
		}, resourcePolicy)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resourcePolicy.Spec.ResourcePool.Name).To(Equal(clusterName))
		g.Expect(resourcePolicy.Spec.Folder).To(Equal(clusterName))
		g.Expect(resourcePolicy.Spec.ClusterModuleGroups).To(ConsistOf(
			ControlPlaneVMClusterModuleGroupName,
		))
	})

	for _, tc := range []struct {
		name              string
		featureEnabled    bool
		existingWorkerCMG bool
	}{
		{
			name: "Restores legacy worker group when feature gate is disabled",
		},
		{
			name:              "Preserves legacy worker group when feature gate is enabled",
			featureEnabled:    true,
			existingWorkerCMG: true,
		},
		{
			name:           "Keeps worker group absent when feature gate is enabled",
			featureEnabled: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.WorkerStrictAntiAffinity, tc.featureEnabled)

			clusterName := "test-cluster-existing"
			vsphereClusterName := fmt.Sprintf("%s-%s", clusterName, capi_util.RandomString(6))
			cluster := util.CreateCluster(clusterName)
			vsphereCluster := util.CreateVSphereCluster(vsphereClusterName)
			clusterCtx, controllerCtx := util.CreateClusterContext(cluster, vsphereCluster)
			rpService := RPService{Client: controllerCtx.Client}
			g := NewWithT(t)

			groups := []string{ControlPlaneVMClusterModuleGroupName}
			if tc.existingWorkerCMG {
				groups = append(groups, getMachineDeploymentNameForCluster(cluster))
			}
			existingPolicy := &vmoprv1alpha5.VirtualMachineSetResourcePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
				},
				Spec: vmoprv1alpha5.VirtualMachineSetResourcePolicySpec{
					ResourcePool: vmoprv1alpha5.ResourcePoolSpec{
						Name: "outdated-pool",
					},
					Folder:              "outdated-folder",
					ClusterModuleGroups: groups,
				},
			}
			g.Expect(rpService.Client.Create(ctx, existingPolicy)).To(Succeed())
			g.Expect(rpService.ReconcileResourcePolicy(ctx, clusterCtx)).To(Succeed())

			resourcePolicy := &vmoprv1alpha5.VirtualMachineSetResourcePolicy{}
			g.Expect(rpService.Client.Get(ctx, client.ObjectKeyFromObject(existingPolicy), resourcePolicy)).To(Succeed())
			g.Expect(resourcePolicy.Spec.ResourcePool.Name).To(Equal(clusterName))
			g.Expect(resourcePolicy.Spec.Folder).To(Equal(clusterName))
			expectedGroups := []string{ControlPlaneVMClusterModuleGroupName}
			if !tc.featureEnabled || tc.existingWorkerCMG {
				expectedGroups = append(expectedGroups, getMachineDeploymentNameForCluster(cluster))
			}
			g.Expect(resourcePolicy.Spec.ClusterModuleGroups).To(ConsistOf(expectedGroups))
			g.Expect(resourcePolicy.OwnerReferences).To(HaveLen(1))
			g.Expect(resourcePolicy.OwnerReferences[0].UID).To(Equal(vsphereCluster.UID))

			// Repeated reconciliation must not write an unchanged policy.
			resourceVersion := resourcePolicy.ResourceVersion
			g.Expect(rpService.ReconcileResourcePolicy(ctx, clusterCtx)).To(Succeed())
			g.Expect(rpService.Client.Get(ctx, client.ObjectKeyFromObject(existingPolicy), resourcePolicy)).To(Succeed())
			g.Expect(resourcePolicy.ResourceVersion).To(Equal(resourceVersion))
		})
	}
}
