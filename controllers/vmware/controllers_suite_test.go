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

package vmware

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	reporterConfig := types.NewDefaultReporterConfig()
	if artifactFolder, exists := os.LookupEnv("ARTIFACTS"); exists {
		reporterConfig.JUnitReport = filepath.Join(artifactFolder, "junit.ginkgo.controllers_vmware.xml")
	}
	RunSpecs(t, "VMware Controller Suite", reporterConfig)
}

var (
	testEnv      *helpers.TestEnvironment
	clusterCache clustercache.ClusterCache
	ctx          = ctrl.SetupSignalHandler()
)

func TestMain(m *testing.M) {
	testEnv, clusterCache = setup(ctx)
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup(ctx context.Context) (*helpers.TestEnvironment, clustercache.ClusterCache) {
	utilruntime.Must(infrav1.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(vmwarev1.AddToScheme(scheme.Scheme))

	testEnv := helpers.NewTestEnvironment(ctx)

	secretCachingClient, err := client.New(testEnv.Manager.GetConfig(), client.Options{
		HTTPClient: testEnv.Manager.GetHTTPClient(),
		Cache: &client.CacheOptions{
			Reader: testEnv.Manager.GetCache(),
		},
	})
	if err != nil {
		panic("unable to create secret caching client")
	}

	clusterCache, err = clustercache.SetupWithManager(ctx, testEnv.Manager, clustercache.Options{
		SecretClient: secretCachingClient,
		Client: clustercache.ClientOptions{
			UserAgent: remote.DefaultClusterAPIUserAgent("testenv-manager"),
			Cache: clustercache.ClientCacheOptions{
				DisableFor: []client.Object{
					// Don't cache ConfigMaps & Secrets.
					&corev1.ConfigMap{},
					&corev1.Secret{},
				},
			},
		},
	}, controller.Options{MaxConcurrentReconciles: 10, SkipNameValidation: ptr.To(true)})
	if err != nil {
		panic(fmt.Sprintf("Unable to setup ClusterCache: %v", err))
	}
	go func() {
		<-ctx.Done()
		clusterCache.(interface{ Shutdown() }).Shutdown()
	}()

	controllerOpts := controller.Options{MaxConcurrentReconciles: 10, SkipNameValidation: ptr.To(true)}

	if err := AddServiceAccountProviderControllerToManager(ctx, testEnv.GetControllerManagerContext(), testEnv.Manager, clusterCache, controllerOpts); err != nil {
		panic(fmt.Sprintf("unable to setup ServiceAccount controller: %v", err))
	}
	if err := AddServiceDiscoveryControllerToManager(ctx, testEnv.GetControllerManagerContext(), testEnv.Manager, clusterCache, controllerOpts); err != nil {
		panic(fmt.Sprintf("unable to setup SvcDiscovery controller: %v", err))
	}

	go func() {
		fmt.Println("Starting the manager")
		if err := testEnv.StartManager(ctx); err != nil {
			panic(fmt.Sprintf("failed to start the envtest manager: %v", err))
		}
	}()
	<-testEnv.Manager.Elected()

	// wait for webhook port to be open prior to running tests
	testEnv.WaitForWebhooks()

	// create manager pod namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: manager.DefaultPodNamespace,
		},
	}
	if err := testEnv.Create(ctx, ns); err != nil {
		panic(fmt.Sprintf("unable to create controller namespace: %v", err))
	}

	return testEnv, clusterCache
}

func teardown() {
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop envtest: %v", err))
	}
}
