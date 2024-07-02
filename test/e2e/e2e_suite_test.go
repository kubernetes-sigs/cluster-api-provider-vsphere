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
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	topologyv1 "github.com/vmware-tanzu/vm-operator/external/tanzu-topology/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	capiutil "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	vsphereframework "sigs.k8s.io/cluster-api-provider-vsphere/test/framework"
	vsphereip "sigs.k8s.io/cluster-api-provider-vsphere/test/framework/ip"
	vspherelog "sigs.k8s.io/cluster-api-provider-vsphere/test/framework/log"
	vspherevcsim "sigs.k8s.io/cluster-api-provider-vsphere/test/framework/vcsim"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
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
	// VCenterTestTarget identify tests targeting a real vCenter instance, including also the VMC infrastructure used for CAPV CI.
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

	// e2eIPPool to be used for the e2e test.
	e2eIPPool string

	// inClusterAddressManager is used to claim and cleanup IP addresses used for kubernetes control plane API Servers.
	inClusterAddressManager vsphereip.AddressManager

	// vcsimAddressManager is used to claim and cleanup IP addresses used for kubernetes control plane API Servers.
	vcsimAddressManager vsphereip.AddressManager
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&configOverridesPath, "e2e.config-overrides", "", "path to the e2e config file containing overrides to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder where e2e test artifact should be stored")
	flag.BoolVar(&alsoLogToFile, "e2e.also-log-to-file", true, "if true, ginkgo logs are additionally written to the `ginkgo-log.txt` file in the artifacts folder (including timestamps)")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
	flag.StringVar(&e2eIPPool, "e2e.ip-pool", "", "IPPool to use for the e2e test. Supports the addresses, gateway and prefix fields from the InClusterIPPool CRD https://github.com/kubernetes-sigs/cluster-api-ipam-provider-in-cluster/blob/main/api/v1alpha2/inclusterippool_types.go")
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

	// fetch the current config
	suiteConfig, reporterConfig := GinkgoConfiguration()

	// Detect test target.
	testTarget = VCenterTestTarget
	if strings.Contains(strings.Join(suiteConfig.FocusStrings, " "), "\\[vcsim\\]") {
		testTarget = VCSimTestTarget
	}

	// Detect test mode.
	testMode = GovmomiTestMode
	if strings.Contains(strings.Join(suiteConfig.FocusStrings, " "), "\\[supervisor\\]") {
		testMode = SupervisorTestMode
	}

	RunSpecs(t, "capv-e2e", suiteConfig, reporterConfig)
}

// Using a SynchronizedBeforeSuite for controlling how to create resources shared across ParallelNodes (~ginkgo threads).
// The local clusterctl repository & the bootstrap cluster are created once and shared across all the tests.
var _ = SynchronizedBeforeSuite(func() []byte {
	// Before all ParallelNodes.
	Byf("TestTarget: %s\n", testTarget)
	Byf("TestMode: %s\n", testMode)

	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder) //nolint:gosec // Non-production code

	By("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()

	Byf("Loading the e2e test configuration from %q", configPath)
	var err error
	e2eConfig, err = vsphereframework.LoadE2EConfig(ctx, configPath, configOverridesPath, testTarget, testMode)
	Expect(err).NotTo(HaveOccurred())

	Byf("Creating a clusterctl local repository into %q", artifactFolder)
	clusterctlConfigPath, err = vsphereframework.CreateClusterctlLocalRepository(ctx, e2eConfig, filepath.Join(artifactFolder, "repository"), true)
	Expect(err).NotTo(HaveOccurred())

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy, err = vsphereframework.SetupBootstrapCluster(ctx, e2eConfig, scheme, useExistingCluster)
	Expect(err).NotTo(HaveOccurred())

	By("Initializing the bootstrap cluster")
	vsphereframework.InitBootstrapCluster(ctx, bootstrapClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)

	if testTarget == VCSimTestTarget {
		Byf("Creating a vcsim server")
		Eventually(func() error {
			return vspherevcsim.Create(ctx, bootstrapClusterProxy.GetClient())
		}, 30*time.Second, 3*time.Second).ShouldNot(HaveOccurred(), "Failed to create VCenterSimulator")
	}

	By("Getting AddressClaim labels")
	ipClaimLabels := vsphereip.GetIPAddressClaimLabels()
	var ipClaimLabelsRaw []string
	for k, v := range ipClaimLabels {
		ipClaimLabelsRaw = append(ipClaimLabelsRaw, fmt.Sprintf("%s=%s", k, v))
	}

	return []byte(
		strings.Join([]string{
			artifactFolder,
			configPath,
			configOverridesPath,
			testTarget,
			testMode,
			clusterctlConfigPath,
			bootstrapClusterProxy.GetKubeconfigPath(),
			strings.Join(ipClaimLabelsRaw, ";"),
		}, ","),
	)
}, func(data []byte) {
	// Before each ParallelNode.

	parts := strings.Split(string(data), ",")
	Expect(parts).To(HaveLen(8))

	artifactFolder = parts[0]
	configPath = parts[1]
	configOverridesPath = parts[2]
	testTarget = parts[3]
	testMode = parts[4]
	clusterctlConfigPath = parts[5]
	kubeconfigPath := parts[6]
	ipClaimLabelsRaw := parts[7]

	namespaces = map[*corev1.Namespace]context.CancelFunc{}

	if testTarget == VCenterTestTarget {
		// Some of the tests targeting VCenter relies on an additional VSphere session to check test progress;
		// such session is create once, and shared across many tests.
		// Some changes will be requires to get this working with vcsim e.g. about how to get the credentials/vCenter info,
		// but we are deferring this to future work (if an and when necessary).
		By("Initializing the vSphere session to ensure credentials are working", initVSphereSession)
	}

	var err error
	e2eConfig, err = vsphereframework.LoadE2EConfig(ctx, configPath, configOverridesPath, testTarget, testMode)
	Expect(err).NotTo(HaveOccurred())

	clusterProxyOptions := []framework.Option{}
	// vspherelog.MachineLogCollector tries to ssh to the machines to collect logs.
	// This does not work when using vcsim because there are no real machines running ssh.
	if testTarget != VCSimTestTarget {
		clusterProxyOptions = append(clusterProxyOptions, framework.WithMachineLogCollector(&vspherelog.MachineLogCollector{
			Client: vsphereClient,
			Finder: vsphereFinder,
		}))
	}
	bootstrapClusterProxy = framework.NewClusterProxy("bootstrap", kubeconfigPath, initScheme(), clusterProxyOptions...)

	ipClaimLabels := map[string]string{}
	for _, s := range strings.Split(ipClaimLabelsRaw, ";") {
		splittedLabel := strings.Split(s, "=")
		Expect(splittedLabel).To(HaveLen(2))

		ipClaimLabels[splittedLabel[0]] = splittedLabel[1]
	}

	// Setup the in cluster address manager
	switch testTarget {
	case VCenterTestTarget:
		// Create the in cluster address manager
		inClusterAddressManager, err = vsphereip.InClusterAddressManager(ctx, bootstrapClusterProxy.GetClient(), e2eIPPool, ipClaimLabels, skipCleanup)
		Expect(err).ToNot(HaveOccurred())

	case VCSimTestTarget:
		// Create the in vcsim address manager
		vcsimAddressManager, err = vsphereip.VCSIMAddressManager(bootstrapClusterProxy.GetClient(), ipClaimLabels, skipCleanup)
		Expect(err).ToNot(HaveOccurred())
	}
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
		switch testTarget {
		case VCenterTestTarget:
			// Cleanup the in cluster address manager
			vSphereFolderName := e2eConfig.GetVariable("VSPHERE_FOLDER")
			err := inClusterAddressManager.Teardown(ctx, vsphereip.MachineFolder(vSphereFolderName), vsphereip.VSphereClient(vsphereClient))
			if err != nil {
				Byf("Ignoring Teardown error: %v", err)
			}

		case VCSimTestTarget:
			// Cleanup the vcsim address manager
			Expect(vcsimAddressManager.Teardown(ctx)).To(Succeed())

			// cleanup the vcsim server
			Expect(vspherevcsim.Delete(ctx, bootstrapClusterProxy.GetClient(), skipCleanup)).To(Succeed())
		}
	}

	if testTarget == VCenterTestTarget {
		By("Cleaning up the vSphere session", terminateVSphereSession)
	}

	if !skipCleanup {
		By("Tearing down the management cluster")
		vsphereframework.TearDown(ctx, bootstrapClusterProvider, bootstrapClusterProxy)
	}
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)

	if testTarget == VCSimTestTarget {
		_ = vcsimv1.AddToScheme(sc)
	}

	if testMode == GovmomiTestMode {
		_ = infrav1.AddToScheme(sc)
	}

	if testMode == SupervisorTestMode {
		_ = corev1.AddToScheme(sc)
		_ = storagev1.AddToScheme(sc)
		_ = topologyv1.AddToScheme(sc)
		_ = vmoprv1.AddToScheme(sc)
		_ = vmwarev1.AddToScheme(sc)
	}
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
