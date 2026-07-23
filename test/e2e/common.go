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

// Package e2e contains end to end test code and utils.
package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vapi/rest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KubernetesVersion                   = "KUBERNETES_VERSION"
	KubernetesVersionChainedUpgradeFrom = "KUBERNETES_VERSION_CHAINED_UPGRADE_FROM"
	KubernetesVersionUpgradeTo          = "KUBERNETES_VERSION_UPGRADE_TO"
)

type InfraClients struct {
	Client     *govmomi.Client
	RestClient *rest.Client
	Finder     *find.Finder
}

type GlobalInput struct {
	ArtifactFolder        string
	ClusterctlConfigPath  string
	BootstrapClusterProxy framework.ClusterProxy
	E2EConfig             *clusterctl.E2EConfig
}

func defaultConfigCluster(clusterName, namespace, flavor string, controlPlaneNodeCount, workerNodeCount int64,
	input GlobalInput) clusterctl.ConfigClusterInput {
	configClusterInput := clusterctl.ConfigClusterInput{
		LogFolder:                filepath.Join(input.ArtifactFolder, "clusters", input.BootstrapClusterProxy.GetName()),
		ClusterctlConfigPath:     input.ClusterctlConfigPath,
		KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
		InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
		Flavor:                   clusterctl.DefaultFlavor,
		Namespace:                namespace,
		ClusterName:              clusterName,
		KubernetesVersion:        input.E2EConfig.MustGetVariable(KubernetesVersion),
		ControlPlaneMachineCount: ptr.To(controlPlaneNodeCount),
		WorkerMachineCount:       ptr.To(workerNodeCount),
	}
	if flavor != "" {
		configClusterInput.Flavor = flavor
	}
	return configClusterInput
}

func watchCPIAndCSILogs(ctx context.Context, managementClusterProxy framework.ClusterProxy, namespace string, artifactFolder string) {
	defer ginkgo.GinkgoRecover()

	// Wait for a Cluster to be created in the namespace
	var clusterName string
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		clusters := &clusterv1beta1.ClusterList{}
		if err := managementClusterProxy.GetClient().List(ctx, clusters, client.InNamespace(namespace)); err != nil {
			return false, err
		}
		if len(clusters.Items) > 0 {
			clusterName = clusters.Items[0].Name
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		// No cluster created in this namespace or timed out, nothing to watch
		return
	}

	// Wait for the kubeconfig secret to be available
	secretName := fmt.Sprintf("%s-kubeconfig", clusterName)
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		secret := &corev1.Secret{}
		if err := managementClusterProxy.GetClient().Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, secret); err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		// Kubeconfig never became available
		return
	}

	// Now we can get the workload cluster proxy
	workloadProxy := managementClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)

	// Stream CPI logs
	framework.WatchDaemonSetLogsByLabelSelector(ctx, framework.WatchDaemonSetLogsByLabelSelectorInput{
		GetLister: workloadProxy.GetClient(),
		Cache:     workloadProxy.GetCache(ctx),
		ClientSet: workloadProxy.GetClientSet(),
		Labels: map[string]string{
			"component": "cloud-controller-manager",
		},
		LogPath: filepath.Join(artifactFolder, "clusters", clusterName, "logs"),
	})

	// Stream CSI Deployment logs
	framework.WatchDeploymentLogsByLabelSelector(ctx, framework.WatchDeploymentLogsByLabelSelectorInput{
		GetLister: workloadProxy.GetClient(),
		Cache:     workloadProxy.GetCache(ctx),
		ClientSet: workloadProxy.GetClientSet(),
		Labels: map[string]string{
			"app": "vsphere-csi-controller",
		},
		LogPath: filepath.Join(artifactFolder, "clusters", clusterName, "logs"),
	})

	// Stream CSI Daemonset logs
	framework.WatchDaemonSetLogsByLabelSelector(ctx, framework.WatchDaemonSetLogsByLabelSelectorInput{
		GetLister: workloadProxy.GetClient(),
		Cache:     workloadProxy.GetCache(ctx),
		ClientSet: workloadProxy.GetClientSet(),
		Labels: map[string]string{
			"app": "vsphere-csi-node",
		},
		LogPath: filepath.Join(artifactFolder, "clusters", clusterName, "logs"),
	})
}
