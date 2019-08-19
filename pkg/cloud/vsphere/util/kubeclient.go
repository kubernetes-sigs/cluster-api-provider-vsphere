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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	remotev1 "sigs.k8s.io/cluster-api/pkg/controller/remote"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewKubeClient returns a new client for the target cluster using the KubeConfig
// secret stored in the management cluster.
func NewKubeClient(
	ctx context.Context,
	controllerClient client.Client,
	namespace, clusterName string) (corev1.CoreV1Interface, error) {

	cluster := &clusterv1.Cluster{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      clusterName,
	}
	if err := controllerClient.Get(ctx, namespacedName, cluster); err != nil {
		return nil, errors.Wrap(err, "unable to get target cluster resource")
	}

	clusterClient, err := remotev1.NewClusterClient(controllerClient, cluster)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get client for target cluster")
	}

	coreClient, err := clusterClient.CoreV1()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get core client for target cluster")
	}

	return coreClient, nil
}
