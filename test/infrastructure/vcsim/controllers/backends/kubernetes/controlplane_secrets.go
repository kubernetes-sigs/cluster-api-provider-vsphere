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
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// caSecretHandler implement handling for the secrets storing the control plane certificate authorities.
type caSecretHandler struct {
	// TODO: in a follow up iteration we want to make it possible to store those objects in a dedicate ns on a separated cluster
	//  this brings in the limitation that objects for two clusters with the same name cannot be hosted in a single namespace as well as the need to rethink owner references.
	client client.Client

	cluster        *clusterv1beta1.Cluster
	virtualMachine client.Object
}

func (h *caSecretHandler) LookupOrGenerate(ctx context.Context) error {
	certificates := secret.NewCertificatesForInitialControlPlane(&bootstrapv1.ClusterConfiguration{})

	virtualMachineGVK, err := apiutil.GVKForObject(h.virtualMachine, h.client.Scheme())
	if err != nil {
		return err
	}

	// Generate cluster certificates on the management cluster if not already there.
	// Note: the code is taking care of service cleanup during the deletion workflow,
	// so this controllerRef is mostly used to express a semantic relation.
	controllerRef := metav1.NewControllerRef(h.virtualMachine, virtualMachineGVK)
	if err := certificates.LookupOrGenerate(ctx, h.client, client.ObjectKeyFromObject(h.cluster), *controllerRef); err != nil {
		return errors.Wrap(err, "failed to generate cluster certificates on the management cluster")
	}

	return nil
}
