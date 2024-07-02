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
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

var (
	cloudScheme = runtime.NewScheme()
	scheme      = runtime.NewScheme()

	ctx = context.Background()
)

func init() {
	// scheme used for operating on the management cluster.
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = vmwarev1.AddToScheme(scheme)
	_ = vmoprv1.AddToScheme(scheme)
	_ = vcsimv1.AddToScheme(scheme)

	// scheme used for operating on the cloud resource.
	_ = infrav1.AddToScheme(cloudScheme)
	_ = corev1.AddToScheme(cloudScheme)
	_ = appsv1.AddToScheme(cloudScheme)
	_ = rbacv1.AddToScheme(cloudScheme)
}

func Test_Reconcile_VSphereVM(t *testing.T) {
	t.Run("VSphereMachine not yet provisioned should be ignored", func(t *testing.T) {
		g := NewWithT(t)

		vsphereCluster := &infrav1.VSphereCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				UID:       "bar",
			},
		}

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				UID:       "bar",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "VSphereCluster",
					Namespace:  vsphereCluster.Namespace,
					Name:       vsphereCluster.Name,
					UID:        vsphereCluster.UID,
				},
			},
		}

		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: cluster.Name,
				},
			},
		}

		vSphereMachine := &infrav1.VSphereMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "baz",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       machine.Name,
						UID:        machine.UID,
					},
				},
			},
		}

		vSphereVM := &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "VSphereMachine",
						Name:       vSphereMachine.Name,
						UID:        vSphereMachine.UID,
					},
				},
				Finalizers: []string{
					vcsimv1.VMFinalizer, // Adding this to move past the first reconcile
				},
			},
		}

		// Controller runtime client
		crclient := fake.NewClientBuilder().WithObjects(cluster, vsphereCluster, machine, vSphereMachine, vSphereVM).WithScheme(scheme).Build()

		// Start in memory manager & add a resourceGroup for the cluster
		inmemoryMgr := inmemoryruntime.NewManager(cloudScheme)
		err := inmemoryMgr.Start(ctx)
		g.Expect(err).ToNot(HaveOccurred())

		resourceGroupName := klog.KObj(cluster).String()
		inmemoryMgr.AddResourceGroup(resourceGroupName)
		inmemoryClient := inmemoryMgr.GetResourceGroup(resourceGroupName).GetClient()

		host := "127.0.0.1"
		wcmux, err := inmemoryserver.NewWorkloadClustersMux(inmemoryMgr, host, inmemoryserver.CustomPorts{
			// NOTE: make sure to use ports different than other tests, so we can run tests in parallel
			MinPort:   inmemoryserver.DefaultMinPort + 400,
			MaxPort:   inmemoryserver.DefaultMinPort + 499,
			DebugPort: inmemoryserver.DefaultDebugPort + 4,
		})
		g.Expect(err).ToNot(HaveOccurred())

		listenerName := "foo/bar"
		_, err = wcmux.InitWorkloadClusterListener(listenerName)
		g.Expect(err).ToNot(HaveOccurred())

		err = wcmux.RegisterResourceGroup(listenerName, resourceGroupName)
		g.Expect(err).ToNot(HaveOccurred())

		r := VSphereVMReconciler{
			Client:          crclient,
			InMemoryManager: inmemoryMgr,
			APIServerMux:    wcmux,
		}

		// Reconcile
		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: vSphereVM.Namespace,
			Name:      vSphereVM.Name,
		}})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res).To(Equal(ctrl.Result{RequeueAfter: 5 * time.Second}))

		// Check the conditionsTracker is waiting for infrastructure ready
		conditionsTracker := &infrav1.VSphereVM{}
		err = inmemoryClient.Get(ctx, client.ObjectKeyFromObject(vSphereVM), conditionsTracker)
		g.Expect(err).ToNot(HaveOccurred())

		c := conditions.Get(conditionsTracker, VMProvisionedCondition)
		g.Expect(c.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(c.Severity).To(Equal(clusterv1.ConditionSeverityInfo))
		g.Expect(c.Reason).To(Equal(WaitingControlPlaneInitializedReason))
	})

	t.Run("VSphereMachine provisioned gets a node (worker)", func(t *testing.T) {
		g := NewWithT(t)

		vsphereCluster := &infrav1.VSphereCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				UID:       "bar",
			},
		}

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				UID:       "bar",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "VSphereCluster",
					Namespace:  vsphereCluster.Namespace,
					Name:       vsphereCluster.Name,
					UID:        vsphereCluster.UID,
				},
			},
		}

		machine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: cluster.Name,
				},
			},
			Spec: clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					DataSecretName: ptr.To("foo"), // this unblocks node provisioning
				},
			},
		}

		vSphereMachine := &infrav1.VSphereMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Machine",
						Name:       machine.Name,
						UID:        machine.UID,
					},
				},
			},
		}

		vSphereVM := &infrav1.VSphereVM{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "VSphereMachine",
						Name:       vSphereMachine.Name,
						UID:        vSphereMachine.UID,
					},
				},
				Finalizers: []string{
					vcsimv1.VMFinalizer, // Adding this to move past the first reconcile
				},
			},
			Spec: infrav1.VSphereVMSpec{
				BiosUUID: "foo", // This unblocks provisioning of node
			},
			Status: infrav1.VSphereVMStatus{
				Ready: true, // This unblocks provisioning of node
			},
		}

		// Controller runtime client
		crclient := fake.NewClientBuilder().WithObjects(cluster, vsphereCluster, machine, vSphereMachine, vSphereVM).WithScheme(scheme).Build()

		// Start cloud manager & add a resourceGroup for the cluster
		inmemoryMgr := inmemoryruntime.NewManager(cloudScheme)
		err := inmemoryMgr.Start(ctx)
		g.Expect(err).ToNot(HaveOccurred())

		resourceGroupName := klog.KObj(cluster).String()
		inmemoryMgr.AddResourceGroup(resourceGroupName)
		inmemoryClient := inmemoryMgr.GetResourceGroup(resourceGroupName).GetClient()

		// Start an http server
		apiServerMux, err := inmemoryserver.NewWorkloadClustersMux(inmemoryMgr, "127.0.0.1", inmemoryserver.CustomPorts{
			// NOTE: make sure to use ports different than other tests, so we can run tests in parallel
			MinPort:   inmemoryserver.DefaultMinPort + 300,
			MaxPort:   inmemoryserver.DefaultMinPort + 399,
			DebugPort: inmemoryserver.DefaultDebugPort + 3,
		})
		g.Expect(err).ToNot(HaveOccurred())

		listenerName := "foo/bar"
		_, err = apiServerMux.InitWorkloadClusterListener(listenerName)
		g.Expect(err).ToNot(HaveOccurred())

		err = apiServerMux.RegisterResourceGroup(listenerName, resourceGroupName)
		g.Expect(err).ToNot(HaveOccurred())

		r := VSphereVMReconciler{
			Client:          crclient,
			InMemoryManager: inmemoryMgr,
			APIServerMux:    apiServerMux,
		}

		// Reconcile
		nodeStartupDuration = 0 * time.Second

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: vSphereVM.Namespace,
			Name:      vSphereVM.Name,
		}})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res).To(Equal(ctrl.Result{}))

		// Check the mirrorVSphereMachine reports all provisioned

		conditionsTracker := &infrav1.VSphereVM{}
		err = inmemoryClient.Get(ctx, client.ObjectKeyFromObject(vSphereVM), conditionsTracker)
		g.Expect(err).ToNot(HaveOccurred())

		c := conditions.Get(conditionsTracker, NodeProvisionedCondition)
		g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	})
}
