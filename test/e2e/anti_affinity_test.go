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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/clustermodules"
)

type AntiAffinitySpecInput struct {
	InfraClients
	Global      GlobalInput
	Namespace   *corev1.Namespace
	SkipCleanup bool
}

var _ = Describe("Cluster creation with anti affined nodes", func() {
	var namespace *corev1.Namespace

	BeforeEach(func() {
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
		namespace = setupSpecNamespace("anti-affinity-e2e")
	})

	AfterEach(func() {
		cleanupSpecNamespace(namespace)
	})

	It("should create a cluster with anti-affined nodes", func() {
		// Since the upstream CI has four nodes, worker node count is set to 4.
		VerifyAntiAffinity(ctx, AntiAffinitySpecInput{
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

func VerifyAntiAffinity(ctx context.Context, input AntiAffinitySpecInput) {
	var (
		specName         = "anti-affinity"
		namespace        = input.Namespace
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	)

	clusterName := fmt.Sprintf("anti-affinity-%s", util.RandomString(6))
	Expect(namespace).NotTo(BeNil())

	By("checking if the target system has enough hosts")
	hostSystems, err := input.Finder.HostSystemList(ctx, "*")
	Expect(err).ToNot(HaveOccurred())
	// Setting the number of worker nodes to the number of hosts.
	// Later in the test we check that all worker nodes are located on different hosts.
	workerNodeCount := len(hostSystems)
	// Limit size to not create too much VMs when running in a big environment.
	if workerNodeCount > 10 {
		workerNodeCount = 10
	}

	Byf("creating a workload cluster %s", clusterName)
	configCluster := defaultConfigCluster(clusterName, namespace.Name, "", 1, int64(workerNodeCount),
		input.Global)

	clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy:                 input.Global.BootstrapClusterProxy,
		ConfigCluster:                configCluster,
		WaitForClusterIntervals:      input.Global.E2EConfig.GetIntervals("", "wait-cluster"),
		WaitForControlPlaneIntervals: input.Global.E2EConfig.GetIntervals("", "wait-control-plane"),
		WaitForMachineDeployments:    input.Global.E2EConfig.GetIntervals("", "wait-worker-nodes"),
	}, clusterResources)

	vsphereCluster := FetchVSphereClusterObject(ctx, input.Global.BootstrapClusterProxy, client.ObjectKey{
		Namespace: namespace.Name,
		Name:      clusterName,
	})

	modules := vsphereCluster.Spec.ClusterModules
	By("checking for cluster module info on VSphereCluster object")
	Expect(modules).To(HaveLen(2))
	for _, mod := range vsphereCluster.Spec.ClusterModules {
		Expect(strings.HasPrefix(mod.TargetObjectName, clusterName)).To(BeTrue())
		Expect(mod.ModuleUUID).ToNot(BeEmpty())
	}

	By("verifying presence of cluster modules")
	verifyModuleInfo(ctx, modules, true)

	By("verifying node anti-affinity for worker nodes")
	workerVMs := FetchWorkerVMsForCluster(ctx, input.Global.BootstrapClusterProxy, clusterName, namespace.Name)
	Expect(workerVMs).To(HaveLen(workerNodeCount))
	Expect(verifyAntiAffinityForVMs(ctx, input.Finder, workerVMs)).To(Succeed())

	Byf("Scaling the MachineDeployment out to > %d nodes", workerNodeCount)
	framework.ScaleAndWaitMachineDeployment(ctx, framework.ScaleAndWaitMachineDeploymentInput{
		ClusterProxy:              input.Global.BootstrapClusterProxy,
		Cluster:                   clusterResources.Cluster,
		MachineDeployment:         clusterResources.MachineDeployments[0],
		Replicas:                  int32(workerNodeCount + 2),
		WaitForMachineDeployments: input.Global.E2EConfig.GetIntervals("", "wait-worker-nodes"),
	})

	Byf("Scaling the MachineDeployment down to %d nodes", workerNodeCount)
	framework.ScaleAndWaitMachineDeployment(ctx, framework.ScaleAndWaitMachineDeploymentInput{
		ClusterProxy:              input.Global.BootstrapClusterProxy,
		Cluster:                   clusterResources.Cluster,
		MachineDeployment:         clusterResources.MachineDeployments[0],
		Replicas:                  int32(workerNodeCount),
		WaitForMachineDeployments: input.Global.E2EConfig.GetIntervals("", "wait-worker-nodes"),
	})

	// Refetch the updated list of worker VMs
	workerVMs = FetchWorkerVMsForCluster(ctx, input.Global.BootstrapClusterProxy, clusterName, namespace.Name)
	Expect(workerVMs).To(HaveLen(workerNodeCount))

	By("worker nodes should be anti-affined again since enough hosts are available")
	Eventually(func() error {
		return verifyAntiAffinityForVMs(ctx, input.Finder, workerVMs)
	}, input.Global.E2EConfig.GetIntervals(specName, "wait-vm-redistribution")...).Should(Succeed())

	Byf("Deleting the cluster %s in namespace %s",
		clusterName, namespace.Name)
	framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
		Client:    input.Global.BootstrapClusterProxy.GetClient(),
		Namespace: namespace.Name,
	}, input.Global.E2EConfig.GetIntervals("", "wait-delete-cluster")...)

	By("confirming deletion of cluster module constructs")
	verifyModuleInfo(ctx, modules, false)
}

func verifyAntiAffinityForVMs(ctx context.Context, finder *find.Finder, vms []infrav1.VSphereVM) error {
	// set to hold the name of the host that each VM belongs to
	hostInfo := map[string]struct{}{}
	for _, vm := range vms {
		vmObj, err := finder.VirtualMachine(ctx, vm.Name)
		if err != nil {
			return err
		}

		host, err := vmObj.HostSystem(ctx)
		if err != nil {
			return err
		}

		name, err := host.ObjectName(ctx)
		if err != nil {
			return err
		}

		if _, ok := hostInfo[name]; ok {
			return errors.New("multiple VMs exist on single host")
		}
		hostInfo[name] = struct{}{}
	}
	return nil
}

func FetchVSphereClusterObject(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, key client.ObjectKey) *infrav1.VSphereCluster {
	vSphereCluster := &infrav1.VSphereCluster{}
	Expect(bootstrapClusterProxy.GetClient().Get(ctx, key, vSphereCluster)).To(Succeed())
	return vSphereCluster
}

func FetchWorkerVMsForCluster(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, clusterName, ns string) []infrav1.VSphereVM {
	vms := &infrav1.VSphereVMList{}
	err := bootstrapClusterProxy.GetClient().List(
		ctx,
		vms,
		client.InNamespace(ns),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel: clusterName,
		})
	Expect(err).ToNot(HaveOccurred())

	workerVMs := []infrav1.VSphereVM{}
	for _, vm := range vms.Items {
		if _, ok := vm.Labels[clusterv1.MachineControlPlaneLabel]; !ok {
			workerVMs = append(workerVMs, vm)
		}
	}
	return workerVMs
}

func verifyModuleInfo(ctx context.Context, modules []infrav1.ClusterModule, toExist bool) {
	provider := clustermodules.NewProvider(restClient)

	for _, mod := range modules {
		exists, err := provider.DoesModuleExist(ctx, mod.ModuleUUID)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(Equal(toExist))
	}
}
