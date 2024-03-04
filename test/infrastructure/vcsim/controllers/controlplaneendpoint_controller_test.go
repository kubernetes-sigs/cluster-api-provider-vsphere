/*
Copyright 2024 The Kubernetes Authors.

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

package controllers

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

func Test_Reconcile_ControlPlaneEndpoint(t *testing.T) {
	g := NewWithT(t)

	// Start a manager to handle resources that we are going to store in the fake API servers for the workload clusters.
	workloadClustersManager := inmemoryruntime.NewManager(cloudScheme)
	err := workloadClustersManager.Start(ctx)
	g.Expect(err).ToNot(HaveOccurred())

	// Start an Mux for the API servers for the workload clusters.
	podIP := "127.0.0.1"
	workloadClustersMux, err := inmemoryserver.NewWorkloadClustersMux(workloadClustersManager, podIP, inmemoryserver.CustomPorts{
		// NOTE: make sure to use ports different than other tests, so we can run tests in parallel
		MinPort:   inmemoryserver.DefaultMinPort + 100,
		MaxPort:   inmemoryserver.DefaultMinPort + 199,
		DebugPort: inmemoryserver.DefaultDebugPort + 1,
	})
	g.Expect(err).ToNot(HaveOccurred())

	controlPlaneEndpoint := &vcsimv1.ControlPlaneEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "foo",
			Finalizers: []string{
				vcsimv1.ControlPlaneEndpointFinalizer, // Adding this to move past the first reconcile
			},
		},
	}

	crclient := fake.NewClientBuilder().WithObjects(controlPlaneEndpoint).WithStatusSubresource(controlPlaneEndpoint).WithScheme(scheme).Build()
	r := &ControlPlaneEndpointReconciler{
		Client:          crclient,
		InMemoryManager: workloadClustersManager,
		APIServerMux:    workloadClustersMux,
		PodIP:           podIP,
	}

	// PART 1: Should create a new ControlPlaneEndpoint

	res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{
		Namespace: controlPlaneEndpoint.Namespace,
		Name:      controlPlaneEndpoint.Name,
	}})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(Equal(ctrl.Result{}))

	// Gets the reconciled object
	err = crclient.Get(ctx, client.ObjectKeyFromObject(controlPlaneEndpoint), controlPlaneEndpoint)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(controlPlaneEndpoint.Status.Host).ToNot(BeEmpty())
	g.Expect(controlPlaneEndpoint.Status.Port).ToNot(BeZero())

	// Check manager and server internal status
	listenerName := klog.KObj(controlPlaneEndpoint).String()
	g.Expect(workloadClustersMux.ListListeners()).To(HaveKey(listenerName))

	// PART 2: Should delete a ControlPlaneEndpoint

	err = crclient.Delete(ctx, controlPlaneEndpoint)
	g.Expect(err).ToNot(HaveOccurred())

	res, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{
		Namespace: controlPlaneEndpoint.Namespace,
		Name:      controlPlaneEndpoint.Name,
	}})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res).To(Equal(ctrl.Result{}))
}
