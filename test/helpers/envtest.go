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

package helpers

import (
	goctx "context"
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"

	"github.com/onsi/ginkgo"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"

	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

func init() {
	klog.InitFlags(nil)
	logger := klogr.New()

	// use klog as the internal logger for this envtest environment.
	log.SetLogger(logger)
	// additionally force all of the controllers to use the Ginkgo logger.
	ctrl.SetLogger(logger)
	// add logger for ginkgo
	klog.SetOutput(ginkgo.GinkgoWriter)
}

var (
	scheme                 = runtime.NewScheme()
	env                    *envtest.Environment
	clusterAPIVersionRegex = regexp.MustCompile(`^(\W)sigs.k8s.io/cluster-api v(.+)`)
)

func init() {
	// Calculate the scheme.
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(admissionv1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(infrav1.AddToScheme(scheme))
	utilruntime.Must(infrav1alpha3.AddToScheme(scheme))

	// Get the root of the current file to use in CRD paths.
	_, filename, _, _ := goruntime.Caller(0) //nolint
	root := path.Join(path.Dir(filename), "..", "..")

	crdPaths := []string{
		filepath.Join(root, "config", "crd", "bases"),
	}

	// append CAPI CRDs path
	if capiPath := getFilePathToCAPICRDs(root); capiPath != "" {
		crdPaths = append(crdPaths, capiPath)
	}

	// Create the test environment.
	env = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     crdPaths,
	}
}

type (
	// TestEnvironment encapsulates a Kubernetes local test environment.
	TestEnvironment struct {
		manager.Manager
		client.Client
		Config *rest.Config

		cancel goctx.CancelFunc
	}
)

// NewTestEnvironment creates a new environment spinning up a local api-server.
func NewTestEnvironment() *TestEnvironment {
	// initialize webhook here to be able to test the envtest install via webhookOptions
	initializeWebhookInEnvironment()

	if _, err := env.Start(); err != nil {
		err = kerrors.NewAggregate([]error{err, env.Stop()})
		panic(err)
	}

	managerOpts := manager.Options{
		Scheme:      scheme,
		MetricsAddr: "0",
		WebhookPort: env.WebhookInstallOptions.LocalServingPort,
		CertDir:     env.WebhookInstallOptions.LocalServingCertDir,
		KubeConfig:  env.Config,
		// TODO (srm09): might need to supply some mock for
		// 		vCenter interactions
		Username: "blah",
		Password: "blah2",
	}
	managerOpts.AddToManager = func(ctx *context.ControllerManagerContext, mgr ctrlmgr.Manager) error {
		if err := (&infrav1.VSphereCluster{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}
		if err := (&infrav1.VSphereClusterList{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}

		if err := (&infrav1.VSphereMachine{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}
		if err := (&infrav1.VSphereMachineList{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}

		if err := (&infrav1.VSphereMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}
		if err := (&infrav1.VSphereMachineTemplateList{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}

		if err := (&infrav1.VSphereVM{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}
		if err := (&infrav1.VSphereVMList{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}

		return nil
	}

	mgr, err := manager.New(managerOpts)
	if err != nil {
		klog.Fatalf("failed to create the CAPV controller manager: %v", err)
	}

	return &TestEnvironment{
		Manager: mgr,
		Client:  mgr.GetClient(),
		Config:  mgr.GetConfig(),
	}
}

func (t *TestEnvironment) StartManager(ctx goctx.Context) error {
	ctx, cancel := goctx.WithCancel(ctx)
	t.cancel = cancel
	return t.Manager.Start(ctx)
}

func (t *TestEnvironment) Stop() error {
	t.cancel()
	return env.Stop()
}

func getFilePathToCAPICRDs(root string) string {
	modBits, err := ioutil.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}

	var clusterAPIVersion string
	for _, line := range strings.Split(string(modBits), "\n") {
		matches := clusterAPIVersionRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			clusterAPIVersion = matches[2]
		}
	}

	if clusterAPIVersion == "" {
		return ""
	}

	gopath := envOr("GOPATH", build.Default.GOPATH)
	return filepath.Join(gopath, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api@v%s", clusterAPIVersion), "config", "crd", "bases")
}

func envOr(envKey, defaultValue string) string {
	if value, ok := os.LookupEnv(envKey); ok {
		return value
	}
	return defaultValue
}
