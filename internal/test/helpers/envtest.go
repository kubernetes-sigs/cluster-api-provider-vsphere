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

// Package helpers contains helpers for creating a test environment.
package helpers

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/simulator"
	"golang.org/x/tools/go/packages"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/webhooks"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

func init() {
	// Set log level 5 as default for testing (default for prod is 2).
	logOptions := logs.NewOptions()
	logOptions.Verbosity = 5
	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		panic(err)
	}
	ctrl.SetLogger(klog.Background())

	// add logger for ginkgo
	klog.SetOutput(ginkgo.GinkgoWriter)
}

var (
	scheme   = runtime.NewScheme()
	crdPaths []string
)

func init() {
	// Calculate the scheme.
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(admissionv1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(infrav1.AddToScheme(scheme))

	// Get the root of the current file to use in CRD paths.
	_, filename, _, ok := goruntime.Caller(0)
	if !ok {
		klog.Fatalf("Failed to get information for current file from runtime")
	}
	root := path.Join(path.Dir(filename), "..", "..", "..")

	crdPaths = []string{
		filepath.Join(root, "config", "default", "crd", "bases"),
		filepath.Join(root, "config", "supervisor", "crd", "bases"),
	}

	// append CAPI CRDs path
	if capiPaths := getFilePathToCAPICRDs(); capiPaths != nil {
		crdPaths = append(crdPaths, capiPaths...)
	}
}

var (
	cacheSyncBackoff = wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1.5,
		Steps:    8,
		Jitter:   0.4,
	}
)

type (
	// TestEnvironment encapsulates a Kubernetes local test environment.
	TestEnvironment struct {
		manager.Manager
		client.Client
		Config    *rest.Config
		Simulator *vcsim.Simulator
		env       *envtest.Environment

		cancel context.CancelFunc
	}
)

// NewTestEnvironment creates a new environment spinning up a local api-server.
func NewTestEnvironment(ctx context.Context) *TestEnvironment {
	// Create the test environment.
	env := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     crdPaths,
	}

	// Get the root of the current file to use in CRD paths.
	_, filename, _, ok := goruntime.Caller(0)
	if !ok {
		klog.Fatalf("Failed to get information for current file from runtime")
	}
	root := path.Join(path.Dir(filename), "..", "..", "..")
	configPath := filepath.Join(root, "config", "govmomi", "webhook", "manifests.yaml")

	// initialize webhook here to be able to test the envtest install via webhookOptions
	InitializeWebhookInEnvironment(env, configPath)

	if _, err := env.Start(); err != nil {
		err = kerrors.NewAggregate([]error{err, env.Stop()})
		panic(err)
	}

	model := simulator.VPX()
	model.Pool = 1
	simr, err := vcsim.NewBuilder().
		WithModel(model).
		Build()
	if err != nil {
		klog.Fatalf("unable to start vc simulator %s", err)
	}
	// Localhost is used on MacOS to avoid Firewall warning popups.
	host := "localhost"
	if strings.EqualFold(os.Getenv("USE_EXISTING_CLUSTER"), "true") {
		// 0.0.0.0 is required on Linux when using kind because otherwise the kube-apiserver running in kind
		// is unable to reach the webhook, because the webhook would be only listening on 127.0.0.1.
		// Somehow that's not an issue on MacOS.
		if goruntime.GOOS == "linux" {
			host = "0.0.0.0"
		}
	}

	managerOpts := manager.Options{
		Options: ctrl.Options{
			Controller: config.Controller{
				UsePriorityQueue: ptr.To[bool](feature.Gates.Enabled(feature.PriorityQueue)),
			},
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: "0",
			},
			WebhookServer: webhook.NewServer(
				webhook.Options{
					Port:    env.WebhookInstallOptions.LocalServingPort,
					CertDir: env.WebhookInstallOptions.LocalServingCertDir,
					Host:    host,
				},
			),
		},
		KubeConfig: env.Config,
		Username:   simr.Username(),
		Password:   simr.Password(),
	}
	managerOpts.AddToManager = func(_ context.Context, _ *capvcontext.ControllerManagerContext, mgr ctrlmgr.Manager) error {
		if err := (&webhooks.VSphereClusterTemplateWebhook{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}

		if err := (&webhooks.VSphereMachineWebhook{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}

		if err := (&webhooks.VSphereMachineTemplateWebhook{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}

		if err := (&webhooks.VSphereVMWebhook{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}

		if err := (&webhooks.VSphereDeploymentZoneWebhook{}).SetupWebhookWithManager(mgr); err != nil {
			return err
		}

		return (&webhooks.VSphereFailureDomainWebhook{}).SetupWebhookWithManager(mgr)
	}

	mgr, err := manager.New(ctx, managerOpts)
	if err != nil {
		klog.Fatalf("failed to create the CAPV controller manager: %v", err)
	}

	return &TestEnvironment{
		Manager:   mgr,
		Client:    mgr.GetClient(),
		Config:    mgr.GetConfig(),
		Simulator: simr,
		env:       env,
	}
}

// StartManager starts the TestEnvironment's controller runtime manager.
func (t *TestEnvironment) StartManager(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	return t.Manager.Start(ctx)
}

// Stop stops the TestEnvironment's controller runtime manager.
func (t *TestEnvironment) Stop() error {
	t.cancel()
	t.Simulator.Destroy()
	return t.env.Stop()
}

// Cleanup removes objects from the TestEnvironment.
func (t *TestEnvironment) Cleanup(ctx context.Context, objs ...client.Object) error {
	errs := make([]error, 0, len(objs))
	for _, o := range objs {
		err := t.Client.Delete(ctx, o)
		if apierrors.IsNotFound(err) {
			// If the object is not found, it must've been garbage collected
			// already. For example, if we delete namespace first and then
			// objects within it.
			continue
		}
		errs = append(errs, err)
	}
	return kerrors.NewAggregate(errs)
}

// CleanupAndWait removes objects from the TestEnvironment and waits for them to be gone.
func (t *TestEnvironment) CleanupAndWait(ctx context.Context, objs ...client.Object) error {
	if err := t.Cleanup(ctx, objs...); err != nil {
		return err
	}

	// Makes sure the cache is updated with the deleted object
	errs := []error{}
	for _, o := range objs {
		// Ignoring namespaces because in testenv the namespace cleaner is not running.
		if _, ok := o.(*corev1.Namespace); ok {
			continue
		}

		oCopy := o.DeepCopyObject().(client.Object)
		key := client.ObjectKeyFromObject(o)
		err := wait.ExponentialBackoff(
			cacheSyncBackoff,
			func() (done bool, err error) {
				if err := t.Get(ctx, key, oCopy); err != nil {
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					return false, err
				}
				return false, nil
			})
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "key %s, %s is not being deleted from the testenv client cache", o.GetObjectKind().GroupVersionKind().String(), key))
		}
	}
	return kerrors.NewAggregate(errs)
}

// CreateNamespace creates a new namespace in the TestEnvironment.
func (t *TestEnvironment) CreateNamespace(ctx context.Context, generateName string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", generateName),
			Labels: map[string]string{
				"testenv/original-name": generateName,
			},
		},
	}
	if err := t.Client.Create(ctx, ns); err != nil {
		return nil, err
	}

	return ns, nil
}

// CreateKubeconfigSecret creates a secret with the KubeConfig to access a cluster in the TestEnvironment.
func (t *TestEnvironment) CreateKubeconfigSecret(ctx context.Context, cluster *clusterv1.Cluster) error {
	return t.Create(ctx, kubeconfig.GenerateSecret(cluster, kubeconfig.FromEnvTestConfig(t.Config, cluster)))
}

func getFilePathToCAPICRDs() []string {
	packageName := "sigs.k8s.io/cluster-api"
	packageConfig := &packages.Config{
		Mode: packages.NeedModule,
	}

	pkgs, err := packages.Load(packageConfig, packageName)
	if err != nil {
		return nil
	}

	pkg := pkgs[0]

	return []string{
		filepath.Join(pkg.Module.Dir, "config", "crd", "bases"),
		filepath.Join(pkg.Module.Dir, "controlplane", "kubeadm", "config", "crd", "bases"),
	}
}
