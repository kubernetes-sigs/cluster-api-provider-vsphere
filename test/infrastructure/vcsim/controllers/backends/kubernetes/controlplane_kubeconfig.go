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

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// kubeConfigSecretHandler implement handling for the secret storing the cluster admin kubeconfig.
type kubeConfigSecretHandler struct {
	// TODO: in a follow up iteration we want to make it possible to store those objects in a dedicate ns on a separated cluster
	//  this brings in the limitation that objects for two clusters with the same name cannot be hosted in a single namespace as well as the need to rethink owner references.
	client client.Client

	cluster        *clusterv1beta1.Cluster
	virtualMachine client.Object
}

func (h *kubeConfigSecretHandler) LookupOrGenerate(ctx context.Context) error {
	// If the secret with the KubeConfig already exists, then no-op.
	if s, _ := secret.GetFromNamespacedName(ctx, h.client, client.ObjectKeyFromObject(h.cluster), secret.Kubeconfig); s != nil {
		return nil
	}

	virtualMachineGVK, err := apiutil.GVKForObject(h.virtualMachine, h.client.Scheme())
	if err != nil {
		return err
	}

	// Otherwise it is required to generate the secret storing the cluster admin kubeconfig.
	// Note: the code is taking care of service cleanup during the deletion workflow,
	// so this controllerRef is mostly used to express a semantic relation.
	controllerRef := metav1.NewControllerRef(h.virtualMachine, virtualMachineGVK)
	if err := kubeconfig.CreateSecretWithOwner(ctx, h.client, client.ObjectKeyFromObject(h.cluster), h.cluster.Spec.ControlPlaneEndpoint.String(), *controllerRef); err != nil {
		return errors.Wrap(err, "failed to generate cluster certificates on the management cluster")
	}
	return nil
}
