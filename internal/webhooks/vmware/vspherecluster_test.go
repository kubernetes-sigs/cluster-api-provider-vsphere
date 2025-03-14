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

package vmware

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	utilfeature "k8s.io/component-base/featuregate/testing"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
)

func TestVSphereCluster_ValidateCreate(t *testing.T) {
	tests := []struct {
		name               string
		vsphereCluster     *vmwarev1.VSphereCluster
		workerAntiAffinity bool
		wantErr            bool
	}{
		{
			name:               "Allow Cluster (WorkerAntiAffinity=false)",
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			workerAntiAffinity: false,
			wantErr:            false,
		},
		{
			name:               "Allow Cluster (WorkerAntiAffinity=true)",
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			workerAntiAffinity: true,
			wantErr:            false,
		},
		{
			name:               "Allow None (WorkerAntiAffinity=false)",
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			workerAntiAffinity: false,
			wantErr:            false,
		},
		{
			name:               "Allow None (WorkerAntiAffinity=true)",
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			workerAntiAffinity: true,
			wantErr:            false,
		},
		{
			name:               "Deny MachineDeployment (WorkerAntiAffinity=false)",
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeMachineDeployment),
			workerAntiAffinity: false,
			wantErr:            true,
		},
		{
			name:               "Allow MachineDeployment (WorkerAntiAffinity=true)",
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeMachineDeployment),
			workerAntiAffinity: true,
			wantErr:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.WorkerAntiAffinity, tt.workerAntiAffinity)

			webhook := &VSphereClusterWebhook{}
			_, err := webhook.ValidateCreate(context.Background(), tt.vsphereCluster)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
func TestVSphereCluster_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name               string
		oldVSphereCluster  *vmwarev1.VSphereCluster
		vsphereCluster     *vmwarev1.VSphereCluster
		workerAntiAffinity bool
		wantErr            bool
	}{
		{
			name:               "noop (WorkerAntiAffinity=false)",
			oldVSphereCluster:  createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			workerAntiAffinity: false,
			wantErr:            false,
		},
		{
			name:               "noop (WorkerAntiAffinity=true)",
			oldVSphereCluster:  createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			workerAntiAffinity: true,
			wantErr:            false,
		},
		{
			name:               "Allow Cluster to None (WorkerAntiAffinity=false)",
			oldVSphereCluster:  createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeNone),
			workerAntiAffinity: false,
			wantErr:            false,
		},
		{
			name:               "Allow Cluster to None (WorkerAntiAffinity=true)",
			oldVSphereCluster:  createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeNone),
			workerAntiAffinity: true,
			wantErr:            false,
		},
		{
			name:               "Disallow Cluster to MachineDeployment (WorkerAntiAffinity=false)",
			oldVSphereCluster:  createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeMachineDeployment),
			workerAntiAffinity: false,
			wantErr:            true,
		},
		{
			name:               "Allow Cluster to MachineDeployment (WorkerAntiAffinity=true)",
			oldVSphereCluster:  createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster),
			vsphereCluster:     createVSphereCluster(vmwarev1.VSphereClusterWorkerAntiAffinityModeMachineDeployment),
			workerAntiAffinity: true,
			wantErr:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.WorkerAntiAffinity, tt.workerAntiAffinity)

			webhook := &VSphereClusterWebhook{}
			_, err := webhook.ValidateUpdate(context.Background(), tt.oldVSphereCluster, tt.vsphereCluster)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createVSphereCluster(mode vmwarev1.VSphereClusterWorkerAntiAffinityMode) *vmwarev1.VSphereCluster {
	vSphereCluster := &vmwarev1.VSphereCluster{}
	if mode != "" {
		vSphereCluster.Spec.Placement = &vmwarev1.VSphereClusterPlacement{
			WorkerAntiAffinity: &vmwarev1.VSphereClusterWorkerAntiAffinity{
				Mode: mode,
			},
		}
	}
	return vSphereCluster
}
