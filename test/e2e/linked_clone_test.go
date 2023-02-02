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
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

type LinkedCloneSpecInput struct {
	InfraClients
	Global    GlobalInput
	Namespace *corev1.Namespace
}

var _ = Describe("Cluster creation with linked clone mode", func() {
	var (
		namespace *corev1.Namespace
	)

	BeforeEach(func() {
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
		namespace = setupSpecNamespace("linked-clone-e2e")
	})

	AfterEach(func() {
		cleanupSpecNamespace(namespace)
	})

	It("should create a cluster by using linked clone to create nodes", func() {
		VerifyLinkedClone(ctx, LinkedCloneSpecInput{
			Namespace: namespace,
			InfraClients: InfraClients{
				Client:     vsphereClient,
				RestClient: restClient,
				Finder:     vsphereFinder,
			},
			Global: GlobalInput{
				BootstrapClusterProxy: bootstrapClusterProxy,
				ClusterctlConfigPath:  clusterctlConfigPath,
				E2EConfig:             e2eConfig,
				ArtifactFolder:        artifactFolder,
			},
		})
	})
})

func VerifyLinkedClone(ctx context.Context, input LinkedCloneSpecInput) {
	var (
		specName         = "linked-clone"
		namespace        = input.Namespace
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	)

	clusterName := fmt.Sprintf("%s-%s", specName, util.RandomString(6))
	Expect(namespace).NotTo(BeNil())

	Byf("creating a workload cluster %s", clusterName)
	configCluster := defaultConfigCluster(clusterName, namespace.Name, "", 1, 1, input.Global)

	clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy:                 input.Global.BootstrapClusterProxy,
		ConfigCluster:                configCluster,
		WaitForClusterIntervals:      input.Global.E2EConfig.GetIntervals("", "wait-cluster"),
		WaitForControlPlaneIntervals: input.Global.E2EConfig.GetIntervals("", "wait-control-plane"),
		WaitForMachineDeployments:    input.Global.E2EConfig.GetIntervals("", "wait-worker-nodes"),
	}, clusterResources)

	By("verifying linked clone")
	vsphereVMs := FetchVsphereVMsForCluster(ctx, input.Global.BootstrapClusterProxy, clusterName, namespace.Name)
	Expect(verifyDiskLayoutOfVMs(ctx, input.Finder, vsphereVMs)).To(Succeed())
}

func verifyDiskLayoutOfVMs(ctx context.Context, finder *find.Finder, vms []infrav1.VSphereVM) error {
	for _, vm := range vms {
		vmObj, err := finder.VirtualMachine(ctx, vm.Name)
		if err != nil {
			return err
		}

		var vmMo mo.VirtualMachine
		err = vmObj.Properties(ctx, vmObj.Reference(), []string{"layoutEx"}, &vmMo)
		if err != nil {
			return err
		}

		for _, disk := range vmMo.LayoutEx.Disk {
			if len(disk.Chain) < 2 {
				return errors.New("Disk file is not chained")
			}
		}
	}
	return nil
}

func FetchVsphereVMsForCluster(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, clusterName, ns string) []infrav1.VSphereVM {
	vms := &infrav1.VSphereVMList{}
	err := bootstrapClusterProxy.GetClient().List(
		ctx,
		vms,
		client.InNamespace(ns),
		client.MatchingLabels{
			clusterv1.ClusterLabelName: clusterName,
		})
	Expect(err).ToNot(HaveOccurred())

	return vms.Items
}
