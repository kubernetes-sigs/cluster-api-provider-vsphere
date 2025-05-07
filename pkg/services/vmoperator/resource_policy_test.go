/*
Copyright 2022 The Kubernetes Authors.

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

package vmoperator

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	utilfeature "k8s.io/component-base/featuregate/testing"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
)

func vSphereVMWithAntiAffinity(clusterNamespace, clusterName string, entry *vmwarev1.TopologyMachineDeploymentAffinityTerm) *vmwarev1.VSphereMachine {
	vSphereMachine := &vmwarev1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", clusterName, rand.String(5)),
			Namespace: clusterNamespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
	}
	if entry != nil {
		vSphereMachine.Spec.Affinity = &vmwarev1.VSphereMachineAffinity{
			MachineDeploymentMachineAntiAffinity: &vmwarev1.VSphereMachineMachineDeploymentMachineAntiAffinity{
				PreferredDuringSchedulingPreferredDuringExecution: []vmwarev1.TopologyMachineDeploymentAffinityTerm{
					*entry,
				},
			},
		}
	}
	return vSphereMachine
}

func TestRPService_ReconcileResourcePolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = vmwarev1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = vmoprv1.AddToScheme(scheme)
	ctx := context.Background()

	tests := []struct {
		name                    string
		cluster                 *clusterv1.Cluster
		additionalObjs          []client.Object
		wantClusterModuleGroups []string
		wantErr                 bool
		workerAntiAffinity      bool
	}{
		{
			name:    "create VirtualMachinesetResourcePolicy with fallback names while feature-gate is disabled (WorkerAntiAffinity: false)",
			cluster: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "cluster"}},
			// additionalObjs: []client.Object{
			// 	vSphereVMWithAntiAffinity("some", "cluster", &vmwarev1.VSphereMachinePreferredDuringSchedulingPreferredDuringExecution{MatchLabelKeys: []string{"foo"}, TopologyKey: vmwarev1.TopologyKeyESXHost}),
			// },
			wantErr:                 false,
			wantClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName, getFallbackWorkerClusterModuleGroupName("cluster")},
			workerAntiAffinity:      false,
		},
		{
			name:                    "create VirtualMachinesetResourcePolicy for control-plane only when no VSphereMachine exists (WorkerAntiAffinity: true)",
			cluster:                 &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "cluster"}},
			wantErr:                 false,
			wantClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName},
			workerAntiAffinity:      true,
		},
		{
			name:    "create VirtualMachinesetResourcePolicy for control-plane only VSphereMachine does not define matchLabelKeys (WorkerAntiAffinity: true)",
			cluster: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "cluster"}},
			additionalObjs: []client.Object{
				vSphereVMWithAntiAffinity("some", "cluster", &vmwarev1.TopologyMachineDeploymentAffinityTerm{MatchLabelKeys: []string{"foo"}, TopologyKey: vmwarev1.TopologyKeyESXIHost}),
			},
			wantErr:                 false,
			wantClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName},
			workerAntiAffinity:      true,
		},
		// {
		// 	name:    "create VirtualMachinesetResourcePolicy for control-plane and workers on Cluster mode (WorkerAntiAffinity: false)",
		// 	cluster: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "cluster"}},
		// 	// vSphereCluster: &vmwarev1.VSphereCluster{Spec: vmwarev1.VSphereClusterSpec{Placement: &vmwarev1.VSphereClusterPlacement{WorkerAntiAffinity: &vmwarev1.VSphereClusterWorkerAntiAffinity{
		// 	// 	Mode: vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster,
		// 	// }}}},
		// 	wantErr:                 false,
		// 	wantClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName, getFallbackWorkerClusterModuleGroupName("cluster")},
		// 	workerAntiAffinity:      false,
		// },
		// {
		// 	name:    "create VirtualMachinesetResourcePolicy for control-plane and workers on Cluster mode (WorkerAntiAffinity: true)",
		// 	cluster: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "cluster"}},
		// 	// vSphereCluster: &vmwarev1.VSphereCluster{Spec: vmwarev1.VSphereClusterSpec{Placement: &vmwarev1.VSphereClusterPlacement{WorkerAntiAffinity: &vmwarev1.VSphereClusterWorkerAntiAffinity{
		// 	// 	Mode: vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster,
		// 	// }}}},
		// 	wantErr:                 false,
		// 	wantClusterModuleGroups: []string{ClusterWorkerVMClusterModuleGroupName, ControlPlaneVMClusterModuleGroupName},
		// 	workerAntiAffinity:      true,
		// },
		// {
		// 	name:    "create VirtualMachinesetResourcePolicy for control-plane only when no MachineDeployments exist on MachineDeployment mode (WorkerAntiAffinity: true)",
		// 	cluster: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "cluster"}},
		// 	// vSphereCluster: &vmwarev1.VSphereCluster{Spec: vmwarev1.VSphereClusterSpec{Placement: &vmwarev1.VSphereClusterPlacement{WorkerAntiAffinity: &vmwarev1.VSphereClusterWorkerAntiAffinity{
		// 	// 	Mode: vmwarev1.VSphereClusterWorkerAntiAffinityModeMachineDeployment,
		// 	// }}}},
		// 	wantErr:                 false,
		// 	wantClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName},
		// 	workerAntiAffinity:      true,
		// },
		// {
		// 	name:    "create VirtualMachinesetResourcePolicy for control-plane and workers on MachineDeployment mode (WorkerAntiAffinity: true)",
		// 	cluster: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "cluster"}},
		// 	// vSphereCluster: &vmwarev1.VSphereCluster{Spec: vmwarev1.VSphereClusterSpec{Placement: &vmwarev1.VSphereClusterPlacement{WorkerAntiAffinity: &vmwarev1.VSphereClusterWorkerAntiAffinity{
		// 	// 	Mode: vmwarev1.VSphereClusterWorkerAntiAffinityModeMachineDeployment,
		// 	// }}}},
		// 	additionalObjs: []client.Object{
		// 		&clusterv1.MachineDeployment{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "md-1", Labels: map[string]string{clusterv1.ClusterNameLabel: "cluster"}}},
		// 		&clusterv1.MachineDeployment{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "md-0", Labels: map[string]string{clusterv1.ClusterNameLabel: "cluster"}}},
		// 		&clusterv1.MachineDeployment{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "other-cluster-md-0", Labels: map[string]string{clusterv1.ClusterNameLabel: "other"}}},
		// 	},
		// 	wantErr:                 false,
		// 	wantClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName, "md-0", "md-1"},
		// 	workerAntiAffinity:      true,
		// },
		// {
		// 	name:    "update VirtualMachinesetResourcePolicy for control-plane only on None mode and preserve used cluster modules from VirtualMachine's (WorkerAntiAffinity: true)",
		// 	cluster: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "cluster"}},
		// 	// vSphereCluster: &vmwarev1.VSphereCluster{Spec: vmwarev1.VSphereClusterSpec{Placement: &vmwarev1.VSphereClusterPlacement{WorkerAntiAffinity: &vmwarev1.VSphereClusterWorkerAntiAffinity{
		// 	// 	Mode: vmwarev1.VSphereClusterWorkerAntiAffinityModeNone,
		// 	// }}}},
		// 	additionalObjs: []client.Object{
		// 		&vmoprv1.VirtualMachineSetResourcePolicy{
		// 			ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "cluster"},
		// 		},
		// 		&vmoprv1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Namespace: "some", Name: "machine-0", Labels: map[string]string{clusterv1.ClusterNameLabel: "cluster"}, Annotations: map[string]string{ClusterModuleNameAnnotationKey: "deleted-md-0"}}},
		// 	},
		// 	wantErr:                 false,
		// 	wantClusterModuleGroups: []string{ControlPlaneVMClusterModuleGroupName, "deleted-md-0"},
		// 	workerAntiAffinity:      true,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.WorkerAntiAffinity, tt.workerAntiAffinity)

			s := &RPService{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(
					&vmoprv1.VirtualMachineService{},
					&vmoprv1.VirtualMachine{},
				).WithObjects(tt.additionalObjs...).Build(),
			}
			vSphereCluster := &vmwarev1.VSphereCluster{ObjectMeta: metav1.ObjectMeta{
				Name:      tt.cluster.Name,
				Namespace: tt.cluster.Namespace,
			}}
			got, err := s.ReconcileResourcePolicy(ctx, tt.cluster, vSphereCluster)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(got).To(BeEquivalentTo(""))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(got).To(BeEquivalentTo(tt.cluster.Name))
			}

			var resourcePolicy vmoprv1.VirtualMachineSetResourcePolicy

			g.Expect(s.Client.Get(ctx, client.ObjectKey{Name: got, Namespace: tt.cluster.Namespace}, &resourcePolicy)).To(Succeed())
			g.Expect(resourcePolicy.Spec.ClusterModuleGroups).To(BeEquivalentTo(tt.wantClusterModuleGroups))
		})
	}
}
