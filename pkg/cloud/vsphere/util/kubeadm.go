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

package util

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	bootstrapv1 "sigs.k8s.io/cluster-api-bootstrap-provider-kubeadm/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

// GetKubeadmConfigForMachine gets a CAPI Machine's associated KubeadmConfig
// resource.
func GetKubeadmConfigForMachine(
	ctx context.Context,
	controllerClient client.Client,
	machine *clusterv1.Machine) (*bootstrapv1.KubeadmConfig, error) {

	kubeadmConfig := &bootstrapv1.KubeadmConfig{}
	kubeadmConfigKey := client.ObjectKey{
		Name:      machine.Spec.Bootstrap.ConfigRef.Name,
		Namespace: machine.Spec.Bootstrap.ConfigRef.Namespace,
	}
	if err := controllerClient.Get(ctx, kubeadmConfigKey, kubeadmConfig); err != nil {
		return nil, errors.Wrapf(err,
			"failed to get KubeadmConfig %s/%s for Machine %s/%s/%s",
			kubeadmConfigKey.Name, kubeadmConfigKey.Namespace,
			machine.Namespace, machine.ClusterName, machine.Name)
	}

	return kubeadmConfig, nil
}

// GetAPIEndpointForControlPlaneEndpoint parses the provided ControlPlaneEndpoint
// and returns an APIEndpoint.
func GetAPIEndpointForControlPlaneEndpoint(controlPlaneEndpoint string) (*infrav1.APIEndpoint, error) {
	if controlPlaneEndpoint == "" {
		return nil, errors.Errorf("invalid ControlPlaneEndpoint: %q", controlPlaneEndpoint)
	}

	// If the control plane endpoint doesn't start with "http" then prefix
	// it with "http".
	//
	// This is because a controlPlaneEndpoint can technically be a properly
	// formed URL as found in kubeconfig files. This approach ensures a single
	// method of parsing the controlPlaneEndpoint for its host:port information
	// can be used. The "http://" prefix is thus used to satisfy the "url.Parse"
	// function below.
	if !strings.HasPrefix(controlPlaneEndpoint, "http") {
		controlPlaneEndpoint = "http://" + controlPlaneEndpoint
	}

	// Try to parse the control plane endpoint as a URL.
	u, err := url.Parse(controlPlaneEndpoint)
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"failed to parse ControlPlaneEndpoint as URL: %q",
			controlPlaneEndpoint)
	}

	apiEndpoint := &infrav1.APIEndpoint{
		Host: u.Hostname(),
	}

	if szPort := u.Port(); szPort != "" {
		port, err := strconv.Atoi(szPort)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"failed to parse port=%s for ControlPlaneEndpoint=%s",
				szPort,
				controlPlaneEndpoint)
		}
		apiEndpoint.Port = port
	}

	return apiEndpoint, nil
}
