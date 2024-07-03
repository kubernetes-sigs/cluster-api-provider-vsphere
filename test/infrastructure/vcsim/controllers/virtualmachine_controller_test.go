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
	"time"

	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func Test_Reconcile_VirtualMachine(t *testing.T) {
	t.Run("VirtualMachine not yet provisioned should be ignored", func(t *testing.T) {
		g := NewWithT(t)

		vsphereCluster := &vmwarev1.VSphereCluster{
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
					APIVersion: vmwarev1.GroupVersion.String(),
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

		vSphereMachine := &vmwarev1.VSphereMachine{
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

		virtualMachine := &vmoprv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: vmwarev1.GroupVersion.String(),
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
		crclient := fake.NewClientBuilder().WithObjects(cluster, vsphereCluster, machine, vSphereMachine, virtualMachine).WithScheme(scheme).Build()

		// Start in memory manager & add a resourceGroup for the cluster
		inmemoryMgr := inmemoryruntime.NewManager(cloudScheme)
		err := inmemoryMgr.Start(ctx)
		g.Expect(err).ToNot(HaveOccurred())

		resourceGroupName := klog.KObj(cluster).String()
		inmemoryMgr.AddResourceGroup(resourceGroupName)
		inmemoryClient := inmemoryMgr.GetResourceGroup(resourceGroupName).GetClient()

		host := "127.0.0.1"
		apiServerMux, err := inmemoryserver.NewWorkloadClustersMux(inmemoryMgr, host, inmemoryserver.CustomPorts{
			// NOTE: make sure to use ports different than other tests, so we can run tests in parallel
			MinPort:   inmemoryserver.DefaultMinPort,
			MaxPort:   inmemoryserver.DefaultMinPort + 99,
			DebugPort: inmemoryserver.DefaultDebugPort,
		})
		g.Expect(err).ToNot(HaveOccurred())

		listenerName := "foo/bar"
		_, err = apiServerMux.InitWorkloadClusterListener(listenerName)
		g.Expect(err).ToNot(HaveOccurred())

		err = apiServerMux.RegisterResourceGroup(listenerName, resourceGroupName)
		g.Expect(err).ToNot(HaveOccurred())

		r := VirtualMachineReconciler{
			Client:          crclient,
			InMemoryManager: inmemoryMgr,
			APIServerMux:    apiServerMux,
		}

		// Reconcile
		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: virtualMachine.Namespace,
			Name:      virtualMachine.Name,
		}})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res).To(Equal(ctrl.Result{RequeueAfter: 5 * time.Second}))

		// Check the conditionsTracker is waiting for infrastructure ready
		conditionsTracker := &infrav1.VSphereVM{}
		err = inmemoryClient.Get(ctx, client.ObjectKeyFromObject(virtualMachine), conditionsTracker)
		g.Expect(err).ToNot(HaveOccurred())

		c := conditions.Get(conditionsTracker, VMProvisionedCondition)
		g.Expect(c.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(c.Severity).To(Equal(clusterv1.ConditionSeverityInfo))
		g.Expect(c.Reason).To(Equal(WaitingControlPlaneInitializedReason))
	})

	t.Run("VirtualMachine provisioned gets a node (worker)", func(t *testing.T) {
		g := NewWithT(t)

		vsphereCluster := &vmwarev1.VSphereCluster{
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
					APIVersion: vmwarev1.GroupVersion.String(),
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

		vSphereMachine := &vmwarev1.VSphereMachine{
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

		virtualMachine := &vmoprv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: vmwarev1.GroupVersion.String(),
						Kind:       "VSphereMachine",
						Name:       vSphereMachine.Name,
						UID:        vSphereMachine.UID,
					},
				},
				Finalizers: []string{
					vcsimv1.VMFinalizer, // Adding this to move past the first reconcile
				},
			},
			Status: vmoprv1.VirtualMachineStatus{
				// Those values are required to unblock provisioning of node
				BiosUUID: "foo",
				Network: &vmoprv1.VirtualMachineNetworkStatus{
					PrimaryIP4: "1.2.3.4",
				},
				PowerState: vmoprv1.VirtualMachinePowerStateOn,
			},
		}

		// Controller runtime client
		crclient := fake.NewClientBuilder().WithObjects(cluster, vsphereCluster, machine, vSphereMachine, virtualMachine).WithScheme(scheme).Build()

		// Start in memory manager & add a resourceGroup for the cluster
		inmemoryMgr := inmemoryruntime.NewManager(cloudScheme)
		err := inmemoryMgr.Start(ctx)
		g.Expect(err).ToNot(HaveOccurred())

		resourceGroupName := klog.KObj(cluster).String()
		inmemoryMgr.AddResourceGroup(resourceGroupName)
		inmemoryClient := inmemoryMgr.GetResourceGroup(resourceGroupName).GetClient()

		// Start an http server
		apiServerMux, err := inmemoryserver.NewWorkloadClustersMux(inmemoryMgr, "127.0.0.1", inmemoryserver.CustomPorts{
			// NOTE: make sure to use ports different than other tests, so we can run tests in parallel
			MinPort:   inmemoryserver.DefaultMinPort + 200,
			MaxPort:   inmemoryserver.DefaultMinPort + 299,
			DebugPort: inmemoryserver.DefaultDebugPort + 2,
		})
		g.Expect(err).ToNot(HaveOccurred())

		listenerName := "foo/bar"
		_, err = apiServerMux.InitWorkloadClusterListener(listenerName)
		g.Expect(err).ToNot(HaveOccurred())

		err = apiServerMux.RegisterResourceGroup(listenerName, resourceGroupName)
		g.Expect(err).ToNot(HaveOccurred())

		r := VirtualMachineReconciler{
			Client:          crclient,
			InMemoryManager: inmemoryMgr,
			APIServerMux:    apiServerMux,
		}

		// Reconcile
		nodeStartupDuration = 0 * time.Second

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{
			Namespace: virtualMachine.Namespace,
			Name:      virtualMachine.Name,
		}})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res).To(Equal(ctrl.Result{}))

		// Check the mirrorVSphereMachine reports all provisioned

		conditionsTracker := &infrav1.VSphereVM{}
		err = inmemoryClient.Get(ctx, client.ObjectKeyFromObject(virtualMachine), conditionsTracker)
		g.Expect(err).ToNot(HaveOccurred())

		c := conditions.Get(conditionsTracker, NodeProvisionedCondition)
		g.Expect(c.Status).To(Equal(corev1.ConditionTrue))
	})
}
