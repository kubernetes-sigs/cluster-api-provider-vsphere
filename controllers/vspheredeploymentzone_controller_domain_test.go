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

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/vmware/govmomi/simulator"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/govmomi/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

func TestVsphereDeploymentZoneReconciler_Reconcile_VerifyFailureDomain_ComputeClusterZone(t *testing.T) {
	g := NewWithT(t)

	model := simulator.VPX()
	model.Cluster = 2

	simr, err := vcsim.NewBuilder().
		WithModel(model).
		WithOperations("tags.category.create k8s-region",
			"tags.create -c k8s-region k8s-region-west",
			"tags.create -c k8s-region k8s-region-west-2",
			"tags.category.create diff-k8s-region",
			"tags.attach k8s-region-west /DC0",
			"tags.attach k8s-region-west-2 /DC0/host/DC0_C0").
		Build()
	if err != nil {
		t.Fatalf("failed to create VC simulator %s", err)
	}
	t.Cleanup(simr.Destroy)

	controllerManagerContext := fake.NewControllerManagerContext()
	controllerManagerContext.Username = simr.Username()
	controllerManagerContext.Password = simr.Password()

	params := session.NewParams().
		WithServer(simr.ServerURL().Host).
		WithUserInfo(simr.Username(), simr.Password()).
		WithDatacenter("*")
	authSession, err := session.GetOrCreate(ctx, params)
	g.Expect(err).NotTo(HaveOccurred())

	vsphereFailureDomain := &infrav1.VSphereFailureDomain{
		Spec: infrav1.VSphereFailureDomainSpec{
			Region: infrav1.FailureDomain{
				Name:        "k8s-region-west",
				Type:        infrav1.DatacenterFailureDomain,
				TagCategory: "k8s-region",
			},
			Zone: infrav1.FailureDomain{
				Name:        "k8s-region-west-2",
				Type:        infrav1.ComputeClusterFailureDomain,
				TagCategory: "k8s-region",
			},
			Topology: infrav1.Topology{
				Datacenter:     "DC0",
				ComputeCluster: "DC0_C0",
			},
		},
	}

	deploymentZoneCtx := &capvcontext.VSphereDeploymentZoneContext{
		ControllerManagerContext: controllerManagerContext,
		AuthSession:              authSession,
	}

	reconciler := vsphereDeploymentZoneReconciler{controllerManagerContext}

	g.Expect(reconciler.verifyFailureDomain(ctx, deploymentZoneCtx, vsphereFailureDomain, vsphereFailureDomain.Spec.Region)).To(Succeed())
	stdout := gbytes.NewBuffer()
	g.Expect(simr.Run("tags.attached.ls k8s-region-west", stdout)).To(Succeed())
	g.Expect(stdout).Should(gbytes.Say("Datacenter"))

	g.Expect(reconciler.verifyFailureDomain(ctx, deploymentZoneCtx, vsphereFailureDomain, vsphereFailureDomain.Spec.Zone)).To(Succeed())
	stdout = gbytes.NewBuffer()
	g.Expect(simr.Run("tags.attached.ls k8s-region-west-2", stdout)).To(Succeed())
	g.Expect(stdout).Should(gbytes.Say("ClusterComputeResource"))

	vsphereFailureDomain.Spec.Topology.ComputeCluster = "DC0_C1"
	// Since association is verified, the method errors since the tag is not associated to the object.
	g.Expect(reconciler.verifyFailureDomain(ctx, deploymentZoneCtx, vsphereFailureDomain, vsphereFailureDomain.Spec.Zone)).To(HaveOccurred())

	// Since the tag does not belong to the category
	vsphereFailureDomain.Spec.Zone.TagCategory = "diff-k8s-region"
	g.Expect(reconciler.verifyFailureDomain(ctx, deploymentZoneCtx, vsphereFailureDomain, vsphereFailureDomain.Spec.Zone)).To(HaveOccurred())
}

func TestVsphereDeploymentZoneReconciler_Reconcile_VerifyFailureDomain_HostGroupZone(t *testing.T) {
	g := NewWithT(t)

	model := simulator.VPX()
	model.Cluster = 2

	simr, err := vcsim.NewBuilder().
		WithModel(model).
		WithOperations("tags.category.create k8s-region",
			"tags.create -c k8s-region k8s-region-west",
			"tags.create -c k8s-region k8s-region-west-2",
			"cluster.group.create -cluster DC0_C0 -name test_grp_1 -host DC0_C0_H0 DC0_C0_H1",
			"tags.attach k8s-region-west /DC0/host/DC0_C0").
		Build()
	if err != nil {
		t.Fatalf("failed to create VC simulator")
	}
	t.Cleanup(simr.Destroy)

	controllerManagerContext := fake.NewControllerManagerContext()
	controllerManagerContext.Username = simr.Username()
	controllerManagerContext.Password = simr.Password()

	params := session.NewParams().
		WithServer(simr.ServerURL().Host).
		WithUserInfo(simr.Username(), simr.Password()).
		WithDatacenter("*")
	authSession, err := session.GetOrCreate(ctx, params)
	g.Expect(err).NotTo(HaveOccurred())

	vsphereFailureDomain := &infrav1.VSphereFailureDomain{
		Spec: infrav1.VSphereFailureDomainSpec{
			Region: infrav1.FailureDomain{
				Name:        "k8s-region-west",
				Type:        infrav1.ComputeClusterFailureDomain,
				TagCategory: "k8s-region",
			},
			Zone: infrav1.FailureDomain{
				Name:        "k8s-region-west-2",
				Type:        infrav1.HostGroupFailureDomain,
				TagCategory: "k8s-region",
			},
			Topology: infrav1.Topology{
				Datacenter:     "DC0",
				ComputeCluster: "DC0_C0",
				Hosts: infrav1.FailureDomainHosts{
					HostGroupName: "test_grp_1",
				},
			},
		},
	}

	deploymentZoneCtx := &capvcontext.VSphereDeploymentZoneContext{
		ControllerManagerContext: controllerManagerContext,
		AuthSession:              authSession,
	}

	reconciler := vsphereDeploymentZoneReconciler{controllerManagerContext}

	// Fails since no hosts are tagged
	g.Expect(reconciler.verifyFailureDomain(ctx, deploymentZoneCtx, vsphereFailureDomain, vsphereFailureDomain.Spec.Zone)).To(HaveOccurred())
	stdout := gbytes.NewBuffer()

	g.Expect(simr.Run("tags.attach k8s-region-west-2 /DC0/host/DC0_C0/DC0_C0_H0", stdout)).To(Succeed())
	// Fails as not all hosts are tagged
	g.Expect(reconciler.verifyFailureDomain(ctx, deploymentZoneCtx, vsphereFailureDomain, vsphereFailureDomain.Spec.Zone)).To(HaveOccurred())

	g.Expect(simr.Run("tags.attach k8s-region-west-2 /DC0/host/DC0_C0/DC0_C0_H1", stdout)).To(Succeed())
	// Succeeds as all hosts are tagged
	g.Expect(reconciler.verifyFailureDomain(ctx, deploymentZoneCtx, vsphereFailureDomain, vsphereFailureDomain.Spec.Zone)).To(Succeed())

	// Since the tag does not belong to the category
	vsphereFailureDomain.Spec.Zone.TagCategory = "diff-k8s-region"
	g.Expect(reconciler.verifyFailureDomain(ctx, deploymentZoneCtx, vsphereFailureDomain, vsphereFailureDomain.Spec.Zone)).To(HaveOccurred())
}
