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
	"net"
	"net/url"
	"strconv"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	inmemoryproxy "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server/proxy"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type vcSimClusterProxy struct {
	framework.ClusterProxy
	isWorkloadCluster bool
}

// NewClusterProxy creates a ClusterProxy for usage with VCSim.
func NewClusterProxy(name string, kubeconfigPath string, scheme *runtime.Scheme, options ...framework.Option) framework.ClusterProxy {
	return &vcSimClusterProxy{
		ClusterProxy: framework.NewClusterProxy(name, kubeconfigPath, scheme, options...),
	}
}

// GetWorkloadCluster returns ClusterProxy for the workload cluster.
func (p *vcSimClusterProxy) GetWorkloadCluster(ctx context.Context, namespace, name string, options ...framework.Option) framework.ClusterProxy {
	// Get the vcSim pod information.
	pods := corev1.PodList{}
	Expect(p.GetClient().List(context.Background(), &pods, client.InNamespace("vcsim-system"))).To(Succeed())
	Expect(pods.Items).To(HaveLen(1), "expecting to run vcsim with a single replica")
	vcSimPod := pods.Items[0]

	// Get the port on the vcsim pod from the RESTConfig of the workload cluster.
	restConfig := p.ClusterProxy.GetWorkloadCluster(ctx, namespace, name).GetRESTConfig()
	u, err := url.Parse(restConfig.Host)
	Expect(err).ToNot(HaveOccurred())
	port, err := strconv.Atoi(u.Port())
	Expect(err).ToNot(HaveOccurred())

	// Create a dialer which proxies through the kube-apiserver to the vcsim pod's port where the simulated kube-apiserver
	// is running.
	proxyDialer, err := inmemoryproxy.NewDialer(inmemoryproxy.Proxy{
		Kind:         "pods",
		Namespace:    vcSimPod.GetNamespace(),
		ResourceName: vcSimPod.GetName(),
		Port:         port,
		KubeConfig:   p.GetRESTConfig(),
	})
	Expect(err).ToNot(HaveOccurred())

	modifierFunc := func(c *rest.Config) {
		c.Dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
			// Always use vcSimPodName as addr.
			return proxyDialer.DialContext(ctx, "", vcSimPod.GetName())
		}
	}

	return &vcSimClusterProxy{
		ClusterProxy:      p.ClusterProxy.GetWorkloadCluster(ctx, namespace, name, append(options, framework.WithRESTConfigModifier(modifierFunc))...),
		isWorkloadCluster: true,
	}
}

// GetLogCollector returns the machine log collector for the Kubernetes cluster.
func (p *vcSimClusterProxy) GetLogCollector() framework.ClusterLogCollector {
	if p.isWorkloadCluster {
		return &noopLogCollector{}
	}
	return p.ClusterProxy.GetLogCollector()
}

// CollectWorkloadClusterLogs collects machines and infrastructure logs from the workload cluster.
func (p *vcSimClusterProxy) CollectWorkloadClusterLogs(ctx context.Context, namespace, name, outputPath string) {
	if p.isWorkloadCluster {
		// Workload Clusters using VCSim do not have real backing nodes so we can't collect logs.
		return
	}
	p.ClusterProxy.CollectWorkloadClusterLogs(ctx, namespace, name, outputPath)
}

type noopLogCollector struct{}

func (*noopLogCollector) CollectMachineLog(_ context.Context, _ client.Client, _ *clusterv1.Machine, _ string) error {
	return nil
}
func (*noopLogCollector) CollectMachinePoolLog(_ context.Context, _ client.Client, _ *clusterv1.MachinePool, _ string) error {
	return nil
}
func (*noopLogCollector) CollectInfrastructureLogs(_ context.Context, _ client.Client, _ *clusterv1.Cluster, _ string) error {
	return nil
}
