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

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	capiutil "sigs.k8s.io/cluster-api/util"

	"sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers"
)

const (
	KubernetesVersion    = "KUBERNETES_VERSION"
	VsphereStoragePolicy = "VSPHERE_STORAGE_POLICY"
)

// Test suite flags.
var (
	// configPath is the path to the e2e config file.
	configPath string

	// useExistingCluster instructs the test to use the current cluster instead
	// of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool
)

// Test suite global vars.
var (
	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// bootstrapClusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	bootstrapClusterProvider bootstrap.ClusterProvider

	// bootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	bootstrapClusterProxy framework.ClusterProxy

	namespaces map[*corev1.Namespace]context.CancelFunc
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "", "folder where e2e test artifact should be stored")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
}

func TestE2E(t *testing.T) {
	// If running in prow, make sure to use the artifacts folder that will be reported in test grid (ignoring the value provided by flag).
	if prowArtifactFolder, exists := os.LookupEnv("ARTIFACTS"); exists {
		artifactFolder = prowArtifactFolder
	}

	RegisterFailHandler(Fail)
	junitPath := filepath.Join(artifactFolder, fmt.Sprintf("junit.e2e_suite.%d.xml", config.GinkgoConfig.ParallelNode))
	junitReporter := reporters.NewJUnitReporter(junitPath)
	RunSpecsWithDefaultAndCustomReporters(t, "capv-e2e", []Reporter{junitReporter})
}

// Using a SynchronizedBeforeSuite for controlling how to create resources shared across ParallelNodes (~ginkgo threads).
// The local clusterctl repository & the bootstrap cluster are created once and shared across all the tests.
var _ = SynchronizedBeforeSuite(func() []byte {
	// Before all ParallelNodes.

	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder) //nolint:gofumpt

	By("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()

	Byf("Loading the e2e test configuration from %q", configPath)
	var err error
	e2eConfig, err = helpers.LoadE2EConfig(configPath)
	Expect(err).NotTo(HaveOccurred())

	By("Initializing the vSphere session to ensure credentials are working", initVSphereSession)

	Byf("Creating a clusterctl local repository into %q", artifactFolder)
	clusterctlConfigPath, err = helpers.CreateClusterctlLocalRepository(e2eConfig, filepath.Join(artifactFolder, "repository"), true)
	Expect(err).NotTo(HaveOccurred())

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy, err = helpers.SetupBootstrapCluster(e2eConfig, scheme, useExistingCluster)
	Expect(err).NotTo(HaveOccurred())

	By("Initializing the bootstrap cluster")
	helpers.InitBootstrapCluster(bootstrapClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)
	namespaces = map[*corev1.Namespace]context.CancelFunc{}
	return []byte(
		strings.Join([]string{
			artifactFolder,
			configPath,
			clusterctlConfigPath,
			bootstrapClusterProxy.GetKubeconfigPath(),
		}, ","),
	)
}, func(data []byte) {
	// Before each ParallelNode.
	parts := strings.Split(string(data), ",")
	Expect(parts).To(HaveLen(4))

	artifactFolder = parts[0]
	configPath = parts[1]
	clusterctlConfigPath = parts[2]
	kubeconfigPath := parts[3]

	var err error
	e2eConfig, err = helpers.LoadE2EConfig(configPath)
	Expect(err).NotTo(HaveOccurred())
	bootstrapClusterProxy = framework.NewClusterProxy("bootstrap", kubeconfigPath, initScheme(), framework.WithMachineLogCollector(LogCollector{}))
})

// Using a SynchronizedAfterSuite for controlling how to delete resources shared across ParallelNodes (~ginkgo threads).
// The bootstrap cluster is shared across all the tests, so it should be deleted only after all ParallelNodes completes.
// The local clusterctl repository is preserved like everything else created into the artifact folder.
var _ = SynchronizedAfterSuite(func() {
	// After each ParallelNode.
}, func() {
	// After all ParallelNodes.

	By("Cleaning up the vSphere session", terminateVSphereSession)
	By("Tearing down the management cluster")
	if !skipCleanup {
		helpers.TearDown(bootstrapClusterProvider, bootstrapClusterProxy)
	}
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)
	_ = v1beta1.AddToScheme(sc)
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
