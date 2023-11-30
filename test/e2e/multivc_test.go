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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers"
)

type MultiVCenterSpecInput struct {
	InfraClients
	Global     GlobalInput
	Namespace  *corev1.Namespace
	Datacenter string
}

var _ = Describe("Cluster creation with multivc [specialized-infra]", func() {
	var namespace *corev1.Namespace

	BeforeEach(func() {
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
		namespace = setupSpecNamespace("capv-e2e")
	})

	AfterEach(func() {
		cleanupSpecNamespace(namespace)
	})

	It("should create a cluster successfully", func() {
		VerifyMultiVC(ctx, MultiVCenterSpecInput{
			Namespace:  namespace,
			Datacenter: vsphereDatacenter,
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

func VerifyMultiVC(ctx context.Context, input MultiVCenterSpecInput) {
	var (
		specName         = "" // default template
		namespace        = input.Namespace
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		mgmtClusterProxy        framework.ClusterProxy
		selfHostedNamespace     *corev1.Namespace
		selfHostedCancelWatches context.CancelFunc
	)

	clusterName := fmt.Sprintf("%s-%s", "mgmtcluster", util.RandomString(6))
	Expect(namespace).NotTo(BeNil())

	By("creating a workload cluster")
	configCluster := defaultConfigCluster(clusterName, namespace.Name, specName, 1, 1, GlobalInput{
		BootstrapClusterProxy: bootstrapClusterProxy,
		ClusterctlConfigPath:  clusterctlConfigPath,
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

	vms := getVSphereVMsForCluster(clusterName, namespace.Name)
	Expect(vms.Items).ToNot(BeEmpty())

	_, err := vsphereFinder.DatacenterOrDefault(ctx, input.Datacenter)
	Expect(err).ShouldNot(HaveOccurred())

	By("Turning the workload cluster into a management cluster")

	// Get a ClusterBroker so we can interact with the workload cluster
	mgmtClusterProxy = input.Global.BootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)

	Byf("Creating a namespace for hosting the %s test spec", specName)
	selfHostedNamespace, selfHostedCancelWatches = framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   mgmtClusterProxy.GetClient(),
		ClientSet: mgmtClusterProxy.GetClientSet(),
		Name:      namespace.Name,
		LogFolder: filepath.Join(artifactFolder, "clusters", "bootstrap"),
	})

	By("Initializing the workload cluster")
	helpers.InitBootstrapCluster(ctx, mgmtClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)

	By("Ensure API servers are stable before doing move")
	// Nb. This check was introduced to prevent doing move to self-hosted in an aggressive way and thus avoid flakes.
	// More specifically, we were observing the test failing to get objects from the API server during move, so we
	// are now testing the API servers are stable before starting move.
	Consistently(func() error {
		kubeSystem := &corev1.Namespace{}
		return input.Global.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
	}, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")
	Consistently(func() error {
		kubeSystem := &corev1.Namespace{}
		return mgmtClusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
	}, "5s", "100ms").Should(BeNil(), "Failed to assert self-hosted API server stability")

	// Get the machines of the workloadCluster before it is moved to become self-hosted to make sure that the move did not trigger
	// any unexpected rollouts.
	preMoveMachineList := &unstructured.UnstructuredList{}
	preMoveMachineList.SetGroupVersionKind(clusterv1.GroupVersion.WithKind("MachineList"))
	err = input.Global.BootstrapClusterProxy.GetClient().List(
		ctx,
		preMoveMachineList,
		client.InNamespace(namespace.Name),
		client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName},
	)
	Expect(err).NotTo(HaveOccurred(), "Failed to list machines before move")

	By("Moving the cluster to self hosted")
	clusterctl.Move(ctx, clusterctl.MoveInput{
		LogFolder:            filepath.Join(input.Global.ArtifactFolder, "clusters", "bootstrap"),
		ClusterctlConfigPath: input.Global.ClusterctlConfigPath,
		FromKubeconfigPath:   input.Global.BootstrapClusterProxy.GetKubeconfigPath(),
		ToKubeconfigPath:     mgmtClusterProxy.GetKubeconfigPath(),
		Namespace:            namespace.Name,
	})

	Expect(selfHostedNamespace.Name).ShouldNot(BeNil(), "namespace should have name")

	wlClusterName := fmt.Sprintf("%s-%s", "wlcluster", util.RandomString(6))

	_ = os.Setenv("VSPHERE_SERVER", e2eConfig.GetVariable("VSPHERE2_SERVER"))

	_ = os.Setenv("VSPHERE_TLS_THUMBPRINT", e2eConfig.GetVariable("VSPHERE2_TLS_THUMBPRINT"))
	_ = os.Setenv("VSPHERE_USERNAME", os.Getenv("VSPHERE2_USERNAME"))
	_ = os.Setenv("VSPHERE_PASSWORD", os.Getenv("VSPHERE2_PASSWORD"))
	_ = os.Setenv("VSPHERE_RESOURCE_POOL", e2eConfig.GetVariable("VSPHERE2_RESOURCE_POOL"))
	_ = os.Setenv("VSPHERE_TEMPLATE", e2eConfig.GetVariable("VSPHERE2_TEMPLATE"))
	_ = os.Setenv("CONTROL_PLANE_ENDPOINT_IP", e2eConfig.GetVariable("VSPHERE2_CONTROL_PLANE_ENDPOINT_IP"))

	By("creating a workload cluster from vsphere hosted management cluster")
	wlConfigCluster := defaultConfigCluster(wlClusterName, namespace.Name, specName, 1, 1, GlobalInput{
		BootstrapClusterProxy: mgmtClusterProxy,
		ClusterctlConfigPath:  clusterctlConfigPath,
		E2EConfig:             e2eConfig,
		ArtifactFolder:        artifactFolder,
	})

	clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy:                 mgmtClusterProxy,
		ConfigCluster:                wlConfigCluster,
		WaitForClusterIntervals:      input.Global.E2EConfig.GetIntervals("", "wait-cluster"),
		WaitForControlPlaneIntervals: input.Global.E2EConfig.GetIntervals("", "wait-control-plane"),
		WaitForMachineDeployments:    input.Global.E2EConfig.GetIntervals("", "wait-worker-nodes"),
	}, clusterResources)

	vms = getVSphereVMs(mgmtClusterProxy, wlClusterName, namespace.Name)
	Expect(vms.Items).ToNot(BeEmpty())
	if selfHostedCancelWatches != nil {
		selfHostedCancelWatches()
	}
}
func getVSphereVMs(clusterProxy framework.ClusterProxy, clusterName, namespace string) *infrav1.VSphereVMList {
	var vms infrav1.VSphereVMList
	err := clusterProxy.GetClient().List(
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
