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
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

var (
	testScheme = runtime.NewScheme()
	ctx        = ctrl.SetupSignalHandler()
)

func init() {
	_ = corev1.AddToScheme(testScheme)
	_ = vcsimv1.AddToScheme(testScheme)
}

func TestLBServiceHandler(t *testing.T) {
	t.Run("Test Generate, Lookup, Delete", func(t *testing.T) {
		g := NewWithT(t)

		lb := lbServiceHandler{
			client: fake.NewClientBuilder().WithScheme(testScheme).Build(),
			controlPlaneEndpoint: &vcsimv1.ControlPlaneEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "test",
				},
			},
		}

		// Generate
		svc1, err := lb.Generate(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(svc1).ToNot(BeNil())

		g.Expect(svc1.Name).To(Equal(fmt.Sprintf("%s-lb", lb.controlPlaneEndpoint.Name)))
		g.Expect(svc1.Namespace).To(Equal(lb.controlPlaneEndpoint.Namespace))
		g.Expect(svc1.OwnerReferences).To(HaveLen(1))
		g.Expect(svc1.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		g.Expect(svc1.Spec.Ports).To(ConsistOf(corev1.ServicePort{Port: lbServicePort, TargetPort: intstr.FromInt(apiServerPodPort)}))

		// Fake ClusterIP address being assigned
		patch := client.MergeFrom(svc1.DeepCopy())
		svc1.Spec.ClusterIP = "1.2.3.4"
		g.Expect(lb.client.Patch(ctx, svc1, patch)).To(Succeed())

		// Lookup
		svc2, err := lb.Lookup(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(svc2).ToNot(BeNil())

		g.Expect(svc1.Spec.ClusterIP).To(Equal("1.2.3.4"))

		// Delete
		err = lb.Delete(ctx)
		g.Expect(err).ToNot(HaveOccurred())

		svc3 := &corev1.Service{}
		err = lb.client.Get(ctx, lb.ObjectKey(), svc3)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
	t.Run("Test LookupOrGenerate", func(t *testing.T) {
		g := NewWithT(t)

		lb := lbServiceHandler{
			client: fake.NewClientBuilder().WithScheme(testScheme).Build(),
			controlPlaneEndpoint: &vcsimv1.ControlPlaneEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "test",
				},
			},
		}

		// LookupOrGenerate must create if the service is not already there
		svc1, err := lb.LookupOrGenerate(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(svc1).ToNot(BeNil())

		g.Expect(svc1.Name).To(Equal(fmt.Sprintf("%s-lb", lb.controlPlaneEndpoint.Name)))
		g.Expect(svc1.Namespace).To(Equal(lb.controlPlaneEndpoint.Namespace))
		g.Expect(svc1.OwnerReferences).To(HaveLen(1))
		g.Expect(svc1.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		g.Expect(svc1.Spec.Ports).To(ConsistOf(corev1.ServicePort{Port: lbServicePort, TargetPort: intstr.FromInt(apiServerPodPort)}))

		// Fake ClusterIP address being assigned
		patch := client.MergeFrom(svc1.DeepCopy())
		svc1.Spec.ClusterIP = "1.2.3.4"
		g.Expect(lb.client.Patch(ctx, svc1, patch)).To(Succeed())

		// LookupOrGenerate must read if the service already there
		svc2, err := lb.LookupOrGenerate(ctx)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(svc2).ToNot(BeNil())

		g.Expect(svc2.Spec.ClusterIP).To(Equal("1.2.3.4"))
	})
}
