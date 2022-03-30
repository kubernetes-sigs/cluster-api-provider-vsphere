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

package fake

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
)

// NewGuestClusterContext returns a fake GuestClusterContext for unit testing
// guest cluster controllers with a fake client.
func NewGuestClusterContext(ctx *vmware.ClusterContext, prototypeCluster bool, gcInitObjects ...client.Object) *vmware.GuestClusterContext {
	if prototypeCluster {
		cluster := newCluster(ctx.VSphereCluster)
		if err := ctx.Client.Create(ctx, cluster); err != nil {
			panic(err)
		}
	}

	return &vmware.GuestClusterContext{
		ClusterContext: ctx,
		GuestClient:    NewFakeGuestClusterClient(gcInitObjects...),
	}
}

func NewFakeGuestClusterClient(initObjects ...client.Object) client.Client {
	scheme := scheme.Scheme
	_ = apiextv1.AddToScheme(scheme)

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
}
