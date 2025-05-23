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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/pbm"
	pbmypes "github.com/vmware/govmomi/pbm/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

type StoragePolicySpecInput struct {
	InfraClients
	Global     GlobalInput
	SpecName   string
	Namespace  *corev1.Namespace
	Datacenter string
}

var _ = Describe("Cluster creation with storage policy", func() {
	const specName = "storage-policy"
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		var namespace *corev1.Namespace

		BeforeEach(func() {
			Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
			namespace = setupSpecNamespace("capv-e2e", testSpecificSettingsGetter().PostNamespaceCreatedFunc)
		})

		AfterEach(func() {
			cleanupSpecNamespace(namespace)
		})

		It("should create a cluster successfully", func() {
			VerifyStoragePolicy(ctx, StoragePolicySpecInput{
				SpecName:   specName,
				Namespace:  namespace,
				Datacenter: vsphereDatacenter,
				InfraClients: InfraClients{
					Client:     vsphereClient,
					RestClient: restClient,
					Finder:     vsphereFinder,
				},
				Global: GlobalInput{
					BootstrapClusterProxy: bootstrapClusterProxy,
					ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
					E2EConfig:             e2eConfig,
					ArtifactFolder:        artifactFolder,
				},
			})
		})
	})
})

func VerifyStoragePolicy(ctx context.Context, input StoragePolicySpecInput) {
	var (
		specName         = input.SpecName
		namespace        = input.Namespace
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	)

	clusterName := fmt.Sprintf("%s-%s", specName, util.RandomString(6))
	Expect(namespace).NotTo(BeNil())

	By("creating a workload cluster")
	configCluster := defaultConfigCluster(clusterName, namespace.Name, specName, 1, 0, GlobalInput{
		BootstrapClusterProxy: bootstrapClusterProxy,
		ClusterctlConfigPath:  input.Global.ClusterctlConfigPath,
		E2EConfig:             e2eConfig,
		ArtifactFolder:        artifactFolder,
	})

	clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy:                 input.Global.BootstrapClusterProxy,
		ConfigCluster:                configCluster,
		WaitForClusterIntervals:      input.Global.E2EConfig.GetIntervals("", "wait-cluster"),
		WaitForControlPlaneIntervals: input.Global.E2EConfig.GetIntervals("", "wait-control-plane"),
		WaitForMachineDeployments:    input.Global.E2EConfig.GetIntervals("", "wait-worker-nodes"),
	}, clusterResources)

	pbmClient, err := pbm.NewClient(ctx, input.Client.Client)
	Expect(err).NotTo(HaveOccurred())
	var res []pbmypes.PbmServerObjectRef
	if pbmClient != nil {
		spName := input.Global.E2EConfig.MustGetVariable(VsphereStoragePolicy)
		if spName == "" {
			Fail("storage policy test run without setting VSPHERE_STORAGE_POLICY")
		}

		spID, err := pbmClient.ProfileIDByName(ctx, spName)
		Expect(err).NotTo(HaveOccurred())

		res, err = pbmClient.QueryAssociatedEntity(ctx, pbmypes.PbmProfileId{UniqueId: spID}, "virtualMachine")
		Expect(err).NotTo(HaveOccurred())
	}
	Expect(res).ToNot(BeEmpty())

	vms := getVSphereVMsForCluster(clusterName, namespace.Name)
	Expect(vms.Items).ToNot(BeEmpty())

	datacenter, err := vsphereFinder.DatacenterOrDefault(ctx, input.Datacenter)
	Expect(err).ShouldNot(HaveOccurred())
	By("verifying storage policy is used by VMs")
	for _, vm := range vms.Items {
		si := object.NewSearchIndex(input.Client.Client)
		ref, err := si.FindByUuid(ctx, datacenter, vm.Spec.BiosUUID, true, ptr.To(false))
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
}

func getVSphereVMsForCluster(clusterName, namespace string) *infrav1.VSphereVMList {
	var vms infrav1.VSphereVMList
	err := bootstrapClusterProxy.GetClient().List(
		ctx,
		&vms,
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel: clusterName,
		},
	)
	Expect(err).NotTo(HaveOccurred())

	return &vms
}
