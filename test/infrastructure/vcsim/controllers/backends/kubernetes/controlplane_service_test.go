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
	"os"
	"testing"

	. "github.com/onsi/gomega"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
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
	_ = vmopv1.AddToScheme(testScheme)
	_ = clusterv1.AddToScheme(testScheme)
	_ = rbacv1.AddToScheme(testScheme)
}

func TestControlPlaneInPod(t *testing.T) {
	g := NewWithT(t)
	ctx = ctrl.LoggerInto(ctx, textlogger.NewLogger(textlogger.NewConfig(textlogger.Verbosity(5), textlogger.Output(os.Stdout))))

	// Create client
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	g.Expect(err).NotTo(HaveOccurred())

	restConfig, err := clientcmd.NewDefaultClientConfig(*config, nil).ClientConfig()
	g.Expect(err).NotTo(HaveOccurred())

	c, err := client.New(restConfig, client.Options{Scheme: testScheme})
	g.Expect(err).NotTo(HaveOccurred())

	// Create or Get ControlPlaneEndpoint
	rcpe := &ControlPlaneEndpointReconciler{
		Client: c,
	}

	cpe := &vcsimv1.ControlPlaneEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
	}

	err = c.Create(ctx, cpe)
	if !apierrors.IsAlreadyExists(err) {
		g.Expect(err).NotTo(HaveOccurred())
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(cpe), cpe)
	g.Expect(err).NotTo(HaveOccurred())

	// Reconcile & Patch ControlPlaneEndpoint
	originalCPE := cpe.DeepCopy()
	patch := client.MergeFrom(originalCPE)

	res, err := rcpe.ReconcileNormal(ctx, cpe)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res.IsZero()).To(BeTrue())
	g.Expect(cpe.Status.Host).ToNot(BeEmpty())
	g.Expect(cpe.Status.Port).ToNot(BeZero())

	err = c.Status().Patch(ctx, cpe, patch)
	g.Expect(err).NotTo(HaveOccurred())

	// Create or Get Cluster
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: cpe.Status.Host,
				Port: cpe.Status.Port,
			},
		},
	}
	err = c.Create(ctx, cluster)
	if !apierrors.IsAlreadyExists(err) {
		g.Expect(err).NotTo(HaveOccurred())
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
	g.Expect(err).NotTo(HaveOccurred())

	// Create or Get Machine
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: cluster.Name,
			Version:     ptr.To("v1.33.0"),
		},
	}
	err = c.Create(ctx, machine)
	if !apierrors.IsAlreadyExists(err) {
		g.Expect(err).NotTo(HaveOccurred())
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(machine), machine)
	g.Expect(err).NotTo(HaveOccurred())

	// Create or Get VirtualMachine
	virtualMachine := &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
	}
	err = c.Create(ctx, virtualMachine)
	if !apierrors.IsAlreadyExists(err) {
		g.Expect(err).NotTo(HaveOccurred())
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(virtualMachine), virtualMachine)
	g.Expect(err).NotTo(HaveOccurred())

	// Reconcile VM
	rvm := &VirtualMachineReconciler{
		Client:    c,
		IsVMReady: func() bool { return true },
		overrideGetManagerContainer: func(ctx context.Context) (*corev1.Container, error) {
			return &corev1.Container{
				Image: "gcr.io/broadcom-451918/cluster-api-vcsim-controller-amd64:dev",
			}, nil
		},
	}

	res, err = rvm.reconcileCertificates(ctx, cluster, machine, virtualMachine)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res.IsZero()).To(BeTrue())

	res, err = rvm.reconcileKubeConfig(ctx, cluster, machine, virtualMachine)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res.IsZero()).To(BeTrue())

	res, err = rvm.reconcilePods(ctx, cluster, machine, virtualMachine)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestWorkerInPodWithKindNode(t *testing.T) {
	g := NewWithT(t)
	ctx = ctrl.LoggerInto(ctx, textlogger.NewLogger(textlogger.NewConfig(textlogger.Verbosity(5), textlogger.Output(os.Stdout))))

	// Create client
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	g.Expect(err).NotTo(HaveOccurred())

	restConfig, err := clientcmd.NewDefaultClientConfig(*config, nil).ClientConfig()
	g.Expect(err).NotTo(HaveOccurred())

	c, err := client.New(restConfig, client.Options{Scheme: testScheme})
	g.Expect(err).NotTo(HaveOccurred())

	// Create or Get ControlPlaneEndpoint
	rcpe := &ControlPlaneEndpointReconciler{
		Client: c,
	}

	cpe := &vcsimv1.ControlPlaneEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
	}

	err = c.Create(ctx, cpe)
	if !apierrors.IsAlreadyExists(err) {
		g.Expect(err).NotTo(HaveOccurred())
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(cpe), cpe)
	g.Expect(err).NotTo(HaveOccurred())

	// Reconcile & Patch ControlPlaneEndpoint
	originalCPE := cpe.DeepCopy()
	patch := client.MergeFrom(originalCPE)

	res, err := rcpe.ReconcileNormal(ctx, cpe)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res.IsZero()).To(BeTrue())
	g.Expect(cpe.Status.Host).ToNot(BeEmpty())
	g.Expect(cpe.Status.Port).ToNot(BeZero())

	err = c.Status().Patch(ctx, cpe, patch)
	g.Expect(err).NotTo(HaveOccurred())

	// Create or Get Cluster
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: cpe.Status.Host,
				Port: cpe.Status.Port,
			},
		},
	}
	err = c.Create(ctx, cluster)
	if !apierrors.IsAlreadyExists(err) {
		g.Expect(err).NotTo(HaveOccurred())
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
	g.Expect(err).NotTo(HaveOccurred())

	// Create or Get Machine
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-worker",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: cluster.Name,
			Version:     ptr.To("v1.33.0"),
		},
	}
	err = c.Create(ctx, machine)
	if !apierrors.IsAlreadyExists(err) {
		g.Expect(err).NotTo(HaveOccurred())
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(machine), machine)
	g.Expect(err).NotTo(HaveOccurred())

	// Create or Get VirtualMachine
	virtualMachine := &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-worker",
			Namespace: metav1.NamespaceDefault,
		},
	}
	err = c.Create(ctx, virtualMachine)
	if !apierrors.IsAlreadyExists(err) {
		g.Expect(err).NotTo(HaveOccurred())
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(virtualMachine), virtualMachine)
	g.Expect(err).NotTo(HaveOccurred())

	w := workerPodHandler{
		client:               c,
		controlPlaneEndpoint: cpe,
		cluster:              cluster,
		virtualMachine:       virtualMachine,
		overrideGetManagerContainer: func(ctx context.Context) (*corev1.Container, error) {
			return &corev1.Container{
				Image: "gcr.io/broadcom-451918/cluster-api-vcsim-controller-amd64:dev",
			}, nil
		},
	}

	w.Generate(ctx, *machine.Spec.Version)
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
