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
	"encoding/base64"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetMachineDeploymentName returns the MachineDeployment name for a Cluster.
// This is also the name used by VSphereMachineTemplate and KubeadmConfigTemplate.
func GetMachineDeploymentNameForCluster(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-workers-0", cluster.Name)
}

// GetBootstrapConfigMapName returns the name of the bootstrap data ConfigMap
// for a VM Operator VirtualMachine.
func GetBootstrapConfigMapName(machineName string) string {
	return fmt.Sprintf("%s-cloud-init", machineName)
}

func GetBootstrapData(ctx context.Context, c client.Client, machine *clusterv1.Machine) (string, error) {
	value, err := GetRawBootstrapData(ctx, c, machine)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(value), nil
}

// GetRawBootstrapData returns the bootstrap data from the secret in the
// Machine's bootstrap.dataSecretName.
func GetRawBootstrapData(ctx context.Context, c client.Client, machine *clusterv1.Machine) ([]byte, error) {
	if machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := apitypes.NamespacedName{Namespace: machine.GetNamespace(), Name: *machine.Spec.Bootstrap.DataSecretName}
	if err := c.Get(ctx, key, secret); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve bootstrap data secret for Machine %s/%s", machine.GetNamespace(), machine.GetName())
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}
