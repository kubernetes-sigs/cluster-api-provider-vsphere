/*
Copyright 2021 The Kubernetes Authors.

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

package test

import (
	goctx "context"
	"encoding/json"
	"os/exec"
	"path"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/controllers"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

var (
	k8sClient     client.Client
	testEnv       *envtest.Environment
	ctx, cancel   = goctx.WithCancel(goctx.Background())
	isLB          = false
	clusterAPIDir = findModuleDir("sigs.k8s.io/cluster-api")
)

func init() {
	klog.InitFlags(nil)
	klog.SetOutput(GinkgoWriter)
	logf.SetLogger(klogr.New())
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"VMware Controllers Suite",
		[]Reporter{printer.NewlineReporter{}})
}

// TODO: can't call RunSpecs twice. Need to parameterize this externally and call the test suite twice
// func TestAPIsLB(t *testing.T) {
// 	isLB = true
// 	RegisterFailHandler(Fail)

// 	RunSpecsWithDefaultAndCustomReporters(t,
// 		"VMware Controller Suite with LB network provider",
// 		[]Reporter{printer.NewlineReporter{}})
// }

func getTestEnv() (*envtest.Environment, *rest.Config) {
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(vmwarev1.AddToScheme(scheme.Scheme))

	// Get the root of the current file to use in CRD paths.
	_, filename, _, _ := goruntime.Caller(0) //nolint
	root := path.Join(path.Dir(filename), "..", "..", "..")

	localTestEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(root, "config", "supervisor", "crd"),
			filepath.Join(root, "config", "deployments", "integration-tests", "crds"),
			filepath.Join(clusterAPIDir, "config", "crd", "bases"),
		},
	}

	localCfg, err := localTestEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(localCfg).ToNot(BeNil())
	return localTestEnv, localCfg
}

func getManager(cfg *rest.Config, networkProvider string) manager.Manager {
	opts := manager.Options{
		Options: ctrlmgr.Options{
			Scheme: scheme.Scheme,
			NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
				syncPeriod := 1 * time.Second
				opts.Resync = &syncPeriod
				return cache.New(config, opts)
			},
		},
		KubeConfig:      cfg,
		NetworkProvider: networkProvider,
	}

	opts.AddToManager = func(ctx *context.ControllerManagerContext, mgr ctrlmgr.Manager) error {
		if err := controllers.AddClusterControllerToManager(ctx, mgr, &vmwarev1.VSphereCluster{}); err != nil {
			return err
		}

		if err := controllers.AddMachineControllerToManager(ctx, mgr, &vmwarev1.VSphereMachine{}); err != nil {
			return err
		}
		return nil
	}

	mgr, err := manager.New(opts)
	Expect(err).NotTo(HaveOccurred())
	return mgr
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environments")
	var cfg *rest.Config
	testEnv, cfg = getTestEnv()
	networkProvider := ""
	if isLB {
		networkProvider = manager.DummyLBNetworkProvider
	}

	By("setting up a new manager")
	mgr := getManager(cfg, networkProvider)
	k8sClient = mgr.GetClient()

	By("starting the manager")
	go func() {
		Expect(mgr.Start(ctx)).ToNot(HaveOccurred())
	}()
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environments")
	cancel()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func findModuleDir(module string) string {
	cmd := exec.Command("go", "mod", "download", "-json", module)
	out, err := cmd.Output()
	if err != nil {
		klog.Fatalf("Failed to run go mod to find module %q directory", module)
	}
	info := struct{ Dir string }{}
	if err := json.Unmarshal(out, &info); err != nil {
		klog.Fatalf("Failed to unmarshal output from go mod command: %v", err)
	} else if info.Dir == "" {
		klog.Fatalf("Failed to find go module %q directory, received %v", module, string(out))
	}
	return info.Dir
}
