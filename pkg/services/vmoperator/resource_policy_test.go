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
	"testing"

	. "github.com/onsi/gomega"
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

func TestRPService(t *testing.T) {
	ctx := context.Background()
	const clusterName = "test-cluster"
	const workerGroupName = "test-cluster-workers-0"

	expectedPolicyWithWorkerGroup := &vmoprv1alpha5.VirtualMachineSetResourcePolicy{
		Spec: vmoprv1alpha5.VirtualMachineSetResourcePolicySpec{
			ResourcePool: vmoprv1alpha5.ResourcePoolSpec{
				Name: clusterName,
			},
			Folder:              clusterName,
			ClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName, workerGroupName},
		},
	}
	expectedPolicyWithoutWorkerGroup := &vmoprv1alpha5.VirtualMachineSetResourcePolicy{
		Spec: vmoprv1alpha5.VirtualMachineSetResourcePolicySpec{
			ResourcePool: vmoprv1alpha5.ResourcePoolSpec{
				Name: clusterName,
			},
			Folder:              clusterName,
			ClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName},
		},
	}
	existingPolicyWithWorkerGroup := &vmoprv1alpha5.VirtualMachineSetResourcePolicy{
		Spec: vmoprv1alpha5.VirtualMachineSetResourcePolicySpec{
			ResourcePool: vmoprv1alpha5.ResourcePoolSpec{
				Name: "outdated-pool",
			},
			Folder:              "outdated-folder",
			ClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName, workerGroupName},
		},
	}
	existingPolicyWithoutWorkerGroup := &vmoprv1alpha5.VirtualMachineSetResourcePolicy{
		Spec: vmoprv1alpha5.VirtualMachineSetResourcePolicySpec{
			ResourcePool: vmoprv1alpha5.ResourcePoolSpec{
				Name: "outdated-pool",
			},
			Folder:              "outdated-folder",
			ClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName},
		},
	}

	for _, tc := range []struct {
		name           string
		featureEnabled bool
		existingPolicy *vmoprv1alpha5.VirtualMachineSetResourcePolicy
		expectedPolicy *vmoprv1alpha5.VirtualMachineSetResourcePolicy
	}{
		{
			name:           "Creates legacy worker group when feature gate is disabled",
			expectedPolicy: expectedPolicyWithWorkerGroup,
		},
		{
			name:           "Creates only control plane group when feature gate is enabled",
			featureEnabled: true,
			expectedPolicy: expectedPolicyWithoutWorkerGroup,
		},
		{
			name:           "Restores legacy worker group when feature gate is disabled",
			existingPolicy: existingPolicyWithoutWorkerGroup,
			expectedPolicy: expectedPolicyWithWorkerGroup,
		},
		{
			name:           "Preserves legacy worker group when feature gate is enabled",
			featureEnabled: true,
			existingPolicy: existingPolicyWithWorkerGroup,
			expectedPolicy: expectedPolicyWithWorkerGroup,
		},
		{
			name:           "Keeps worker group absent when feature gate is enabled",
			featureEnabled: true,
			existingPolicy: existingPolicyWithoutWorkerGroup,
			expectedPolicy: expectedPolicyWithoutWorkerGroup,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.WorkerStrictAntiAffinity, tc.featureEnabled)

			cluster := util.CreateCluster(clusterName)
			vsphereCluster := util.CreateVSphereCluster("test-vsphere-cluster")
			vsphereCluster.UID = "test-vsphere-cluster-uid"
			clusterCtx, controllerCtx := util.CreateClusterContext(cluster, vsphereCluster)
			rpService := RPService{Client: controllerCtx.Client}
			key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}

			if tc.existingPolicy != nil {
				existingPolicy := tc.existingPolicy.DeepCopy()
				existingPolicy.Name = key.Name
				existingPolicy.Namespace = key.Namespace
				g.Expect(rpService.Client.Create(ctx, existingPolicy)).To(Succeed())
			}
			g.Expect(rpService.ReconcileResourcePolicy(ctx, clusterCtx)).To(Succeed())

			// Use VM Operator native types to verify the policy persisted through the conversion client.
			resourcePolicy := &vmoprv1alpha5.VirtualMachineSetResourcePolicy{}
			g.Expect(rpService.Client.Get(ctx, key, resourcePolicy)).To(Succeed())
			g.Expect(apiequality.Semantic.DeepEqual(resourcePolicy.Spec, tc.expectedPolicy.Spec)).To(BeTrue(),
				"got policy spec %#v, want %#v", resourcePolicy.Spec, tc.expectedPolicy.Spec)
			g.Expect(resourcePolicy.OwnerReferences).To(Equal([]metav1.OwnerReference{
				{
					APIVersion: vsphereCluster.APIVersion,
					Kind:       vsphereCluster.Kind,
					Name:       vsphereCluster.Name,
					UID:        vsphereCluster.UID,
				},
			}))

			// Repeated reconciliation must not write an unchanged policy.
			resourceVersion := resourcePolicy.ResourceVersion
			g.Expect(rpService.ReconcileResourcePolicy(ctx, clusterCtx)).To(Succeed())
			g.Expect(rpService.Client.Get(ctx, key, resourcePolicy)).To(Succeed())
			g.Expect(resourcePolicy.ResourceVersion).To(Equal(resourceVersion))
		})
	}
}
