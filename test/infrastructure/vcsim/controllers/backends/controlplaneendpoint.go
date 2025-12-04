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

package backends

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type ControlPlaneEndpointReconciler interface {
	ReconcileNormal(ctx context.Context, controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint) (ctrl.Result, error)
	ReconcileDelete(ctx context.Context, controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint) (ctrl.Result, error)
}
