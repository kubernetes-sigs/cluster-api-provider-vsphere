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

package controllers

import (
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/simulator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers"
)

func Success(t *testing.T) {
	g := NewWithT(t)

	model := simulator.VPX()
	model.Pool = 1

	simr, err := helpers.VCSimBuilder().
		WithModel(model).
		WithOperations("tags.category.create -t Datacenter,ClusterComputeResource k8s-region",
			"tags.category.create -t Datacenter,ClusterComputeResource k8s-zone",
			"tags.create -c k8s-region k8s-region-west",
			"tags.create -c k8s-zone k8s-zone-west-1",
			"tags.attach -c k8s-region k8s-region-west /DC0",
			"tags.attach -c k8s-zone k8s-zone-west-1 /DC0/host/DC0_C0").
		Build()
	if err != nil {
		t.Fatalf("unable to create simulator %s", err)
	}
	defer simr.Destroy()

	mgmtContext := fake.NewControllerManagerContext()
	mgmtContext.Username = simr.ServerURL().User.Username()
	pass, _ := simr.ServerURL().User.Password()
	mgmtContext.Password = pass

	controllerCtx := fake.NewControllerContext(mgmtContext)

	deploymentZoneCtx := &context.VSphereDeploymentZoneContext{
		ControllerContext: controllerCtx,
		VSphereDeploymentZone: &infrav1.VSphereDeploymentZone{Spec: infrav1.VSphereDeploymentZoneSpec{
			Server:        simr.ServerURL().Host,
			FailureDomain: "blah",
			ControlPlane:  pointer.BoolPtr(true),
			PlacementConstraint: infrav1.PlacementConstraint{
				ResourcePool: "DC0_C0_RP1",
				Folder:       "/",
			},
		}},
		VSphereFailureDomain: &infrav1.VSphereFailureDomain{
			ObjectMeta: metav1.ObjectMeta{
				Name: "blah",
			},
			Spec: infrav1.VSphereFailureDomainSpec{
				Region: infrav1.FailureDomain{
					Name:          "k8s-region-west",
					Type:          infrav1.DatacenterFailureDomain,
					TagCategory:   "k8s-region",
					AutoConfigure: nil,
				},
				Zone: infrav1.FailureDomain{
					Name:          "k8s-zone-west-1",
					Type:          infrav1.ComputeClusterFailureDomain,
					TagCategory:   "k8s-zone",
					AutoConfigure: nil,
				},
				Topology: infrav1.Topology{
					Datacenter:     "DC0",
					ComputeCluster: pointer.String("DC0_C0"),
					Datastore:      "LocalDS_0",
					Networks:       []string{"VM Network"},
				},
			},
		},
		Logger: logr.DiscardLogger{},
	}

	reconciler := vsphereDeploymentZoneReconciler{controllerCtx}
	_, err = reconciler.reconcileNormal(deploymentZoneCtx)
	g.Expect(err).NotTo(HaveOccurred())
}

func FailResourcePoolNotOwnedComputeCluster(t *testing.T) {
	g := NewWithT(t)

	model := simulator.VPX()
	model.Cluster = 2
	model.Pool = 2

	simr, err := helpers.VCSimBuilder().
		WithModel(model).
		WithOperations("tags.category.create -t Datacenter,ClusterComputeResource k8s-region",
			"tags.category.create -t Datacenter,ClusterComputeResource k8s-zone",
			"tags.create -c k8s-region k8s-region-west",
			"tags.create -c k8s-zone k8s-zone-west-1",
			"tags.attach -c k8s-region k8s-region-west /DC0",
			"tags.attach -c k8s-zone k8s-zone-west-1 /DC0/host/DC0_C0").
		Build()
	if err != nil {
		t.Fatalf("unable to create simulator %s", err)
	}
	defer simr.Destroy()

	mgmtContext := fake.NewControllerManagerContext()
	mgmtContext.Username = simr.ServerURL().User.Username()
	pass, _ := simr.ServerURL().User.Password()
	mgmtContext.Password = pass

	controllerCtx := fake.NewControllerContext(mgmtContext)

	deploymentZoneCtx := &context.VSphereDeploymentZoneContext{
		ControllerContext: controllerCtx,
		VSphereDeploymentZone: &infrav1.VSphereDeploymentZone{Spec: infrav1.VSphereDeploymentZoneSpec{
			Server:        simr.ServerURL().Host,
			FailureDomain: "blah",
			ControlPlane:  pointer.BoolPtr(true),
			PlacementConstraint: infrav1.PlacementConstraint{
				ResourcePool: "DC0_C1_RP1",
				Folder:       "/",
			},
		}},
		VSphereFailureDomain: &infrav1.VSphereFailureDomain{
			ObjectMeta: metav1.ObjectMeta{
				Name: "blah",
			},
			Spec: infrav1.VSphereFailureDomainSpec{
				Region: infrav1.FailureDomain{
					Name:          "k8s-region-west",
					Type:          infrav1.DatacenterFailureDomain,
					TagCategory:   "k8s-region",
					AutoConfigure: nil,
				},
				Zone: infrav1.FailureDomain{
					Name:          "k8s-zone-west-1",
					Type:          infrav1.ComputeClusterFailureDomain,
					TagCategory:   "k8s-zone",
					AutoConfigure: nil,
				},
				Topology: infrav1.Topology{
					Datacenter:     "DC0",
					ComputeCluster: pointer.String("DC0_C0"),
					Datastore:      "LocalDS_0",
					Networks:       []string{"VM Network"},
				},
			},
		},
		Logger: logr.DiscardLogger{},
	}

	reconciler := vsphereDeploymentZoneReconciler{controllerCtx}
	_, err = reconciler.reconcileNormal(deploymentZoneCtx)
	g.Expect(err).To(HaveOccurred())
}

func TestVsphereDeploymentZoneReconciler(t *testing.T) {
	t.Run("VSphereDeploymentZone reconciliation is successful", Success)
	t.Run("VSphereDeploymentZone reconciliation fails when resource pool is not owned by compute cluster", FailResourcePoolNotOwnedComputeCluster)
}

func TestVsphereDeploymentZone_Failed_ReconcilePlacementConstraint(t *testing.T) {
	tests := []struct {
		name                string
		placementConstraint infrav1.PlacementConstraint
	}{
		{
			name: "when resource pool is not found",
			placementConstraint: infrav1.PlacementConstraint{
				ResourcePool: "DC0_C1_RP3",
				Folder:       "/",
			},
		},
		{
			name: "when folder is not found",
			placementConstraint: infrav1.PlacementConstraint{
				ResourcePool: "DC0_C1_RP1",
				Folder:       "/does-not-exist",
			},
		},
	}

	// nolint:scopelint
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			model := simulator.VPX()
			model.Cluster = 2
			model.Pool = 2

			simr, err := helpers.VCSimBuilder().
				WithModel(model).
				Build()
			if err != nil {
				t.Fatalf("unable to create simulator %s", err)
			}
			defer simr.Destroy()

			mgmtContext := fake.NewControllerManagerContext()
			mgmtContext.Username = simr.ServerURL().User.Username()
			pass, _ := simr.ServerURL().User.Password()
			mgmtContext.Password = pass

			controllerCtx := fake.NewControllerContext(mgmtContext)

			deploymentZoneCtx := &context.VSphereDeploymentZoneContext{
				ControllerContext: controllerCtx,
				VSphereDeploymentZone: &infrav1.VSphereDeploymentZone{Spec: infrav1.VSphereDeploymentZoneSpec{
					Server:              simr.ServerURL().Host,
					FailureDomain:       "blah",
					ControlPlane:        pointer.BoolPtr(true),
					PlacementConstraint: tt.placementConstraint,
				}},
				VSphereFailureDomain: &infrav1.VSphereFailureDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name: "blah",
					},
					Spec: infrav1.VSphereFailureDomainSpec{
						Topology: infrav1.Topology{
							Datacenter:     "DC0",
							ComputeCluster: pointer.String("DC0_C0"),
						},
					},
				},
				Logger: logr.DiscardLogger{},
			}

			reconciler := vsphereDeploymentZoneReconciler{controllerCtx}
			_, err = reconciler.reconcileNormal(deploymentZoneCtx)
			g.Expect(err).To(HaveOccurred())
		})
	}
}
