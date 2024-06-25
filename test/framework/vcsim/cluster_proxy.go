/*
Copyright 2024 The Kubernetes Authors.

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

// Package vcsim provide helpers for vcsim controller.
package vcsim

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/test/framework"
	inmemoryproxy "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server/proxy"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	retryableOperationInterval = 3 * time.Second
	// retryableOperationTimeout requires a higher value especially for self-hosted upgrades.
	// Short unavailability of the Kube APIServer due to joining etcd members paired with unreachable conversion webhooks due to
	// failed leader election and thus controller restarts lead to longer taking retries.
	// The timeout occurs when listing machines in `GetControlPlaneMachinesByCluster`.
	retryableOperationTimeout = 3 * time.Minute

	initialCacheSyncTimeout = time.Minute
)

type vcSimManagementClusterProxy struct {
	framework.ClusterProxy
}

// NewClusterProxy creates a ClusterProxy for usage with VCSim.
func NewClusterProxy(name string, kubeconfigPath string, scheme *runtime.Scheme, options ...framework.Option) framework.ClusterProxy {
	return &vcSimManagementClusterProxy{
		ClusterProxy: framework.NewClusterProxy(name, kubeconfigPath, scheme, options...),
	}
}

// GetWorkloadCluster returns ClusterProxy for the workload cluster.
func (p *vcSimManagementClusterProxy) GetWorkloadCluster(ctx context.Context, namespace, name string, options ...framework.Option) framework.ClusterProxy {
	wlProxy := p.ClusterProxy.GetWorkloadCluster(ctx, namespace, name, options...)

	// Get the vcSim pod information.
	pods := corev1.PodList{}
	Expect(p.GetClient().List(context.Background(), &pods, client.InNamespace("vcsim-system"))).To(Succeed())
	Expect(pods.Items).To(HaveLen(1), "expecting to run vcsim with a single replica")
	vcSimPod := pods.Items[0]

	// Get the target port number from the restconfig of the workload cluster.
	u, err := url.Parse(wlProxy.GetRESTConfig().Host)
	Expect(err).ToNot(HaveOccurred())

	port, err := strconv.Atoi(u.Port())
	Expect(err).ToNot(HaveOccurred())

	// Create a dialer which proxies through the kube-apiserver to the vcsim pod's port where the simulated kube-apiserver
	// is running.
	d, err := inmemoryproxy.NewDialer(inmemoryproxy.Proxy{
		Kind:         "pods",
		Namespace:    vcSimPod.GetNamespace(),
		ResourceName: vcSimPod.GetName(),
		Port:         port,
		KubeConfig:   p.GetRESTConfig(),
	})
	Expect(err).ToNot(HaveOccurred())

	dialFunc := func(ctx context.Context, _, _ string) (net.Conn, error) {
		// Always use vcSimPodName as url to have a successful port forward.
		return d.DialContext(ctx, "", vcSimPod.GetName())
	}

	return &vcSimWorkloadClusterProxy{
		realProxy: wlProxy,
		dialFunc:  dialFunc,
	}
}

type vcSimWorkloadClusterProxy struct {
	realProxy framework.ClusterProxy
	dialFunc  func(ctx context.Context, _, _ string) (net.Conn, error)

	cache     cache.Cache
	onceCache sync.Once
}

func (p *vcSimWorkloadClusterProxy) GetName() string {
	return p.realProxy.GetName()
}

func (p *vcSimWorkloadClusterProxy) GetKubeconfigPath() string {
	return p.realProxy.GetKubeconfigPath()
}

func (p *vcSimWorkloadClusterProxy) GetScheme() *runtime.Scheme {
	return p.realProxy.GetScheme()
}

func (p *vcSimWorkloadClusterProxy) GetClient() client.Client {
	config := p.GetRESTConfig()

	var c client.Client
	var newClientErr error
	err := wait.PollUntilContextTimeout(context.TODO(), retryableOperationInterval, retryableOperationTimeout, true, func(context.Context) (bool, error) {
		c, newClientErr = client.New(config, client.Options{Scheme: p.realProxy.GetScheme()})
		if newClientErr != nil {
			return false, nil //nolint:nilerr
		}
		return true, nil
	})
	errorString := "Failed to get controller-runtime client"
	Expect(newClientErr).ToNot(HaveOccurred(), errorString)
	Expect(err).ToNot(HaveOccurred(), errorString)

	return c
}

func (p *vcSimWorkloadClusterProxy) GetClientSet() *kubernetes.Clientset {
	restConfig := p.GetRESTConfig()

	cs, err := kubernetes.NewForConfig(restConfig)
	Expect(err).ToNot(HaveOccurred(), "Failed to get client-go client")

	return cs
}

func (p *vcSimWorkloadClusterProxy) GetRESTConfig() *rest.Config {
	config := p.realProxy.GetRESTConfig()
	config.Dial = p.dialFunc

	return config
}

func (p *vcSimWorkloadClusterProxy) GetCache(ctx context.Context) cache.Cache {
	p.onceCache.Do(func() {
		var err error
		p.cache, err = cache.New(p.GetRESTConfig(), cache.Options{
			Scheme: p.GetScheme(),
			Mapper: p.GetClient().RESTMapper(),
		})
		Expect(err).ToNot(HaveOccurred(), "Failed to create controller-runtime cache")

		go func() {
			defer GinkgoRecover()
			Expect(p.cache.Start(ctx)).To(Succeed())
		}()

		cacheSyncCtx, cacheSyncCtxCancel := context.WithTimeout(ctx, initialCacheSyncTimeout)
		defer cacheSyncCtxCancel()
		Expect(p.cache.WaitForCacheSync(cacheSyncCtx)).
			To(BeTrue(), fmt.Sprintf("failed waiting for cache for cluster proxy to sync: %v", ctx.Err()))
	})

	return p.cache
}

func (p *vcSimWorkloadClusterProxy) GetLogCollector() framework.ClusterLogCollector {
	// There are no logs in simulated clusters.
	return nil
}

func (p *vcSimWorkloadClusterProxy) Apply(_ context.Context, _ []byte, _ ...string) error {
	return fmt.Errorf("not supported")
}

func (p *vcSimWorkloadClusterProxy) GetWorkloadCluster(_ context.Context, _, _ string, _ ...framework.Option) framework.ClusterProxy {
	Expect(fmt.Errorf("simulated workload clusters can't have nested workload clusters")).ToNot(HaveOccurred())
	return nil
}

func (p *vcSimWorkloadClusterProxy) CollectWorkloadClusterLogs(_ context.Context, _, _, _ string) {
	// There are no logs in simulated clusters.
}

func (p *vcSimWorkloadClusterProxy) Dispose(ctx context.Context) {
	p.realProxy.Dispose(ctx)
}
