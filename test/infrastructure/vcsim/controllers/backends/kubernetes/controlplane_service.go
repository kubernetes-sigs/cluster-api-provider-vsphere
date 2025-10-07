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
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

const (
	apiServerPodPort = 6443
	lbServicePort    = 6443
)

// lbServiceHandler implement handling for the Kubernetes Service acting as a load balancer in front of all the control plane instances.
type lbServiceHandler struct {
	// TODO: in a follow up iteration we want to make it possible to store those objects in a dedicate ns on a separated cluster
	//  this brings in the limitation that objects for two clusters with the same name cannot be hosted in a single namespace as well as the need to rethink owner references.
	client client.Client

	controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint
}

func (lb *lbServiceHandler) ObjectKey() client.ObjectKey {
	return client.ObjectKey{
		Namespace: lb.controlPlaneEndpoint.Namespace,
		Name:      fmt.Sprintf("%s-lb", lb.controlPlaneEndpoint.Name),
	}
}

func (lb *lbServiceHandler) LookupOrGenerate(ctx context.Context) (*corev1.Service, error) {
	// Lookup the load balancer service.
	svc, err := lb.Lookup(ctx)
	if err != nil {
		return nil, err
	}
	if svc != nil {
		return svc, nil
	}
	return lb.Generate(ctx)
}

func (lb *lbServiceHandler) Lookup(ctx context.Context) (*corev1.Service, error) {
	key := lb.ObjectKey()
	secret := &corev1.Service{}
	if err := lb.client.Get(ctx, key, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to get load balance service")
	}
	return secret, nil
}

func (lb *lbServiceHandler) Generate(ctx context.Context) (*corev1.Service, error) {
	key := lb.ObjectKey()
	secret := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			// Note: the code is taking care of service cleanup during the deletion workflow,
			// so this ownerRef is mostly used to express a semantic relation.
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         vcsimv1.GroupVersion.String(),
					Kind:               "ControlPlaneEndpoint",
					Name:               lb.controlPlaneEndpoint.Name,
					UID:                lb.controlPlaneEndpoint.UID,
					Controller:         pointer.Bool(true),
					BlockOwnerDeletion: pointer.Bool(true),
				},
			},
		},
		Spec: corev1.ServiceSpec{
			// This selector must match labels on apiServerPods.
			Selector: map[string]string{
				"control-plane-endpoint.vcsim.infrastructure.cluster.x-k8s.io": lb.controlPlaneEndpoint.Name,
			},
			// Currently we support only services of type IP, also
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:       lbServicePort,
					TargetPort: intstr.FromInt(apiServerPodPort),
				},
			},
		},
	}
	if err := lb.client.Create(ctx, secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "failed to create load balance service")
	}
	return secret, nil
}

func (lb *lbServiceHandler) Delete(ctx context.Context) error {
	key := lb.ObjectKey()
	secret := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	if err := lb.client.Delete(ctx, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "failed to delete load balance service")
	}
	return nil
}
