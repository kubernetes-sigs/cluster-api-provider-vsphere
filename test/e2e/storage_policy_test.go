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

package e2e

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/pbm"
	"github.com/vmware/govmomi/pbm/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	v1beta12 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

var _ = Describe("Cluster creation with storage policy", func() {
	var namespace *v1.Namespace

	BeforeEach(func() {
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
		namespace = setupSpecNamespace("capv-e2e")
	})

	AfterEach(func() {
		cleanupSpecNamespace(namespace)
	})

	It("should create a cluster successfully", func() {
		clusterName := fmt.Sprintf("cluster-%s", util.RandomString(6))
		Expect(namespace).NotTo(BeNil())

		By("creating a workload cluster")
		configCluster := defaultConfigCluster(clusterName, namespace.Name, 1, 0)

		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy:                 bootstrapClusterProxy,
			ConfigCluster:                configCluster,
			WaitForClusterIntervals:      e2eConfig.GetIntervals("", "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals("", "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals("", "wait-worker-nodes"),
		}, &clusterctl.ApplyClusterTemplateAndWaitResult{})

		pbmClient, err := pbm.NewClient(ctx, vsphereClient.Client)
		Expect(err).NotTo(HaveOccurred())
		var res []types.PbmServerObjectRef
		if pbmClient != nil {
			spName := e2eConfig.GetVariable(VsphereStoragePolicy)
			if spName == "" {
				Fail("storage policy test run without setting VSPHERE_STORAGE_POLICY")
			}

			spID, err := pbmClient.ProfileIDByName(ctx, spName)
			Expect(err).NotTo(HaveOccurred())

			res, err = pbmClient.QueryAssociatedEntity(ctx, types.PbmProfileId{UniqueId: spID}, "virtualMachine")
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(len(res)).To(BeNumerically(">", 0))

		vms := getVSphereVMsForCluster(clusterName, namespace.Name)
		Expect(len(vms.Items)).To(BeNumerically(">", 0))

		datacenter, err := vsphereFinder.DatacenterOrDefault(ctx, vsphereDatacenter)
		Expect(err).ShouldNot(HaveOccurred())
		By("verifying storage policy is used by VMs")
		for _, vm := range vms.Items {
			si := object.NewSearchIndex(vsphereClient.Client)
			ref, err := si.FindByUuid(ctx, datacenter, vm.Spec.BiosUUID, true, pointer.BoolPtr(false))
			Expect(err).NotTo(HaveOccurred())
			found := false

			for _, o := range res {
				if ref.Reference().Value == o.Key {
					found = true
					break
				}
			}

			Expect(found).To(BeTrue(), "failed to find vm in list of vms using storage policy")
		}
	})
})

func defaultConfigCluster(clusterName, namespace string, controlPlaneNodeCount, workerNodeCount int64) clusterctl.ConfigClusterInput {
	return clusterctl.ConfigClusterInput{
		LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
		ClusterctlConfigPath:     clusterctlConfigPath,
		KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
		InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
		Flavor:                   clusterctl.DefaultFlavor,
		Namespace:                namespace,
		ClusterName:              clusterName,
		KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
		ControlPlaneMachineCount: pointer.Int64Ptr(controlPlaneNodeCount),
		WorkerMachineCount:       pointer.Int64Ptr(workerNodeCount),
	}
}

func getVSphereVMsForCluster(clusterName, namespace string) *v1beta1.VSphereVMList {
	var vms v1beta1.VSphereVMList
	err := bootstrapClusterProxy.GetClient().List(
		ctx,
		&vms,
		client.InNamespace(namespace),
		client.MatchingLabels{
			v1beta12.ClusterLabelName: clusterName,
		},
	)
	Expect(err).NotTo(HaveOccurred())

	return &vms
}
