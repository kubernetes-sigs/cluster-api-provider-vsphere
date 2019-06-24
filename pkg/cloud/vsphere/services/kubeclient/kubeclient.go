/*
Copyright 2019 The Kubernetes Authors.

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

package kubeclient

import (
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	capierr "sigs.k8s.io/cluster-api/pkg/controller/error"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/certificates"
)

// GetKubeConfig returns the kubeconfig for the given cluster.
func GetKubeConfig(ctx context.KubeContext) (string, error) {
	cert, err := certificates.DecodeCertPEM(ctx.ClusterProviderConfig().CAKeyPair.Cert)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode CA Cert")
	} else if cert == nil {
		return "", errors.New("certificate not found in config")
	}

	key, err := certificates.DecodePrivateKeyPEM(ctx.ClusterProviderConfig().CAKeyPair.Key)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode private key")
	} else if key == nil {
		return "", errors.New("key not found in status")
	}

	controlPlaneEndpoint, err := ctx.ControlPlaneEndpoint()
	if err != nil {
		return "", err
	}

	server := fmt.Sprintf("https://%s", controlPlaneEndpoint)

	cfg, err := certificates.NewKubeconfig(ctx.ClusterName(), server, cert, key)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate a kubeconfig")
	}

	yaml, err := clientcmd.Write(*cfg)
	if err != nil {
		return "", errors.Wrap(err, "failed to serialize config to yaml")
	}

	return string(yaml), nil
}

// GetControlPlaneStatus returns a flag indicating whether or not the cluster
// is online.
// If the flag is true then the second return value is the cluster's control
// plane endpoint.
func GetControlPlaneStatus(ctx context.KubeContext) (bool, string, error) {
	controlPlaneEndpoint, err := getControlPlaneStatus(ctx)
	if err != nil {
		return false, "", errors.Wrapf(
			&capierr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue},
			"unable to get control plane status for cluster %q: %v", ctx, err)
	}
	return true, controlPlaneEndpoint, nil
}

func getControlPlaneStatus(ctx context.KubeContext) (string, error) {
	kubeClient, err := GetKubeClientForCluster(ctx)
	if err != nil {
		return "", err
	}
	if _, err := kubeClient.Nodes().List(metav1.ListOptions{}); err != nil {
		return "", errors.Wrapf(err, "unable to list nodes")
	}
	return ctx.ControlPlaneEndpoint()
}

// GetKubeClientForCluster returns a Kubernetes client for the given cluster.
func GetKubeClientForCluster(ctx context.KubeContext) (corev1.CoreV1Interface, error) {
	kubeconfig, err := GetKubeConfig(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get kubeconfig for cluster %q", ctx)
	}
	clientConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to create client config for cluster %q", ctx)
	}
	return corev1.NewForConfig(clientConfig)
}
