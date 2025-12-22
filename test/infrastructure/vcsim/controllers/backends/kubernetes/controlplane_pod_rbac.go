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
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (h *controlPlanePodHandler) LookupAndGenerateRBAC(ctx context.Context) error {
	// TODO: think about cleanup or comment that cleanup of RBAC rules won't happen.
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.virtualMachine.GetNamespace(),
			Name:      "kubemark-control-plane",
		},
		Rules: []rbacv1.PolicyRule{
			{
				// TODO: consider if to restrict this somehow
				Verbs:     []string{"get"},
				APIGroups: []string{""}, // "" indicates the core API group
				Resources: []string{"secrets"},
			},
		},
	}
	if err := h.client.Get(ctx, client.ObjectKeyFromObject(role), role); err != nil {
		switch {
		case apierrors.IsNotFound(err):
			if err := h.client.Create(ctx, role); err != nil {
				return errors.Wrap(err, "failed to create kubemark-control-plane Role")
			}
			break
		case apierrors.IsAlreadyExists(err):
			break
		default:
			return errors.Wrap(err, "failed to get kubemark-control-plane Role")
		}
	}
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.virtualMachine.GetNamespace(),
			Name:      "kubemark-control-plane",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				APIGroup: "rbac.authorization.k8s.io",
				// TODO: create a service account and use it here instead of default + use it in the Pod
				Name:      "system:serviceaccount:default:default",
				Namespace: h.virtualMachine.GetNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "kubemark-control-plane",
		},
	}
	if err := h.client.Get(ctx, client.ObjectKeyFromObject(roleBinding), roleBinding); err != nil {
		switch {
		case apierrors.IsNotFound(err):
			if err := h.client.Create(ctx, roleBinding); err != nil {
				return errors.Wrap(err, "failed to create kubemark-control-plane RoleBinding")
			}
			break
		case apierrors.IsAlreadyExists(err):
			break
		default:
			return errors.Wrap(err, "failed to get kubemark-control-plane RoleBinding")
		}
	}
	return nil
}
