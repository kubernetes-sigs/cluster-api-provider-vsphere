/*
Copyright 2020 The Kubernetes Authors.

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
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	capiutil "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vsphereframework "sigs.k8s.io/cluster-api-provider-vsphere/test/framework"
	vsphereip "sigs.k8s.io/cluster-api-provider-vsphere/test/framework/ip"
	vspherelog "sigs.k8s.io/cluster-api-provider-vsphere/test/framework/log"
)

const (
	VsphereStoragePolicy = "VSPHERE_STORAGE_POLICY"
)

const (
	// GovmomiTestMode identify tests run with CAPV using govmomi to access vCenter.
	GovmomiTestMode string = "govmomi"

	// SupervisorTestMode identify tests run with CAPV in supervisor mode (delegating to vm-operator all the interaction with vCenter).
	SupervisorTestMode string = "supervisor"
)

const (
	// VMCTestTarget identify tests targeting VMC infrastructure used for CAPV CI.
	VMCTestTarget string = "vmc"

	// VCenterTestTarget identify tests targeting a user provided vCenter instance.
	VCenterTestTarget string = "vcenter"

	// VCSimTestTarget identify tests targeting a vcsim instance (instead of a real vCenter).
	VCSimTestTarget string = "vcsim"
)

// Test suite flags.
var (
	// configPath is the path to the e2e config file.
	configPath string

	// configOverridesPath is the path to the e2e config file containing overrides to the content of configPath config file.
	// Only variables and intervals are considered.
	configOverridesPath string

	// useExistingCluster instructs the test to use the current cluster instead
	// of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// alsoLogToFile enables additional logging to the 'ginkgo-log.txt' file in the artifact folder.
	// These logs also contain timestamps.
	alsoLogToFile bool

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool

	// defines how CAPV should behave during this test.
	testMode string

	// defines which type of infrastructure this test targets.
	testTarget string
)

// Test suite global vars.
var (
	ctx = ctrl.SetupSignalHandler()

	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// bootstrapClusterProvider manages provisioning of the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	bootstrapClusterProvider bootstrap.ClusterProvider

	// bootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	bootstrapClusterProxy framework.ClusterProxy

	namespaces map[*corev1.Namespace]context.CancelFunc

	// e2eIPAMKubeconfig is a kubeconfig to a cluster which provides IP address management via an in-cluster
	// IPAM provider to claim IPs for the control plane IPs of created clusters.
	e2eIPAMKubeconfig string

	// ipAddressManager is used to claim and cleanup IP addresses used for kubernetes control plane API Servers.
	ipAddressManager vsphereip.AddressManager
)

type configOverrides struct {
	Variables map[string]string   `json:"variables,omitempty"`
	Intervals map[string][]string `json:"intervals,omitempty"`
}

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&configOverridesPath, "e2e.config-overrides", "", "path to the e2e config file containing overrides to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder where e2e test artifact should be stored")
	flag.BoolVar(&alsoLogToFile, "e2e.also-log-to-file", true, "if true, ginkgo logs are additionally written to the `ginkgo-log.txt` file in the artifacts folder (including timestamps)")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
	flag.StringVar(&e2eIPAMKubeconfig, "e2e.ipam-kubeconfig", "", "path to the kubeconfig for the IPAM cluster")
	flag.StringVar(&testMode, "e2e.capv-mode", GovmomiTestMode, "defines how CAPV should behave during this test, one of govmomi|supervisor")
	flag.StringVar(&testTarget, "e2e.target-type", VMCTestTarget, "defines which type of infrastructure this test targets, one of vmc|vcenter|vcsim")
}

func TestE2E(t *testing.T) {
	g := NewWithT(t)

	ctrl.SetLogger(klog.Background())

	// If running in prow, make sure to use the artifacts folder that will be reported in test grid (ignoring the value provided by flag).
	if prowArtifactFolder, exists := os.LookupEnv("ARTIFACTS"); exists {
		artifactFolder = prowArtifactFolder
	}

	// ensure the artifacts folder exists
	g.Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder) //nolint:gosec

	RegisterFailHandler(Fail)

	if alsoLogToFile {
		w, err := EnableFileLogging(filepath.Join(artifactFolder, "ginkgo-log.txt"))
		NewWithT(t).Expect(err).ToNot(HaveOccurred())
		defer w.Close()
	}

	RunSpecs(t, "capv-e2e")
}

// Using a SynchronizedBeforeSuite for controlling how to create resources shared across ParallelNodes (~ginkgo threads).
// The local clusterctl repository & the bootstrap cluster are created once and shared across all the tests.
var _ = SynchronizedBeforeSuite(func() []byte {
	// Before all ParallelNodes.

	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder) //nolint:gosec // Non-production code

	By("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()

	Byf("Loading the e2e test configuration from %q", configPath)
	var err error
	e2eConfig, err = vsphereframework.LoadE2EConfig(ctx, configPath)
	Expect(err).NotTo(HaveOccurred())
	if configOverridesPath != "" {
		Expect(configOverridesPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config-overrides should be an existing file.")

		Byf("Merging with e2e config overrides from %q", configOverridesPath)
		configData, err := os.ReadFile(configOverridesPath) //nolint:gosec
		Expect(err).ToNot(HaveOccurred(), "Failed to read e2e config overrides")
		Expect(configData).ToNot(BeEmpty(), "The e2e config overrides should not be empty")

		configOverrides := &configOverrides{}
		Expect(yaml.Unmarshal(configData, configOverrides)).To(Succeed(), "Failed to convert e2e config overrides to yaml")

		for k, v := range configOverrides.Variables {
			e2eConfig.Variables[k] = v
		}
		for k, v := range configOverrides.Intervals {
			e2eConfig.Intervals[k] = v
		}
	}

	Byf("Creating a clusterctl local repository into %q", artifactFolder)
	clusterctlConfigPath, err = vsphereframework.CreateClusterctlLocalRepository(ctx, e2eConfig, filepath.Join(artifactFolder, "repository"), true)
	Expect(err).NotTo(HaveOccurred())

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy, err = vsphereframework.SetupBootstrapCluster(ctx, e2eConfig, scheme, useExistingCluster)
	Expect(err).NotTo(HaveOccurred())

	By("Initializing the bootstrap cluster")
	vsphereframework.InitBootstrapCluster(ctx, bootstrapClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)

	ipamLabels := vsphereip.GetIPAddressClaimLabels()
	var ipamLabelsRaw []string
	for k, v := range ipamLabels {
		ipamLabelsRaw = append(ipamLabelsRaw, fmt.Sprintf("%s=%s", k, v))
	}

	return []byte(
		strings.Join([]string{
			artifactFolder,
			configPath,
			clusterctlConfigPath,
			bootstrapClusterProxy.GetKubeconfigPath(),
			strings.Join(ipamLabelsRaw, ";"),
		}, ","),
	)
}, func(data []byte) {
	// Before each ParallelNode.
	parts := strings.Split(string(data), ",")
	Expect(parts).To(HaveLen(5))

	artifactFolder = parts[0]
	configPath = parts[1]
	clusterctlConfigPath = parts[2]
	kubeconfigPath := parts[3]
	ipamLabelsRaw := parts[4]

	namespaces = map[*corev1.Namespace]context.CancelFunc{}

	By("Initializing the vSphere session to ensure credentials are working", initVSphereSession)

	var err error
	e2eConfig, err = vsphereframework.LoadE2EConfig(ctx, configPath)
	Expect(err).NotTo(HaveOccurred())
	bootstrapClusterProxy = framework.NewClusterProxy("bootstrap", kubeconfigPath, initScheme(), framework.WithMachineLogCollector(vspherelog.MachineLogCollector{}))

	ipamLabels := map[string]string{}
	for _, s := range strings.Split(ipamLabelsRaw, ";") {
		splittedLabel := strings.Split(s, "=")
		Expect(splittedLabel).To(HaveLen(2))

		ipamLabels[splittedLabel[0]] = splittedLabel[1]
	}
	ipAddressManager, err = vsphereip.InClusterAddressManager(e2eIPAMKubeconfig, ipamLabels, skipCleanup)
	Expect(err).ToNot(HaveOccurred())
})

// Using a SynchronizedAfterSuite for controlling how to delete resources shared across ParallelNodes (~ginkgo threads).
// The bootstrap cluster is shared across all the tests, so it should be deleted only after all ParallelNodes completes.
// The local clusterctl repository is preserved like everything else created into the artifact folder.
var _ = SynchronizedAfterSuite(func() {
	// After each ParallelNode.
}, func() {
	// After all ParallelNodes.
	if !skipCleanup {
		By("Cleaning up orphaned IPAddressClaims")
		vSphereFolderName := e2eConfig.GetVariable("VSPHERE_FOLDER")
		Expect(ipAddressManager.Teardown(ctx, vSphereFolderName, vsphereClient)).To(Succeed())
	}

	By("Cleaning up the vSphere session", terminateVSphereSession)
	if !skipCleanup {
		By("Tearing down the management cluster")
		vsphereframework.TearDown(ctx, bootstrapClusterProvider, bootstrapClusterProxy)
	}
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)
	_ = infrav1.AddToScheme(sc)
	return sc
}

func setupSpecNamespace(specName string) *corev1.Namespace {
	Byf("Creating a namespace for hosting the %q test spec", specName)
	namespace, cancelWatches := framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   bootstrapClusterProxy.GetClient(),
		ClientSet: bootstrapClusterProxy.GetClientSet(),
		Name:      fmt.Sprintf("%s-%s", specName, capiutil.RandomString(6)),
		LogFolder: filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
	})

	namespaces[namespace] = cancelWatches

	return namespace
}

func cleanupSpecNamespace(namespace *corev1.Namespace) {
	Byf("Dumping all the Cluster API resources in the %q namespace", namespace.Name)

	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    bootstrapClusterProxy.GetClient(),
		Namespace: namespace.Name,
		LogPath:   filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName(), "resources"),
	})

	dumpDeploymentLogsByName(ctx, dumpDeploymentLogsByNameInput{
		LogPath:    filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName(), "logs"),
		GetLister:  bootstrapClusterProxy.GetClient(),
		Deployment: types.NamespacedName{Namespace: "vmware-system-csi", Name: "vsphere-csi-controller"},
		ClientSet:  bootstrapClusterProxy.GetClientSet(),
	})

	Byf("cleaning up namespace: %s", namespace.Name)
	cancelWatches := namespaces[namespace]

	if !skipCleanup {
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			Client:    bootstrapClusterProxy.GetClient(),
			Namespace: namespace.Name,
		}, e2eConfig.GetIntervals("", "wait-delete-cluster")...)

		By("Deleting namespace used for hosting test spec")
		framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
			Deleter: bootstrapClusterProxy.GetClient(),
			Name:    namespace.Name,
		})
	}

	cancelWatches()
	delete(namespaces, namespace)
}

const (
	retryableOperationTimeout  = 3 * time.Minute
	retryableOperationInterval = 3 * time.Second
)

// dumpDeploymentLogsByNameInput is the input for dumpDeploymentLogsByName.
type dumpDeploymentLogsByNameInput struct {
	Deployment types.NamespacedName
	ClientSet  *kubernetes.Clientset
	GetLister  framework.GetLister
	LogPath    string
}

func dumpDeploymentLogsByName(ctx context.Context, input dumpDeploymentLogsByNameInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for WatchDeploymentLogsByName")
	Expect(input.ClientSet).NotTo(BeNil(), "input.ClientSet is required for WatchDeploymentLogsByName")
	Expect(input.Deployment).NotTo(BeNil(), "input.Deployment is required for WatchDeploymentLogsByName")

	deployment := &appsv1.Deployment{}
	Eventually(func() error {
		return input.GetLister.Get(ctx, input.Deployment, deployment)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to get deployment %s", input.Deployment)

	pods := &corev1.PodList{}
	Eventually(func() error {
		return input.GetLister.List(ctx, pods, client.MatchingLabels(deployment.Spec.Selector.MatchLabels))
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to list pods for deployment %s", input.Deployment)

	for _, pod := range pods.Items {
		dumpPodLogs(ctx, dumpPodLogsInput{
			ClientSet:            input.ClientSet,
			Pod:                  &pod,
			ManagingResourceName: deployment.Name,
			LogPath:              input.LogPath,
		})
	}
}

type dumpPodLogsInput struct {
	ClientSet            *kubernetes.Clientset
	Pod                  *corev1.Pod
	ManagingResourceName string
	LogPath              string
}

func dumpPodLogs(ctx context.Context, input dumpPodLogsInput) {
	pod := &corev1.Pod{}

	for _, container := range input.Pod.Spec.Containers {
		logFile := filepath.Clean(path.Join(input.LogPath, input.ManagingResourceName, pod.Name, container.Name+".log"))
		Expect(os.MkdirAll(filepath.Dir(logFile), 0750)).To(Succeed())

		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		opts := &corev1.PodLogOptions{
			Container: container.Name,
			Follow:    true,
		}

		podLogs, err := input.ClientSet.CoreV1().Pods(input.Pod.GetNamespace()).GetLogs(input.Pod.GetName(), opts).Stream(ctx)
		Expect(err).ToNot(HaveOccurred())
		defer podLogs.Close()

		out := bufio.NewWriter(f)
		defer out.Flush()

		_, err = out.ReadFrom(podLogs)
		if err != nil && err != io.ErrUnexpectedEOF {
			Expect(err).ToNot(HaveOccurred())
		}
	}
}
