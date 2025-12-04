/*
Copyright 2025 The Kubernetes Authors.

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

package kubernetes

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GenerateWorkerFiles generates control plane files for the current pod.
// The implementation assumes this code to be run as init container in the control plane pod
// Note: we are using the manager instead of another binary for convenience (the manager is already built and packaged
// into an image that is published during the release process).
func GenerateWorkerFiles(ctx context.Context, client client.Client) error {
	log := ctrl.LoggerFrom(ctx)

	// Gets the info about current pod.
	podNamespace := os.Getenv("POD_NAMESPACE")
	podName := os.Getenv("POD_NAME")
	podIP := os.Getenv("POD_IP")

	// Gets some additional info about the cluster.
	clusterName := os.Getenv("CLUSTER_NAME")
	controlPlaneEndpointHost := os.Getenv("CONTROL_PLANE_ENDPOINT_HOST")
	clusterKey := types.NamespacedName{Namespace: podNamespace, Name: clusterName}

	virtualMachineName, err := os.Hostname()
	if err != nil {
		return err
	}

	log.Info("Generating files", "POD_NAME", podName, "POD_NAMESPACE", podNamespace, "POD_IP", podIP, "CLUSTER_NAME", clusterName, "CONTROL_PLANE_ENDPOINT_HOST", controlPlaneEndpointHost, "VIRTUAL_MACHINE_NAME", virtualMachineName)
	log.Info("Generating ca.crt, kubelet.conf, /var/lib/kubelet/config.yaml")

	ca, err := getKeyCertPair(ctx, client, clusterKey, secret.ClusterCA)
	if err != nil {
		return err
	}

	if err := ca.WriteCert("/etc/kubernetes/pki", "ca"); err != nil {
		return err
	}

	kubeletClient, err := ca.NewCertAndKey(kubeletClientCertificateConfig(virtualMachineName))
	if err != nil {
		return err
	}

	kubeletKubeConfig := api.Config{
		Clusters: map[string]*api.Cluster{
			clusterName: {
				Server:                   fmt.Sprintf("https://%s:%d", controlPlaneEndpointHost, 6443),
				CertificateAuthorityData: certs.EncodeCertPEM(ca.cert),
			},
		},
		Contexts: map[string]*api.Context{
			clusterName: {
				Cluster:  clusterName,
				AuthInfo: "kubelet",
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			"kubelet": {
				ClientKeyData:         certs.EncodePrivateKeyPEM(kubeletClient.key),
				ClientCertificateData: certs.EncodeCertPEM(kubeletClient.cert),
			},
		},
		CurrentContext: clusterName,
	}
	if err := clientcmd.WriteToFile(kubeletKubeConfig, "/etc/kubernetes/kubelet.conf"); err != nil {
		return errors.Wrap(err, "failed to write kubelet kubeconfig")
	}

	if err := writeFile("/etc/systemd/system/kubelet.service.d/10-kubeadm.conf", kubeadmSystemdDropIn, 0644); err != nil {
		return errors.Wrap(err, "failed to write kubelet 10-kubeadm.conf")
	}

	if err := writeFile("/var/lib/kubelet/config.yaml", kubeletConfig, 0644); err != nil {
		return errors.Wrap(err, "failed to write kubelet config.yaml")
	}

	adminClient, err := ca.NewCertAndKey(adminClientCertificateConfig())
	if err != nil {
		return errors.Wrap(err, "failed to create admin client certificate")
	}

	adminKubeConfig := api.Config{
		Clusters: map[string]*api.Cluster{
			clusterKey.Name: {
				Server:                   fmt.Sprintf("https://%s:6443", controlPlaneEndpointHost),
				CertificateAuthorityData: certs.EncodeCertPEM(ca.cert),
			},
		},
		Contexts: map[string]*api.Context{
			clusterKey.Name: {
				Cluster:  clusterKey.Name,
				AuthInfo: "admin",
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			"admin": {
				ClientKeyData:         certs.EncodePrivateKeyPEM(adminClient.key),
				ClientCertificateData: certs.EncodeCertPEM(adminClient.cert),
			},
		},
		CurrentContext: clusterKey.Name,
	}
	if err := clientcmd.WriteToFile(adminKubeConfig, "/etc/kubernetes/admin.conf"); err != nil {
		return errors.Wrap(err, "failed to write admin kubeconfig")
	}

	log.Info("All file generated!")
	return nil
}

func writeFile(filename string, data string, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(filename, []byte(data), perm)
}

func kubeletClientCertificateConfig(nodeName string) *certs.Config {
	return &certs.Config{
		CommonName:   fmt.Sprintf("system:node:%s", nodeName),
		Organization: []string{"system:nodes"},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

// /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
// without --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf
var kubeadmSystemdDropIn = `# https://github.com/kubernetes/kubernetes/blob/ba8fcafaf8c502a454acd86b728c857932555315/build/debs/10-kubeadm.conf
# Note: This dropin only works with kubeadm and kubelet v1.11+
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/admin.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
# This is a file that "kubeadm init" and "kubeadm join" generates at runtime, populating the KUBELET_KUBEADM_ARGS variable dynamically
EnvironmentFile=-/var/lib/kubelet/kubeadm-flags.env
# This is a file that the user can use for overrides of the kubelet args as a last resort. Preferably, the user should use
# the .NodeRegistration.KubeletExtraArgs object in the configuration files instead. KUBELET_EXTRA_ARGS should be sourced from this file.
EnvironmentFile=-/etc/default/kubelet
ExecStart=
ExecStart=/usr/bin/kubelet $KUBELET_KUBECONFIG_ARGS $KUBELET_CONFIG_ARGS $KUBELET_KUBEADM_ARGS $KUBELET_EXTRA_ARGS
`

// /var/lib/kubelet/config.yaml
var kubeletConfig = `apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 0s
    cacheUnauthorizedTTL: 0s
cgroupDriver: systemd
cgroupRoot: /kubelet
clusterDNS:
- 10.96.0.10
clusterDomain: cluster.local
containerRuntimeEndpoint: ""
cpuManagerReconcilePeriod: 0s
crashLoopBackOff: {}
evictionHard:
  imagefs.available: 0%
  nodefs.available: 0%
  nodefs.inodesFree: 0%
evictionPressureTransitionPeriod: 0s
failSwapOn: false
fileCheckFrequency: 0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 0s
imageGCHighThresholdPercent: 100
imageMaximumGCAge: 0s
imageMinimumGCAge: 0s
kind: KubeletConfiguration
logging:
  flushFrequency: 0
  options:
    json:
      infoBufferSize: "0"
    text:
      infoBufferSize: "0"
  verbosity: 0
memorySwap: {}
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 0s
rotateCertificates: true
runtimeRequestTimeout: 0s
shutdownGracePeriod: 0s
shutdownGracePeriodCriticalPods: 0s
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 0s
syncFrequency: 0s
volumeStatsAggPeriod: 0s
`
